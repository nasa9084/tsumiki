package tsumiki

import (
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// markKind is the kind of an annotation element.
type markKind int

const (
	markLine   markKind = iota // thin continuous polyline
	markCenter                 // thin chain line (center lines, cutting planes)
	markThick                  // thick polyline (ends of a cutting plane)
	markArrow                  // filled triangle
	markText                   // text at points[0]
)

// mark is an annotation element in paper millimeters, relative to its panel's box.
type mark struct {
	kind     markKind
	points   []Vector2
	text     string
	anchor   string // text-anchor: start, middle or end
	vertical bool   // text rotated to read upwards
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

// bounds estimates the paper area the mark covers.
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
	before := map[string]float64{"start": 0, "middle": width / 2, "end": width}[m.anchor]
	if m.vertical { // reads upwards from at: the glyphs lie left of the baseline
		return Vector2{at.X - m.size, at.Y - (width - before)}, Vector2{at.X, at.Y + before}
	}
	return Vector2{at.X - before, at.Y - m.size}, Vector2{at.X + width - before, at.Y + m.size*0.25}
}

func (s *sheet) write(w io.Writer) {
	o := s.options
	_, _ = fmt.Fprintf(w, `<svg xmlns="http://www.w3.org/2000/svg" width="%smm" height="%smm" viewBox="0 0 %s %s" `+
		`font-family="sans-serif" font-size="%s">`+"\n", number(s.width), number(s.height),
		number(s.width), number(s.height), number(o.textSize))
	_, _ = fmt.Fprintf(w, `<defs><pattern id="hatch" width="1.5" height="1.5" patternUnits="userSpaceOnUse" `+
		`patternTransform="rotate(45)"><path d="M0 0V1.5" stroke="#000" stroke-width="%s"/></pattern></defs>`+"\n",
		number(o.strokeThin))
	_, _ = fmt.Fprintf(w, `<rect width="100%%" height="100%%" fill="#fff"/>`+"\n")
	for _, p := range s.panels {
		s.writePanel(w, p)
	}
	s.writeTitleBlock(w)
	_, _ = fmt.Fprintln(w, "</svg>")
}

func (s *sheet) writePanel(w io.Writer, p *panel) {
	o := s.options
	if len(p.hatch) > 0 {
		_, _ = fmt.Fprint(w, `<path fill="url(#hatch)" fill-rule="evenodd" stroke="none" d="`)
		for _, loop := range p.hatch {
			for i, point := range loop {
				command := "L"
				if i == 0 {
					command = "M"
				}
				paper := p.toPaper(point)
				_, _ = fmt.Fprintf(w, "%s%s %s", command, number(paper.X), number(paper.Y))
			}
			_, _ = fmt.Fprint(w, "Z")
		}
		_, _ = fmt.Fprintln(w, `"/>`)
	}
	s.writeLines(w, p, p.hidden, `stroke="#777" stroke-width="%s" stroke-dasharray="1.5 1"`, o.strokeThin)
	s.writeLines(w, p, p.visible, `stroke="#000" stroke-width="%s" stroke-linecap="round"`, o.strokeVisible)
	offset := Vector2{p.left, p.top}
	for _, m := range p.marks {
		writeMark(w, m, offset, o)
	}
	_, _ = fmt.Fprintf(w, `<text x="%s" y="%s">%s</text>`+"\n",
		number(p.left), number(p.labelY(o)), escapeXML(p.label))
}

// labelY is where the panel's label sits: above everything drawn above the box, or level with
// the other labels of its row when the layout aligned them.
func (p *panel) labelY(o drawingOptions) float64 {
	if p.labelLine != 0 {
		return p.labelLine
	}
	return p.top - p.margins(o).top + o.textSize
}

func (s *sheet) writeLines(w io.Writer, p *panel, lines []segment2, style string, stroke float64) {
	if len(lines) == 0 {
		return
	}
	_, _ = fmt.Fprintf(w, `<path fill="none" `+style+` d="`, number(stroke))
	for _, line := range lines {
		a, b := p.toPaper(line[0]), p.toPaper(line[1])
		_, _ = fmt.Fprintf(w, "M%s %sL%s %s", number(a.X), number(a.Y), number(b.X), number(b.Y))
	}
	_, _ = fmt.Fprintln(w, `"/>`)
}

func writeMark(w io.Writer, m mark, offset Vector2, o drawingOptions) {
	points := make([]string, len(m.points))
	for i, point := range m.points {
		paper := point.Add(offset)
		points[i] = number(paper.X) + " " + number(paper.Y)
	}
	switch m.kind {
	case markLine:
		_, _ = fmt.Fprintf(w, `<path fill="none" stroke="#000" stroke-width="%s" d="M%s"/>`+"\n",
			number(o.strokeThin), strings.Join(points, "L"))
	case markCenter:
		_, _ = fmt.Fprintf(w, `<path fill="none" stroke="#000" stroke-width="%s" stroke-dasharray="6 1 1 1" d="M%s"/>`+"\n",
			number(o.strokeThin), strings.Join(points, "L"))
	case markThick:
		_, _ = fmt.Fprintf(w, `<path fill="none" stroke="#000" stroke-width="%s" d="M%s"/>`+"\n",
			number(o.strokeVisible*1.5), strings.Join(points, "L"))
	case markArrow:
		_, _ = fmt.Fprintf(w, `<path fill="#000" d="M%sZ"/>`+"\n", strings.Join(points, "L"))
	case markText:
		at := m.points[0].Add(offset)
		transform := ""
		if m.vertical {
			transform = fmt.Sprintf(` transform="rotate(-90 %s %s)"`, number(at.X), number(at.Y))
		}
		_, _ = fmt.Fprintf(w, `<text x="%s" y="%s" text-anchor="%s" font-size="%s"%s>%s</text>`+"\n",
			number(at.X), number(at.Y), m.anchor, number(m.size), transform, escapeXML(m.text))
	}
}

func (s *sheet) writeTitleBlock(w io.Writer) {
	o := s.options
	x, y := s.width-sheetMargin, s.height-sheetMargin
	note := fmt.Sprintf("scale %s:1  ·  millimeters  ·  third-angle projection", number(o.scale))
	if o.scale < 1 {
		note = fmt.Sprintf("scale 1:%s  ·  millimeters  ·  third-angle projection", number(1/o.scale))
	}
	_, _ = fmt.Fprintf(w, `<text x="%s" y="%s" text-anchor="end" font-size="%s">%s</text>`+"\n",
		number(x), number(y), number(o.textSize*0.8), note)
	if o.title != "" {
		_, _ = fmt.Fprintf(w, `<text x="%s" y="%s" text-anchor="end" font-size="%s" font-weight="bold">%s</text>`+"\n",
			number(x), number(y-o.textSize*1.4), number(o.textSize*1.4), escapeXML(o.title))
	}
}

// number formats a coordinate with at most three decimals.
func number(x float64) string {
	text := strconv.FormatFloat(x, 'f', 3, 64)
	text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
	if text == "-0" {
		return "0"
	}
	return text
}

// length formats a dimension value with at most two decimals.
func length(x float64) string {
	return number(math.Round(x*100) / 100)
}

func escapeXML(text string) string {
	var sb strings.Builder
	_ = xml.EscapeText(&sb, []byte(text)) // writing to a strings.Builder cannot fail
	return sb.String()
}

// unit returns v scaled to length 1 (or v itself when it has no length).
func unit(v Vector2) Vector2 {
	if l := v.Length(); l > 0 {
		return v.Scale(1 / l)
	}
	return v
}
