package tsumiki

// Exact plane-based BSP booleans.
//
// Why: a floating-point BSP (csg.js style) decides "front/back/coplanar" with an epsilon.
// Splitting big triangles by many nearly-parallel planes creates slivers whose
// classification is inconsistent, which leaves cracks and overlapping faces.
//
// Here every input vertex is snapped to a 1 µm integer lattice, every polygon carries an
// exact integer plane, and every vertex is *defined* as the intersection of three of those
// planes (the polygon's plane and the two planes of the edges meeting there). Coordinates
// are rational numbers held in big.Int, so classification is exact: no epsilon anywhere.
// Plane coefficients never grow, because a split only ever adds an existing input plane to
// the list of edge planes. (Bernstein & Fussell, "Fast, Exact, Linear Booleans".)

import (
	"fmt"
	"math"
	"math/big"
)

const (
	lattice  = 1000.0   // lattice units per mm (1 unit = 1 µm)
	maxCoord = 500000.0 // |coordinate| limit in lattice units (500 mm); keeps int64 plane math safe
)

// integerPlane is the plane a*x + b*y + c*z = d with integer coefficients (lattice units).
type integerPlane struct{ a, b, c, d int64 }

func (p integerPlane) neg() integerPlane { return integerPlane{-p.a, -p.b, -p.c, -p.d} }

func gcd64(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	if a < 0 {
		a = -a
	}
	return a
}

func normalizePlane(a, b, c, d int64) integerPlane {
	g := gcd64(gcd64(a, b), gcd64(c, d))
	if g > 1 {
		a, b, c, d = a/g, b/g, c/g, d/g
	}
	return integerPlane{a, b, c, d}
}

// homogeneousVertex is a point (x/w, y/w, z/w) with w > 0, plus its correctly rounded float coordinates.
type homogeneousVertex struct {
	x, y, z, w             *big.Int
	floatX, floatY, floatZ float64 // lattice units
}

func bigInt(x int64) *big.Int { return big.NewInt(x) }

func ratio(x, w *big.Int) float64 {
	f, _ := new(big.Rat).SetFrac(x, w).Float64()
	return f
}

func latticeVertex(x, y, z int64) homogeneousVertex {
	return homogeneousVertex{bigInt(x), bigInt(y), bigInt(z), bigInt(1), float64(x), float64(y), float64(z)}
}

func cross3(a, b [3]*big.Int) [3]*big.Int {
	var r [3]*big.Int
	t := new(big.Int)
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		r[i] = new(big.Int).Mul(a[j], b[k])
		r[i].Sub(r[i], t.Mul(a[k], b[j]))
	}
	return r
}

// vertexOf intersects three planes. ok is false when they do not meet in a single point.
func vertexOf(plane1, plane2, plane3 *integerPlane) (homogeneousVertex, bool) {
	normal1 := [3]*big.Int{bigInt(plane1.a), bigInt(plane1.b), bigInt(plane1.c)}
	normal2 := [3]*big.Int{bigInt(plane2.a), bigInt(plane2.b), bigInt(plane2.c)}
	normal3 := [3]*big.Int{bigInt(plane3.a), bigInt(plane3.b), bigInt(plane3.c)}
	cross23, cross31, cross12 := cross3(normal2, normal3), cross3(normal3, normal1), cross3(normal1, normal2)
	t := new(big.Int)
	determinant := new(big.Int)
	for i := 0; i < 3; i++ {
		determinant.Add(determinant, t.Mul(normal1[i], cross23[i]))
	}
	if determinant.Sign() == 0 {
		return homogeneousVertex{}, false
	}
	constant1, constant2, constant3 := bigInt(plane1.d), bigInt(plane2.d), bigInt(plane3.d)
	var numerators [3]*big.Int
	for i := 0; i < 3; i++ {
		numerators[i] = new(big.Int).Mul(constant1, cross23[i])
		numerators[i].Add(numerators[i], t.Mul(constant2, cross31[i]))
		numerators[i].Add(numerators[i], t.Mul(constant3, cross12[i]))
	}
	if determinant.Sign() < 0 {
		determinant.Neg(determinant)
		for i := range numerators {
			numerators[i].Neg(numerators[i])
		}
	}
	return homogeneousVertex{numerators[0], numerators[1], numerators[2], determinant, ratio(numerators[0], determinant), ratio(numerators[1], determinant), ratio(numerators[2], determinant)}, true
}

