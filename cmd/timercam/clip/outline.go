package main

import (
	"math"

	"github.com/nasa9084/tsumiki"
)

type point = tsumiki.Vector2

// fillet is an arc of radius filletRadius that blends the line y = lineY into a circle.
type fillet struct {
	center, onLine, onCircle point
}

// newFillet places a fillet whose center lies above the line (y = lineY + filletRadius), on the
// side of the circle that side says (-1 for smaller x, +1 for larger x). An external fillet touches
// the circle from outside (a concave corner), an internal one from inside (a convex corner).
func newFillet(lineY float64, circleCenter point, circleRadius, filletRadius, side float64, external bool) fillet {
	distance := circleRadius - filletRadius
	if external {
		distance = circleRadius + filletRadius
	}
	centerY := lineY + filletRadius
	dy := centerY - circleCenter.Y
	center := point{X: circleCenter.X + side*math.Sqrt(distance*distance-dy*dy), Y: centerY}
	return fillet{
		center:   center,
		onLine:   point{X: center.X, Y: lineY},
		onCircle: circleCenter.Add(center.Sub(circleCenter).Scale(circleRadius / distance)),
	}
}

// mirrored is the fillet reflected in the x axis.
func (f fillet) mirrored() fillet {
	flip := func(p point) point { return point{X: p.X, Y: -p.Y} }
	return fillet{center: flip(f.center), onLine: flip(f.onLine), onCircle: flip(f.onCircle)}
}

// outline draws the footprint of the clip, the arm and the socket as one loop, clockwise: the outer
// arc of the clip, the arm with concave fillets where it meets the clip and the socket, the socket,
// and back through the opening, whose four corners are rounded.
func outline(plane tsumiki.Plane) *tsumiki.Sketch {
	origin := point{}
	socket := point{X: socketX}
	armClip := newFillet(armHalfWidth, origin, clipOuterRadius, armFillet, 1, true)
	armSocket := newFillet(armHalfWidth, socket, socketRadius, armFillet, -1, true)
	openingOuter := newFillet(openingHalfWidth, origin, clipOuterRadius, openingFillet, -1, false)
	openingInner := newFillet(openingHalfWidth, origin, clipInnerRadius, openingFillet, -1, true)

	path := tsumiki.NewSketch(plane).Path(openingOuter.onCircle.X, openingOuter.onCircle.Y)
	arc := func(to, center point, counterClockwise bool) {
		path.ArcTo(to.X, to.Y, center.X, center.Y, counterClockwise)
	}
	line := func(to point) { path.LineTo(to.X, to.Y) }

	arc(armClip.onCircle, origin, false)
	arc(armClip.onLine, armClip.center, true)
	line(armSocket.onLine)
	arc(armSocket.onCircle, armSocket.center, true)
	arc(armSocket.mirrored().onCircle, socket, false)
	arc(armSocket.mirrored().onLine, armSocket.mirrored().center, true)
	line(armClip.mirrored().onLine)
	arc(armClip.mirrored().onCircle, armClip.mirrored().center, true)
	arc(openingOuter.mirrored().onCircle, origin, false)
	arc(openingOuter.mirrored().onLine, openingOuter.mirrored().center, false)
	line(openingInner.mirrored().onLine)
	arc(openingInner.mirrored().onCircle, openingInner.mirrored().center, false)
	arc(openingInner.onCircle, origin, true)
	arc(openingInner.onLine, openingInner.center, false)
	line(openingOuter.onLine)
	arc(openingOuter.onCircle, openingOuter.center, false) // back to the start
	return path.Close()
}
