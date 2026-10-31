package tsumiki

import (
	"bufio"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// View is a direction from which a drawing shows the part.
type View int

const (
	// ViewFront looks along +Y: X to the right, Z up.
	ViewFront View = iota
	// ViewTop looks down along -Z: X to the right, Y up.
	ViewTop
	// ViewRight looks along -X: Y to the right, Z up.
	ViewRight
	// ViewIsometric looks from the front-right-top corner, with Z up.
	ViewIsometric
)

func (v View) String() string { return [...]string{"front", "top", "right", "isometric"}[v] }

func (v View) camera() camera {
	switch v {
	case ViewFront:
		return camera{forward: Vector3{0, 1, 0}, right: Vector3{1, 0, 0}, up: Vector3{0, 0, 1}}
	case ViewTop:
		return camera{forward: Vector3{0, 0, -1}, right: Vector3{1, 0, 0}, up: Vector3{0, 1, 0}}
	case ViewRight:
		return camera{forward: Vector3{-1, 0, 0}, right: Vector3{0, 1, 0}, up: Vector3{0, 0, 1}}
	case ViewIsometric:
		forward, right := Vector3{-1, 1, -1}.Normalize(), Vector3{1, 1, 0}.Normalize()
		return camera{forward: forward, right: right, up: right.Cross(forward)}
	}
	panic(fmt.Sprintf("tsumiki: unknown view %d", int(v)))
}

// DrawingOption configures WriteSVG and SaveSVG.
type DrawingOption func(*drawingOptions)

type drawingOptions struct {
	title           string
	scale           float64
	hiddenLines     bool
	featureAngle    float64
	dimensions      bool
	sampleFraction  float64
	strokeVisible   float64
	strokeThin      float64
	textSize        float64
	viewGap         float64
	dimensionOffset float64
}

func resolveDrawingOptions(options []DrawingOption) drawingOptions {
	o := drawingOptions{
		scale:           1,
		hiddenLines:     true,
		featureAngle:    30,
		dimensions:      true,
		sampleFraction:  1.0 / 2000,
		strokeVisible:   0.35,
		strokeThin:      0.18,
		textSize:        3.5,
		viewGap:         25,
		dimensionOffset: 8,
	}
	for _, option := range options {
		option(&o)
	}
	return o
}

// DrawingTitle prints title in the drawing's title block.
func DrawingTitle(title string) DrawingOption { return func(o *drawingOptions) { o.title = title } }

// DrawingScale sets the paper millimeters per model millimeter (default 1, i.e. 1:1).
func DrawingScale(scale float64) DrawingOption { return func(o *drawingOptions) { o.scale = scale } }

// WithoutHiddenLines leaves out the dashed lines of edges hidden behind the part.
func WithoutHiddenLines() DrawingOption { return func(o *drawingOptions) { o.hiddenLines = false } }

// WithoutDimensions leaves out the overall dimensions.
func WithoutDimensions() DrawingOption { return func(o *drawingOptions) { o.dimensions = false } }

// FeatureAngle sets how sharply two faces must meet, in degrees, for their shared edge to be
// drawn (default 30). Smaller values also draw the facets of gently curved surfaces.
func FeatureAngle(degrees float64) DrawingOption {
	return func(o *drawingOptions) { o.featureAngle = degrees }
}

// drawnView is one view's line work in model millimeters (screen coordinates of the view).
type drawnView struct {
	view            View
	visible, hidden []segment2
	low, high       Vector2
}

func (d *drawnView) size() Vector2 { return d.high.Sub(d.low) }

// drawView projects the mesh for one view and sorts its edges into visible and hidden pieces.
func drawView(set *edgeSet, view View, o drawingOptions) drawnView {
	c := view.camera()
	d := drawnView{view: view, low: Vector2{math.Inf(1), math.Inf(1)}, high: Vector2{math.Inf(-1), math.Inf(-1)}}
	for _, vertex := range set.mesh.Vertices {
		s, _ := c.project(vertex)
		d.low = Vector2{math.Min(d.low.X, s.X), math.Min(d.low.Y, s.Y)}
		d.high = Vector2{math.Max(d.high.X, s.X), math.Max(d.high.Y, s.Y)}
	}
	buffer := newDepthBuffer(set, c)
	step := math.Max(d.size().Length()*o.sampleFraction, 1e-3)
	cosFeatureAngle := math.Cos(Radians(o.featureAngle))
	for i := range set.edges {
		edge := &set.edges[i]
		if !set.isFeature(edge, cosFeatureAngle) && !set.isSilhouette(edge, c.forward) {
			continue
		}
		a, b := set.mesh.Vertices[edge.a], set.mesh.Vertices[edge.b]
		screenA, _ := c.project(a)
		screenB, _ := c.project(b)
		if screenA.Sub(screenB).Length() < 1e-6 {
			continue // seen end-on
		}
		visible, hidden := buffer.splitByVisibility(c, a, b, step)
		d.visible = append(d.visible, visible...)
		if o.hiddenLines && view != ViewIsometric {
			d.hidden = append(d.hidden, hidden...)
		}
	}
	if len(d.hidden) > 0 {
		cell := math.Max(d.size().Length()/200, 10*coincidentDistance)
		d.hidden = newLineIndex(d.visible, d.low, d.high, cell).withoutCovered(d.hidden, step)
	}
	return d
}

// WriteSVG writes a drawing of the mesh as SVG: front, top and right views in third-angle
// projection plus an isometric view, with the overall dimensions in millimeters.
func (m *Mesh) WriteSVG(w io.Writer, options ...DrawingOption) error {
	if m == nil || len(m.Triangles) == 0 {
		return errors.New("tsumiki: cannot draw an empty mesh")
	}
	o := resolveDrawingOptions(options)
	if o.scale <= 0 || math.IsNaN(o.scale) || math.IsInf(o.scale, 0) {
		return fmt.Errorf("tsumiki: invalid drawing scale %g", o.scale)
	}
	set := newEdgeSet(m)
	views := [4]drawnView{}
	for i, view := range []View{ViewFront, ViewTop, ViewRight, ViewIsometric} {
		views[i] = drawView(set, view, o)
	}
	sheet := layOut(views, o)
	bufferedWriter := bufio.NewWriter(w)
	sheet.write(bufferedWriter)
	return bufferedWriter.Flush()
}

// SaveSVG writes the drawing made by WriteSVG to path.
func (m *Mesh) SaveSVG(path string, options ...DrawingOption) error {
	return saveFile(path, func(w io.Writer) error { return m.WriteSVG(w, options...) })
}

// placedView is a drawn view with its position on the sheet (paper millimeters).
type placedView struct {
	drawnView
	left, top float64
}

// toPaper maps a point of the view to the sheet: x grows to the right, y grows downwards.
func (p *placedView) toPaper(s Vector2, scale float64) Vector2 {
	return Vector2{p.left + (s.X-p.low.X)*scale, p.top + (p.high.Y-s.Y)*scale}
}

const sheetMargin = 10.0 // blank border around the sheet (paper mm)

type sheet struct {
	options       drawingOptions
	width, height float64
	views         [4]placedView // front, top, right, isometric
}

// layOut arranges the views in third-angle projection: the top view above the front view, the
// right view to the right of it, and the isometric view in the remaining corner.
func layOut(views [4]drawnView, o drawingOptions) *sheet {
	front, top, right, iso := views[0], views[1], views[2], views[3]
	s := o.scale
	margin := sheetMargin
	dimensionSpace := 0.0
	if o.dimensions {
		dimensionSpace = o.dimensionOffset + o.textSize*2
	}
	leftColumn := math.Max(front.size().X, top.size().X) * s
	rightColumn := math.Max(right.size().X, iso.size().X) * s
	upperRow := math.Max(top.size().Y, iso.size().Y) * s
	lowerRow := math.Max(front.size().Y, right.size().Y) * s
	labelSpace := o.textSize * 2
	titleSpace := o.textSize * 5
	x0 := margin + dimensionSpace
	x1 := x0 + leftColumn + o.viewGap + dimensionSpace
	y0 := margin + labelSpace
	y1 := y0 + upperRow + o.viewGap + labelSpace
	return &sheet{
		options: o,
		width:   x1 + rightColumn + margin,
		height:  y1 + lowerRow + dimensionSpace + titleSpace + margin,
		views: [4]placedView{
			{drawnView: front, left: x0, top: y1},
			{drawnView: top, left: x0, top: y0 + upperRow - top.size().Y*s},
			{drawnView: right, left: x1, top: y1},
			{drawnView: iso, left: x1, top: y0 + upperRow - iso.size().Y*s},
		},
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

func (s *sheet) write(w io.Writer) {
	o := s.options
	_, _ = fmt.Fprintf(w, `<svg xmlns="http://www.w3.org/2000/svg" width="%smm" height="%smm" viewBox="0 0 %s %s" `+
		`font-family="sans-serif" font-size="%s">`+"\n", number(s.width), number(s.height),
		number(s.width), number(s.height), number(o.textSize))
	_, _ = fmt.Fprintf(w, `<rect width="100%%" height="100%%" fill="#fff"/>`+"\n")
	for i := range s.views {
		view := &s.views[i]
		s.writeLines(w, view, view.hidden, `stroke="#777" stroke-width="%s" stroke-dasharray="1.5 1"`, o.strokeThin)
		s.writeLines(w, view, view.visible, `stroke="#000" stroke-width="%s" stroke-linecap="round"`, o.strokeVisible)
		_, _ = fmt.Fprintf(w, `<text x="%s" y="%s">%s</text>`+"\n",
			number(view.left), number(view.top-o.textSize*0.8), strings.ToUpper(view.view.String()))
	}
	if o.dimensions {
		front, top := &s.views[0], &s.views[1]
		s.horizontalDimension(w, front)
		s.verticalDimension(w, front)
		s.verticalDimension(w, top) // the right view's width is the same Y extent: not repeated
	}
	s.writeTitleBlock(w)
	_, _ = fmt.Fprintln(w, "</svg>")
}

func (s *sheet) writeLines(w io.Writer, view *placedView, lines []segment2, style string, stroke float64) {
	if len(lines) == 0 {
		return
	}
	_, _ = fmt.Fprintf(w, `<path fill="none" `+style+` d="`, number(stroke))
	for _, line := range lines {
		a, b := view.toPaper(line[0], s.options.scale), view.toPaper(line[1], s.options.scale)
		_, _ = fmt.Fprintf(w, "M%s %sL%s %s", number(a.X), number(a.Y), number(b.X), number(b.Y))
	}
	_, _ = fmt.Fprintln(w, `"/>`)
}

// horizontalDimension draws the view's overall width below it.
func (s *sheet) horizontalDimension(w io.Writer, view *placedView) {
	o := s.options
	low := view.toPaper(view.low, o.scale)
	high := view.toPaper(view.high, o.scale)
	bottom := low.Y
	lineY := bottom + o.dimensionOffset
	s.writeDimension(w, Vector2{low.X, bottom}, Vector2{high.X, bottom}, Vector2{low.X, lineY}, Vector2{high.X, lineY},
		view.size().X)
	_, _ = fmt.Fprintf(w, `<text x="%s" y="%s" text-anchor="middle">%s</text>`+"\n",
		number((low.X+high.X)/2), number(lineY-o.textSize*0.4), length(view.size().X))
}

// verticalDimension draws the view's overall height to its left.
func (s *sheet) verticalDimension(w io.Writer, view *placedView) {
	o := s.options
	low := view.toPaper(view.low, o.scale)
	high := view.toPaper(view.high, o.scale)
	lineX := low.X - o.dimensionOffset
	s.writeDimension(w, Vector2{low.X, low.Y}, Vector2{low.X, high.Y}, Vector2{lineX, low.Y}, Vector2{lineX, high.Y},
		view.size().Y)
	textX, textY := lineX-o.textSize*0.4, (low.Y+high.Y)/2
	_, _ = fmt.Fprintf(w, `<text x="%s" y="%s" text-anchor="middle" transform="rotate(-90 %s %s)">%s</text>`+"\n",
		number(textX), number(textY), number(textX), number(textY), length(view.size().Y))
}

// writeDimension draws two extension lines from the part (from1, from2) to the dimension line
// (to1-to2) and the dimension line with an arrowhead at each end.
func (s *sheet) writeDimension(w io.Writer, from1, from2, to1, to2 Vector2, value float64) {
	if value <= 0 {
		return
	}
	extend := func(from, to Vector2) Vector2 { return to.Add(unit(to.Sub(from)).Scale(1.5)) }
	gap := func(from, to Vector2) Vector2 { return from.Add(unit(to.Sub(from)).Scale(1)) }
	lines := []segment2{{gap(from1, to1), extend(from1, to1)}, {gap(from2, to2), extend(from2, to2)}, {to1, to2}}
	_, _ = fmt.Fprintf(w, `<path fill="none" stroke="#000" stroke-width="%s" d="`, number(s.options.strokeThin))
	for _, line := range lines {
		_, _ = fmt.Fprintf(w, "M%s %sL%s %s", number(line[0].X), number(line[0].Y), number(line[1].X), number(line[1].Y))
	}
	_, _ = fmt.Fprintln(w, `"/>`)
	s.writeArrow(w, to1, unit(to2.Sub(to1)).Scale(-1))
	s.writeArrow(w, to2, unit(to1.Sub(to2)).Scale(-1))
}

// writeArrow draws an arrowhead with its tip at tip, pointing along direction.
func (s *sheet) writeArrow(w io.Writer, tip, direction Vector2) {
	const arrowLength, arrowHalfWidth = 2.5, 0.6
	base := tip.Sub(direction.Scale(arrowLength))
	side := Vector2{-direction.Y, direction.X}.Scale(arrowHalfWidth)
	a, b := base.Add(side), base.Sub(side)
	_, _ = fmt.Fprintf(w, `<path fill="#000" d="M%s %sL%s %sL%s %sZ"/>`+"\n",
		number(tip.X), number(tip.Y), number(a.X), number(a.Y), number(b.X), number(b.Y))
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
