package tsumiki

import (
	"bufio"
	"fmt"
	"io"
	"math"
)

// A3 landscape sheet after JIS Z 8311: a 10 mm border, widened to 20 mm on the left for filing.
const (
	a3Width, a3Height     = 420.0, 297.0
	frameLeft, frameOther = 20.0, 10.0
	titleBlockWidth       = 170.0 // JIS Z 8311 keeps the title block at most 170 mm long
	titleBlockRow         = 8.0
	titleBlockHeight      = 4 * titleBlockRow
	contentPadding        = 5.0 // between the frame and the views
)

// standardScales are the preferred scales of JIS Z 8314, largest first.
var standardScales = []float64{10, 5, 2, 1, 1.0 / 2, 1.0 / 5, 1.0 / 10, 1.0 / 20, 1.0 / 50, 1.0 / 100}

// WritePDF writes a drawing of the mesh as a one-page A3 PDF following the JIS layout for part
// drawings: a frame with a filing margin and center marks, the views of WriteSVG labeled in
// Japanese and English, and a title block with the title (DrawingTitle), drawing number
// (DrawingNumber), author (DrawingAuthor), date (DrawingDate), scale, units and projection.
//
// Unless DrawingScale is given, the largest JIS standard scale at which the views fit is used.
// Text is drawn as outlines from built-in glyphs (printable ASCII, the labels' Japanese and the
// symbols drawings use); other characters are an error. The file holds no timestamps, so the
// same part always gives the same bytes.
func (m *Mesh) WritePDF(w io.Writer, options ...DrawingOption) error {
	o, err := checkDrawing(m, options)
	if err != nil {
		return err
	}
	o.bilingual = true
	views, err := m.drawViews(o)
	if err != nil {
		return err
	}
	sheet, region, err := fitSheet(views, o)
	if err != nil {
		return err
	}
	offset := Vector2{region.left + (region.width-sheet.width)/2, region.top + (region.height-sheet.height)/2}
	var marks []mark
	for _, mk := range sheet.display(false) {
		marks = append(marks, mk.moved(offset))
	}
	marks = append(marks, frameMarks()...)
	marks = append(marks, titleBlockMarks(sheet.options)...)
	bufferedWriter := bufio.NewWriter(w)
	subject := fmt.Sprintf("A3 part drawing, scale %s, third-angle projection, millimeters",
		scaleText(sheet.options.scale))
	if err := writePDF(bufferedWriter, a3Width, a3Height, marks, o.title, subject, o); err != nil {
		return err
	}
	return bufferedWriter.Flush()
}

// SavePDF writes the drawing made by WritePDF to path.
func (m *Mesh) SavePDF(path string, options ...DrawingOption) error {
	return saveFile(path, func(w io.Writer) error { return m.WritePDF(w, options...) })
}

// area is a rectangle on the sheet (paper millimeters).
type area struct{ left, top, width, height float64 }

// contentAreas are where the views may go: the whole width above the title block, or the whole
// height left of it.
func contentAreas() []area {
	left, top := frameLeft+contentPadding, frameOther+contentPadding
	right, bottom := a3Width-frameOther-contentPadding, a3Height-frameOther-contentPadding
	return []area{
		{left, top, right - left, bottom - titleBlockHeight - top},
		{left, top, right - titleBlockWidth - left, bottom - top},
	}
}

// fitSheet lays the views out at the largest standard scale (or the chosen one) at which they fit
// one of the content areas, trying the section views both below and beside the others.
func fitSheet(views *drawingViews, o drawingOptions) (*sheet, area, error) {
	scales := standardScales
	if o.scaleChosen {
		scales = []float64{o.scale}
	}
	for _, scale := range scales {
		o.scale = scale
		for _, beside := range []bool{false, true} {
			s := views.sheet(o, beside)
			for _, a := range contentAreas() {
				if s.width <= a.width && s.height <= a.height {
					return s, a, nil
				}
			}
		}
	}
	return nil, area{}, fmt.Errorf("tsumiki: the drawing does not fit an A3 sheet at scale %s",
		scaleText(scales[len(scales)-1]))
}

// rectangle is a closed polyline around low..high.
func rectangle(low, high Vector2) []Vector2 {
	return []Vector2{low, {high.X, low.Y}, high, {low.X, high.Y}, low}
}

