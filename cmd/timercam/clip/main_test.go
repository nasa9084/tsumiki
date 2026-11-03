package main

import (
	"io"
	"testing"

	"github.com/nasa9084/tsumiki"
)

func TestClipMatchesTheSpecPoints(t *testing.T) {
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
		{"the pillar", tsumiki.Vector3{X: 0, Y: 0, Z: 10}, false},
		{"the clip behind the pillar", tsumiki.Vector3{X: 10.5, Y: 0, Z: 10}, true},
		{"the opening", tsumiki.Vector3{X: -11, Y: 0, Z: 10}, false},
		{"the side of the clip", tsumiki.Vector3{X: 0, Y: 10.9, Z: 10}, true},
		{"the arm", tsumiki.Vector3{X: 25, Y: 0, Z: 10}, true},
		{"the ball cavity", tsumiki.Vector3{X: 37.4, Y: 0, Z: 10}, false},
		{"the nut", tsumiki.Vector3{X: 37.4, Y: 0, Z: 19}, false},
		{"the socket wall", tsumiki.Vector3{X: 37.4, Y: 6.5, Z: 10}, true},
		{"past the flat front of the socket", tsumiki.Vector3{X: 40, Y: 0, Z: 10}, false},
		{"the slit", tsumiki.Vector3{X: 38.9, Y: 0, Z: 3}, false},
		{"beside the slit", tsumiki.Vector3{X: 38.9, Y: 2, Z: 3}, true},
		{"the socket above the opening", tsumiki.Vector3{X: 39, Y: 6, Z: 20}, true},
		{"the 45° roof over the flat front", tsumiki.Vector3{X: 45, Y: 0, Z: 20}, false},
		{"behind the nut thread", tsumiki.Vector3{X: 37.4 - 4.3, Y: 0, Z: 19}, true},
	} {
		if got := mesh.Contains(c.point); got != c.inside {
			t.Errorf("%s: Contains(%v) = %v, want %v", c.name, c.point, got, c.inside)
		}
	}
	if report := part.Report(); !report.Watertight() {
		t.Errorf("not watertight: %+v", report)
	}
}
