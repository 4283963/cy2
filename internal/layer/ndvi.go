package layer

import (
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
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, 0, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, 0, 0, err
	}

	bounds := img.Bounds()
	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y

	data := make([][]float64, height)
	for y := 0; y < height; y++ {
		data[y] = make([]float64, width)
		for x := 0; x < width; x++ {
			r, g, b, _ := img.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
			gray := (0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(b>>8)) / 255.0
			data[y][x] = gray
		}
	}

	return data, width, height, nil
}

func CalculateNDVI(redPath, nirPath string) (*NDVIResult, error) {
	redData, width, height, err := LoadImage(redPath)
	if err != nil {
		return nil, err
	}

	nirData, _, _, err := LoadImage(nirPath)
	if err != nil {
		return nil, err
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
			if nir+red == 0 {
				ndvi = 0
			} else {
				ndvi = (nir - red) / (nir + red)
			}

			ndvi = math.Max(-1.0, math.Min(1.0, ndvi))
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
	img := image.NewRGBA(image.Rect(0, 0, ndvi.Width, ndvi.Height))

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

	return png.Encode(file, img)
}

func ndviToRGB(val float64) (uint8, uint8, uint8) {
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