// frameMarks draws the frame and the center marks, which run from the edge of the sheet 5 mm
// into the frame.
func frameMarks() []mark {
	frameRight, frameBottom := a3Width-frameOther, a3Height-frameOther
	middleX, middleY := a3Width/2, a3Height/2
	return []mark{
		thickMark(rectangle(Vector2{frameLeft, frameOther}, Vector2{frameRight, frameBottom})...),
		thickMark(Vector2{middleX, 0}, Vector2{middleX, frameOther + 5}),
		thickMark(Vector2{middleX, a3Height}, Vector2{middleX, frameBottom - 5}),
		thickMark(Vector2{0, middleY}, Vector2{frameLeft + 5, middleY}),
		thickMark(Vector2{a3Width, middleY}, Vector2{frameRight - 5, middleY}),
	}
}

// titleCell is one labeled field of the title block: its left edge and width within the block,
// the row, the Japanese and English label and the value.
type titleCell struct {
	left, width       float64
	row               int
	japanese, english string
	value             string
	valueSize         float64
	bold              bool
}

// labelWidth is the width of the label part of a title block cell.
const labelWidth = 26.0

func titleCells(o drawingOptions) []titleCell {
	const value = 3.2
	return []titleCell{
		{0, titleBlockWidth, 0, "図名", "TITLE", o.title, 4.2, true},
		{0, 80, 1, "図番", "DRAWING No.", o.number, value, false},
		{80, 90, 1, "作成者", "DRAWN BY", o.author, value, false},
		{0, 55, 2, "尺度", "SCALE", scaleText(o.scale), value, false},
		{55, 40, 2, "単位", "UNIT", "mm", value, false},
		{95, 75, 2, "日付", "DATE", o.date, value, false},
		{0, titleBlockWidth, 3, "投影法", "PROJECTION", "第三角法  THIRD ANGLE", value, false},
	}
}

// titleBlockMarks draws the title block in the bottom right corner of the frame.
func titleBlockMarks(o drawingOptions) []mark {
	right, bottom := a3Width-frameOther, a3Height-frameOther
	origin := Vector2{right - titleBlockWidth, bottom - titleBlockHeight}
	marks := []mark{thickMark(rectangle(origin, Vector2{right, bottom})...)}
	for row := 1; row < 4; row++ {
		y := origin.Y + float64(row)*titleBlockRow
		marks = append(marks, lineMark(Vector2{origin.X, y}, Vector2{right, y}))
	}
	for _, cell := range titleCells(o) {
		top := origin.Y + float64(cell.row)*titleBlockRow
		left := origin.X + cell.left
		if cell.left > 0 {
			marks = append(marks, lineMark(Vector2{left, top}, Vector2{left, top + titleBlockRow}))
		}
		marks = append(marks, lineMark(Vector2{left + labelWidth, top}, Vector2{left + labelWidth, top + titleBlockRow}),
			textMark(Vector2{left + 1.5, top + 3.4}, cell.japanese, anchorStart, false, 2.6),
			textMark(Vector2{left + 1.5, top + 6.6}, cell.english, anchorStart, false, 1.7))
		if cell.value != "" {
			value := textMark(Vector2{left + labelWidth + 2, top + 5.6}, cell.value, anchorStart, false, cell.valueSize)
			value.bold = cell.bold
			marks = append(marks, value)
		}
	}
	center := Vector2{right - 22, origin.Y + 3.5*titleBlockRow}
	return append(marks, thirdAngleSymbol(center, titleBlockRow-2.5)...)
}

// thirdAngleSymbol draws the symbol of third-angle projection (JIS B 0001): a truncated cone seen
// from the side with its small end towards its end view, which is drawn to its left as two
// circles. size is the larger diameter.
func thirdAngleSymbol(center Vector2, size float64) []mark {
	large, small := size, size/2
	coneLeft, coneRight := center.X, center.X+size*1.2
	circles := Vector2{center.X - size*0.9, center.Y}
	circle := func(c Vector2, diameter float64) []Vector2 {
		var points []Vector2
		for k := 0; k <= 48; k++ {
			angle := 2 * math.Pi * float64(k) / 48
			points = append(points, c.Add(Vector2{math.Cos(angle), math.Sin(angle)}.Scale(diameter/2)))
		}
		return points
	}
	return []mark{
		lineMark(circle(circles, large)...),
		lineMark(circle(circles, small)...),
		lineMark(Vector2{coneLeft, center.Y - small/2}, Vector2{coneRight, center.Y - large/2},
			Vector2{coneRight, center.Y + large/2}, Vector2{coneLeft, center.Y + small/2},
			Vector2{coneLeft, center.Y - small/2}),
		centerMark(Vector2{circles.X - large*0.7, center.Y}, Vector2{coneRight + 1.5, center.Y}),
	}
}
