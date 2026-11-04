package main

import (
	"io"
	"testing"

	"github.com/nasa9084/tsumiki"
)

func TestBandMatchesTheSpecPoints(t *testing.T) {
	part, err := build(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	mesh := part.Mesh()
	for _, c := range []struct {
		name   string
		point  tsumiki.Vector3
		inside bool
	}{
		{"inside the band", tsumiki.Vector3{X: 0, Y: 0, Z: 7}, false},
		{"the side wall", tsumiki.Vector3{X: 13.3, Y: 0, Z: 7}, true},
		{"the front wall", tsumiki.Vector3{X: 0, Y: -11.3, Z: 7}, true},
		{"above the band", tsumiki.Vector3{X: 0, Y: -11.3, Z: 14.5}, false},
		{"the neck", tsumiki.Vector3{X: 0, Y: 15, Z: 7}, true},
		{"beside the neck", tsumiki.Vector3{X: 0, Y: 15, Z: 10.5}, false},
		{"the fillet at the root of the neck", tsumiki.Vector3{X: 0, Y: 12.6, Z: 10.5}, true},
		{"the rounded outer corner", tsumiki.Vector3{X: 14.1, Y: 12.1, Z: 7}, false},
		{"the outer bottom chamfer", tsumiki.Vector3{X: 14.2, Y: 0, Z: 0.2}, false},
		{"the outer top chamfer", tsumiki.Vector3{X: 14.2, Y: 0, Z: 13.8}, false},
		{"the inner lead-in at the bottom", tsumiki.Vector3{X: 12.45, Y: 0, Z: 0.1}, false},
		{"the inner top edge stays square", tsumiki.Vector3{X: 12.45, Y: 0, Z: 13.9}, true},
		{"the ball", tsumiki.Vector3{X: 0, Y: 24.3, Z: 7}, true},
		{"above the ball", tsumiki.Vector3{X: 0, Y: 24.3, Z: 13.2}, false},
	} {
		if got := mesh.Contains(c.point); got != c.inside {
			t.Errorf("%s: Contains(%v) = %v, want %v", c.name, c.point, got, c.inside)
		}
	}
	if report := part.Report(); !report.Watertight() {
		t.Errorf("not watertight: %+v", report)
	}
}
