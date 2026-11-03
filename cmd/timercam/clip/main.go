// clip is the part of the Timer Camera F mount (part A) that snaps onto a G-Fun S pillar (19 mm)
// and holds the ball of the band in a socket, locked by the printed thumb screw (see the project
// doc timercam-f-mount-spec.md).
package main

//go:generate go run . -drawings .

import (
	"io"
	"math"

	"github.com/nasa9084/tsumiki"
	"github.com/nasa9084/tsumiki/cmd/timercam/internal/thread"
	"github.com/nasa9084/tsumiki/modelcmd"
)

// The pillar runs along z through the origin; the arm points to +x and the clip opens to -x.
// z = 0 is the bottom of the part (the build plate).
const (
	clipInnerRadius  = 9.4 // 0.2 below the 19 mm pillar, so the clip grips it
	clipOuterRadius  = 12.4
	clipHeight       = 20.0
	openingHalfWidth = 7.75
	chamferSize      = 0.8
	armStartX        = 11.0
	armHalfWidth     = 5.0
	socketX          = 37.4 // center of the socket, 25 from the outside of the clip
	socketRadius     = 9.0
	socketHeight     = 21.0
	socketCenterZ    = 10.0
	cavityRadius     = 6.1 // the 12 mm ball plus 0.1
	lipReach         = 2.0 // the socket wraps the ball this far past its center
	slitHalfWidth    = 0.5
	slitLength       = 4.0
	nutMajorRadius   = 4.15 // the 3.9 screw plus 0.25
	pitch            = 1.25
	nutStartZ        = 15.8 // just inside the top of the cavity
	frontFlatTop     = 15.8 // the flat front stops above the opening and rises at 45° from there
	countersink      = 0.7
)

type point = tsumiki.Vector2

func onCircle(radius, degrees float64) point {
	s, c := math.Sincos(tsumiki.Radians(degrees))
	return point{X: radius * c, Y: radius * s}
}

// chamfers returns the four triangles that bevel the edges of the clip's opening by chamferSize.
func chamfers() *tsumiki.Sketch {
	sketch := tsumiki.NewSketch(tsumiki.XY(0))
	for _, side := range []float64{1, -1} {
		innerCornerDegrees := math.Atan2(openingHalfWidth, -math.Sqrt(clipInnerRadius*clipInnerRadius-
			openingHalfWidth*openingHalfWidth)) * 180 / math.Pi
		outerCornerDegrees := math.Atan2(openingHalfWidth, -math.Sqrt(clipOuterRadius*clipOuterRadius-
			openingHalfWidth*openingHalfWidth)) * 180 / math.Pi
		inner := onCircle(clipInnerRadius, innerCornerDegrees)
		outer := onCircle(clipOuterRadius, outerCornerDegrees)
		mirror := func(p point) point { return point{X: p.X, Y: side * p.Y} }
		// each triangle: a point along the opening face, a point along the circle, and a point in
		// the empty opening past the corner
		sketch.Polygon(
			mirror(point{X: inner.X - chamferSize, Y: inner.Y}),
			mirror(point{X: inner.X + 0.4, Y: inner.Y - 0.6}),
			mirror(onCircle(clipInnerRadius, innerCornerDegrees-chamferSize/clipInnerRadius*180/math.Pi)),
		)
		sketch.Polygon(
			mirror(point{X: outer.X + chamferSize, Y: outer.Y}),
			mirror(onCircle(clipOuterRadius, outerCornerDegrees-chamferSize/clipOuterRadius*180/math.Pi)),
			mirror(point{X: outer.X - 0.6, Y: outer.Y - 0.4}),
		)
	}
	return sketch
}

// revolvedAt revolves a (radius, z) profile about the vertical line through (x, 0).
func revolvedAt(name string, x float64, profile ...point) *tsumiki.Part {
	part := tsumiki.NewPart(name)
	part.Revolve(tsumiki.NewSketch(tsumiki.XZ(0)).Polygon(profile...), 360)
	return part.Translate(x, 0, 0)
}

