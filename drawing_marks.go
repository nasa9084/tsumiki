package tsumiki

import (
	"fmt"
	"math"
	"unicode/utf8"
)

// Text anchors: where a text's reference point lies along the text.
const (
	anchorStart  = "start"
	anchorMiddle = "middle"
	anchorEnd    = "end"
)

// markKind is the kind of an annotation element.
type markKind int

const (
	markLine    markKind = iota // thin continuous polyline
	markCenter                  // thin chain line (center lines, cutting planes)
	markThick                   // thick polyline (ends of a cutting plane, frames)
	markArrow                   // filled polygon
	markText                    // text at points[0]
	markVisible                 // visible edges of the part (segments)
	markHidden                  // hidden edges of the part, dashed (segments)
	markHatch                   // hatched area (loops, filled even-odd)
)

// mark is an element of a drawing in paper millimeters. Annotations are made relative to their
// panel's box and moved onto the sheet when the drawing is put together.
type mark struct {
	kind     markKind
	points   []Vector2
	segments []segment2  // markVisible, markHidden
	loops    [][]Vector2 // markHatch
	text     string
	anchor   string // text-anchor: start, middle or end
	vertical bool   // text rotated to read upwards
	bold     bool
	size     float64
}

func lineMark(points ...Vector2) mark   { return mark{kind: markLine, points: points} }
func centerMark(points ...Vector2) mark { return mark{kind: markCenter, points: points} }
func thickMark(points ...Vector2) mark  { return mark{kind: markThick, points: points} }

func textMark(at Vector2, text, anchor string, vertical bool, size float64) mark {
	return mark{kind: markText, points: []Vector2{at}, text: text, anchor: anchor, vertical: vertical, size: size}
}

// arrowMark is an arrowhead with its tip at tip, pointing along direction (a unit vector).
func arrowMark(tip, direction Vector2) mark {
	const arrowLength, arrowHalfWidth = 2.5, 0.6
	base := tip.Sub(direction.Scale(arrowLength))
	side := Vector2{-direction.Y, direction.X}.Scale(arrowHalfWidth)
	return mark{kind: markArrow, points: []Vector2{tip, base.Add(side), base.Sub(side)}}
}

// bounds estimates the paper area the mark covers (annotations only: points and text).
func (m mark) bounds() (low, high Vector2) {
	if m.kind != markText {
		low, high = m.points[0], m.points[0]
		for _, point := range m.points[1:] {
			low = Vector2{math.Min(low.X, point.X), math.Min(low.Y, point.Y)}
			high = Vector2{math.Max(high.X, point.X), math.Max(high.Y, point.Y)}
		}
		return low, high
	}
	at := m.points[0]
	width := float64(utf8.RuneCountInString(m.text)) * m.size * 0.6
	before := map[string]float64{anchorStart: 0, anchorMiddle: width / 2, anchorEnd: width}[m.anchor]
	if m.vertical { // reads upwards from at: the glyphs lie left of the baseline
		return Vector2{at.X - m.size, at.Y - (width - before)}, Vector2{at.X, at.Y + before}
	}
	return Vector2{at.X - before, at.Y - m.size}, Vector2{at.X + width - before, at.Y + m.size*0.25}
}

// moved returns the mark shifted by offset.
func (m mark) moved(offset Vector2) mark {
	shift := func(points []Vector2) []Vector2 {
		out := make([]Vector2, len(points))
		for i, point := range points {
			out[i] = point.Add(offset)
		}
		return out
	}
	m.points = shift(m.points)
	if m.segments != nil {
		segments := make([]segment2, len(m.segments))
		for i, s := range m.segments {
			segments[i] = segment2{s[0].Add(offset), s[1].Add(offset)}
		}
		m.segments = segments
	}
	if m.loops != nil {
		loops := make([][]Vector2, len(m.loops))
		for i, loop := range m.loops {
			loops[i] = shift(loop)
		}
		m.loops = loops
	}
	return m
}

// display lists everything on the sheet in paper millimeters: each panel's hatching, hidden and
// visible edges, annotations and label, then the note with the scale when withNote is set.
func (s *sheet) display(withNote bool) []mark {
	o := s.options
	var marks []mark
	for _, p := range s.panels {
		toPaper := func(points []Vector2) []Vector2 {
			out := make([]Vector2, len(points))
			for i, point := range points {
				out[i] = p.toPaper(point)
			}
			return out
		}
		if len(p.hatch) > 0 {
			hatch := mark{kind: markHatch}
			for _, loop := range p.hatch {
				hatch.loops = append(hatch.loops, toPaper(loop))
			}
			marks = append(marks, hatch)
		}
		for _, kind := range []markKind{markHidden, markVisible} {
			lines := map[markKind][]segment2{markHidden: p.hidden, markVisible: p.visible}[kind]
			if len(lines) == 0 {
				continue
			}
			m := mark{kind: kind, segments: make([]segment2, len(lines))}
			for i, line := range lines {
				m.segments[i] = segment2{p.toPaper(line[0]), p.toPaper(line[1])}
			}
			marks = append(marks, m)
		}
		for _, m := range p.marks {
			marks = append(marks, m.moved(Vector2{p.left, p.top}))
		}
		marks = append(marks, textMark(Vector2{p.left, p.labelY(o)}, p.label, anchorStart, false, o.textSize))
	}
	if withNote {
		marks = append(marks, s.note()...)
	}
	return marks
}

// labelY is where the panel's label sits: above everything drawn above the box, or level with
// the other labels of its row when the layout aligned them.
func (p *panel) labelY(o drawingOptions) float64 {
	if p.labelLine != 0 {
		return p.labelLine
	}
	return p.top - p.margins(o).top + o.textSize
}

// note is the title and scale written in the corner of a plain (SVG) sheet.
func (s *sheet) note() []mark {
	o := s.options
	at := Vector2{s.width - sheetMargin, s.height - sheetMargin}
	marks := []mark{textMark(at, fmt.Sprintf("scale %s  ·  millimeters  ·  third-angle projection",
		scaleText(o.scale)), anchorEnd, false, o.textSize*0.8)}
	if o.title != "" {
		title := textMark(at.Sub(Vector2{0, o.textSize * 1.4}), o.title, anchorEnd, false, o.textSize*1.4)
		title.bold = true
		marks = append(marks, title)
	}
	return marks
}

// scaleText writes a scale the way drawings do: 2:1, 1:1, 1:2.
func scaleText(scale float64) string {
	if scale < 1 {
		return "1:" + number(1/scale)
	}
	return number(scale) + ":1"
}
