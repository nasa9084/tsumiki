package tsumiki

import (
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// writeSVG writes marks as an SVG document of the given size (paper millimeters).
func writeSVG(w io.Writer, width, height float64, marks []mark, o drawingOptions) {
	_, _ = fmt.Fprintf(w, `<svg xmlns="http://www.w3.org/2000/svg" width="%smm" height="%smm" viewBox="0 0 %s %s" `+
		`font-family="sans-serif" font-size="%s">`+"\n", number(width), number(height),
		number(width), number(height), number(o.textSize))
	_, _ = fmt.Fprintf(w, `<defs><pattern id="hatch" width="1.5" height="1.5" patternUnits="userSpaceOnUse" `+
		`patternTransform="rotate(45)"><path d="M0 0V1.5" stroke="#000" stroke-width="%s"/></pattern></defs>`+"\n",
		number(o.strokeThin))
	_, _ = fmt.Fprintf(w, `<rect width="100%%" height="100%%" fill="#fff"/>`+"\n")
	for _, m := range marks {
		writeSVGMark(w, m, o)
	}
	_, _ = fmt.Fprintln(w, "</svg>")
}

func svgPoints(points []Vector2) string {
	parts := make([]string, len(points))
	for i, point := range points {
		parts[i] = number(point.X) + " " + number(point.Y)
	}
	return strings.Join(parts, "L")
}

func svgSegments(segments []segment2) string {
	var sb strings.Builder
	for _, s := range segments {
		sb.WriteString("M" + svgPoints(s[:]))
	}
	return sb.String()
}

func writeSVGMark(w io.Writer, m mark, o drawingOptions) {
	switch m.kind {
	case markLine:
		_, _ = fmt.Fprintf(w, `<path fill="none" stroke="#000" stroke-width="%s" d="M%s"/>`+"\n",
			number(o.strokeThin), svgPoints(m.points))
	case markCenter:
		_, _ = fmt.Fprintf(w, `<path fill="none" stroke="#000" stroke-width="%s" stroke-dasharray="6 1 1 1" d="M%s"/>`+"\n",
			number(o.strokeThin), svgPoints(m.points))
	case markThick:
		_, _ = fmt.Fprintf(w, `<path fill="none" stroke="#000" stroke-width="%s" d="M%s"/>`+"\n",
			number(o.strokeVisible*1.5), svgPoints(m.points))
	case markArrow:
		_, _ = fmt.Fprintf(w, `<path fill="#000" d="M%sZ"/>`+"\n", svgPoints(m.points))
	case markVisible:
		_, _ = fmt.Fprintf(w, `<path fill="none" stroke="#000" stroke-width="%s" stroke-linecap="round" d="%s"/>`+"\n",
			number(o.strokeVisible), svgSegments(m.segments))
	case markHidden:
		_, _ = fmt.Fprintf(w, `<path fill="none" stroke="#777" stroke-width="%s" stroke-dasharray="1.5 1" d="%s"/>`+"\n",
			number(o.strokeThin), svgSegments(m.segments))
	case markHatch:
		var d strings.Builder
		for _, loop := range m.loops {
			d.WriteString("M" + svgPoints(loop) + "Z")
		}
		_, _ = fmt.Fprintf(w, `<path fill="url(#hatch)" fill-rule="evenodd" stroke="none" d="%s"/>`+"\n", d.String())
	case markText:
		at := m.points[0]
		attributes := ""
		if m.vertical {
			attributes = fmt.Sprintf(` transform="rotate(-90 %s %s)"`, number(at.X), number(at.Y))
		}
		if m.bold {
			attributes += ` font-weight="bold"`
		}
		_, _ = fmt.Fprintf(w, `<text x="%s" y="%s" text-anchor="%s" font-size="%s"%s>%s</text>`+"\n",
			number(at.X), number(at.Y), m.anchor, number(m.size), attributes, escapeXML(m.text))
	}
}

// number formats a coordinate with at most two decimals (0.01 mm). Coarser than the arithmetic
// that produced it, so that the output does not change with the last bits of floating point
// rounding (which can differ between processor architectures).
func number(x float64) string {
	text := strconv.FormatFloat(x, 'f', 2, 64)
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