func addClipAndArm(part *tsumiki.Part) {
	part.Extrude(tsumiki.NewSketch(tsumiki.XY(0)).
		Circle(0, 0, clipOuterRadius).Circle(0, 0, clipInnerRadius), clipHeight)
	part.Extrude(tsumiki.NewSketch(tsumiki.XY(0)).
		Rect(-clipOuterRadius, 0, 2*clipOuterRadius, 2*openingHalfWidth), clipHeight, tsumiki.Cut())
	part.Extrude(chamfers(), clipHeight, tsumiki.Cut())
	part.Extrude(tsumiki.NewSketch(tsumiki.XY(0)).
		Rect((armStartX+socketX)/2, 0, socketX-armStartX, 2*armHalfWidth), clipHeight)
}

func addSocket(part *tsumiki.Part) {
	part.Extrude(tsumiki.NewSketch(tsumiki.XY(0)).Circle(socketX, 0, socketRadius), socketHeight)
	// flatten the front of the socket where the ball goes in; above the opening the cut rises at
	// 45° (printable without supports) so that the socket is whole around the nut
	front := socketX + lipReach
	far := socketX + socketRadius + 1
	part.Extrude(tsumiki.NewSketch(tsumiki.XZ(socketRadius+1)).Polygon(
		point{X: front, Y: -1},
		point{X: far, Y: -1},
		point{X: far, Y: frontFlatTop + far - front},
		point{X: front, Y: frontFlatTop},
	), 2*(socketRadius+1), tsumiki.Cut())

	// the ball cavity, with a 45° teardrop roof so that it prints without supports
	cavity := tsumiki.NewPart("cavity")
	cavity.Revolve(tsumiki.NewSketch(tsumiki.XZ(0)).
		Path(0, socketCenterZ-cavityRadius).
		ArcTo(0, socketCenterZ+cavityRadius, 0, socketCenterZ, true).
		Close(), 360)
	cavity.Translate(socketX, 0, 0)
	part.Combine(tsumiki.OperationCut, cavity)
	tangent := cavityRadius / math.Sqrt2
	part.Combine(tsumiki.OperationCut, revolvedAt("teardrop", socketX,
		point{X: 0, Y: socketCenterZ},
		point{X: tangent, Y: socketCenterZ + tangent},
		point{X: 0, Y: socketCenterZ + cavityRadius*math.Sqrt2},
	))

	// a slit through the lower half of the lip lets it spread when the ball snaps in
	part.Extrude(tsumiki.NewSketch(tsumiki.XY(0)).
		Rect(socketX+lipReach-slitLength/2, 0, slitLength, 2*slitHalfWidth), socketCenterZ, tsumiki.Cut())
}

func addNut(part *tsumiki.Part) {
	length := socketHeight + 0.5 - nutStartZ
	// the section turns about its sketch origin, so put that on the socket axis
	plane := tsumiki.XY(nutStartZ)
	plane.Origin.X = socketX
	part.Extrude(thread.Section(plane, nutMajorRadius, pitch), length,
		tsumiki.Twist(thread.TwistFor(length, pitch)), tsumiki.Cut())
	// a 45° countersink so the screw finds the thread
	minor := nutMajorRadius - thread.DepthPerPitch*pitch
	part.Combine(tsumiki.OperationCut, revolvedAt("countersink", socketX,
		point{X: minor - 0.5, Y: socketHeight - countersink - (nutMajorRadius - minor) - 0.5},
		point{X: nutMajorRadius + countersink + 0.5, Y: socketHeight + 0.5},
		point{X: minor - 0.5, Y: socketHeight + 0.5},
	))
}

func build(_ io.Writer) (*tsumiki.Part, error) {
	part := tsumiki.NewPart("timercam clip")
	addClipAndArm(part)
	addSocket(part)
	addNut(part)

	return part, part.Err()
}

func main() {
	modelcmd.Main(modelcmd.Model{
		Name:  "timercam_clip",
		Build: build,
		Drawing: []tsumiki.DrawingOption{
			tsumiki.DrawingTitle("Timer Camera F mount: pillar clip with socket (A)"),
			tsumiki.SectionY(0),
		},
	})
}
