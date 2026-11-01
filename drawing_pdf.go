package tsumiki

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"math"
	"strings"
)

// pointsPerMillimeter converts paper millimeters to PDF points.
const pointsPerMillimeter = 72 / 25.4

// pdfContent builds a PDF content stream in paper millimeters with y growing downwards.
type pdfContent struct {
	bytes.Buffer
	o drawingOptions
}

func (c *pdfContent) op(format string, args ...any) { _, _ = fmt.Fprintf(c, format+"\n", args...) }

func (c *pdfContent) moveTo(p Vector2) { c.op("%s %s m", number(p.X), number(p.Y)) }
func (c *pdfContent) lineTo(p Vector2) { c.op("%s %s l", number(p.X), number(p.Y)) }

func (c *pdfContent) polyline(points []Vector2) {
	c.moveTo(points[0])
	for _, point := range points[1:] {
		c.lineTo(point)
	}
}

// stroke sets up a pen: width, gray level (0 black), dash pattern and line cap.
func (c *pdfContent) stroke(width, gray float64, dash string, roundCap bool) {
	capStyle := 0
	if roundCap {
		capStyle = 1
	}
	c.op("%s w %s G [%s] 0 d %d J", number(width), number(gray), dash, capStyle)
}

func (c *pdfContent) mark(m mark) error {
	o := c.o
	switch m.kind {
	case markLine, markCenter, markThick:
		width, dash := o.strokeThin, ""
		if m.kind == markCenter {
			dash = "6 1 1 1"
		}
		if m.kind == markThick {
			width = o.strokeVisible * 1.5
		}
		c.stroke(width, 0, dash, false)
		c.polyline(m.points)
		c.op("S")
	case markArrow:
		c.op("0 g")
		c.polyline(m.points)
		c.op("h f")
	case markVisible, markHidden:
		if m.kind == markVisible {
			c.stroke(o.strokeVisible, 0, "", true)
		} else {
			c.stroke(o.strokeThin, 0.47, "1.5 1", false)
		}
		for _, s := range m.segments {
			c.moveTo(s[0])
			c.lineTo(s[1])
		}
		c.op("S")
	case markHatch:
		c.hatch(m.loops)
	case markText:
		return c.text(m)
	}
	return nil
}

// hatch fills the even-odd area of loops with thin lines at 45°, 1.5 mm apart.
func (c *pdfContent) hatch(loops [][]Vector2) {
	low, high := Vector2{math.Inf(1), math.Inf(1)}, Vector2{math.Inf(-1), math.Inf(-1)}
	c.op("q")
	for _, loop := range loops {
		c.polyline(loop)
		c.op("h")
		for _, p := range loop {
			low = Vector2{math.Min(low.X, p.X), math.Min(low.Y, p.Y)}
			high = Vector2{math.Max(high.X, p.X), math.Max(high.Y, p.Y)}
		}
	}
	c.op("W* n")
	c.stroke(c.o.strokeThin, 0, "", false)
	// lines x + y = k, every 1.5·√2 along x, covering the box
	const spacing = 1.5 * math.Sqrt2
	for k := math.Floor((low.X+low.Y)/spacing) * spacing; k <= high.X+high.Y; k += spacing {
		c.moveTo(Vector2{k - low.Y, low.Y})
		c.lineTo(Vector2{k - high.Y, high.Y})
	}
	c.op("S Q")
}

// text draws a text mark as filled glyph outlines; bold text is also stroked.
func (c *pdfContent) text(m mark) error {
	outlines, err := textOutlines(m)
	if err != nil {
		return err
	}
	var current, start Vector2
	for _, glyph := range outlines {
		for _, command := range glyph {
			switch command.op {
			case 'M':
				c.moveTo(command.points[0])
				current, start = command.points[0], command.points[0]
			case 'L':
				c.lineTo(command.points[0])
				current = command.points[0]
			case 'Q': // PDF has no quadratic curves: raise it to the equivalent cubic
				control, end := command.points[0], command.points[1]
				c.curveTo(current.Add(control.Sub(current).Scale(2.0/3)), end.Add(control.Sub(end).Scale(2.0/3)), end)
				current = end
			case 'C':
				c.curveTo(command.points[0], command.points[1], command.points[2])
				current = command.points[2]
			case 'Z':
				c.op("h")
				current = start
			}
		}
	}
	if m.bold {
		c.stroke(m.size*0.04, 0, "", false)
		c.op("0 g B")
	} else {
		c.op("0 g f")
	}
	return nil
}

func (c *pdfContent) curveTo(control1, control2, end Vector2) {
	c.op("%s %s %s %s %s %s c", number(control1.X), number(control1.Y), number(control2.X), number(control2.Y),
		number(end.X), number(end.Y))
}

// writePDF writes marks as a one-page PDF of the given size (paper millimeters). The file holds
// no dates or random identifiers, so the same drawing always gives the same bytes.
func writePDF(w io.Writer, width, height float64, marks []mark, title, subject string, o drawingOptions) error {
	content := &pdfContent{o: o}
	// millimeters with y down, to points with y up
	content.op("%s 0 0 %s 0 %s cm 1 j", number4(pointsPerMillimeter), number4(-pointsPerMillimeter),
		number4(height*pointsPerMillimeter))
	for _, m := range marks {
		if err := content.mark(m); err != nil {
			return err
		}
	}
	var compressed bytes.Buffer
	zw, err := zlib.NewWriterLevel(&compressed, zlib.BestCompression)
	if err != nil {
		return err
	}
	_, _ = zw.Write(content.Bytes())
	if err := zw.Close(); err != nil {
		return err
	}
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %s %s] /Contents 4 0 R /Resources << >> >>",
			number4(width*pointsPerMillimeter), number4(height*pointsPerMillimeter)),
		fmt.Sprintf("<< /Length %d /Filter /FlateDecode >>\nstream\n%s\nendstream", compressed.Len(), compressed.String()),
		fmt.Sprintf("<< /Title %s /Subject %s /Producer (tsumiki) >>", pdfString(title), pdfString(subject)),
	}
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	offsets := make([]int, len(objects))
	for i, object := range objects {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R /Info %d 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		len(objects)+1, len(objects), xref)
	_, err = w.Write(out.Bytes())
	return err
}

// number4 formats a number with up to four decimals (page geometry in points).
func number4(x float64) string {
	text := fmt.Sprintf("%.4f", x)
	return strings.TrimRight(strings.TrimRight(text, "0"), ".")
}

// pdfString writes text as a PDF text string: UTF-16BE with a byte order mark when it is not
// plain ASCII.
func pdfString(text string) string {
	ascii := true
	for _, r := range text {
		if r > 0x7e || r < 0x20 {
			ascii = false
		}
	}
	if ascii {
		replacer := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`)
		return "(" + replacer.Replace(text) + ")"
	}
	var sb strings.Builder
	sb.WriteString("<FEFF")
	for _, unit := range utf16Units(text) {
		fmt.Fprintf(&sb, "%04X", unit)
	}
	sb.WriteString(">")
	return sb.String()
}

func utf16Units(text string) []uint16 {
	var units []uint16
	for _, r := range text {
		if r >= 0x10000 {
			r -= 0x10000
			units = append(units, uint16(0xd800+(r>>10)), uint16(0xdc00+(r&0x3ff)))
			continue
		}
		units = append(units, uint16(r))
	}
	return units
}
