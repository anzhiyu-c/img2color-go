package handler

import (
	"encoding/json"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http"
	"os"

	"github.com/chai2010/webp"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/nfnt/resize"
)

func extractMainColor(imgURL string) (string, error) {
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

	return mainColor.Hex(), nil
}

func handleImageColor(w http.ResponseWriter, r *http.Request) {
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
	http.HandleFunc("/api/img2color", handleImageColor)

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

