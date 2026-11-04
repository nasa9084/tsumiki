// Package bevel extrudes a profile with 45° chamfers on its top and bottom edges.
package bevel

import "github.com/nasa9084/tsumiki"

// Extrude joins to part the profile that outline draws, extruded along +z from z0 by height, with
// every top and bottom edge chamfered by size at 45°. outline is called once per plane it is drawn on.
func Extrude(part *tsumiki.Part, outline func(tsumiki.Plane) *tsumiki.Sketch, z0, height, size float64) {
	part.Extrude(outline(tsumiki.XY(z0+size)), -size, tsumiki.Taper(-45))
	part.Extrude(outline(tsumiki.XY(z0+size)), height-2*size)
	part.Extrude(outline(tsumiki.XY(z0+height-size)), size, tsumiki.Taper(-45))
}
