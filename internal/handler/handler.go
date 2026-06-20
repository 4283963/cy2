package handler

import (
	"cy2/internal/layer"
	"cy2/internal/stats"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AnalysisResult struct {
	ID        string          `json:"id"`
	Timestamp time.Time       `json:"timestamp"`
	NDVI      NDVIInfo        `json:"ndvi"`
	Grid      stats.GridStats `json:"grid"`
	NDVIImage string          `json:"ndvi_image"`
}

type NDVIInfo struct {
	Width  int     `json:"width"`
	Height int     `json:"height"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Mean   float64 `json:"mean"`
}

var (
	results = make(map[string]*AnalysisResult)
	mu      sync.RWMutex
)

func init() {
	os.MkdirAll("uploads", 0755)
	os.MkdirAll("web/static/results", 0755)
}

func UploadAndAnalyze(c *gin.Context) {
	redFile, err := c.FormFile("red")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "红光图片上传失败: " + err.Error()})
		return
	}

	nirFile, err := c.FormFile("nir")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "红外光图片上传失败: " + err.Error()})
		return
	}

	gridSizeStr := c.DefaultPostForm("grid_size", "50")
	gridSize, err := strconv.Atoi(gridSizeStr)
	if err != nil || gridSize <= 0 {
		gridSize = 50
	}

	id := uuid.New().String()
	redPath := filepath.Join("uploads", fmt.Sprintf("%s_red%s", id, filepath.Ext(redFile.Filename)))
	nirPath := filepath.Join("uploads", fmt.Sprintf("%s_nir%s", id, filepath.Ext(nirFile.Filename)))

	if err := c.SaveUploadedFile(redFile, redPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存红光图片失败: " + err.Error()})
		return
	}

	if err := c.SaveUploadedFile(nirFile, nirPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存红外光图片失败: " + err.Error()})
		return
	}

	ndviResult, err := layer.CalculateNDVI(redPath, nirPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "NDVI计算失败: " + err.Error()})
		return
	}

	ndviImagePath := filepath.Join("web/static/results", fmt.Sprintf("%s_ndvi.png", id))
	if err := layer.GenerateNDVIPaletteImage(ndviResult, ndviImagePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成NDVI图像失败: " + err.Error()})
		return
	}

	gridStats := stats.AnalyzeGrid(ndviResult, gridSize)

	result := &AnalysisResult{
		ID:        id,
		Timestamp: time.Now(),
		NDVI: NDVIInfo{
			Width:  ndviResult.Width,
			Height: ndviResult.Height,
			Min:    ndviResult.Min,
			Max:    ndviResult.Max,
			Mean:   ndviResult.Mean,
		},
		Grid:      *gridStats,
		NDVIImage: fmt.Sprintf("/static/results/%s_ndvi.png", id),
	}

	mu.Lock()
	results[id] = result
	mu.Unlock()

	c.JSON(http.StatusOK, result)
}

func GetResult(c *gin.Context) {
	id := c.Param("id")

	mu.RLock()
	result, exists := results[id]
	mu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "结果不存在"})
		return
	}

	c.JSON(http.StatusOK, result)
}
