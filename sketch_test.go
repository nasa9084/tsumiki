package tsumiki_test

import (
	"math"
	"testing"

	"github.com/nasa9084/tsumiki"
)

func TestPlanesAreRightHanded(t *testing.T) {
	for name, plane := range map[string]tsumiki.Plane{"XY": tsumiki.XY(2), "XZ": tsumiki.XZ(2), "YZ": tsumiki.YZ(2)} {
		if got := plane.U.Cross(plane.V); got != plane.Normal {
			t.Errorf("%s: U x V = %v, want Normal %v", name, got, plane.Normal)
		}
	}
}

func TestPlanePointMapping(t *testing.T) {
	tests := []struct {
		name  string
		plane tsumiki.Plane
		want  tsumiki.Vector3
	}{
		{"XY", tsumiki.XY(5), v3(1, 2, 5+3)},
		{"XZ", tsumiki.XZ(5), v3(1, 5-3, 2)}, // normal is -Y
		{"YZ", tsumiki.YZ(5), v3(5+3, 1, 2)},
	}
	for _, test := range tests {
		if got := test.plane.Point(1, 2, 3); got != test.want {
			t.Errorf("%s.Point(1,2,3) = %v, want %v", test.name, got, test.want)
		}
	}
}

func TestLoopAreaSign(t *testing.T) {
	counterClockwise := tsumiki.Loop{{0, 0}, {2, 0}, {2, 3}, {0, 3}}
	clockwise := tsumiki.Loop{{0, 0}, {0, 3}, {2, 3}, {2, 0}}
	near(t, "ccw", counterClockwise.Area(), 6, 1e-12)
	near(t, "cw", clockwise.Area(), -6, 1e-12)
}

// bounds returns the bounding box of the sketch extruded by one millimeter.
func bounds(t *testing.T, sketch *tsumiki.Sketch) (lo, hi tsumiki.Vector3) {
	t.Helper()
	part := tsumiki.NewPart("bounds").Extrude(sketch, 1)
	mustWatertight(t, "bounds", part)
	return part.Mesh().Bounds()
}

func TestSketchNestingAlternatesMaterialAndHole(t *testing.T) {
	// outer square, a hole inside it, and an island inside the hole
	sketch := tsumiki.NewSketch(tsumiki.XY(0)).Rect(0, 0, 20, 20).Rect(0, 0, 10, 10).Rect(0, 0, 4, 4)
	near(t, "frame plus island", profileArea(t, "nested", sketch), 400-100+16, 1e-9)
}

func TestSketchOrientationDoesNotMatter(t *testing.T) {
	clockwise := tsumiki.NewSketch(tsumiki.XY(0)).Polygon(v2(0, 0), v2(0, 5), v2(5, 5), v2(5, 0))
	counterClockwise := tsumiki.NewSketch(tsumiki.XY(0)).Polygon(v2(0, 0), v2(5, 0), v2(5, 5), v2(0, 5))
	near(t, "clockwise", profileArea(t, "cw", clockwise), 25, 1e-9)
	near(t, "counter-clockwise", profileArea(t, "ccw", counterClockwise), 25, 1e-9)
}

func TestSketchWithoutAreaCannotBeExtruded(t *testing.T) {
	sketch := tsumiki.NewSketch(tsumiki.XY(0)).Polygon(v2(0, 0), v2(1, 1)).Polygon(v2(0, 0), v2(1, 1), v2(2, 2))
	if part := tsumiki.NewPart("flat").Extrude(sketch, 1); part.Err() == nil {
		t.Error("a line and a collinear polygon enclose no area, extruding them must fail")
	}
}

func TestCircleStaysWithinChordTolerance(t *testing.T) {
	const radius = 30.0
	volume := profileArea(t, "circle", tsumiki.NewSketch(tsumiki.XY(0)).Circle(1, 2, radius))
	exact := math.Pi * radius * radius
	if volume > exact+0.2 { // 0.2: the 1 µm snapping of vertices
		t.Errorf("a polygon inscribed in the circle cannot be larger: %g > %g", volume, exact)
	}
	if lowest := exact - 2*math.Pi*radius*tsumiki.DefaultTolerance; volume < lowest {
		t.Errorf("area %g is further from the circle than the tolerance allows (%g)", volume, lowest)
	}
	lo, hi := bounds(t, tsumiki.NewSketch(tsumiki.XY(0)).Circle(1, 2, radius))
	near(t, "extent in x", hi.X-lo.X, 2*radius, 2*tsumiki.DefaultTolerance)
	near(t, "center x", (hi.X+lo.X)/2, 1, 0.01)
	near(t, "center y", (hi.Y+lo.Y)/2, 2, 0.01)
}

