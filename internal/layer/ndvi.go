package layer

import (
	"errors"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"math"
	"os"
)

type NDVIResult struct {
	Width  int
	Height int
	Data   [][]float64
	Min    float64
	Max    float64
	Mean   float64
}

func LoadImage(path string) ([][]float64, int, int, error) {
	if path == "" {
		return nil, 0, 0, errors.New("图片路径为空")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, 0, 0, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, 0, 0, err
	}
	if img == nil {
		return nil, 0, 0, errors.New("解码图像为空")
	}

	bounds := img.Bounds()
	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y

	if width <= 0 || height <= 0 {
		return nil, 0, 0, errors.New("图像尺寸无效")
	}

	data := make([][]float64, height)
	for y := 0; y < height; y++ {
		data[y] = make([]float64, width)
		for x := 0; x < width; x++ {
			r, g, b, _ := img.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
			gray := (0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(b>>8)) / 255.0
			if gray < 0 {
				gray = 0
			}
			if gray > 1 {
				gray = 1
			}
			data[y][x] = gray
		}
	}

	return data, width, height, nil
}

func CalculateNDVI(redPath, nirPath string) (*NDVIResult, error) {
	if redPath == "" || nirPath == "" {
		return nil, errors.New("图片路径不能为空")
	}

	redData, redWidth, redHeight, err := LoadImage(redPath)
	if err != nil {
		return nil, errors.New("读取红光图片失败: " + err.Error())
	}
	if redData == nil || redWidth <= 0 || redHeight <= 0 {
		return nil, errors.New("红光图片数据无效")
	}

	nirData, nirWidth, nirHeight, err := LoadImage(nirPath)
	if err != nil {
		return nil, errors.New("读取红外光图片失败: " + err.Error())
	}
	if nirData == nil || nirWidth <= 0 || nirHeight <= 0 {
		return nil, errors.New("红外光图片数据无效")
	}

	if redWidth != nirWidth || redHeight != nirHeight {
		return nil, errors.New("红光和红外光图片尺寸不一致，" +
			"红光尺寸: " + itoa(redWidth) + "x" + itoa(redHeight) +
			", 红外光尺寸: " + itoa(nirWidth) + "x" + itoa(nirHeight))
	}

	width := redWidth
	height := redHeight

	if len(redData) != height || len(nirData) != height {
		return nil, errors.New("图像数据行数与高度不匹配")
	}
	for y := 0; y < height; y++ {
		if len(redData[y]) != width || len(nirData[y]) != width {
			return nil, errors.New("图像第 " + itoa(y) + " 行列数与宽度不匹配")
		}
	}

	ndviData := make([][]float64, height)
	minVal := 1.0
	maxVal := -1.0
	sum := 0.0
	count := 0

	for y := 0; y < height; y++ {
		ndviData[y] = make([]float64, width)
		for x := 0; x < width; x++ {
			red := redData[y][x]
			nir := nirData[y][x]

			var ndvi float64
			denominator := nir + red
			if denominator == 0 {
				ndvi = 0
			} else {
				ndvi = (nir - red) / denominator
			}

			if math.IsNaN(ndvi) || math.IsInf(ndvi, 0) {
				ndvi = 0
			}

			if ndvi < -1.0 {
				ndvi = -1.0
			}
			if ndvi > 1.0 {
				ndvi = 1.0
			}
			ndviData[y][x] = ndvi

			if ndvi < minVal {
				minVal = ndvi
			}
			if ndvi > maxVal {
				maxVal = ndvi
			}
			sum += ndvi
			count++
		}
	}

	mean := 0.0
	if count > 0 {
		mean = sum / float64(count)
	}
	if math.IsNaN(mean) || math.IsInf(mean, 0) {
		mean = 0
	}

	return &NDVIResult{
		Width:  width,
		Height: height,
		Data:   ndviData,
		Min:    minVal,
		Max:    maxVal,
		Mean:   mean,
	}, nil
}

func GenerateNDVIPaletteImage(ndvi *NDVIResult, outputPath string) error {
	if ndvi == nil {
		return errors.New("NDVI结果为空")
	}
	if ndvi.Width <= 0 || ndvi.Height <= 0 {
		return errors.New("NDVI图像尺寸无效")
	}
	if len(ndvi.Data) != ndvi.Height {
		return errors.New("NDVI数据行数与高度不匹配")
	}
	for y := 0; y < ndvi.Height; y++ {
		if len(ndvi.Data[y]) != ndvi.Width {
			return errors.New("NDVI第 " + itoa(y) + " 行列数与宽度不匹配")
		}
	}
	if outputPath == "" {
		return errors.New("输出路径为空")
	}

	img := image.NewRGBA(image.Rect(0, 0, ndvi.Width, ndvi.Height))
	if img == nil {
		return errors.New("创建图像失败")
	}

	for y := 0; y < ndvi.Height; y++ {
		for x := 0; x < ndvi.Width; x++ {
			val := ndvi.Data[y][x]
			r, g, b := ndviToRGB(val)
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		os.Remove(outputPath)
		return err
	}

	return nil
}

func ndviToRGB(val float64) (uint8, uint8, uint8) {
	if math.IsNaN(val) || math.IsInf(val, 0) {
		val = 0
	}
	if val < -1.0 {
		val = -1.0
	}
	if val > 1.0 {
		val = 1.0
	}

	t := (val + 1.0) / 2.0

	switch {
	case t < 0.2:
		return 128, 0, 0
	case t < 0.4:
		return 255, 128, 0
	case t < 0.6:
		return 255, 255, 0
	case t < 0.8:
		return 128, 255, 0
	default:
		return 0, 128, 0
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
