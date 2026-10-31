package tsumiki_test

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/nasa9084/tsumiki"
)

func (d svgDocument) countText(text string) int {
	count := 0
	for _, t := range d.Texts {
		if strings.TrimSpace(t) == text {
			count++
		}
	}
	return count
}

func (d svgDocument) hatched() bool {
	for _, path := range d.Paths {
		if path.Fill == "url(#hatch)" {
			return true
		}
	}
	return false
}

func (d svgDocument) hasTextStartingWith(prefix string) bool {
	for _, t := range d.Texts {
		if strings.HasPrefix(strings.TrimSpace(t), prefix) {
			return true
		}
	}
	return false
}

// plateWithHoles is a 30×20×4 plate with two ⌀4 holes and one ⌀6 hole.
func plateWithHoles() *tsumiki.Part {
	plate := box(0, 0, 0, 30, 20, 4)
	holes := tsumiki.NewSketch(tsumiki.XY(0)).Circle(6, 5, 2).Circle(6, 15, 2).Circle(20, 10, 3)
	return plate.Extrude(holes, 4, tsumiki.Cut())
}

func TestDrawingDimensionsHoleSizes(t *testing.T) {
	document := drawing(t, plateWithHoles())
	if !document.hasText("2× ⌀4") {
		t.Errorf("two equal holes are labeled once with their count: %q", document.Texts)
	}
	if !document.hasText("⌀6") {
		t.Errorf("the ⌀6 hole is not labeled: %q", document.Texts)
	}
}

func TestDrawingDimensionsHolePositions(t *testing.T) {
	document := drawing(t, plateWithHoles())
	// measured from the lower left corner of the top view: x = 6 and 20, y = 5, 10 and 15
	for _, position := range []string{"6", "20", "5", "10", "15"} {
		if !document.hasText(position) {
			t.Errorf("hole position %s missing from %q", position, document.Texts)
		}
	}
}

func TestDrawingDimensionsCornerRadii(t *testing.T) {
	part := tsumiki.NewPart("rounded").Extrude(tsumiki.NewSketch(tsumiki.XY(0)).RoundedRect(0, 0, 40, 20, 5, 0), 3)
	document := drawing(t, part)
	if !document.hasText("4× R5") {
		t.Errorf("four equal corner radii are labeled once with their count: %q", document.Texts)
	}
}

func TestDrawingDimensionsHeights(t *testing.T) {
	// a step: 10 high on the left half, 4 high on the right half
	part := box(0, 0, 0, 20, 10, 4).Combine(tsumiki.OperationJoin, box(0, 0, 0, 10, 10, 10))
	document := drawing(t, part)
	for _, height := range []string{"0", "4", "10"} {
		if !document.hasText(height) {
			t.Errorf("height %s missing from %q", height, document.Texts)
		}
	}
}

func TestDrawingWithoutDetailDimensionsKeepsOverallOnes(t *testing.T) {
	document := drawing(t, plateWithHoles(), tsumiki.WithoutDetailDimensions())
	if document.hasTextStartingWith("⌀") || document.hasTextStartingWith("2×") {
		t.Errorf("hole sizes drawn although detail dimensions are off: %q", document.Texts)
	}
	for _, value := range []string{"30", "20", "4"} {
		if !document.hasText(value) {
			t.Errorf("overall dimension %s missing from %q", value, document.Texts)
		}
	}
}

func TestDrawingSectionView(t *testing.T) {
	document := drawing(t, pocketedBlock(), tsumiki.SectionY(5))
	if !document.hasText("SECTION A-A") {
		t.Errorf("no section view label in %q", document.Texts)
	}
	if !document.hatched() {
		t.Error("the cut face of a section is hatched")
	}
	if n := document.countText("A"); n != 2 {
		t.Errorf("the cutting plane is marked with its letter at both ends, got %d", n)
	}
	// the section shows the depth of the pocket, which the outside views only show dashed
	if !document.hasText("3") {
		t.Errorf("the pocket's height is missing from the section: %q", document.Texts)
	}
}

