package handler

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"strconv"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http"
	"os"
	"path/filepath"

	"github.com/chai2010/webp"
	"github.com/go-redis/redis/v8" // Import the Redis client package
	"github.com/lucasb-eyer/go-colorful"
	"github.com/nfnt/resize"
	"golang.org/x/net/context"
	"github.com/joho/godotenv"
)
var redisClient *redis.Client
var cacheEnabled bool
var redisDB int
var ctx = context.Background()

func init() {
	// Load environment variables from the .env file in the root directory
	// You need to get the absolute path to the root directory first
	rootDir, err := filepath.Abs("..") // Assuming "api" is a subdirectory of the root
	if err != nil {
		fmt.Printf("Error getting root directory path: %v\n", err)
		return
	}

	envFile := filepath.Join(rootDir, ".env")
	err = godotenv.Load(envFile)
	if err != nil {
		fmt.Printf("Error loading .env file: %v\n", err)
	}

	// Retrieve Redis address, password, and database from environment variables
	redisAddr := os.Getenv("REDIS_ADDRESS")
	redisPassword := os.Getenv("REDIS_PASSWORD")
	cacheEnabledStr := os.Getenv("CACHE_ENABLED")
	redisDBStr := os.Getenv("REDIS_DB")

	// Parse the Redis database number from the environment variable
	redisDB, err = strconv.Atoi(redisDBStr)
	if err != nil {
		redisDB = 0 // Default to DB 0 if the environment variable is not set or invalid
	}

	// Create the Redis client with the provided address, password, and database
	redisClient = redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})

	// Check if caching is enabled in the .env file
	cacheEnabled = cacheEnabledStr == "true"
}

func calculateMD5Hash(data []byte) string {
	hash := md5.Sum(data)
	return base64.StdEncoding.EncodeToString(hash[:])
}

func extractMainColor(imgURL string) (string, error) {
	// Calculate the MD5 hash of the image URL to use as a cache key
	md5Hash := calculateMD5Hash([]byte(imgURL))

	// Check if the result is already in the cache
	if cacheEnabled && redisClient != nil {
		cachedColor, err := redisClient.Get(ctx, md5Hash).Result()
		if err == nil && cachedColor != "" {
			return cachedColor, nil
		}
	}

	// 通过HTTP获取图片
	resp, err := http.Get(imgURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// 将图片解码为image.Image
	var img image.Image
	switch resp.Header.Get("Content-Type") {
	case "image/jpeg":
		img, err = jpeg.Decode(resp.Body)
	case "image/png":
		img, err = png.Decode(resp.Body)
	case "image/gif":
		img, err = gif.Decode(resp.Body)
	case "image/webp": // Handle WebP format using the webp package
		img, err = webp.Decode(resp.Body)
	default:
		err = fmt.Errorf("unknown image format")
	}
	if err != nil {
		return "", err
	}

	// 缩小图片以提高处理速度
	img = resize.Resize(50, 0, img, resize.Lanczos3)

	// 获取图片的主色调
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

	mainColor := colorful.Color{float64(averageR) / 0xFFFF, float64(averageG) / 0xFFFF, float64(averageB) / 0xFFFF}

	colorHex := mainColor.Hex()

	// Store the result in the cache if caching is enabled and Redis is available
	if cacheEnabled && redisClient != nil {
		_, err := redisClient.Set(ctx, md5Hash, colorHex, 0).Result()
		if err != nil {
			fmt.Printf("Error storing result in cache: %v\n", err)
		}
	}

	return colorHex, nil
}

func handleImageColor(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers to allow requests from all domains
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization")

	// Handle preflight requests (OPTIONS method)
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
	// Directly call the handleImageColor function here
	handleImageColor(w, r)
}

func main() {
	http.HandleFunc("/api", Handler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	fmt.Printf("服务器监听在：%s...\n", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		fmt.Printf("启动服务器时出错：%v\n", err)
	}
}