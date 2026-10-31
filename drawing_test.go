package tsumiki_test

import (
	"bytes"
	"encoding/xml"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/nasa9084/tsumiki"
)

// svgDocument is the part of an SVG drawing the tests look at.
type svgDocument struct {
	Width  string    `xml:"width,attr"`
	Height string    `xml:"height,attr"`
	Paths  []svgPath `xml:"path"`
	Texts  []string  `xml:"text"`
}

type svgPath struct {
	Fill      string `xml:"fill,attr"`
	Dasharray string `xml:"stroke-dasharray,attr"`
	Linecap   string `xml:"stroke-linecap,attr"`
	D         string `xml:"d,attr"`
}

// lineCount counts the separate lines ("M" commands) of the part's hidden (dashed) or visible
// (solid, round-capped) edges, leaving out dimension lines.
func (d svgDocument) lineCount(dashed bool) int {
	count := 0
	for _, path := range d.Paths {
		if (dashed && path.Dasharray != "") || (!dashed && path.Dasharray == "" && path.Linecap == "round") {
			count += strings.Count(path.D, "M")
		}
	}
	return count
}

func (d svgDocument) hasText(text string) bool {
	for _, t := range d.Texts {
		if strings.TrimSpace(t) == text {
			return true
		}
	}
	return false
}

func (d svgDocument) widthMillimeters(t *testing.T) float64 {
	t.Helper()
	width, err := strconv.ParseFloat(strings.TrimSuffix(d.Width, "mm"), 64)
	if err != nil {
		t.Fatalf("width %q: %v", d.Width, err)
	}
	return width
}

func drawing(t *testing.T, part *tsumiki.Part, options ...tsumiki.DrawingOption) svgDocument {
	t.Helper()
	mustWatertight(t, "drawn part", part)
	var buffer bytes.Buffer
	if err := part.Mesh().WriteSVG(&buffer, options...); err != nil {
		t.Fatal(err)
	}
	var document svgDocument
	if err := xml.Unmarshal(buffer.Bytes(), &document); err != nil {
		t.Fatalf("the drawing is not valid XML: %v", err)
	}
	return document
}

// pocketedBlock is a block with a blind pocket cut into its bottom face: from the top, the pocket
// can only be shown with hidden lines.
func pocketedBlock() *tsumiki.Part {
	part := box(0, 0, 0, 20, 10, 6)
	return part.Extrude(tsumiki.NewSketch(tsumiki.XY(0)).Rect(10, 5, 8, 4), 3, tsumiki.Cut())
}

func TestDrawingLabelsEveryView(t *testing.T) {
	document := drawing(t, box(0, 0, 0, 2, 3, 4))
	for _, label := range []string{"FRONT", "TOP", "RIGHT", "ISOMETRIC"} {
		if !document.hasText(label) {
			t.Errorf("no %s view label in %q", label, document.Texts)
		}
	}
}

func TestDrawingShowsOverallDimensions(t *testing.T) {
	document := drawing(t, box(0, 0, 0, 20, 12.5, 6))
	for _, value := range []string{"20", "12.5", "6"} {
		if !document.hasText(value) {
			t.Errorf("dimension %s missing from %q", value, document.Texts)
		}
	}
	plain := drawing(t, box(0, 0, 0, 20, 12.5, 6), tsumiki.WithoutDimensions())
	for _, value := range []string{"20", "12.5", "6"} {
		if plain.hasText(value) {
			t.Errorf("dimension %s drawn although dimensions are off", value)
		}
	}
}

func TestDrawingOfABoxHasNoHiddenLines(t *testing.T) {
	// every back edge of a box lies exactly behind a visible edge, so nothing is dashed
	document := drawing(t, box(0, 0, 0, 2, 3, 4))
	if n := document.lineCount(true); n != 0 {
		t.Errorf("got %d hidden lines", n)
	}
	// front, top and right show 4 outline edges each; the isometric view shows 9
	if n := document.lineCount(false); n < 4*3+9 {
		t.Errorf("got %d visible lines, want at least %d", n, 4*3+9)
	}
}