func TestDrawingSectionsAreLetteredInOrder(t *testing.T) {
	document := drawing(t, pocketedBlock(), tsumiki.SectionX(10), tsumiki.SectionY(5), tsumiki.SectionZ(1))
	for _, label := range []string{"SECTION A-A", "SECTION B-B", "SECTION C-C"} {
		if !document.hasText(label) {
			t.Errorf("%s missing from %q", label, document.Texts)
		}
	}
}

func TestDrawingWithoutSectionsHasNoHatching(t *testing.T) {
	if drawing(t, pocketedBlock()).hatched() {
		t.Error("only section views are hatched")
	}
}

func TestDrawingRejectsSectionThatMissesThePart(t *testing.T) {
	var buffer bytes.Buffer
	err := pocketedBlock().Mesh().WriteSVG(&buffer, tsumiki.SectionZ(50))
	if err == nil || !strings.Contains(err.Error(), "section A-A") {
		t.Errorf("a cutting plane above the part must be reported, got %v", err)
	}
}

// hexagon is a regular hexagon with the given distance across flats, flats at top and bottom.
func hexagon(sketch *tsumiki.Sketch, centerX, centerY, acrossFlats float64) *tsumiki.Sketch {
	radius := acrossFlats / math.Sqrt(3)
	var corners []tsumiki.Vector2
	for k := range 6 {
		angle := float64(k) * math.Pi / 3
		corners = append(corners, v2(centerX+radius*math.Cos(angle), centerY+radius*math.Sin(angle)))
	}
	return sketch.Polygon(corners...)
}

func TestDrawingDimensionsHexagonalHolesWithPitch(t *testing.T) {
	holes := tsumiki.NewSketch(tsumiki.XY(0))
	for _, x := range []float64{6, 10, 14, 18} {
		hexagon(holes, x, 10, 3.2)
	}
	part := box(0, 0, 0, 24, 20, 2).Extrude(holes, 2, tsumiki.Cut())
	if document := drawing(t, part); !document.hasText("4× HEX 3.2 A/F, pitch 4") {
		t.Errorf("four hexagonal holes 4 apart are labeled with size, count and pitch: %q", document.Texts)
	}
}

func TestDrawingDimensionsSquareHole(t *testing.T) {
	part := box(0, 0, 0, 20, 20, 3).Extrude(tsumiki.NewSketch(tsumiki.XY(0)).Rect(10, 10, 5, 5), 3, tsumiki.Cut())
	if document := drawing(t, part); !document.hasText("□5") {
		t.Errorf("a square hole is labeled with its side: %q", document.Texts)
	}
	if document := drawing(t, box(0, 0, 0, 20, 20, 3)); document.hasTextStartingWith("□") {
		t.Errorf("the outline of a square block is dimensioned overall, not as a square: %q", document.Texts)
	}
}

func TestDrawingDimensionsBlindHoleDepth(t *testing.T) {
	// a ⌀4 hole 2 deep from the top of a 5 high block, and a ⌀3 hole all the way through
	part := box(0, 0, 0, 20, 10, 5).
		Extrude(tsumiki.NewSketch(tsumiki.XY(3)).Circle(5, 5, 2), 2, tsumiki.Cut()).
		Extrude(tsumiki.NewSketch(tsumiki.XY(0)).Circle(15, 5, 1.5), 5, tsumiki.Cut())
	document := drawing(t, part)
	if !document.hasText("⌀4, depth 2") {
		t.Errorf("a blind hole is labeled with its depth: %q", document.Texts)
	}
	if !document.hasText("⌀3") {
		t.Errorf("a through hole has no depth: %q", document.Texts)
	}
}

func TestDrawingDimensionsThroughHoleAboveAnotherFace(t *testing.T) {
	// a shelf with a ⌀4 hole, 10 above a base: the base seen through the hole is no floor
	part := box(0, 0, 0, 30, 20, 3).
		Combine(tsumiki.OperationJoin, box(0, 0, 0, 4, 20, 13)).
		Combine(tsumiki.OperationJoin, box(0, 0, 10, 30, 20, 13)).
		Extrude(tsumiki.NewSketch(tsumiki.XY(10)).Circle(20, 10, 2), 3, tsumiki.Cut())
	document := drawing(t, part)
	if !document.hasText("⌀4") || document.hasTextStartingWith("⌀4, depth") {
		t.Errorf("the hole goes through the shelf: %q", document.Texts)
	}
}
