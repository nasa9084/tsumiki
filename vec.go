// Package cad is a small history-style (timeline) 3D CAD kernel for 3D printing.
//
// Conventions (fixed on purpose, see DESIGN.md):
//   - units are millimetres, right-handed, Z up
//   - sketch planes have U x V = N; extrusion distance is measured along N
//   - taper angle: positive = the cross-section GROWS with distance travelled
//   - triangle winding is counter-clockwise seen from outside (outward normals)
package tsumiki

import "math"

// V2 is a 2D vector in sketch coordinates.
type V2 struct{ X, Y float64 }

// V3 is a 3D vector in world coordinates.
type V3 struct{ X, Y, Z float64 }

func (a V2) Add(b V2) V2        { return V2{a.X + b.X, a.Y + b.Y} }
func (a V2) Sub(b V2) V2        { return V2{a.X - b.X, a.Y - b.Y} }
func (a V2) Mul(s float64) V2   { return V2{a.X * s, a.Y * s} }
func (a V2) Dot(b V2) float64   { return a.X*b.X + a.Y*b.Y }
func (a V2) Cross(b V2) float64 { return a.X*b.Y - a.Y*b.X }
func (a V2) Len() float64       { return math.Hypot(a.X, a.Y) }

// Rot rotates a counter-clockwise by rad around the origin.
func (a V2) Rot(rad float64) V2 {
	s, c := math.Sincos(rad)
	return V2{a.X*c - a.Y*s, a.X*s + a.Y*c}
}

func (a V3) Add(b V3) V3             { return V3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func (a V3) Sub(b V3) V3             { return V3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }
func (a V3) Mul(s float64) V3        { return V3{a.X * s, a.Y * s, a.Z * s} }
func (a V3) Neg() V3                 { return V3{-a.X, -a.Y, -a.Z} }
func (a V3) Dot(b V3) float64        { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }
func (a V3) Len() float64            { return math.Sqrt(a.Dot(a)) }
func (a V3) Lerp(b V3, t float64) V3 { return a.Add(b.Sub(a).Mul(t)) }
func (a V3) Cross(b V3) V3 {
	return V3{a.Y*b.Z - a.Z*b.Y, a.Z*b.X - a.X*b.Z, a.X*b.Y - a.Y*b.X}
}
func (a V3) Norm() V3 {
	l := a.Len()
	if l == 0 {
		return a
	}
	return a.Mul(1 / l)
}

// Rad converts degrees to radians.
func Rad(deg float64) float64 { return deg * math.Pi / 180 }
