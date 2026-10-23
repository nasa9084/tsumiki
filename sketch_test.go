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
	near(t, "ccw", counterClockwise.Area(), 6, 1e-12)
	near(t, "cw", counterClockwise.Reversed().Area(), -6, 1e-12)
}

func TestSketchRegionsNesting(t *testing.T) {
	// outer square, a hole inside it, and an island inside the hole
	sketch := tsumiki.NewSketch(tsumiki.XY(0)).Rect(0, 0, 20, 20).Rect(0, 0, 10, 10).Rect(0, 0, 4, 4)
	regions := sketch.Regions()
	if len(regions) != 2 {
		t.Fatalf("got %d regions, want 2 (the frame and the island)", len(regions))
	}
	var frame, island *tsumiki.Region
	for i := range regions {
		if len(regions[i].Holes) == 1 {
			frame = &regions[i]
		} else {
			island = &regions[i]
		}
	}
	if frame == nil || island == nil {
		t.Fatalf("expected one region with a hole and one without: %+v", regions)
	}
	near(t, "frame outer", frame.Outer.Area(), 400, 1e-9)
	near(t, "frame hole", frame.Holes[0].Area(), -100, 1e-9)
	near(t, "island", island.Outer.Area(), 16, 1e-9)
}

func TestSketchRegionsOrientation(t *testing.T) {
	// a clockwise outer loop must be normalised to counter-clockwise
	sketch := tsumiki.NewSketch(tsumiki.XY(0)).Polygon(v2(0, 0), v2(0, 5), v2(5, 5), v2(5, 0))
	regions := sketch.Regions()
	if len(regions) != 1 || regions[0].Outer.Area() <= 0 {
		t.Fatalf("outer loop is not counter-clockwise: %+v", regions)
	}
}

func TestSketchIgnoresDegenerateLoops(t *testing.T) {
	sketch := tsumiki.NewSketch(tsumiki.XY(0)).Polygon(v2(0, 0), v2(1, 1)).Polygon(v2(0, 0), v2(1, 1), v2(2, 2))
	if regions := sketch.Regions(); len(regions) != 0 {
		t.Errorf("a line and a collinear polygon have no area, got %d regions", len(regions))
	}
}

func TestCircleStaysWithinChordTolerance(t *testing.T) {
	const radius = 30.0
	sketch := tsumiki.NewSketch(tsumiki.XY(0)).Circle(1, 2, radius)
	loop := sketch.Loops()[0]
	for i, point := range loop {
		near(t, "vertex radius", point.Sub(v2(1, 2)).Length(), radius, 1e-9)
		next := loop[(i+1)%len(loop)]
		midpoint := point.Add(next).Scale(0.5)
		if sagitta := radius - midpoint.Sub(v2(1, 2)).Length(); sagitta > tsumiki.DefaultTolerance*1.0001 {
			t.Fatalf("chord %d sags %.5f mm, tolerance is %.5f", i, sagitta, tsumiki.DefaultTolerance)
		}
	}
}

func TestCircleHasAtLeastEightSegments(t *testing.T) {
	if n := len(tsumiki.NewSketch(tsumiki.XY(0)).Circle(0, 0, 0.001).Loops()[0]); n < 8 {
		t.Errorf("tiny circle has %d segments", n)
	}
}

func TestRectDimensions(t *testing.T) {
	regions := tsumiki.NewSketch(tsumiki.XY(0)).Rect(5, -3, 4, 6).Regions()
	near(t, "area", regions[0].Outer.Area(), 24, 1e-12)
	min, max := v2(math.Inf(1), math.Inf(1)), v2(math.Inf(-1), math.Inf(-1))
	for _, point := range regions[0].Outer {
		min = v2(math.Min(min.X, point.X), math.Min(min.Y, point.Y))
		max = v2(math.Max(max.X, point.X), math.Max(max.Y, point.Y))
	}
	if min != (v2(3, -6)) || max != (v2(7, 0)) {
		t.Errorf("bounds %v..%v", min, max)
	}
}

func TestRoundedRectArea(t *testing.T) {
	const width, height, radius = 20.0, 12.0, 3.0
	area := tsumiki.NewSketch(tsumiki.XY(0)).RoundedRect(0, 0, width, height, radius, 0).Regions()[0].Outer.Area()
	want := width*height - (4-math.Pi)*radius*radius
	near(t, "area", area, want, 0.2) // inscribed arcs lose about perimeter*tolerance*2/3
}

