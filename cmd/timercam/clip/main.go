// clip is the part of the Timer Camera F mount (part A) that snaps onto a G-Fun S pillar (19 mm)
// and holds the ball of the band in a socket, locked by the printed thumb screw (see the project
// doc timercam-f-mount-spec.md).
package main

//go:generate go run . -drawings .

import (
	"io"
	"math"

	"github.com/nasa9084/tsumiki"
	"github.com/nasa9084/tsumiki/cmd/timercam/internal/bevel"
	"github.com/nasa9084/tsumiki/cmd/timercam/internal/thread"
	"github.com/nasa9084/tsumiki/modelcmd"
)

// The pillar runs along z through the origin; the arm points to +x and the clip opens to -x.
// z = 0 is the bottom of the part (the build plate).
const (
	clipInnerRadius  = 9.4 // 0.2 below the 19 mm pillar, so the clip grips it
	clipOuterRadius  = 12.4
	openingHalfWidth = 7.75
	openingFillet    = 1.2 // rounds the corners of the opening
	armHalfWidth     = 5.0
	armFillet        = 3.0  // blends the arm into the clip and the socket
	edgeChamfer      = 0.8  // 45° chamfer on every top and bottom edge
	socketX          = 37.4 // center of the socket, 25 from the outside of the clip
	socketRadius     = 9.0
	socketHeight     = 21.0 // the height of the whole part
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

// revolvedAt revolves a (radius, z) profile about the vertical line through (x, 0).
func revolvedAt(name string, x float64, profile ...point) *tsumiki.Part {
	part := tsumiki.NewPart(name)
	part.Revolve(tsumiki.NewSketch(tsumiki.XZ(0)).Polygon(profile...), 360)
	return part.Translate(x, 0, 0)
}

func addSocket(part *tsumiki.Part) {
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
	bevel.Extrude(part, outline, 0, socketHeight, edgeChamfer)
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
