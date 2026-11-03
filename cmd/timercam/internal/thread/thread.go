// Package thread draws the cross-section of a printable metric screw thread, to be extruded with
// tsumiki.Twist into a helix.
package thread

import (
	"math"

	"github.com/nasa9084/tsumiki"
)

const (
	// DepthPerPitch is the radial depth of the thread as a fraction of the pitch (ISO 68-1 external
	// thread, 17/24·H with H = √3/2·P).
	DepthPerPitch = 0.6134
	// crestPerPitch is the axial width of the flat crest as a fraction of the pitch (P/8).
	crestPerPitch = 0.125
	// pointsPerTurn is how finely the section is sampled around the axis.
	pointsPerTurn = 72
	// tolerance is the chord tolerance of the helix: a coarser one than the default keeps the
	// mesh small, and a 0.05 mm facet is far below what the printer resolves on a thread.
	tolerance = 0.05
)

// Profile is the radius of the thread at axial phase u (mm, any value; the profile repeats every
// pitch): a flat crest of P/8 at majorRadius, 60° flanks and a flat root majorRadius − depth deep.
func Profile(u, majorRadius, pitch float64) float64 {
	depth := DepthPerPitch * pitch
	flankWidth := depth * math.Tan(math.Pi/6)
	u = math.Mod(u, pitch)
	if u < 0 {
		u += pitch
	}
	fromCrest := math.Min(u, pitch-u) // distance from the middle of the crest at u = 0
	switch {
	case fromCrest <= crestPerPitch*pitch/2:
		return majorRadius
	case fromCrest <= crestPerPitch*pitch/2+flankWidth:
		return majorRadius - depth*(fromCrest-crestPerPitch*pitch/2)/flankWidth
	default:
		return majorRadius - depth
	}
}

// Section is the cross-section of a right-hand single-start thread on plane, centered on its origin.
// Extruding it by a length L with tsumiki.Twist(360·L/pitch) sweeps the thread.
func Section(plane tsumiki.Plane, majorRadius, pitch float64) *tsumiki.Sketch {
	points := make([]tsumiki.Vector2, pointsPerTurn)
	for i := range points {
		angle := 2 * math.Pi * float64(i) / pointsPerTurn
		radius := Profile(angle/(2*math.Pi)*pitch, majorRadius, pitch)
		points[i] = tsumiki.Vector2{X: radius * math.Cos(angle), Y: radius * math.Sin(angle)}
	}
	sketch := tsumiki.NewSketch(plane).Polygon(points...)
	sketch.Tolerance = tolerance
	return sketch
}

// TwistFor is the twist in degrees that turns a thread section once per pitch over length.
func TwistFor(length, pitch float64) float64 { return 360 * length / pitch }
