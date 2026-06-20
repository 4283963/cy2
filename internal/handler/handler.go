package handler

import (
	"cy2/internal/layer"
	"cy2/internal/stats"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
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

const (
	maxFileSize = 50 * 1024 * 1024
)

func init() {
	os.MkdirAll("uploads", 0755)
	os.MkdirAll("web/static/results", 0755)
}

func UploadAndAnalyze(c *gin.Context) {
	redFileHeader, err := c.FormFile("red")
	if err != nil || redFileHeader == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少红光图片 (red)"})
		return
	}

	nirFileHeader, err := c.FormFile("nir")
	if err != nil || nirFileHeader == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少红外光图片 (nir)"})
		return
	}

	if redFileHeader.Size <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "红光图片文件为空"})
		return
	}
	if nirFileHeader.Size <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "红外光图片文件为空"})
		return
	}
	if redFileHeader.Size > maxFileSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "红光图片文件过大，最大支持 50MB"})
		return
	}
	if nirFileHeader.Size > maxFileSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "红外光图片文件过大，最大支持 50MB"})
		return
	}

	redFile, err := redFileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "红光图片读取失败: " + err.Error()})
		return
	}
	defer redFile.Close()

	nirFile, err := nirFileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "红外光图片读取失败: " + err.Error()})
		return
	}
	defer nirFile.Close()

	if !isValidImage(redFile) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "红光图片格式损坏或不是有效图片"})
		return
	}
	if !isValidImage(nirFile) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "红外光图片格式损坏或不是有效图片"})
		return
	}

	gridSizeStr := c.DefaultPostForm("grid_size", "50")
	gridSize, err := strconv.Atoi(gridSizeStr)
	if err != nil || gridSize <= 0 {
		gridSize = 50
	}
	if gridSize > 2000 {
		gridSize = 2000
	}

	warningThresholdStr := c.DefaultPostForm("warning_threshold", "0.3")
	warningThreshold, err := strconv.ParseFloat(warningThresholdStr, 64)
	if err != nil {
		warningThreshold = 0.3
	}
	if warningThreshold < -1.0 {
		warningThreshold = -1.0
	}
	if warningThreshold > 1.0 {
		warningThreshold = 1.0
	}

	id := uuid.New().String()
	redExt := filepath.Ext(redFileHeader.Filename)
	nirExt := filepath.Ext(nirFileHeader.Filename)
	if redExt == "" {
		redExt = ".png"
	}
	if nirExt == "" {
		nirExt = ".png"
	}
	redPath := filepath.Join("uploads", fmt.Sprintf("%s_red%s", id, redExt))
	nirPath := filepath.Join("uploads", fmt.Sprintf("%s_nir%s", id, nirExt))

	if _, err := redFile.Seek(0, io.SeekStart); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "红光图片重置失败"})
		return
	}
	if err := saveUploadedFile(redFile, redPath, redFileHeader.Size); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存红光图片失败: " + err.Error()})
		return
	}

	if _, err := nirFile.Seek(0, io.SeekStart); err != nil {
		os.Remove(redPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "红外光图片重置失败"})
		return
	}
	if err := saveUploadedFile(nirFile, nirPath, nirFileHeader.Size); err != nil {
		os.Remove(redPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存红外光图片失败: " + err.Error()})
		return
	}

	ndviResult, err := layer.CalculateNDVI(redPath, nirPath)
	if err != nil {
		os.Remove(redPath)
		os.Remove(nirPath)
		c.JSON(http.StatusBadRequest, gin.H{"error": "NDVI计算失败: " + err.Error()})
		return
	}
	if ndviResult == nil {
		os.Remove(redPath)
		os.Remove(nirPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "NDVI计算返回空结果"})
		return
	}

	ndviImagePath := filepath.Join("web/static/results", fmt.Sprintf("%s_ndvi.png", id))
	if err := layer.GenerateNDVIPaletteImage(ndviResult, ndviImagePath); err != nil {
		os.Remove(redPath)
		os.Remove(nirPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成NDVI图像失败: " + err.Error()})
		return
	}

	gridStats := stats.AnalyzeGrid(ndviResult, gridSize, warningThreshold)
	if gridStats == nil {
		os.Remove(redPath)
		os.Remove(nirPath)
		os.Remove(ndviImagePath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "网格统计失败"})
		return
	}

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
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "结果ID不能为空"})
		return
	}

	mu.RLock()
	result, exists := results[id]
	mu.RUnlock()

	if !exists || result == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "结果不存在"})
		return
	}

	c.JSON(http.StatusOK, result)
}

func isValidImage(f multipart.File) bool {
	if f == nil {
		return false
	}
	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return false
	}
	if n == 0 {
		return false
	}
	contentType := http.DetectContentType(buf[:n])
	switch contentType {
	case "image/png", "image/jpeg", "image/gif", "image/bmp", "image/tiff":
		return true
	default:
		return false
	}
}

func saveUploadedFile(src multipart.File, dstPath string, expectedSize int64) error {
	if src == nil {
		return errors.New("源文件为空")
	}

	out, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer out.Close()

	written, err := io.Copy(out, src)
	if err != nil {
		os.Remove(dstPath)
		return err
	}

	if written == 0 {
		os.Remove(dstPath)
		return errors.New("写入文件为空")
	}

	if expectedSize > 0 && written != expectedSize {
		os.Remove(dstPath)
		return fmt.Errorf("文件大小不匹配，期望 %d 字节，实际写入 %d 字节", expectedSize, written)
	}

	return nil
}