func TestDrawingShowsHiddenFeaturesDashed(t *testing.T) {
	document := drawing(t, pocketedBlock())
	if n := document.lineCount(true); n == 0 {
		t.Error("a pocket in the bottom face must show as hidden lines from the top and the sides")
	}
	if n := drawing(t, pocketedBlock(), tsumiki.WithoutHiddenLines()).lineCount(true); n != 0 {
		t.Errorf("got %d hidden lines although hidden lines are off", n)
	}
}

func TestDrawingScaleEnlargesTheSheet(t *testing.T) {
	small := drawing(t, box(0, 0, 0, 20, 10, 5)).widthMillimeters(t)
	large := drawing(t, box(0, 0, 0, 20, 10, 5), tsumiki.DrawingScale(2)).widthMillimeters(t)
	if large <= small {
		t.Errorf("a 2:1 sheet (%g mm) must be wider than a 1:1 sheet (%g mm)", large, small)
	}
	if !drawing(t, box(0, 0, 0, 20, 10, 5), tsumiki.DrawingScale(2)).hasText("20") {
		t.Error("dimensions show model millimeters whatever the scale")
	}
}

func TestDrawingFeatureAngleControlsCurvedSurfaceEdges(t *testing.T) {
	cylinder := func() *tsumiki.Part {
		return tsumiki.NewPart("cylinder").Extrude(tsumiki.NewSketch(tsumiki.XY(0)).Circle(0, 0, 10), 10)
	}
	smooth := drawing(t, cylinder()).lineCount(false)
	faceted := drawing(t, cylinder(), tsumiki.FeatureAngle(0.5)).lineCount(false)
	if faceted <= smooth {
		t.Errorf("a small feature angle must also draw the facets: %d lines vs %d", faceted, smooth)
	}
}

func TestDrawingTitleIsEscaped(t *testing.T) {
	document := drawing(t, box(0, 0, 0, 1, 1, 1), tsumiki.DrawingTitle(`Bracket <rev "A"> & co`))
	if !document.hasText(`Bracket <rev "A"> & co`) {
		t.Errorf("title missing from %q", document.Texts)
	}
}

func TestDrawingRejectsEmptyMeshAndBadScale(t *testing.T) {
	var buffer bytes.Buffer
	if err := (&tsumiki.Mesh{}).WriteSVG(&buffer); err == nil {
		t.Error("drawing an empty mesh must fail")
	}
	var missing *tsumiki.Mesh
	if err := missing.WriteSVG(&buffer); err == nil {
		t.Error("drawing a nil mesh (an empty part) must fail")
	}
	for _, scale := range []float64{0, -1} {
		if err := box(0, 0, 0, 1, 1, 1).Mesh().WriteSVG(&buffer, tsumiki.DrawingScale(scale)); err == nil {
			t.Errorf("scale %g must be rejected", scale)
		}
	}
}

func TestSaveSVG(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cube.svg")
	if err := box(0, 0, 0, 1, 1, 1).Mesh().SaveSVG(path); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() == 0 {
		t.Errorf("%s: %v, %v", path, info, err)
	}
	missing := filepath.Join(t.TempDir(), "no-such-directory", "cube.svg")
	if err := box(0, 0, 0, 1, 1, 1).Mesh().SaveSVG(missing); err == nil {
		t.Error("SaveSVG into a missing directory should fail")
	}
}

func TestViewString(t *testing.T) {
	names := map[tsumiki.View]string{
		tsumiki.ViewFront:     "front",
		tsumiki.ViewTop:       "top",
		tsumiki.ViewRight:     "right",
		tsumiki.ViewIsometric: "isometric",
	}
	for view, want := range names {
		if got := view.String(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}
