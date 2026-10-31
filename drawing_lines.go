package tsumiki

import "math"

// lineIndex finds drawn segments near a point quickly.
type lineIndex struct {
	segments []segment2
	origin   Vector2
	cell     float64
	columns  int
	rows     int
	cells    [][]int32
}

const (
	coincidentDistance = 1e-3 // lines closer than this on the paper are drawn on top of each other (mm)
	parallelSine       = 1e-3 // lines whose directions differ by less than this angle are parallel (rad)
)

func newLineIndex(segments []segment2, low, high Vector2, cell float64) *lineIndex {
	size := high.Sub(low)
	index := &lineIndex{
		segments: segments,
		origin:   low,
		cell:     cell,
		columns:  int(size.X/cell) + 1,
		rows:     int(size.Y/cell) + 1,
	}
	index.cells = make([][]int32, index.columns*index.rows)
	for i, segment := range segments {
		firstColumn, firstRow := index.cellOf(Vector2{
			math.Min(segment[0].X, segment[1].X) - coincidentDistance,
			math.Min(segment[0].Y, segment[1].Y) - coincidentDistance,
		})
		lastColumn, lastRow := index.cellOf(Vector2{
			math.Max(segment[0].X, segment[1].X) + coincidentDistance,
			math.Max(segment[0].Y, segment[1].Y) + coincidentDistance,
		})
		for column := firstColumn; column <= lastColumn; column++ {
			for row := firstRow; row <= lastRow; row++ {
				index.cells[row*index.columns+column] = append(index.cells[row*index.columns+column], int32(i))
			}
		}
	}
	return index
}

func (index *lineIndex) cellOf(p Vector2) (column, row int) {
	column = min(max(int((p.X-index.origin.X)/index.cell), 0), index.columns-1)
	row = min(max(int((p.Y-index.origin.Y)/index.cell), 0), index.rows-1)
	return column, row
}

// covers reports whether a segment of the index runs through p in the given (unit) direction.
func (index *lineIndex) covers(p, direction Vector2) bool {
	column, row := index.cellOf(p)
	for _, i := range index.cells[row*index.columns+column] {
		segment := index.segments[i]
		along := segment[1].Sub(segment[0])
		length := along.Length()
		if length == 0 || math.Abs(unit(along).Cross(direction)) > parallelSine {
			continue
		}
		t := p.Sub(segment[0]).Dot(along) / (length * length)
		if t < -coincidentDistance/length || t > 1+coincidentDistance/length {
			continue
		}
		if math.Abs(along.Cross(p.Sub(segment[0])))/length <= coincidentDistance {
			return true
		}
	}
	return false
}

// withoutCovered drops the parts of lines that run along a line of the index: a hidden edge
// behind a visible one is not drawn.
func (index *lineIndex) withoutCovered(lines []segment2, step float64) []segment2 {
	var kept []segment2
	for _, line := range lines {
		along := line[1].Sub(line[0])
		direction := unit(along)
		pointAt := func(t float64) Vector2 { return line[0].Add(along.Scale(t)) }
		covered := func(t float64) bool { return index.covers(pointAt(t), direction) }
		splitRuns(samplesFor(along.Length(), step), covered, func(start, end float64, isCovered bool) {
			if !isCovered && (end-start)*along.Length() > coincidentDistance {
				kept = append(kept, segment2{pointAt(start), pointAt(end)})
			}
		})
	}
	return kept
}
