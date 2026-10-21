// Package tsumiki is a small history-style (timeline) 3D CAD kernel for 3D printing.
//
// Conventions (fixed on purpose, see DESIGN.md):
//   - units are millimetres, right-handed, Z up
//   - sketch planes have U x V = Normal; extrusion distance is measured along Normal
//   - taper angle: positive = the cross-section GROWS with distance travelled
//   - triangle winding is counter-clockwise seen from outside (outward normals)
package tsumiki

import "math"

// Vector2 is a 2D vector in sketch coordinates.
type Vector2 struct{ X, Y float64 }

// Vector3 is a 3D vector in world coordinates.
type Vector3 struct{ X, Y, Z float64 }

func (a Vector2) Add(b Vector2) Vector2   { return Vector2{a.X + b.X, a.Y + b.Y} }
func (a Vector2) Sub(b Vector2) Vector2   { return Vector2{a.X - b.X, a.Y - b.Y} }
func (a Vector2) Scale(s float64) Vector2 { return Vector2{a.X * s, a.Y * s} }
func (a Vector2) Dot(b Vector2) float64   { return a.X*b.X + a.Y*b.Y }
func (a Vector2) Cross(b Vector2) float64 { return a.X*b.Y - a.Y*b.X }
func (a Vector2) Length() float64         { return math.Hypot(a.X, a.Y) }

// Rotate rotates a counter-clockwise by radians around the origin.
func (a Vector2) Rotate(radians float64) Vector2 {
	s, c := math.Sincos(radians)
	return Vector2{a.X*c - a.Y*s, a.X*s + a.Y*c}
}

func (a Vector3) Add(b Vector3) Vector3                    { return Vector3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func (a Vector3) Sub(b Vector3) Vector3                    { return Vector3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }
func (a Vector3) Scale(s float64) Vector3                  { return Vector3{a.X * s, a.Y * s, a.Z * s} }
func (a Vector3) Negate() Vector3                          { return Vector3{-a.X, -a.Y, -a.Z} }
func (a Vector3) Dot(b Vector3) float64                    { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }
func (a Vector3) Length() float64                          { return math.Sqrt(a.Dot(a)) }
func (a Vector3) Interpolate(b Vector3, t float64) Vector3 { return a.Add(b.Sub(a).Scale(t)) }
func (a Vector3) Cross(b Vector3) Vector3 {
	return Vector3{a.Y*b.Z - a.Z*b.Y, a.Z*b.X - a.X*b.Z, a.X*b.Y - a.Y*b.X}
}
func (a Vector3) Normalize() Vector3 {
	l := a.Length()
	if l == 0 {
		return a
	}
	return a.Scale(1 / l)
}

// Radians converts degrees to radians.
func Radians(degrees float64) float64 { return degrees * math.Pi / 180 }
