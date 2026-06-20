package stats

import (
	"cy2/internal/layer"
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
	GridSize   int        `json:"grid_size"`
	Rows       int        `json:"rows"`
	Cols       int        `json:"cols"`
	Cells      []GridCell `json:"cells"`
	BadCells   []GridCell `json:"bad_cells"`
	BadCount   int        `json:"bad_count"`
	TotalCount int        `json:"total_count"`
	BadRatio   float64    `json:"bad_ratio"`
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

func AnalyzeGrid(ndvi *layer.NDVIResult, gridSize int) *GridStats {
	if gridSize <= 0 {
		gridSize = 50
	}

	cols := (ndvi.Width + gridSize - 1) / gridSize
	rows := (ndvi.Height + gridSize - 1) / gridSize

	cells := make([]GridCell, 0, rows*cols)
	badCells := make([]GridCell, 0)

	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			cell := calculateCell(ndvi, r, c, gridSize)
			cells = append(cells, cell)

			if cell.Status == StatusPoor || cell.Status == StatusBad {
				badCells = append(badCells, cell)
			}
		}
	}

	totalCount := len(cells)
	badCount := len(badCells)
	badRatio := 0.0
	if totalCount > 0 {
		badRatio = float64(badCount) / float64(totalCount)
	}

	return &GridStats{
		GridSize:   gridSize,
		Rows:       rows,
		Cols:       cols,
		Cells:      cells,
		BadCells:   badCells,
		BadCount:   badCount,
		TotalCount: totalCount,
		BadRatio:   badRatio,
	}
}

func calculateCell(ndvi *layer.NDVIResult, row, col, gridSize int) GridCell {
	startY := row * gridSize
	endY := (row + 1) * gridSize
	if endY > ndvi.Height {
		endY = ndvi.Height
	}

	startX := col * gridSize
	endX := (col + 1) * gridSize
	if endX > ndvi.Width {
		endX = ndvi.Width
	}

	sum := 0.0
	count := 0
	minVal := 1.0
	maxVal := -1.0

	for y := startY; y < endY; y++ {
		for x := startX; x < endX; x++ {
			val := ndvi.Data[y][x]
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