// side returns the sign of (a*x + b*y + c*z - d) at the vertex: +1 front, -1 back, 0 on the plane.
func (vertex *homogeneousVertex) side(plane *integerPlane) int {
	a, b, c, d := float64(plane.a), float64(plane.b), float64(plane.c), float64(plane.d)
	signedValue := a*vertex.floatX + b*vertex.floatY + c*vertex.floatZ - d
	errorBound := (math.Abs(a*vertex.floatX) + math.Abs(b*vertex.floatY) + math.Abs(c*vertex.floatZ) + math.Abs(d)) * 1e-13
	if signedValue > errorBound {
		return 1
	}
	if signedValue < -errorBound {
		return -1
	}
	t := new(big.Int).Mul(vertex.x, bigInt(plane.a))
	u := new(big.Int)
	t.Add(t, u.Mul(vertex.y, bigInt(plane.b)))
	t.Add(t, u.Mul(vertex.z, bigInt(plane.c)))
	t.Sub(t, u.Mul(vertex.w, bigInt(plane.d)))
	return t.Sign()
}

// exactPolygon is a convex polygon on plane p. Edge i runs from vertex i to vertex i+1 and lies on
// plane e[i]; vertex i is the intersection of p, e[i-1] and e[i].
type exactPolygon struct {
	p integerPlane
	e []integerPlane
	v []homogeneousVertex
}

// Solid is a closed polyhedral surface as a soup of exact convex polygons.
type Solid []*exactPolygon

func snap(p Vector3) ([3]int64, error) {
	c := [3]float64{p.X * lattice, p.Y * lattice, p.Z * lattice}
	var r [3]int64
	for i, v := range c {
		if math.Abs(v) > maxCoord || math.IsNaN(v) {
			return r, fmt.Errorf("coordinate %.3f mm is outside the supported ±%.0f mm build volume", v/lattice, maxCoord/lattice)
		}
		r[i] = int64(math.Round(v))
	}
	return r, nil
}