func TestRoundedRectRadiusIsClamped(t *testing.T) {
	// radius larger than half the short side must not self-intersect: it becomes a stadium
	regions := tsumiki.NewSketch(tsumiki.XY(0)).RoundedRect(0, 0, 20, 4, 50, 0).Regions()
	if len(regions) != 1 {
		t.Fatalf("got %d regions", len(regions))
	}
	want := 20*4 - (4-math.Pi)*2*2
	near(t, "stadium area", regions[0].Outer.Area(), want, 0.15)
}

func TestRoundedRectRotation(t *testing.T) {
	loop := tsumiki.NewSketch(tsumiki.XY(0)).RoundedRect(0, 0, 20, 10, 0, 90).Regions()[0].Outer
	var maxX, maxY float64
	for _, point := range loop {
		maxX, maxY = math.Max(maxX, point.X), math.Max(maxY, point.Y)
	}
	near(t, "half width after a quarter turn", maxX, 5, 1e-9)
	near(t, "half height after a quarter turn", maxY, 10, 1e-9)
}

func TestSketchTranslateAndRotate(t *testing.T) {
	sketch := tsumiki.NewSketch(tsumiki.XY(0)).Polygon(v2(0, 0), v2(1, 0), v2(1, 1), v2(0, 1))
	sketch.Translate(2, 3)
	if got := sketch.Loops()[0][0]; got != (v2(2, 3)) {
		t.Errorf("translated first vertex = %v", got)
	}
	sketch.Rotate(90)
	got := sketch.Loops()[0][0]
	near(t, "rotated x", got.X, -3, 1e-12)
	near(t, "rotated y", got.Y, 2, 1e-12)
}

func TestPathLines(t *testing.T) {
	sketch := tsumiki.NewSketch(tsumiki.XY(0))
	sketch.Path(0, 0).LineTo(4, 0).LineTo(4, 3).Close()
	regions := sketch.Regions()
	if len(regions) != 1 {
		t.Fatalf("got %d regions", len(regions))
	}
	near(t, "triangle area", regions[0].Outer.Area(), 6, 1e-12)
}

func TestPathArcDirection(t *testing.T) {
	// the same two end points around the same center: the two sweeps are complementary
	counterClockwise, clockwise := tsumiki.NewSketch(tsumiki.XY(0)), tsumiki.NewSketch(tsumiki.XY(0))
	counterClockwise.Path(10, 0).ArcTo(0, 10, 0, 0, true).LineTo(0, 0).Close()
	clockwise.Path(10, 0).ArcTo(0, 10, 0, 0, false).LineTo(0, 0).Close()
	quarter := counterClockwise.Regions()[0].Outer.Area()
	threeQuarters := clockwise.Regions()[0].Outer.Area()
	near(t, "quarter disc", quarter, math.Pi*100/4, 0.1)
	near(t, "three-quarter disc", threeQuarters, math.Pi*100*3/4, 0.5)
}

func TestArcSteps(t *testing.T) {
	if n := tsumiki.ArcSteps(10, 0.0001, 0.01); n != 1 {
		t.Errorf("a tiny sweep needs one segment, got %d", n)
	}
	if n := tsumiki.ArcSteps(10, 2*math.Pi, 0); n < 8 {
		t.Errorf("zero tolerance should fall back to the default, got %d", n)
	}
	coarse, fine := tsumiki.ArcSteps(50, math.Pi, 0.1), tsumiki.ArcSteps(50, math.Pi, 0.001)
	if fine <= coarse {
		t.Errorf("a finer tolerance must need more segments: coarse=%d fine=%d", coarse, fine)
	}
	if tsumiki.ArcSteps(0.001, math.Pi, 0.01) < 1 {
		t.Error("a radius below the tolerance must still return at least one segment")
	}
}

func TestCleanLoop(t *testing.T) {
	loop := tsumiki.Loop{{0, 0}, {0, 0}, {1, 0}, {2, 0}, {2, 2}, {0, 2}, {0, 0}}
	got := tsumiki.CleanLoop(loop)
	if len(got) != 4 {
		t.Errorf("duplicate and collinear points must be removed, got %v", got)
	}
}

func TestPointInLoop(t *testing.T) {
	square := tsumiki.Loop{{0, 0}, {4, 0}, {4, 4}, {0, 4}}
	if !tsumiki.PointInLoop(v2(2, 2), square) {
		t.Error("center should be inside")
	}
	if tsumiki.PointInLoop(v2(5, 2), square) || tsumiki.PointInLoop(v2(-1, -1), square) {
		t.Error("outside points reported inside")
	}
}
