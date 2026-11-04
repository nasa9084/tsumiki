package main

import (
	"io"
	"math"
	"testing"

	"github.com/nasa9084/tsumiki"
	"github.com/nasa9084/tsumiki/cmd/timercam/internal/thread"
)

func TestScrewThreadFollowsTheProfile(t *testing.T) {
	part, err := build(io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	mesh := part.Mesh()
	// up the x axis the thread is a 60° profile: crest, flank, root, flank, crest every pitch
	for z := knobThickness + 1.0; z < knobThickness+threadLength-1.5; z += pitch / 10 {
		// the section at height h is the base section turned by 360·h/pitch, so along +x it shows
		// the profile at phase −h
		radius := thread.Profile(-(z - knobThickness), majorRadius, pitch)
		if !mesh.Contains(tsumiki.Vector3{X: radius - 0.08, Y: 0, Z: z}) {
			t.Errorf("z=%.3f: just inside the thread (r=%.3f) is empty", z, radius-0.08)
		}
		if mesh.Contains(tsumiki.Vector3{X: radius + 0.08, Y: 0, Z: z}) {
			t.Errorf("z=%.3f: just outside the thread (r=%.3f) is solid", z, radius+0.08)
		}
	}
	for _, c := range []struct {
		name   string
		point  tsumiki.Vector3
		inside bool
	}{
		{"the knob", tsumiki.Vector3{X: 6, Y: 0, Z: 2}, true},
		{"a notch in the knob", tsumiki.Vector3{X: 7.5, Y: 0, Z: 2}, false},
		{"the chamfered bottom edge of the knob", tsumiki.Vector3{X: 0, Y: 7.85, Z: 0.1}, false},
		{"the chamfered tip edge", tsumiki.Vector3{X: 3.6, Y: 0, Z: 9.9}, false},
		{"the middle of the tip", tsumiki.Vector3{X: 0, Y: 0, Z: 9.9}, true},
	} {
		if got := mesh.Contains(c.point); got != c.inside {
			t.Errorf("%s: Contains(%v) = %v, want %v", c.name, c.point, got, c.inside)
		}
	}
}

func TestProfileRepeatsEveryPitch(t *testing.T) {
	for _, u := range []float64{0, 0.1, 0.3, 0.6, 1.0} {
		a := thread.Profile(u, majorRadius, pitch)
		b := thread.Profile(u+3*pitch, majorRadius, pitch)
		if math.Abs(a-b) > 1e-12 {
			t.Errorf("Profile(%g) = %g but Profile(%g) = %g", u, a, u+3*pitch, b)
		}
	}
	if got := thread.Profile(0, majorRadius, pitch); got != majorRadius {
		t.Errorf("crest = %g, want %g", got, majorRadius)
	}
	if got, want := thread.Profile(pitch/2, majorRadius, pitch), majorRadius-thread.DepthPerPitch*pitch; got != want {
		t.Errorf("root = %g, want %g", got, want)
	}
}
