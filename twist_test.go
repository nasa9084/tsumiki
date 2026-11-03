package tsumiki_test

import (
	"math"
	"strings"
	"testing"

	"github.com/nasa9084/tsumiki"
)

func TestTwistedExtrusionKeepsTheSectionAndTurnsIt(t *testing.T) {
	part := tsumiki.NewPart("twisted bar")
	part.Extrude(tsumiki.NewSketch(tsumiki.XY(0)).Rect(0, 0, 8, 2), 10, tsumiki.Twist(90))
	if err := part.Err(); err != nil {
		t.Fatal(err)
	}
	report := part.Report()
	if !report.Watertight() {
		t.Fatalf("not watertight: %+v", report)
	}
	// every layer is the same rectangle, so the volume is the area times the height
	if want := 8.0 * 2 * 10; math.Abs(report.Volume-want) > want*0.01 {
		t.Errorf("volume = %g, want about %g", report.Volume, want)
	}
	mesh := part.Mesh()
	for _, c := range []struct {
		point  tsumiki.Vector3
		inside bool
	}{
		{v3(3.5, 0, 0.1), true},  // the long side lies along x at the bottom
		{v3(0, 3.5, 0.1), false}, //
		{v3(0, 3.5, 9.9), true},  // and along y at the top (turned counterclockwise by 90°)
		{v3(3.5, 0, 9.9), false},
		{v3(2.5, 2.5, 5), true}, // halfway up it lies along the diagonal
	} {
		if got := mesh.Contains(c.point); got != c.inside {
			t.Errorf("Contains(%v) = %v, want %v", c.point, got, c.inside)
		}
	}
	if !strings.Contains(part.Timeline(), "twist 90°") {
		t.Errorf("timeline lacks the twist:\n%s", part.Timeline())
	}
}

func TestTwistedExtrusionIsARightHandHelix(t *testing.T) {
	// a small square away from the axis, turned once over 10 mm, climbs counterclockwise
	part := tsumiki.NewPart("helix")
	part.Extrude(tsumiki.NewSketch(tsumiki.XY(0)).Rect(5, 0, 1, 1), 10, tsumiki.Twist(360))
	mesh := part.Mesh()
	if !mesh.Contains(v3(0, 5, 2.5)) || mesh.Contains(v3(0, -5, 2.5)) {
		t.Error("a quarter of the way up the section should be at +y")
	}
	if !mesh.Contains(v3(0, -5, 7.5)) {
		t.Error("three quarters of the way up the section should be at -y")
	}
}

func TestTwistedExtrusionWithANegativeDistanceAndHoles(t *testing.T) {
	sketch := tsumiki.NewSketch(tsumiki.XY(0)).Rect(0, 0, 10, 10).Circle(0, 0, 2)
	part := tsumiki.NewPart("tube")
	part.Extrude(sketch, -4, tsumiki.Twist(45))
	report := part.Report()
	if !report.Watertight() {
		t.Fatalf("not watertight: %+v", report)
	}
	if want := (100 - math.Pi*4) * 4; math.Abs(report.Volume-want) > want*0.01 {
		t.Errorf("volume = %g, want about %g", report.Volume, want)
	}
	if part.Mesh().Contains(v3(0, 0, -2)) {
		t.Error("the hole should stay open")
	}
}

func TestTwistCannotBeTapered(t *testing.T) {
	part := tsumiki.NewPart("bad")
	part.Extrude(tsumiki.NewSketch(tsumiki.XY(0)).Rect(0, 0, 4, 4), 4, tsumiki.Twist(30), tsumiki.Taper(5))
	if part.Err() == nil {
		t.Error("a twisted, tapered extrusion should fail")
	}
}
