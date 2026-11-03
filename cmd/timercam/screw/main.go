// screw is the printed thumb screw (part C) that locks the ball joint of the Timer Camera F
// mount (see the project doc timercam-f-mount-spec.md).
package main

//go:generate go run . -drawings .

import (
	"io"
	"math"

	"github.com/nasa9084/tsumiki"
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
)

func build(_ io.Writer) (*tsumiki.Part, error) {
	part := tsumiki.NewPart("timercam screw")

	knob := tsumiki.NewSketch(tsumiki.XY(0)).Circle(0, 0, knobRadius)
	part.Extrude(knob, knobThickness)
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
