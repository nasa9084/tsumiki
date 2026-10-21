package tsumiki

import (
	"math"
	"testing"
)

func TestPlanesAreRightHanded(t *testing.T) {
	for name, plane := range map[string]Plane{"XY": XY(2), "XZ": XZ(2), "YZ": YZ(2)} {
		if got := plane.U.Cross(plane.V); got != plane.Normal {
			t.Errorf("%s: U x V = %v, want Normal %v", name, got, plane.Normal)
		}
	}
}

func TestPlanePointMapping(t *testing.T) {
	tests := []struct {
		name  string
		plane Plane
		want  Vector3
	}{
		{"XY", XY(5), Vector3{1, 2, 5 + 3}},
		{"XZ", XZ(5), Vector3{1, 5 - 3, 2}}, // normal is -Y
		{"YZ", YZ(5), Vector3{5 + 3, 1, 2}},
	}
	for _, test := range tests {
		if got := test.plane.Point(1, 2, 3); got != test.want {
			t.Errorf("%s.Point(1,2,3) = %v, want %v", test.name, got, test.want)
		}
	}
}

func TestLoopAreaSign(t *testing.T) {
	counterClockwise := Loop{{0, 0}, {2, 0}, {2, 3}, {0, 3}}
	near(t, "ccw", counterClockwise.Area(), 6, 1e-12)
	near(t, "cw", counterClockwise.reversed().Area(), -6, 1e-12)
}

func TestSketchRegionsNesting(t *testing.T) {
	// outer square, a hole inside it, and an island inside the hole
	sketch := NewSketch(XY(0)).Rect(0, 0, 20, 20).Rect(0, 0, 10, 10).Rect(0, 0, 4, 4)
	regions := sketch.regions()
	if len(regions) != 2 {
		t.Fatalf("got %d regions, want 2 (the frame and the island)", len(regions))
	}
	var frame, island *Region
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
	sketch := NewSketch(XY(0)).Polygon(Vector2{0, 0}, Vector2{0, 5}, Vector2{5, 5}, Vector2{5, 0})
	regions := sketch.regions()
	if len(regions) != 1 || regions[0].Outer.Area() <= 0 {
		t.Fatalf("outer loop is not counter-clockwise: %+v", regions)
	}
}

func TestSketchIgnoresDegenerateLoops(t *testing.T) {
	sketch := NewSketch(XY(0)).Polygon(Vector2{0, 0}, Vector2{1, 1}).Polygon(Vector2{0, 0}, Vector2{1, 1}, Vector2{2, 2})
	if regions := sketch.regions(); len(regions) != 0 {
		t.Errorf("a line and a collinear polygon have no area, got %d regions", len(regions))
	}
}

func TestCircleStaysWithinChordTolerance(t *testing.T) {
	const radius = 30.0
	sketch := NewSketch(XY(0)).Circle(1, 2, radius)
	loop := sketch.loops[0]
	for i, point := range loop {
		near(t, "vertex radius", point.Sub(Vector2{1, 2}).Length(), radius, 1e-9)
		next := loop[(i+1)%len(loop)]
		midpoint := point.Add(next).Scale(0.5)
		if sagitta := radius - midpoint.Sub(Vector2{1, 2}).Length(); sagitta > DefaultTolerance*1.0001 {
			t.Fatalf("chord %d sags %.5f mm, tolerance is %.5f", i, sagitta, DefaultTolerance)
		}
	}
}

func TestCircleHasAtLeastEightSegments(t *testing.T) {
	if n := len(NewSketch(XY(0)).Circle(0, 0, 0.001).loops[0]); n < 8 {
		t.Errorf("tiny circle has %d segments", n)
	}
}

func TestRectDimensions(t *testing.T) {
	regions := NewSketch(XY(0)).Rect(5, -3, 4, 6).regions()
	near(t, "area", regions[0].Outer.Area(), 24, 1e-12)
	min, max := Vector2{math.Inf(1), math.Inf(1)}, Vector2{math.Inf(-1), math.Inf(-1)}
	for _, point := range regions[0].Outer {
		min = Vector2{math.Min(min.X, point.X), math.Min(min.Y, point.Y)}
		max = Vector2{math.Max(max.X, point.X), math.Max(max.Y, point.Y)}
	}
	if min != (Vector2{3, -6}) || max != (Vector2{7, 0}) {
		t.Errorf("bounds %v..%v", min, max)
	}
}

