package tsumiki

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
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
	title            string
	scale            float64
	hiddenLines      bool
	featureAngle     float64
	dimensions       bool
	detailDimensions bool
	sections         []sectionSpec
	sampleFraction   float64
	strokeVisible    float64
	strokeThin       float64
	textSize         float64
	viewGap          float64
	dimensionOffset  float64
}

func resolveDrawingOptions(options []DrawingOption) drawingOptions {
	o := drawingOptions{
		scale:            1,
		hiddenLines:      true,
		featureAngle:     30,
		dimensions:       true,
		detailDimensions: true,
		sampleFraction:   1.0 / 2000,
		strokeVisible:    0.35,
		strokeThin:       0.18,
		textSize:         3.5,
		viewGap:          12,
		dimensionOffset:  8,
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

// WithoutDimensions leaves out all dimensions.
func WithoutDimensions() DrawingOption { return func(o *drawingOptions) { o.dimensions = false } }

// WithoutDetailDimensions keeps only the overall dimensions: no diameters, radii, hole positions
// or heights.
func WithoutDetailDimensions() DrawingOption {
	return func(o *drawingOptions) { o.detailDimensions = false }
}

// FeatureAngle sets how sharply two faces must meet, in degrees, for their shared edge to be
// drawn (default 30). Smaller values also draw the facets of gently curved surfaces.
func FeatureAngle(degrees float64) DrawingOption {
	return func(o *drawingOptions) { o.featureAngle = degrees }
}

// SectionX adds a section view cut by the plane x = offset, seen from +X (like the right view).
// It is marked on the top view.
func SectionX(offset float64) DrawingOption { return addSection(0, offset) }

// SectionY adds a section view cut by the plane y = offset, seen from -Y (like the front view).
// It is marked on the top view.
func SectionY(offset float64) DrawingOption { return addSection(1, offset) }

// SectionZ adds a section view cut by the plane z = offset, seen from above (like the top view).
// It is marked on the front view.
func SectionZ(offset float64) DrawingOption { return addSection(2, offset) }

func addSection(axis int, offset float64) DrawingOption {
	return func(o *drawingOptions) { o.sections = append(o.sections, sectionSpec{axis: axis, offset: offset}) }
}

// WriteSVG writes a drawing of the mesh as SVG: front, top and right views in third-angle
// projection, an isometric view and any requested section views, with dimensions in millimeters.
func (m *Mesh) WriteSVG(w io.Writer, options ...DrawingOption) error {
	if m == nil || len(m.Triangles) == 0 {
		return errors.New("tsumiki: cannot draw an empty mesh")
	}
	o := resolveDrawingOptions(options)
	if o.scale <= 0 || math.IsNaN(o.scale) || math.IsInf(o.scale, 0) {
		return fmt.Errorf("tsumiki: invalid drawing scale %g", o.scale)
	}
	set := newEdgeSet(m)
	front := newPanel(set, ViewFront.camera(), "FRONT", o, o.hiddenLines)
	top := newPanel(set, ViewTop.camera(), "TOP", o, o.hiddenLines)
	right := newPanel(set, ViewRight.camera(), "RIGHT", o, o.hiddenLines)
	iso := newPanel(set, ViewIsometric.camera(), "ISOMETRIC", o, false)
	sections, err := makeSections(m, o, top, front)
	if err != nil {
		return err
	}
	if o.dimensions {
		front.addOverallDimensions(o, true, true)
		top.addOverallDimensions(o, false, true) // the right view's width is the same Y extent
		if o.detailDimensions {
			front.addDetailDimensions(o, true)
			top.addDetailDimensions(o, false)
			right.addDetailDimensions(o, false)
			for _, section := range sections {
				section.addDetailDimensions(o, section.camera.up.Z > 0.5)
			}
		}
	}
	sheet := layOut([2][2]*panel{{top, iso}, {front, right}}, sections, o)
	bufferedWriter := bufio.NewWriter(w)
	sheet.write(bufferedWriter)
	return bufferedWriter.Flush()
}

// SaveSVG writes the drawing made by WriteSVG to path.
func (m *Mesh) SaveSVG(path string, options ...DrawingOption) error {
	return saveFile(path, func(w io.Writer) error { return m.WriteSVG(w, options...) })
}
