package main

import (
	"io"
	"math"
	"testing"

	tsumiki "github.com/nasa9084/tsumiki"
)

func TestCellCountsMatchTheReferenceModel(t *testing.T) {
	allCells, bandCells := cells()
	if len(allCells) != 364 {
		t.Errorf("got %d hex cells, the reference model has 364", len(allCells))
	}
	if len(bandCells) != 100 {
		t.Errorf("got %d recess cells, the reference model has 100", len(bandCells))
	}
	// every recess cell is one of the through-hole cells
	known := map[point]bool{}
	for _, c := range allCells {
		known[c] = true
	}
	for _, c := range bandCells {
		if !known[c] {
			t.Errorf("recess cell %v is not a through-hole cell", c)
		}
	}
}

func TestCellsStayClearOfTheFrameAndSplitEdge(t *testing.T) {
	allCells, _ := cells()
	for _, c := range allCells {
		radius := math.Hypot(c.X, c.Y)
		if radius > plateRadius || radius < holeRadius {
			t.Errorf("cell %v is outside the plate ring", c)
		}
		if c.X < splitX {
			t.Errorf("cell %v is on the wrong side of the split plane", c)
		}
	}
}

func TestHexPolygonHasTheConfiguredAcrossFlatsWidth(t *testing.T) {
	hexagon := hexPoly(point{X: 10, Y: 5})
	if len(hexagon) != 6 {
		t.Fatalf("got %d corners", len(hexagon))
	}
	// opposite flats are hexAcrossFlats apart: measure between the midpoints of two opposite edges
	first := hexagon[0].Add(hexagon[1]).Scale(0.5)
	opposite := hexagon[3].Add(hexagon[4]).Scale(0.5)
	if got := first.Sub(opposite).Length(); math.Abs(got-hexAcrossFlats) > 1e-9 {
		t.Errorf("across flats = %g, want %g", got, hexAcrossFlats)
	}
}

func TestInHex(t *testing.T) {
	center := point{X: 3, Y: 4}
	if !inHex(center, center) {
		t.Error("the center is inside")
	}
	if inHex(point{X: 3 + hexAcrossFlats, Y: 4}, center) {
		t.Error("a point a whole flat-to-flat width away is outside")
	}
}

func TestRoundedRegionsNestAsTheSpecDescribes(t *testing.T) {
	// on the +x axis (a corner) the skirt wall is 38.3 ... 41.3 half-size, rounded
	cornerRadius := cornerArcCenterDistance + 14.3
	if !inRounded(point{X: cornerRadius - 0.1, Y: 0}, 38.3, 14.3) {
		t.Error("just inside the inner corner")
	}
	if inRounded(point{X: cornerRadius + 0.1, Y: 0}, 38.3, 14.3) {
		t.Error("just outside the inner corner")
	}
	if !inBand(point{X: cornerArcCenterDistance + 17, Y: 0}, 0) {
		t.Error("the wall is inside the reinforcement band")
	}
	if inBand(point{}, 0) {
		t.Error("the plate center is not in the band")
	}
}

func TestCornerCutterEncloses(t *testing.T) {
	for _, sign := range []float64{1, -1} {
		for _, circle := range []struct {
			radius         float64
			materialInside bool
		}{{plateRadius, true}, {holeRadius, false}} {
			polygon := cornerCutter(circle.radius, circle.materialInside, sign)
			area := tsumiki.Loop(polygon).Area()
			if area == 0 || math.Abs(area) > 3*3 {
				t.Errorf("radius %g sign %g: the corner sliver has implausible area %g", circle.radius, sign, area)
			}
			for _, vertex := range polygon {
				if vertex.X < splitX-1e-9 || vertex.X > splitX+3.5 {
					t.Errorf("vertex %v strays outside the corner zone", vertex)
				}
			}
		}
	}
}

func TestContainsRayCast(t *testing.T) {
	cube := tsumiki.NewPart("cube").Extrude(tsumiki.NewSketch(tsumiki.XY(0)).Rect(0, 0, 2, 2), 2).Mesh()
	if !contains(cube, tsumiki.Vector3{X: 0, Y: 0, Z: 1}) {
		t.Error("center of the cube should be inside")
	}
	if contains(cube, tsumiki.Vector3{X: 3, Y: 0, Z: 1}) || contains(cube, tsumiki.Vector3{X: 0, Y: 0, Z: 3}) {
		t.Error("points outside the cube reported inside")
	}
}

// TestBuildMatchesTheReferenceModel runs the whole 13-step build (about 25 s).
func TestBuildMatchesTheReferenceModel(t *testing.T) {
	if testing.Short() {
		t.Skip("the full guard build takes about 25 seconds")
	}
	part, err := build(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	report := part.Report()
	if !report.Watertight() {
		t.Errorf("the guard must be watertight: %+v", report)
	}
	// Fusion reports 9979.6 mm³; tessellation and 1 µm snapping account for the small gap
	if math.Abs(report.Volume-9979.6) > 50 {
		t.Errorf("volume %.1f mm³, the reference is 9979.6", report.Volume)
	}
	lo, hi := part.Mesh().Bounds()
	for name, pair := range map[string][2]float64{
		"xmin": {lo.X, 0.7}, "xmax": {hi.X, 65},
		"ymin": {lo.Y, -64.89}, "ymax": {hi.Y, 64.89},
		"zmin": {lo.Z, -12}, "zmax": {hi.Z, 1.5},
	} {
		if math.Abs(pair[0]-pair[1]) > 0.01 {
			t.Errorf("%s = %.3f, want %.3f", name, pair[0], pair[1])
		}
	}
	if mismatches := checkProbes(io.Discard, part.Mesh()); mismatches != 0 {
		t.Errorf("%d of the reference in/out probes disagree", mismatches)
	}
}
