package tsumiki

import "math"

const sheetMargin = 10.0 // blank border around the sheet (paper mm)

type sheet struct {
	options       drawingOptions
	width, height float64
	panels        []*panel
}

// panelMargins are the paper millimeters a panel's annotations need around its box.
type panelMargins struct{ left, top, right, bottom float64 }

// layOut arranges the views in third-angle projection: the top view above the front view and the
// right view to the right of it, so that each pair shares an axis. The isometric view takes the
// remaining corner, and section views follow in a row below.
func layOut(grid [2][2]*panel, sections []*panel, o drawingOptions) *sheet {
	var columnLeft, columnWidth, columnRight, rowTop, rowHeight, rowBottom [2]float64
	for row := range 2 {
		for column := range 2 {
			p := grid[row][column]
			m, size := p.margins(o), p.boxSize()
			columnLeft[column] = math.Max(columnLeft[column], m.left)
			columnWidth[column] = math.Max(columnWidth[column], size.X)
			columnRight[column] = math.Max(columnRight[column], m.right)
			rowTop[row] = math.Max(rowTop[row], m.top)
			rowHeight[row] = math.Max(rowHeight[row], size.Y)
			rowBottom[row] = math.Max(rowBottom[row], m.bottom)
		}
	}
	x, y := sheetMargin, sheetMargin
	var columnX, rowY [2]float64
	for i := range 2 {
		columnX[i], x = x, x+columnLeft[i]+columnWidth[i]+columnRight[i]+o.viewGap
		rowY[i], y = y, y+rowTop[i]+rowHeight[i]+rowBottom[i]+o.viewGap
	}
	s := &sheet{options: o}
	for row := range 2 {
		for column := range 2 {
			p := grid[row][column]
			p.left = columnX[column] + columnLeft[column]
			p.top = rowY[row] + rowTop[row]
			if row == 0 { // keep the top view close to the front view below it
				p.top += rowHeight[row] - p.boxSize().Y
			} else {
				p.labelLine = rowY[row] + o.textSize
			}
			s.panels = append(s.panels, p)
		}
	}
	stripWidth, stripHeight := layOutStrip(sections, y, o)
	s.panels = append(s.panels, sections...)
	s.width = math.Max(x-o.viewGap, stripWidth) + sheetMargin
	s.height = y + stripHeight + o.textSize*5 + sheetMargin
	return s
}

// layOutStrip places panels side by side in a row starting at y, aligned at the top of their
// boxes, and returns the right edge and the height of the row.
func layOutStrip(panels []*panel, y float64, o drawingOptions) (right, height float64) {
	top, bottom := 0.0, 0.0
	for _, p := range panels {
		m := p.margins(o)
		top = math.Max(top, m.top)
		bottom = math.Max(bottom, m.bottom+p.boxSize().Y)
	}
	x := sheetMargin
	for _, p := range panels {
		m := p.margins(o)
		p.left, p.top, p.labelLine = x+m.left, y+top, y+o.textSize
		x = p.left + p.boxSize().X + m.right + o.viewGap
	}
	if len(panels) == 0 {
		return 0, 0
	}
	return x - o.viewGap, top + bottom + o.viewGap
}
