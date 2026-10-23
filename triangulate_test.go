package tsumiki_test

import (
	"math"
	"testing"

	"github.com/nasa9084/tsumiki"
)

// triangulatedArea sums the triangle areas and fails the test on any clockwise triangle.
func triangulatedArea(t *testing.T, points []tsumiki.Vector2, triangles [][3]int) float64 {
	t.Helper()
	total := 0.0
	for _, triangle := range triangles {
		area := tsumiki.TwiceSignedArea(points[triangle[0]], points[triangle[1]], points[triangle[2]]) / 2
		if area <= 0 {
			t.Errorf("triangle %v is clockwise or degenerate (area %g)", triangle, area)
		}
		total += area
	}
	return total
}

func TestTriangulateSquare(t *testing.T) {
	points, triangles := tsumiki.TriangulateRegion(tsumiki.Loop{{0, 0}, {4, 0}, {4, 4}, {0, 4}}, nil)
	if len(triangles) != 2 {
		t.Errorf("a square needs 2 triangles, got %d", len(triangles))
	}
	near(t, "area", triangulatedArea(t, points, triangles), 16, 1e-12)
}

func TestTriangulateConcave(t *testing.T) {
	// an L shape: area 3 * 3 - 1 * 1... 12 for a 4x4 square minus a 2x2 corner
	l := tsumiki.Loop{{0, 0}, {4, 0}, {4, 2}, {2, 2}, {2, 4}, {0, 4}}
	points, triangles := tsumiki.TriangulateRegion(l, nil)
	if len(triangles) != len(l)-2 {
		t.Errorf("a simple polygon with %d vertices needs %d triangles, got %d", len(l), len(l)-2, len(triangles))
	}
	near(t, "area", triangulatedArea(t, points, triangles), 12, 1e-12)
}

func TestTriangulateWithHole(t *testing.T) {
	outer := tsumiki.Loop{{0, 0}, {10, 0}, {10, 10}, {0, 10}}
	hole := tsumiki.Loop{{3, 3}, {3, 7}, {7, 7}, {7, 3}} // clockwise
	points, triangles := tsumiki.TriangulateRegion(outer, []tsumiki.Loop{hole})
	near(t, "area", triangulatedArea(t, points, triangles), 100-16, 1e-9)
}

func TestTriangulateWithSeveralHoles(t *testing.T) {
	outer := tsumiki.Loop{{0, 0}, {20, 0}, {20, 10}, {0, 10}}
	holes := []tsumiki.Loop{
		{{2, 2}, {2, 8}, {6, 8}, {6, 2}},
		{{9, 2}, {9, 8}, {13, 8}, {13, 2}},
		{{15, 2}, {15, 8}, {18, 8}, {18, 2}},
	}
	points, triangles := tsumiki.TriangulateRegion(outer, holes)
	near(t, "area", triangulatedArea(t, points, triangles), 200-24-24-18, 1e-9)
}

func TestTriangulateCircleWithOffCentreHole(t *testing.T) {
	region := tsumiki.NewSketch(tsumiki.XY(0)).Circle(0, 0, 30).Circle(8, 3, 5).Regions()[0]
	points, triangles := tsumiki.TriangulateRegion(region.Outer, region.Holes)
	want := region.Outer.Area() + region.Holes[0].Area()
	near(t, "area", triangulatedArea(t, points, triangles), want, 1e-6)
}

func TestInTriangleInclusive(t *testing.T) {
	a, b, c := v2(0, 0), v2(4, 0), v2(0, 4)
	for _, inside := range []tsumiki.Vector2{{1, 1}, {0, 0}, {2, 0}, {2, 2}} {
		if !tsumiki.InTriangleInclusive(a, b, c, inside) {
			t.Errorf("%v should count as inside (boundary included)", inside)
		}
	}
	for _, outside := range []tsumiki.Vector2{{3, 3}, {-0.1, 1}, {1, -0.1}} {
		if tsumiki.InTriangleInclusive(a, b, c, outside) {
			t.Errorf("%v should be outside", outside)
		}
	}
	// independent of the winding of the triangle
	if !tsumiki.InTriangleInclusive(a, c, b, v2(1, 1)) {
		t.Error("clockwise triangle should give the same answer")
	}
}

func TestTwiceSignedArea(t *testing.T) {
	near(t, "ccw", tsumiki.TwiceSignedArea(v2(0, 0), v2(2, 0), v2(0, 2)), 4, 0)
	near(t, "cw", tsumiki.TwiceSignedArea(v2(0, 0), v2(0, 2), v2(2, 0)), -4, 0)
	near(t, "collinear", tsumiki.TwiceSignedArea(v2(0, 0), v2(1, 1), v2(2, 2)), 0, 0)
}

func TestTriangulationAreaMatchesExtrudedVolume(t *testing.T) {
	part := tsumiki.NewPart("check").Extrude(
		tsumiki.NewSketch(tsumiki.XY(0)).RoundedRect(0, 0, 30, 20, 4, 15).Circle(3, 2, 2), 2)
	region := tsumiki.NewSketch(tsumiki.XY(0)).RoundedRect(0, 0, 30, 20, 4, 15).Circle(3, 2, 2).Regions()[0]
	area := region.Outer.Area() + region.Holes[0].Area()
	if math.Abs(part.Report().Volume-area*2) > 0.05 {
		t.Errorf("volume %g, want %g", part.Report().Volume, area*2)
	}
}

// Holes whose vertices share coordinates (a grid) make the bridge rays run along hole edges.
// This used to pick a vertex beyond the hole and produce overlapping triangles.
func TestTriangulateAlignedGridOfHoles(t *testing.T) {
	outer := tsumiki.Loop{{0, 0}, {40, 0}, {40, 40}, {0, 40}}
	var holes []tsumiki.Loop
	for column := range 4 {
		for row := range 4 {
			x, y := 4+float64(column)*9, 4+float64(row)*9
			holes = append(holes, tsumiki.Loop{{x, y}, {x, y + 5}, {x + 5, y + 5}, {x + 5, y}}) // clockwise
		}
	}
	points, triangles := tsumiki.TriangulateRegion(outer, holes)
	near(t, "area", triangulatedArea(t, points, triangles), 1600-16*25, 1e-9)
}
