package stats

import (
	"cy2/internal/layer"
	"fmt"
)

type GridCell struct {
	Row        int     `json:"row"`
	Col        int     `json:"col"`
	Mean       float64 `json:"mean"`
	Min        float64 `json:"min"`
	Max        float64 `json:"max"`
	Status     string  `json:"status"`
	PixelCount int     `json:"pixel_count"`
}

type GridStats struct {
	GridSize         int        `json:"grid_size"`
	Rows             int        `json:"rows"`
	Cols             int        `json:"cols"`
	Cells            []GridCell `json:"cells"`
	BadCells         []GridCell `json:"bad_cells"`
	BadCount         int        `json:"bad_count"`
	TotalCount       int        `json:"total_count"`
	BadRatio         float64    `json:"bad_ratio"`
	WarningCount     int        `json:"warning_count"`
	WarningCells     []GridCell `json:"warning_cells"`
	WarningMessage   string     `json:"warning_message"`
	WarningThreshold float64    `json:"warning_threshold"`
}

const (
	StatusExcellent = "excellent"
	StatusGood      = "good"
	StatusModerate  = "moderate"
	StatusPoor      = "poor"
	StatusBad       = "bad"
)

const (
	ThresholdExcellent = 0.7
	ThresholdGood      = 0.5
	ThresholdModerate  = 0.3
	ThresholdPoor      = 0.1
	ThresholdBad       = -1.0
)

func AnalyzeGrid(ndvi *layer.NDVIResult, gridSize int, warningThreshold float64) *GridStats {
	if ndvi == nil {
		return nil
	}
	if ndvi.Width <= 0 || ndvi.Height <= 0 {
		return nil
	}
	if ndvi.Data == nil || len(ndvi.Data) != ndvi.Height {
		return nil
	}

	if warningThreshold <= -1.0 {
		warningThreshold = 0.3
	}
	if warningThreshold > 1.0 {
		warningThreshold = 1.0
	}

	if gridSize <= 0 {
		gridSize = 50
	}
	if gridSize > ndvi.Width+ndvi.Height {
		gridSize = ndvi.Width
		if gridSize <= 0 {
			gridSize = 1
		}
	}

	cols := (ndvi.Width + gridSize - 1) / gridSize
	rows := (ndvi.Height + gridSize - 1) / gridSize
	if cols <= 0 || rows <= 0 {
		return nil
	}

	cells := make([]GridCell, 0, rows*cols)
	badCells := make([]GridCell, 0)
	warningCells := make([]GridCell, 0)

	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			cell := calculateCell(ndvi, r, c, gridSize)
			cells = append(cells, cell)

			if cell.Status == StatusPoor || cell.Status == StatusBad {
				badCells = append(badCells, cell)
			}

			if cell.Mean < warningThreshold {
				warningCells = append(warningCells, cell)
			}
		}
	}

	totalCount := len(cells)
	badCount := len(badCells)
	badRatio := 0.0
	if totalCount > 0 {
		badRatio = float64(badCount) / float64(totalCount)
	}

	warningCount := len(warningCells)
	warningMessage := generateWarningMessage(warningCount, totalCount, warningThreshold)

	return &GridStats{
		GridSize:         gridSize,
		Rows:             rows,
		Cols:             cols,
		Cells:            cells,
		BadCells:         badCells,
		BadCount:         badCount,
		TotalCount:       totalCount,
		BadRatio:         badRatio,
		WarningCount:     warningCount,
		WarningCells:     warningCells,
		WarningMessage:   warningMessage,
		WarningThreshold: warningThreshold,
	}
}

func calculateCell(ndvi *layer.NDVIResult, row, col, gridSize int) GridCell {
	if ndvi == nil || ndvi.Data == nil {
		return GridCell{Row: row, Col: col, Status: StatusBad}
	}

	startY := row * gridSize
	endY := (row + 1) * gridSize
	if endY > ndvi.Height {
		endY = ndvi.Height
	}
	if startY < 0 {
		startY = 0
	}
	if startY >= ndvi.Height {
		return GridCell{Row: row, Col: col, Status: StatusBad}
	}

	startX := col * gridSize
	endX := (col + 1) * gridSize
	if endX > ndvi.Width {
		endX = ndvi.Width
	}
	if startX < 0 {
		startX = 0
	}

	sum := 0.0
	count := 0
	minVal := 1.0
	maxVal := -1.0

	for y := startY; y < endY; y++ {
		if y >= len(ndvi.Data) {
			break
		}
		rowData := ndvi.Data[y]
		if rowData == nil {
			continue
		}
		for x := startX; x < endX; x++ {
			if x >= len(rowData) {
				break
			}
			val := rowData[x]
			sum += val
			count++
			if val < minVal {
				minVal = val
			}
			if val > maxVal {
				maxVal = val
			}
		}
	}

	mean := 0.0
	if count > 0 {
		mean = sum / float64(count)
	}
	if count == 0 {
		minVal = 0
		maxVal = 0
	}

	status := classifyStatus(mean)

	return GridCell{
		Row:        row,
		Col:        col,
		Mean:       mean,
		Min:        minVal,
		Max:        maxVal,
		Status:     status,
		PixelCount: count,
	}
}

func classifyStatus(mean float64) string {
	switch {
	case mean >= ThresholdExcellent:
		return StatusExcellent
	case mean >= ThresholdGood:
		return StatusGood
	case mean >= ThresholdModerate:
		return StatusModerate
	case mean >= ThresholdPoor:
		return StatusPoor
	default:
		return StatusBad
	}
}

func StatusToColor(status string) (int, int, int) {
	switch status {
	case StatusExcellent:
		return 0, 100, 0
	case StatusGood:
		return 34, 139, 34
	case StatusModerate:
		return 255, 215, 0
	case StatusPoor:
		return 255, 140, 0
	case StatusBad:
		return 178, 34, 34
	default:
		return 128, 128, 128
	}
}

func generateWarningMessage(warningCount, totalCount int, threshold float64) string {
	if warningCount == 0 {
		return "🎉 所有区域长势都在警戒线以上，不需要施肥。"
	}
	if totalCount == 0 {
		return "没有可分析的数据"
	}

	ratio := float64(warningCount) / float64(totalCount) * 100

	switch {
	case warningCount == 1:
		return fmt.Sprintf("⚠️ 有 1 个区域需要施肥了（占比 %.1f%%），建议重点照顾一下。", ratio)
	case warningCount <= 3:
		return fmt.Sprintf("⚠️ 有 %d 个区域需要施肥了（占比 %.1f%%），面积不大，可以安排施肥。", warningCount, ratio)
	case warningCount <= 10:
		return fmt.Sprintf("⚠️ 有 %d 个区域需要施肥了（占比 %.1f%%），有一定面积，建议尽快处理。", warningCount, ratio)
	default:
		return fmt.Sprintf("⚠️ 有 %d 个区域需要施肥了（占比 %.1f%%），面积较大，建议全面排查施肥方案。", warningCount, ratio)
	}
}
