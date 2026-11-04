// screw is the printed thumb screw (part C) that locks the ball joint of the Timer Camera F
// mount (see the project doc timercam-f-mount-spec.md).
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

const (
	majorRadius    = 3.9 // M8 (4.0) less 0.1 so that it runs in the printed nut
	pitch          = 1.25
	threadLength   = 6.0
	knobRadius     = 8.0
	knobThickness  = 4.0
	notchCount     = 12
	notchRadius    = 1.0
	tipChamferSize = 0.8 // 45° chamfer at the tip, to find the nut easily
	knobChamfer    = 0.6 // 45° on the top and bottom edges of the knob
	// The thread stops on the ledge where the nut ends (0.2 above the ball), so a thinner pin at
	// the tip reaches past it and presses the ball. The ledge leaves room for r < 2.83.
	pinRadius  = 2.0
	pinLength  = 1.2 // touches the ball with 1 mm to spare before the thread meets the ledge
	pinChamfer = 0.3
)

func build(_ io.Writer) (*tsumiki.Part, error) {
	part := tsumiki.NewPart("timercam screw")

	bevel.Extrude(part, func(plane tsumiki.Plane) *tsumiki.Sketch {
		return tsumiki.NewSketch(plane).Circle(0, 0, knobRadius)
	}, 0, knobThickness, knobChamfer)
	notches := tsumiki.NewSketch(tsumiki.XY(0))
	for k := range notchCount {
		angle := 2 * math.Pi * float64(k) / notchCount
		notches.Circle(knobRadius*math.Cos(angle), knobRadius*math.Sin(angle), notchRadius)
	}
	part.Extrude(notches, knobThickness, tsumiki.Cut())

	part.Extrude(thread.Section(tsumiki.XY(knobThickness), majorRadius, pitch), threadLength,
		tsumiki.Twist(thread.TwistFor(threadLength, pitch)))

	// the chamfer: cut away everything above a 45° cone through the tip edge
	top := knobThickness + threadLength
	tipRadius := majorRadius - tipChamferSize
	chamfer := tsumiki.NewSketch(tsumiki.XZ(0)).Polygon(
		tsumiki.Vector2{X: tipRadius - 0.5, Y: top + 0.5},
		tsumiki.Vector2{X: majorRadius + 1, Y: top - tipChamferSize - 1},
		tsumiki.Vector2{X: majorRadius + 1, Y: top + 0.5},
	)
	part.Revolve(chamfer, 360, tsumiki.Cut())

	part.Extrude(tsumiki.NewSketch(tsumiki.XY(top)).Circle(0, 0, pinRadius), pinLength-pinChamfer)
	part.Extrude(tsumiki.NewSketch(tsumiki.XY(top+pinLength-pinChamfer)).Circle(0, 0, pinRadius),
		pinChamfer, tsumiki.Taper(-45))

	return part, part.Err()
}

func main() {
	modelcmd.Main(modelcmd.Model{
		Name:  "timercam_screw",
		Build: build,
		Drawing: []tsumiki.DrawingOption{
			tsumiki.DrawingTitle("Timer Camera F mount: thumb screw (C)"),
			tsumiki.SectionY(0),
		},
	})
}
