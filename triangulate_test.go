package tsumiki_test

import (
	"testing"

	"github.com/nasa9084/tsumiki"
)

// profileArea extrudes a sketch by one millimeter and returns the volume, which equals the area of
// the profile. It fails the test unless the result is a valid closed solid, so a bad
// triangulation of the profile (gaps, overlaps, flipped triangles) shows up here.
func profileArea(t *testing.T, name string, sketch *tsumiki.Sketch) float64 {
	t.Helper()
	return mustWatertight(t, name, tsumiki.NewPart(name).Extrude(sketch, 1)).Volume
}

func TestProfileSquare(t *testing.T) {
	near(t, "area", profileArea(t, "square", tsumiki.NewSketch(tsumiki.XY(0)).Rect(0, 0, 4, 4)), 16, 1e-9)
}

func TestProfileConcavePolygon(t *testing.T) {
	// an L shape: a 4x4 square minus a 2x2 corner
	l := tsumiki.NewSketch(tsumiki.XY(0)).Polygon(v2(0, 0), v2(4, 0), v2(4, 2), v2(2, 2), v2(2, 4), v2(0, 4))
	near(t, "area", profileArea(t, "L", l), 12, 1e-9)
}

func TestProfileWithHole(t *testing.T) {
	sketch := tsumiki.NewSketch(tsumiki.XY(0)).Rect(5, 5, 10, 10).Rect(5, 5, 4, 4)
	near(t, "area", profileArea(t, "square with hole", sketch), 100-16, 1e-9)
}

func TestProfileWithSeveralHoles(t *testing.T) {
	sketch := tsumiki.NewSketch(tsumiki.XY(0)).Rect(10, 5, 20, 10).
		Rect(4, 5, 4, 6).Rect(11, 5, 4, 6).Rect(16.5, 5, 3, 6)
	near(t, "area", profileArea(t, "three holes", sketch), 200-24-24-18, 1e-9)
}

func TestProfileCircleWithOffCentreHole(t *testing.T) {
	sketch := tsumiki.NewSketch(tsumiki.XY(0)).Circle(0, 0, 30).Circle(8, 3, 5)
	want := 3.141592653589793 * (30*30 - 5*5)
	near(t, "area", profileArea(t, "disc", sketch), want, 2) // inscribed polygons lose about 2/3 of tolerance x perimeter
}

func TestProfileRoundedRectWithHole(t *testing.T) {
	sketch := tsumiki.NewSketch(tsumiki.XY(0)).RoundedRect(0, 0, 30, 20, 4, 15).Circle(3, 2, 2)
	want := 30*20 - (4-3.141592653589793)*16 - 3.141592653589793*4
	near(t, "area", profileArea(t, "rounded rect", sketch), want, 0.3)
}

// Holes whose vertices share coordinates (a grid) make the bridge between hole and outline run
// along hole edges. This used to produce overlapping triangles.
func TestProfileAlignedGridOfHoles(t *testing.T) {
	sketch := tsumiki.NewSketch(tsumiki.XY(0)).Rect(20, 20, 40, 40)
	for column := range 4 {
		for row := range 4 {
			sketch.Rect(6.5+float64(column)*9, 6.5+float64(row)*9, 5, 5)
		}
	}
	near(t, "area", profileArea(t, "grid", sketch), 1600-16*25, 1e-9)
}

func TestProfileOfManyCircularHoles(t *testing.T) {
	sketch := tsumiki.NewSketch(tsumiki.XY(0)).Rect(0, 0, 60, 60)
	for column := range 6 {
		for row := range 6 {
			sketch.Circle(-25+float64(column)*10, -25+float64(row)*10, 3)
		}
	}
	want := 3600 - 36*3.141592653589793*9
	near(t, "area", profileArea(t, "perforated plate", sketch), want, 6)
}
