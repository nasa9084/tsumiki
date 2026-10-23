package tsumiki

import "math/big"

// This file is compiled only for tests. It exposes the internals that the external test package
// (tsumiki_test) exercises directly.

type (
	// IntegerPlane exposes integerPlane to the tests.
	IntegerPlane = integerPlane
	// ExactPolygon exposes exactPolygon to the tests.
	ExactPolygon = exactPolygon
	// HomogeneousVertex exposes homogeneousVertex to the tests.
	HomogeneousVertex = homogeneousVertex
	// CellKey exposes cellKey to the tests.
	CellKey = cellKey
)

var (
	NormalizePlane      = normalizePlane
	GCD64               = gcd64
	Snap                = snap
	VertexOf            = vertexOf
	LatticeVertex       = latticeVertex
	NewTrianglePolygon  = newTrianglePolygon
	MeshToSolid         = meshToSolid
	SplitPolygon        = splitPolygon
	TriangleArea        = triangleArea
	CellKeyOf           = cellKeyOf
	ArcSteps            = arcSteps
	CleanLoop           = cleanLoop
	PointInLoop         = pointInLoop
	TriangulateRegion   = triangulateRegion
	TwiceSignedArea     = twiceSignedArea
	InTriangleInclusive = inTriangleInclusive
)

// NewPlane builds an IntegerPlane a*x + b*y + c*z = d without normalizing it.
func NewPlane(a, b, c, d int64) IntegerPlane { return integerPlane{a: a, b: b, c: c, d: d} }

// Neg returns the plane with the opposite orientation.
func (p integerPlane) Neg() IntegerPlane { return p.neg() }

// Side reports on which side of the plane the vertex lies (1 front, 0 on, -1 back).
func (v *homogeneousVertex) Side(plane *IntegerPlane) int { return v.side(plane) }

// FloatX returns the float approximation of the x coordinate in lattice units.
func (v homogeneousVertex) FloatX() float64 { return v.floatX }

// FloatY returns the float approximation of the y coordinate in lattice units.
func (v homogeneousVertex) FloatY() float64 { return v.floatY }

// FloatZ returns the float approximation of the z coordinate in lattice units.
func (v homogeneousVertex) FloatZ() float64 { return v.floatZ }

// Weight returns the homogeneous denominator.
func (v homogeneousVertex) Weight() *big.Int { return v.w }

// Plane returns the plane the polygon lies in.
func (p *exactPolygon) Plane() IntegerPlane { return p.p }

// Edges returns the planes bounding the polygon's edges.
func (p *exactPolygon) Edges() []IntegerPlane { return p.e }

// Corners returns the polygon's vertices.
func (p *exactPolygon) Corners() []HomogeneousVertex { return p.v }

// Flipped returns the polygon with reversed orientation.
func (p *exactPolygon) Flipped() *ExactPolygon { return p.flipped() }

// Translate moves the solid by a whole number of lattice units.
func (s Solid) Translate(offset [3]int64) Solid { return s.translate(offset) }

// Regions returns the sketch's regions.
func (s *Sketch) Regions() []Region { return s.regions() }

// Loops returns the sketch's raw loops.
func (s *Sketch) Loops() []Loop { return s.loops }

// Reversed returns the loop in the opposite direction.
func (l Loop) Reversed() Loop { return l.reversed() }

// Flip reverses the orientation of every triangle.
func (m *Mesh) Flip() { m.flip() }

// DropRepeated removes repeated triangles.
func (m *Mesh) DropRepeated() { m.dropRepeated() }

// DropDegenerate removes zero-area triangles.
func (m *Mesh) DropDegenerate() { m.dropDegenerate() }

// DropInternal removes internal faces.
func (m *Mesh) DropInternal() { m.dropInternal() }

// DropFlat removes sliver triangles.
func (m *Mesh) DropFlat() { m.dropFlat() }

// FixTJunctions splits edges at vertices lying on them.
func (m *Mesh) FixTJunctions(epsilon float64) { m.fixTJunctions(epsilon) }

// Merge appends other to the mesh.
func (m *Mesh) Merge(other *Mesh) { m.merge(other) }

// Transform maps every vertex and optionally mirrors the orientation.
func (m *Mesh) Transform(transformVertex func(Vector3) Vector3, mirror bool) {
	m.transform(transformVertex, mirror)
}