func TestRoundedRectArea(t *testing.T) {
	const width, height, radius = 20.0, 12.0, 3.0
	area := NewSketch(XY(0)).RoundedRect(0, 0, width, height, radius, 0).regions()[0].Outer.Area()
	want := width*height - (4-math.Pi)*radius*radius
	near(t, "area", area, want, 0.2) // inscribed arcs lose about perimeter*tolerance*2/3
}

func TestRoundedRectRadiusIsClamped(t *testing.T) {
	// radius larger than half the short side must not self-intersect: it becomes a stadium
	regions := NewSketch(XY(0)).RoundedRect(0, 0, 20, 4, 50, 0).regions()
	if len(regions) != 1 {
		t.Fatalf("got %d regions", len(regions))
	}
	want := 20*4 - (4-math.Pi)*2*2
	near(t, "stadium area", regions[0].Outer.Area(), want, 0.15)
}

func TestRoundedRectRotation(t *testing.T) {
	loop := NewSketch(XY(0)).RoundedRect(0, 0, 20, 10, 0, 90).regions()[0].Outer
	var maxX, maxY float64
	for _, point := range loop {
		maxX, maxY = math.Max(maxX, point.X), math.Max(maxY, point.Y)
	}
	near(t, "half width after a quarter turn", maxX, 5, 1e-9)
	near(t, "half height after a quarter turn", maxY, 10, 1e-9)
}

func TestSketchTranslateAndRotate(t *testing.T) {
	sketch := NewSketch(XY(0)).Polygon(Vector2{0, 0}, Vector2{1, 0}, Vector2{1, 1}, Vector2{0, 1})
	sketch.Translate(2, 3)
	if got := sketch.loops[0][0]; got != (Vector2{2, 3}) {
		t.Errorf("translated first vertex = %v", got)
	}
	sketch.Rotate(90)
	got := sketch.loops[0][0]
	near(t, "rotated x", got.X, -3, 1e-12)
	near(t, "rotated y", got.Y, 2, 1e-12)
}

func TestPathLines(t *testing.T) {
	sketch := NewSketch(XY(0))
	sketch.Path(0, 0).LineTo(4, 0).LineTo(4, 3).Close()
	regions := sketch.regions()
	if len(regions) != 1 {
		t.Fatalf("got %d regions", len(regions))
	}
	near(t, "triangle area", regions[0].Outer.Area(), 6, 1e-12)
}

func TestPathArcDirection(t *testing.T) {
	// the same two end points around the same center: the two sweeps are complementary
	counterClockwise, clockwise := NewSketch(XY(0)), NewSketch(XY(0))
	counterClockwise.Path(10, 0).ArcTo(0, 10, 0, 0, true).LineTo(0, 0).Close()
	clockwise.Path(10, 0).ArcTo(0, 10, 0, 0, false).LineTo(0, 0).Close()
	quarter := counterClockwise.regions()[0].Outer.Area()
	threeQuarters := clockwise.regions()[0].Outer.Area()
	near(t, "quarter disc", quarter, math.Pi*100/4, 0.1)
	near(t, "three-quarter disc", threeQuarters, math.Pi*100*3/4, 0.5)
}

func TestArcSteps(t *testing.T) {
	if n := arcSteps(10, 0.0001, 0.01); n != 1 {
		t.Errorf("a tiny sweep needs one segment, got %d", n)
	}
	if n := arcSteps(10, 2*math.Pi, 0); n < 8 {
		t.Errorf("zero tolerance should fall back to the default, got %d", n)
	}
	coarse, fine := arcSteps(50, math.Pi, 0.1), arcSteps(50, math.Pi, 0.001)
	if fine <= coarse {
		t.Errorf("a finer tolerance must need more segments: coarse=%d fine=%d", coarse, fine)
	}
	if arcSteps(0.001, math.Pi, 0.01) < 1 {
		t.Error("a radius below the tolerance must still return at least one segment")
	}
}

func TestCleanLoop(t *testing.T) {
	loop := Loop{{0, 0}, {0, 0}, {1, 0}, {2, 0}, {2, 2}, {0, 2}, {0, 0}}
	got := cleanLoop(loop)
	if len(got) != 4 {
		t.Errorf("duplicate and collinear points must be removed, got %v", got)
	}
}

func TestPointInLoop(t *testing.T) {
	square := Loop{{0, 0}, {4, 0}, {4, 4}, {0, 4}}
	if !pointInLoop(Vector2{2, 2}, square) {
		t.Error("center should be inside")
	}
	if pointInLoop(Vector2{5, 2}, square) || pointInLoop(Vector2{-1, -1}, square) {
		t.Error("outside points reported inside")
	}
}
