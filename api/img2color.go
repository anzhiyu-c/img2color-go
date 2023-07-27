package handler

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/chai2010/webp"
	"github.com/go-redis/redis/v8" // 导入Redis客户端包
	"github.com/joho/godotenv"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/nfnt/resize"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/net/context"
)

var redisClient *redis.Client
var mongoClient *mongo.Client
var cacheEnabled bool
var useMongoDB bool
var redisDB int
var mongoDB string
var ctx = context.Background()

func init() {
	// 获取当前工作目录的绝对路径
	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("获取当前工作目录路径时出错：%v\n", err)
		return
	}

	// 构建 .env 文件的完整路径
	envFile := filepath.Join(currentDir, ".env")

	// 从 .env 文件加载环境变量
	err = godotenv.Load(envFile)
	if err != nil {
		fmt.Printf("加载 .env 文件时出错：%v\n", err)
		return
	}

	// 从环境变量中获取Redis和MongoDB的配置
	redisAddr := os.Getenv("REDIS_ADDRESS")
	redisPassword := os.Getenv("REDIS_PASSWORD")
	cacheEnabledStr := os.Getenv("USE_REDIS_CACHE")
	redisDBStr := os.Getenv("REDIS_DB")
	mongoDB = os.Getenv("MONGO_DB")
	mongoURI := os.Getenv("MONGO_URI")

	// 从环境变量中解析Redis数据库编号
	redisDB, err = strconv.Atoi(redisDBStr)
	if err != nil {
		redisDB = 0 // 如果环境变量未设置或无效，则默认使用DB 0
	}

	// 使用提供的地址、密码和数据库创建Redis客户端
	redisClient = redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})

	// 检查缓存是否在.env文件中启用
	cacheEnabled = cacheEnabledStr == "true"

	// 检查是否应使用MongoDB
	useMongoDBStr := os.Getenv("USE_MONGODB")
	useMongoDB = useMongoDBStr == "true"
	if useMongoDB {
		log.Println("连接到MongoDB...")
		clientOptions := options.Client().ApplyURI(mongoURI)
		mongoClient, err = mongo.Connect(ctx, clientOptions)
		if err != nil {
			log.Fatalf("连接到MongoDB时出错：%v", err)
		}
		log.Println("已连接到MongoDB！")
	}
}

func calculateMD5Hash(data []byte) string {
	hash := md5.Sum(data)
	return base64.StdEncoding.EncodeToString(hash[:])
}

func extractMainColor(imgURL string) (string, error) {
	// 计算图像URL的MD5哈希作为缓存键
	md5Hash := calculateMD5Hash([]byte(imgURL))

	// 检查结果是否已在缓存中
	if cacheEnabled && redisClient != nil {
		cachedColor, err := redisClient.Get(ctx, md5Hash).Result()
		if err == nil && cachedColor != "" {
			return cachedColor, nil
		}
	}

	// 通过HTTP获取图像
	resp, err := http.Get(imgURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// 将图像解码为image.Image类型
	var img image.Image
	switch resp.Header.Get("Content-Type") {
	case "image/jpeg":
		img, err = jpeg.Decode(resp.Body)
	case "image/png":
		img, err = png.Decode(resp.Body)
	case "image/gif":
		img, err = gif.Decode(resp.Body)
	case "image/webp": // 使用webp包处理WebP格式
		img, err = webp.Decode(resp.Body)
	default:
		err = fmt.Errorf("未知的图像格式")
	}
	if err != nil {
		return "", err
	}

	// 调整图像大小以加快处理速度
	img = resize.Resize(50, 0, img, resize.Lanczos3)

	// 获取图像的主色调
	bounds := img.Bounds()
	var r, g, b uint32
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.At(x, y)
			r0, g0, b0, _ := c.RGBA()
			r += r0
			g += g0
			b += b0
		}
	}

	totalPixels := uint32(bounds.Dx() * bounds.Dy())
	averageR := r / totalPixels
	averageG := g / totalPixels
	averageB := b / totalPixels

	mainColor := colorful.Color{R: float64(averageR) / 0xFFFF, G: float64(averageG) / 0xFFFF, B: float64(averageB) / 0xFFFF}

	colorHex := mainColor.Hex()

	// 如果缓存已启用且Redis可用，则将结果存储在缓存中
	if cacheEnabled && redisClient != nil {
		_, err := redisClient.Set(ctx, md5Hash, colorHex, 0).Result()
		if err != nil {
			log.Printf("将结果存储在缓存中时出错：%v\n", err)
		}
	}

	// 如果启用了MongoDB，则将结果存储在其中
	if useMongoDB && mongoClient != nil {
		collection := mongoClient.Database(mongoDB).Collection("colors")
		_, err := collection.InsertOne(ctx, bson.M{
			"url":   imgURL,
			"color": colorHex,
		})
		if err != nil {
			log.Printf("将结果存储在MongoDB中时出错：%v\n", err)
		}
	}

	return colorHex, nil
}

func handleImageColor(w http.ResponseWriter, r *http.Request) {
	// 设置CORS头，允许所有来源访问
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Referer")


	// 处理预检请求 (选项方法)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	imgURL := r.URL.Query().Get("img")
	if imgURL == "" {
		http.Error(w, "缺少img参数", http.StatusBadRequest)
		return
	}

	color, err := extractMainColor(imgURL)
	if err != nil {
		http.Error(w, fmt.Sprintf("提取主色调失败：%v", err), http.StatusInternalServerError)
		return
	}

	// 构建返回的JSON数据
	data := map[string]string{
		"RGB": color,
	}

	// 将数据编码为JSON格式并发送回客户端
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func Handler(w http.ResponseWriter, r *http.Request) {
	// 直接在此处调用handleImageColor函数
	handleImageColor(w, r)
}

func main() {
	http.HandleFunc("/api", Handler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("服务器监听在：%s...\n", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatalf("启动服务器时出错：%v\n", err)
	}
}