func newTrianglePolygon(corners [3][3]int64) *exactPolygon {
	subtract := func(a, b [3]int64) [3]int64 { return [3]int64{a[0] - b[0], a[1] - b[1], a[2] - b[2]} }
	firstEdge, secondEdge := subtract(corners[1], corners[0]), subtract(corners[2], corners[0])
	n := [3]int64{firstEdge[1]*secondEdge[2] - firstEdge[2]*secondEdge[1], firstEdge[2]*secondEdge[0] - firstEdge[0]*secondEdge[2], firstEdge[0]*secondEdge[1] - firstEdge[1]*secondEdge[0]}
	if n == [3]int64{} {
		return nil
	}
	d := n[0]*corners[0][0] + n[1]*corners[0][1] + n[2]*corners[0][2]
	p := &exactPolygon{p: normalizePlane(n[0], n[1], n[2], d)}
	dominantAxis := 0
	for k := 1; k < 3; k++ {
		if abs64(n[k]) > abs64(n[dominantAxis]) {
			dominantAxis = k
		}
	}
	for i := 0; i < 3; i++ {
		a, b := corners[i], corners[(i+1)%3]
		edgeVector := subtract(b, a)
		var edgeNormal [3]int64
		switch dominantAxis {
		case 2: // e x z
			edgeNormal = [3]int64{edgeVector[1], -edgeVector[0], 0}
		case 0: // e x x
			edgeNormal = [3]int64{0, edgeVector[2], -edgeVector[1]}
		default: // e x y
			edgeNormal = [3]int64{-edgeVector[2], 0, edgeVector[0]}
		}
		p.e = append(p.e, normalizePlane(edgeNormal[0], edgeNormal[1], edgeNormal[2], edgeNormal[0]*a[0]+edgeNormal[1]*a[1]+edgeNormal[2]*a[2]))
		p.v = append(p.v, latticeVertex(a[0], a[1], a[2]))
	}
	return p
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func meshToSolid(m *Mesh) (Solid, error) {
	pts := make([][3]int64, len(m.Vertices))
	for i, v := range m.Vertices {
		p, err := snap(v)
		if err != nil {
			return nil, err
		}
		pts[i] = p
	}
	s := make(Solid, 0, len(m.Triangles))
	for _, t := range m.Triangles {
		if p := newTrianglePolygon([3][3]int64{pts[t[0]], pts[t[1]], pts[t[2]]}); p != nil {
			s = append(s, p)
		}
	}
	return s, nil
}

func (p *exactPolygon) flipped() *exactPolygon {
	n := len(p.e)
	q := &exactPolygon{p: p.p.neg(), e: make([]integerPlane, n), v: make([]homogeneousVertex, n)}
	for j := 0; j < n; j++ {
		q.e[j] = p.e[n-1-j]
		q.v[j] = p.v[(n-j)%n]
	}
	return q
}

func sameDir(a, b integerPlane) bool {
	// planes may be exactly coplanar only when normals are parallel: compare along a nonzero axis
	switch {
	case a.a != 0:
		return (a.a > 0) == (b.a > 0)
	case a.b != 0:
		return (a.b > 0) == (b.b > 0)
	}
	return (a.c > 0) == (b.c > 0)
}

// splitPolygon sorts p against plane s into coplanar-front, coplanar-back, front and back lists.
func splitPolygon(s *integerPlane, p *exactPolygon, coplanarFront, coplanarBack, front, back *[]*exactPolygon) {
	n := len(p.v)
	var buf [16]int8
	tags := buf[:0]
	frontCount, backCount := 0, 0
	for i := range p.v {
		t := p.v[i].side(s)
		tags = append(tags, int8(t))
		if t > 0 {
			frontCount++
		} else if t < 0 {
			backCount++
		}
	}
	switch {
	case frontCount == 0 && backCount == 0:
		if sameDir(p.p, *s) {
			*coplanarFront = append(*coplanarFront, p)
		} else {
			*coplanarBack = append(*coplanarBack, p)
		}
	case backCount == 0:
		*front = append(*front, p)
	case frontCount == 0:
		*back = append(*back, p)
	default:
		if q := p.piece(s, tags, n, true); q != nil {
			*front = append(*front, q)
		}
		if q := p.piece(s, tags, n, false); q != nil {
			*back = append(*back, q)
		}
	}
}

// piece cuts out the part of p on the front (or back) of s. p must span s.
//
// The part keeps a contiguous run of p's edges (the first and last are truncated by s) plus
// s itself as the closing edge. The run starts at the edge that crosses (or leaves) s into the
// wanted side and ends at the edge that crosses back.
func (p *exactPolygon) piece(s *integerPlane, tags []int8, n int, wantFront bool) *exactPolygon {
	enter := func(i int) bool {
		a, b := tags[i], tags[(i+1)%n]
		if wantFront {
			return a <= 0 && b > 0
		}
		return a >= 0 && b < 0
	}
	exit := func(i int) bool {
		a, b := tags[i], tags[(i+1)%n]
		if wantFront {
			return a > 0 && b <= 0
		}
		return a < 0 && b >= 0
	}
	start := -1
	for i := 0; i < n; i++ {
		if enter(i) {
			start = i
			break
		}
	}
	if start < 0 {
		return nil
	}
	q := &exactPolygon{p: p.p}
	var startVertex homogeneousVertex
	if tags[start] == 0 {
		startVertex = p.v[start]
	} else {
		var ok bool
		if startVertex, ok = vertexOf(&p.p, s, &p.e[start]); !ok {
			return nil
		}
	}
	q.e = append(q.e, p.e[start])
	q.v = append(q.v, startVertex)
	last := -1
	if exit(start) {
		last = start
	}
	for k := 1; last < 0 && k < n; k++ {
		idx := (start + k) % n
		q.e = append(q.e, p.e[idx])
		q.v = append(q.v, p.v[idx])
		if exit(idx) {
			last = idx
		}
	}
	if last < 0 {
		return nil
	}
	var endVertex homogeneousVertex
	if end := (last + 1) % n; tags[end] == 0 {
		endVertex = p.v[end]
	} else {
		var ok bool
		if endVertex, ok = vertexOf(&p.p, &p.e[last], s); !ok {
			return nil
		}
	}
	q.e = append(q.e, *s)
	q.v = append(q.v, endVertex)
	if len(q.e) < 3 {
		return nil
	}
	return q
}

type node struct {
	plane       *integerPlane
	front, back *node
	polygons    []*exactPolygon
}

func (n *node) invert() {
	for i := range n.polygons {
		n.polygons[i] = n.polygons[i].flipped()
	}
	if n.plane != nil {
		f := n.plane.neg()
		n.plane = &f
	}
	if n.front != nil {
		n.front.invert()
	}
	if n.back != nil {
		n.back.invert()
	}
	n.front, n.back = n.back, n.front
}

func (n *node) clipPolygons(polygons []*exactPolygon) []*exactPolygon {
	if n.plane == nil {
		return append([]*exactPolygon(nil), polygons...)
	}
	var f, b []*exactPolygon
	for _, p := range polygons {
		splitPolygon(n.plane, p, &f, &b, &f, &b)
	}
	if n.front != nil {
		f = n.front.clipPolygons(f)
	}
	if n.back != nil {
		b = n.back.clipPolygons(b)
	} else {
		b = nil
	}
	return append(f, b...)
}

func (n *node) clipTo(o *node) {
	n.polygons = o.clipPolygons(n.polygons)
	if n.front != nil {
		n.front.clipTo(o)
	}
	if n.back != nil {
		n.back.clipTo(o)
	}
}

func (n *node) allPolygons() []*exactPolygon {
	polygons := append([]*exactPolygon(nil), n.polygons...)
	if n.front != nil {
		polygons = append(polygons, n.front.allPolygons()...)
	}
	if n.back != nil {
		polygons = append(polygons, n.back.allPolygons()...)
	}
	return polygons
}

func (n *node) build(polygons []*exactPolygon) {
	if len(polygons) == 0 {
		return
	}
	if n.plane == nil {
		plane := polygons[pickSplitter(polygons)].p
		n.plane = &plane
	}
	var f, b []*exactPolygon
	for _, p := range polygons {
		splitPolygon(n.plane, p, &n.polygons, &n.polygons, &f, &b)
	}
	if len(f) > 0 {
		if n.front == nil {
			n.front = &node{}
		}
		n.front.build(f)
	}
	if len(b) > 0 {
		if n.back == nil {
			n.back = &node{}
		}
		n.back.build(b)
	}
}

// pickSplitter samples candidate planes and prefers few splits and a balanced tree.
func pickSplitter(polygons []*exactPolygon) int {
	if len(polygons) <= 4 {
		return 0
	}
	const candidateCount = 8
	candidateStep := max(1, len(polygons)/candidateCount)
	sampleStep := max(1, len(polygons)/48)
	best, bestScore := 0, 1<<60
	for c := 0; c < len(polygons); c += candidateStep {
		plane := &polygons[c].p
		frontCount, backCount, spanningCount := 0, 0, 0
		for i := 0; i < len(polygons); i += sampleStep {
			hasFront, hasBack := false, false
			for k := range polygons[i].v {
				switch polygons[i].v[k].side(plane) {
				case 1:
					hasFront = true
				case -1:
					hasBack = true
				}
			}
			switch {
			case hasFront && hasBack:
				spanningCount++
			case hasFront:
				frontCount++
			case hasBack:
				backCount++
			}
		}
		imbalance := frontCount - backCount
		if imbalance < 0 {
			imbalance = -imbalance
		}
		if score := spanningCount*8 + imbalance; score < bestScore {
			best, bestScore = c, score
		}
	}
	return best
}

// BoolOperation selects a boolean operation.
type BoolOperation int

const (
	OperationJoin BoolOperation = iota
	OperationCut
	OperationIntersect
)

func (o BoolOperation) String() string { return [...]string{"join", "cut", "intersect"}[o] }

// BoolSolid combines a and b exactly (neither is modified).
func BoolSolid(operation BoolOperation, a, b Solid) Solid {
	A, B := &node{}, &node{}
	A.build(append([]*exactPolygon(nil), a...))
	B.build(append([]*exactPolygon(nil), b...))
	switch operation {
	case OperationJoin:
		A.clipTo(B)
		B.clipTo(A)
		B.invert()
		B.clipTo(A)
		B.invert()
		A.build(B.allPolygons())
	case OperationCut:
		A.invert()
		A.clipTo(B)
		B.clipTo(A)
		B.invert()
		B.clipTo(A)
		B.invert()
		A.build(B.allPolygons())
		A.invert()
	case OperationIntersect:
		A.invert()
		B.clipTo(A)
		B.invert()
		A.clipTo(B)
		B.clipTo(A)
		A.build(B.allPolygons())
		A.invert()
	}
	return Solid(A.allPolygons())
}

// translate shifts the solid by whole lattice units, exactly.
func (s Solid) translate(offset [3]int64) Solid {
	shiftPlane := func(p integerPlane) integerPlane {
		return integerPlane{p.a, p.b, p.c, p.d + p.a*offset[0] + p.b*offset[1] + p.c*offset[2]}
	}
	out := make(Solid, len(s))
	for i, p := range s {
		q := &exactPolygon{p: shiftPlane(p.p), e: make([]integerPlane, len(p.e)), v: make([]homogeneousVertex, len(p.v))}
		for k := range p.e {
			q.e[k] = shiftPlane(p.e[k])
		}
		for k, v := range p.v {
			newX := new(big.Int).Add(v.x, new(big.Int).Mul(bigInt(offset[0]), v.w))
			newY := new(big.Int).Add(v.y, new(big.Int).Mul(bigInt(offset[1]), v.w))
			newZ := new(big.Int).Add(v.z, new(big.Int).Mul(bigInt(offset[2]), v.w))
			q.v[k] = homogeneousVertex{newX, newY, newZ, v.w, v.floatX + float64(offset[0]), v.floatY + float64(offset[1]), v.floatZ + float64(offset[2])}
		}
		out[i] = q
	}
	return out
}

// Mesh converts the exact polygon soup to a triangle mesh without any tolerance heuristics.
//
//  1. Vertices are identified by their (correctly rounded) coordinates, so one exact point is one
//     mesh vertex.
//  2. T-junctions are repaired exactly: a vertex belongs on a polygon edge iff it lies on both the
//     polygon's plane and the edge's plane (exact predicates) and between the edge's ends. The
//     float distance test below is only a spatial pre-filter and never decides anything.
//  3. Every exact polygon is strictly convex, so a fan is free of degenerate triangles; each fan
//     triangle is then split at the extra vertices found on its boundary edges.
func (s Solid) Mesh() *Mesh {
	m := &Mesh{}
	var homogeneousVertices []homogeneousVertex
	vertexIndexByCoordinate := map[[3]float64]int{}
	vertexIndexOf := func(h homogeneousVertex) int {
		k := [3]float64{h.floatX / lattice, h.floatY / lattice, h.floatZ / lattice}
		if id, ok := vertexIndexByCoordinate[k]; ok {
			return id
		}
		id := len(m.Vertices)
		vertexIndexByCoordinate[k] = id
		m.Vertices = append(m.Vertices, Vector3{k[0], k[1], k[2]})
		homogeneousVertices = append(homogeneousVertices, h)
		return id
	}
	polygonVertexIndices := make([][]int, len(s))
	for polygonIndex, p := range s {
		for _, h := range p.v {
			id := vertexIndexOf(h)
			if n := len(polygonVertexIndices[polygonIndex]); n == 0 || polygonVertexIndices[polygonIndex][n-1] != id {
				polygonVertexIndices[polygonIndex] = append(polygonVertexIndices[polygonIndex], id)
			}
		}
		for n := len(polygonVertexIndices[polygonIndex]); n > 1 && polygonVertexIndices[polygonIndex][0] == polygonVertexIndices[polygonIndex][n-1]; n = len(polygonVertexIndices[polygonIndex]) {
			polygonVertexIndices[polygonIndex] = polygonVertexIndices[polygonIndex][:n-1]
		}
	}
	const cell = 2.0
	grid := map[cellKey][]int{}
	for i, p := range m.Vertices {
		k := cellKeyOf(p, cell)
		grid[k] = append(grid[k], i)
	}
	const padding = 1e-6
	extraVerticesOf := func(p *exactPolygon, vertexIndices []int, i int) []int {
		n := len(vertexIndices)
		if len(p.v) != n { // consecutive duplicates were merged: edge planes no longer line up
			return nil
		}
		a, b := vertexIndices[i], vertexIndices[(i+1)%n]
		pointA, pointB := m.Vertices[a], m.Vertices[b]
		lo := Vector3{math.Min(pointA.X, pointB.X) - padding, math.Min(pointA.Y, pointB.Y) - padding, math.Min(pointA.Z, pointB.Z) - padding}
		hi := Vector3{math.Max(pointA.X, pointB.X) + padding, math.Max(pointA.Y, pointB.Y) + padding, math.Max(pointA.Z, pointB.Z) + padding}
		lowKey, highKey := cellKeyOf(lo, cell), cellKeyOf(hi, cell)
		edgeVector := pointB.Sub(pointA)
		edgeLengthSquared := edgeVector.Dot(edgeVector)
		type candidate struct {
			v int
			t float64
		}
		var candidates []candidate
		for x := lowKey[0]; x <= highKey[0]; x++ {
			for y := lowKey[1]; y <= highKey[1]; y++ {
				for z := lowKey[2]; z <= highKey[2]; z++ {
					for _, c := range grid[cellKey{x, y, z}] {
						if c == a || c == b {
							continue
						}
						q := m.Vertices[c]
						t := q.Sub(pointA).Dot(edgeVector) / edgeLengthSquared
						if t <= 0 || t >= 1 || q.Sub(pointA.Add(edgeVector.Scale(t))).Length() > padding {
							continue // pre-filter only
						}
						if homogeneousVertices[c].side(&p.e[i]) != 0 || homogeneousVertices[c].side(&p.p) != 0 {
							continue // exact test
						}
						candidates = append(candidates, candidate{c, t})
					}
				}
			}
		}
		for i := 1; i < len(candidates); i++ {
			for j := i; j > 0 && candidates[j].t < candidates[j-1].t; j-- {
				candidates[j], candidates[j-1] = candidates[j-1], candidates[j]
			}
		}
		out := make([]int, len(candidates))
		for i, c := range candidates {
			out[i] = c.v
		}
		return out
	}
	var subdivide func(a, b, c int, extrasAB, extrasBC, extrasCA []int)
	subdivide = func(a, b, c int, extrasAB, extrasBC, extrasCA []int) {
		switch {
		case len(extrasAB) > 0:
			e := extrasAB[0]
			subdivide(a, e, c, nil, nil, extrasCA)
			subdivide(e, b, c, extrasAB[1:], extrasBC, nil)
		case len(extrasBC) > 0:
			subdivide(b, c, a, extrasBC, extrasCA, extrasAB)
		case len(extrasCA) > 0:
			subdivide(c, a, b, extrasCA, extrasAB, extrasBC)
		default:
			m.Triangles = append(m.Triangles, Triangle{a, b, c})
		}
	}
	for polygonIndex, p := range s {
		vertexIndices := polygonVertexIndices[polygonIndex]
		n := len(vertexIndices)
		if n < 3 {
			continue
		}
		extraVertices := make([][]int, n)
		for i := range extraVertices {
			extraVertices[i] = extraVerticesOf(p, vertexIndices, i)
		}
		for i := 1; i+1 < n; i++ {
			var extrasAB, extrasCA []int
			if i == 1 {
				extrasAB = extraVertices[0]
			}
			if i+1 == n-1 {
				extrasCA = extraVertices[n-1]
			}
			subdivide(vertexIndices[0], vertexIndices[i], vertexIndices[i+1], extrasAB, extraVertices[i], extrasCA)
		}
	}
	return m
}
