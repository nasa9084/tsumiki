package tsumiki

import (
	"math"
	"testing"
)

// triangulatedArea sums the triangle areas and fails the test on any clockwise triangle.
func triangulatedArea(t *testing.T, points []Vector2, triangles [][3]int) float64 {
	t.Helper()
	total := 0.0
	for _, triangle := range triangles {
		area := twiceSignedArea(points[triangle[0]], points[triangle[1]], points[triangle[2]]) / 2
		if area <= 0 {
			t.Errorf("triangle %v is clockwise or degenerate (area %g)", triangle, area)
		}
		total += area
	}
	return total
}

func TestTriangulateSquare(t *testing.T) {
	points, triangles := triangulateRegion(Loop{{0, 0}, {4, 0}, {4, 4}, {0, 4}}, nil)
	if len(triangles) != 2 {
		t.Errorf("a square needs 2 triangles, got %d", len(triangles))
	}
	near(t, "area", triangulatedArea(t, points, triangles), 16, 1e-12)
}

func TestTriangulateConcave(t *testing.T) {
	// an L shape: area 3 * 3 - 1 * 1... 12 for a 4x4 square minus a 2x2 corner
	l := Loop{{0, 0}, {4, 0}, {4, 2}, {2, 2}, {2, 4}, {0, 4}}
	points, triangles := triangulateRegion(l, nil)
	if len(triangles) != len(l)-2 {
		t.Errorf("a simple polygon with %d vertices needs %d triangles, got %d", len(l), len(l)-2, len(triangles))
	}
	near(t, "area", triangulatedArea(t, points, triangles), 12, 1e-12)
}

func TestTriangulateWithHole(t *testing.T) {
	outer := Loop{{0, 0}, {10, 0}, {10, 10}, {0, 10}}
	hole := Loop{{3, 3}, {3, 7}, {7, 7}, {7, 3}} // clockwise
	points, triangles := triangulateRegion(outer, []Loop{hole})
	near(t, "area", triangulatedArea(t, points, triangles), 100-16, 1e-9)
}

func TestTriangulateWithSeveralHoles(t *testing.T) {
	outer := Loop{{0, 0}, {20, 0}, {20, 10}, {0, 10}}
	holes := []Loop{
		{{2, 2}, {2, 8}, {6, 8}, {6, 2}},
		{{9, 2}, {9, 8}, {13, 8}, {13, 2}},
		{{15, 2}, {15, 8}, {18, 8}, {18, 2}},
	}
	points, triangles := triangulateRegion(outer, holes)
	near(t, "area", triangulatedArea(t, points, triangles), 200-24-24-18, 1e-9)
}

func TestTriangulateCircleWithOffCentreHole(t *testing.T) {
	region := NewSketch(XY(0)).Circle(0, 0, 30).Circle(8, 3, 5).regions()[0]
	points, triangles := triangulateRegion(region.Outer, region.Holes)
	want := region.Outer.Area() + region.Holes[0].Area()
	near(t, "area", triangulatedArea(t, points, triangles), want, 1e-6)
}

func TestInTriangleInclusive(t *testing.T) {
	a, b, c := Vector2{0, 0}, Vector2{4, 0}, Vector2{0, 4}
	for _, inside := range []Vector2{{1, 1}, {0, 0}, {2, 0}, {2, 2}} {
		if !inTriangleInclusive(a, b, c, inside) {
			t.Errorf("%v should count as inside (boundary included)", inside)
		}
	}
	for _, outside := range []Vector2{{3, 3}, {-0.1, 1}, {1, -0.1}} {
		if inTriangleInclusive(a, b, c, outside) {
			t.Errorf("%v should be outside", outside)
		}
	}
	// independent of the winding of the triangle
	if !inTriangleInclusive(a, c, b, Vector2{1, 1}) {
		t.Error("clockwise triangle should give the same answer")
	}
}

func TestTwiceSignedArea(t *testing.T) {
	near(t, "ccw", twiceSignedArea(Vector2{0, 0}, Vector2{2, 0}, Vector2{0, 2}), 4, 0)
	near(t, "cw", twiceSignedArea(Vector2{0, 0}, Vector2{0, 2}, Vector2{2, 0}), -4, 0)
	near(t, "collinear", twiceSignedArea(Vector2{0, 0}, Vector2{1, 1}, Vector2{2, 2}), 0, 0)
}

func TestTriangulationAreaMatchesExtrudedVolume(t *testing.T) {
	part := NewPart("check").Extrude(NewSketch(XY(0)).RoundedRect(0, 0, 30, 20, 4, 15).Circle(3, 2, 2), 2)
	region := NewSketch(XY(0)).RoundedRect(0, 0, 30, 20, 4, 15).Circle(3, 2, 2).regions()[0]
	area := region.Outer.Area() + region.Holes[0].Area()
	if math.Abs(part.Report().Volume-area*2) > 0.05 {
		t.Errorf("volume %g, want %g", part.Report().Volume, area*2)
	}
}

// Holes whose vertices share coordinates (a grid) make the bridge rays run along hole edges.
// This used to pick a vertex beyond the hole and produce overlapping triangles.
func TestTriangulateAlignedGridOfHoles(t *testing.T) {
	outer := Loop{{0, 0}, {40, 0}, {40, 40}, {0, 40}}
	var holes []Loop
	for column := range 4 {
		for row := range 4 {
			x, y := 4+float64(column)*9, 4+float64(row)*9
			holes = append(holes, Loop{{x, y}, {x, y + 5}, {x + 5, y + 5}, {x + 5, y}}) // clockwise
		}
	}
	points, triangles := triangulateRegion(outer, holes)
	near(t, "area", triangulatedArea(t, points, triangles), 1600-16*25, 1e-9)
}
