// band is the part of the Timer Camera F mount (part B) that slides up the camera from below until
// it stops under the lens bump, with the ball of the ball joint on its back (see the project doc
// timercam-f-mount-spec.md).
package main

//go:generate go run . -drawings .

import (
	"io"

	"github.com/nasa9084/tsumiki"
	"github.com/nasa9084/tsumiki/cmd/timercam/internal/bevel"
	"github.com/nasa9084/tsumiki/modelcmd"
)

// The camera stands with its lens toward -y; z is up and z = 0 is the bottom of the band.
const (
	cameraWidth    = 24.0 // x, measured
	cameraDepth    = 20.0 // y, measured without the lens bump
	clearance      = 0.3  // on each side
	wallThickness  = 2.0
	bandHeight     = 14.0 // the band covers 11 to 25 mm above the bottom of the camera
	neckRadius     = 3.0
	neckLength     = 6.0
	neckFillet     = 1.5
	ballRadius     = 6.0
	ballAxisHeight = bandHeight / 2
	outerCorner    = 2.5 // radius of the outer vertical edges
	innerCorner    = 0.8 // small enough to clear a square camera corner (0.33 < 0.3·√2 + 0.3)
	edgeChamfer    = 0.6 // 45° on the outer top and bottom edges
	leadIn         = 0.6 // 45° on the inner bottom edge, to slide the band on easily
)

func outerWidth() float64 { return cameraWidth + 2*clearance + 2*wallThickness }

func outerDepth() float64 { return cameraDepth + 2*clearance + 2*wallThickness }

// backFace is the y of the outer face of the back wall.
func backFace() float64 { return outerDepth() / 2 }

// ballCenterY is the y of the center of the ball.
func ballCenterY() float64 { return backFace() + neckLength + ballRadius }

func build(_ io.Writer) (*tsumiki.Part, error) {
	part := tsumiki.NewPart("timercam band")

	// the outside, rounded and chamfered; the inner top edge stays sharp because the lens bump
	// rests on it
	bevel.Extrude(part, func(plane tsumiki.Plane) *tsumiki.Sketch {
		return tsumiki.NewSketch(plane).RoundedRect(0, 0, outerWidth(), outerDepth(), outerCorner, 0)
	}, 0, bandHeight, edgeChamfer)
	innerWidth, innerDepth := cameraWidth+2*clearance, cameraDepth+2*clearance
	part.Extrude(tsumiki.NewSketch(tsumiki.XY(0)).
		RoundedRect(0, 0, innerWidth, innerDepth, innerCorner, 0), bandHeight, tsumiki.Cut())
	part.Extrude(tsumiki.NewSketch(tsumiki.XY(0)).
		RoundedRect(0, 0, innerWidth+2*leadIn, innerDepth+2*leadIn, innerCorner+leadIn, 0),
		leadIn, tsumiki.Taper(-45), tsumiki.Cut())

	// the neck with a concave fillet at its root, revolved about the line x = 0, z = ballAxisHeight
	// (the sketch's u is the distance from that line, v is y)
	axisPlane := tsumiki.XY(ballAxisHeight)
	filletStart := backFace() // where the fillet meets the back wall
	neck := tsumiki.NewSketch(axisPlane).
		Path(0, filletStart-0.5).
		LineTo(neckRadius+neckFillet, filletStart-0.5).
		LineTo(neckRadius+neckFillet, filletStart).
		ArcTo(neckRadius, filletStart+neckFillet, neckRadius+neckFillet, filletStart+neckFillet, false).
		LineTo(neckRadius, ballCenterY()).
		LineTo(0, ballCenterY()).
		Close()
	part.Revolve(neck, 360)

	ball := tsumiki.NewSketch(axisPlane).
		Path(0, ballCenterY()-ballRadius).
		ArcTo(0, ballCenterY()+ballRadius, 0, ballCenterY(), true).
		Close()
	part.Revolve(ball, 360)

	return part, part.Err()
}

func main() {
	modelcmd.Main(modelcmd.Model{
		Name:  "timercam_band",
		Build: build,
		Drawing: []tsumiki.DrawingOption{
			tsumiki.DrawingTitle("Timer Camera F mount: band with ball (B)"),
			tsumiki.SectionX(0),
		},
	})
}