func TestFinerToleranceMakesSmootherCircles(t *testing.T) {
	triangles := func(tolerance float64) int {
		sketch := tsumiki.NewSketch(tsumiki.XY(0))
		sketch.Tolerance = tolerance
		sketch.Circle(0, 0, 20)
		return len(tsumiki.NewPart("circle").Extrude(sketch, 1).Mesh().Triangles)
	}
	if coarse, fine := triangles(0.1), triangles(0.001); fine <= coarse {
		t.Errorf("a finer tolerance must need more triangles: coarse=%d fine=%d", coarse, fine)
	}
}

func TestSmallCircleStillExtrudes(t *testing.T) {
	part := tsumiki.NewPart("pin").Extrude(tsumiki.NewSketch(tsumiki.XY(0)).Circle(0, 0, 0.05), 1)
	r := mustWatertight(t, "pin", part)
	if r.Triangles < 8 {
		t.Errorf("a small circle must still be a recognizable polygon, got %d triangles", r.Triangles)
	}
}

func TestRectDimensions(t *testing.T) {
	sketch := tsumiki.NewSketch(tsumiki.XY(0)).Rect(5, -3, 4, 6)
	lo, hi := bounds(t, sketch)
	if lo.X != 3 || lo.Y != -6 || hi.X != 7 || hi.Y != 0 {
		t.Errorf("bounds %v..%v, want x 3..7, y -6..0", lo, hi)
	}
	near(t, "area", profileArea(t, "rect", sketch), 24, 1e-12)
}

func TestRoundedRectArea(t *testing.T) {
	const width, height, radius = 20.0, 12.0, 3.0
	sketch := tsumiki.NewSketch(tsumiki.XY(0)).RoundedRect(0, 0, width, height, radius, 0)
	want := width*height - (4-math.Pi)*radius*radius
	near(t, "area", profileArea(t, "rounded rect", sketch), want, 0.2) // inscribed arcs lose a little
}

func TestRoundedRectRadiusIsClamped(t *testing.T) {
	// a radius larger than half the short side must not self-intersect: the result is a stadium
	sketch := tsumiki.NewSketch(tsumiki.XY(0)).RoundedRect(0, 0, 20, 4, 50, 0)
	near(t, "stadium area", profileArea(t, "stadium", sketch), 20*4-(4-math.Pi)*2*2, 0.15)
}

func TestRoundedRectRotation(t *testing.T) {
	lo, hi := bounds(t, tsumiki.NewSketch(tsumiki.XY(0)).RoundedRect(0, 0, 20, 10, 0, 90))
	near(t, "half width after a quarter turn", hi.X, 5, 1e-9)
	near(t, "half height after a quarter turn", hi.Y, 10, 1e-9)
	near(t, "symmetric x", lo.X, -5, 1e-9)
	near(t, "symmetric y", lo.Y, -10, 1e-9)
}

func TestSketchTranslateAndRotate(t *testing.T) {
	sketch := tsumiki.NewSketch(tsumiki.XY(0)).Polygon(v2(0, 0), v2(1, 0), v2(1, 1), v2(0, 1))
	sketch.Translate(2, 3)
	lo, hi := bounds(t, sketch)
	if lo.X != 2 || hi.X != 3 || lo.Y != 3 || hi.Y != 4 {
		t.Errorf("translated bounds %v..%v, want x 2..3, y 3..4", lo, hi)
	}
	sketch.Rotate(90) // counter-clockwise about the origin: (x, y) -> (-y, x)
	lo, hi = bounds(t, sketch)
	near(t, "x min", lo.X, -4, 1e-9)
	near(t, "x max", hi.X, -3, 1e-9)
	near(t, "y min", lo.Y, 2, 1e-9)
	near(t, "y max", hi.Y, 3, 1e-9)
}

func TestPathLines(t *testing.T) {
	sketch := tsumiki.NewSketch(tsumiki.XY(0))
	sketch.Path(0, 0).LineTo(4, 0).LineTo(4, 3).Close()
	near(t, "triangle area", profileArea(t, "triangle", sketch), 6, 1e-9)
}

func TestPathArcDirection(t *testing.T) {
	// the same end points around the same center: the two sweeps are complementary
	counterClockwise, clockwise := tsumiki.NewSketch(tsumiki.XY(0)), tsumiki.NewSketch(tsumiki.XY(0))
	counterClockwise.Path(10, 0).ArcTo(0, 10, 0, 0, true).LineTo(0, 0).Close()
	clockwise.Path(10, 0).ArcTo(0, 10, 0, 0, false).LineTo(0, 0).Close()
	near(t, "quarter disc", profileArea(t, "quarter", counterClockwise), math.Pi*100/4, 0.2)
	near(t, "three-quarter disc", profileArea(t, "three quarters", clockwise), math.Pi*100*3/4, 0.5)
}

func TestPathCollinearAndRepeatedPointsAreHarmless(t *testing.T) {
	sketch := tsumiki.NewSketch(tsumiki.XY(0))
	sketch.Path(0, 0).LineTo(0, 0).LineTo(1, 0).LineTo(2, 0).LineTo(2, 2).LineTo(0, 2).LineTo(0, 0).Close()
	near(t, "area", profileArea(t, "square", sketch), 4, 1e-9)
}
