package tsumiki

import (
	"math"
)

// panel is one view on the sheet: its line work in model millimeters (screen coordinates of the
// view) and its annotations in paper millimeters relative to the view's box.
type panel struct {
	label           string
	camera          camera
	set             *edgeSet
	buffer          *depthBuffer
	drawnEdges      []int // indices into set.edges of the edges drawn in this view
	visible, hidden []segment2
	hatch           [][]Vector2 // closed loops of cut faces (section views)
	low, high       Vector2
	scale           float64
	marks           []mark
	left, top       float64 // position of the box's top-left corner on the sheet, set by layOut
	labelLine       float64 // baseline of the label when layOut aligns it with its row (0: just above)
}

// newPanel projects the mesh and sorts the edges to draw into visible and hidden pieces.
func newPanel(set *edgeSet, c camera, label string, o drawingOptions, withHidden bool) *panel {
	p := &panel{label: label, camera: c, set: set, scale: o.scale,
		low: Vector2{math.Inf(1), math.Inf(1)}, high: Vector2{math.Inf(-1), math.Inf(-1)}}
	for _, vertex := range set.mesh.Vertices {
		s, _ := c.project(vertex)
		p.low = Vector2{math.Min(p.low.X, s.X), math.Min(p.low.Y, s.Y)}
		p.high = Vector2{math.Max(p.high.X, s.X), math.Max(p.high.Y, s.Y)}
	}
	p.buffer = newDepthBuffer(set, c)
	step := p.sampleStep(o)
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
		p.drawnEdges = append(p.drawnEdges, i)
		visible, hidden := p.buffer.splitByVisibility(c, a, b, step)
		p.visible = append(p.visible, visible...)
		if withHidden {
			p.hidden = append(p.hidden, hidden...)
		}
	}
	if len(p.hidden) > 0 {
		cell := math.Max(p.size().Length()/200, 10*coincidentDistance)
		p.hidden = newLineIndex(p.visible, p.low, p.high, cell).withoutCovered(p.hidden, step)
	}
	return p
}

func (p *panel) sampleStep(o drawingOptions) float64 {
	return math.Max(p.size().Length()*o.sampleFraction, 1e-3)
}

func (p *panel) size() Vector2 { return p.high.Sub(p.low) }

// boxSize is the size of the view's box on the paper.
func (p *panel) boxSize() Vector2 { return p.size().Scale(p.scale) }

// rel maps a point of the view to paper millimeters relative to the box's top-left corner
// (x grows to the right, y downwards).
func (p *panel) rel(s Vector2) Vector2 {
	return Vector2{(s.X - p.low.X) * p.scale, (p.high.Y - s.Y) * p.scale}
}

// toPaper maps a point of the view to the sheet.
func (p *panel) toPaper(s Vector2) Vector2 { return p.rel(s).Add(Vector2{p.left, p.top}) }

// isVisible reports whether a 3D point is in sight in this view.
func (p *panel) isVisible(point Vector3) bool {
	return !p.buffer.hidden(p.camera.project(point))
}

// margins returns how far the annotations and the label reach beyond the box on the paper:
// left, top, right and bottom.
func (p *panel) margins(o drawingOptions) panelMargins {
	var m panelMargins
	size := p.boxSize()
	for _, mk := range p.marks {
		low, high := mk.bounds()
		m.left = math.Max(m.left, -low.X)
		m.top = math.Max(m.top, -low.Y)
		m.right = math.Max(m.right, high.X-size.X)
		m.bottom = math.Max(m.bottom, high.Y-size.Y)
	}
	m.top += o.textSize * 1.8
	return m
}

// addOverallDimensions dimensions the box: its width below it and/or its height to its left.
func (p *panel) addOverallDimensions(o drawingOptions, width, height bool) {
	size := p.boxSize()
	offset := o.dimensionOffset
	if width && p.size().X > 0 {
		p.marks = append(p.marks, linearDimension(Vector2{0, size.Y}, Vector2{size.X, size.Y},
			Vector2{0, offset}, length(p.size().X), o)...)
	}
	if height && p.size().Y > 0 {
		p.marks = append(p.marks, linearDimension(Vector2{0, size.Y}, Vector2{0, 0},
			Vector2{-offset, 0}, length(p.size().Y), o)...)
	}
}

// linearDimension draws the distance between from1 and from2 (paper, relative to the box) with
// the dimension line moved away from them by offset.
func linearDimension(from1, from2, offset Vector2, text string, o drawingOptions) []mark {
	to1, to2 := from1.Add(offset), from2.Add(offset)
	away := unit(offset)
	marks := []mark{
		lineMark(from1.Add(away), to1.Add(away.Scale(1.5))),
		lineMark(from2.Add(away), to2.Add(away.Scale(1.5))),
		lineMark(to1, to2),
		arrowMark(to1, unit(to1.Sub(to2))),
		arrowMark(to2, unit(to2.Sub(to1))),
	}
	// Horizontal text sits on the dimension line; vertical text (rotated to read upwards) has its
	// glyphs on the left of its baseline, so it moves the other way to end up on the far side.
	vertical := math.Abs(to1.X-to2.X) < 1e-9
	shift := -o.textSize * 0.4
	if vertical {
		shift = -shift
	}
	middle := to1.Add(to2).Scale(0.5).Add(away.Scale(shift))
	return append(marks, textMark(middle, text, "middle", vertical, o.textSize))
}
