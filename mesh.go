package tsumiki

import (
	"math"
)

// Triangle indexes three vertices, counter-clockwise seen from outside.
type Triangle [3]int

// Mesh is an indexed triangle mesh.
type Mesh struct {
	Vertices  []Vector3
	Triangles []Triangle
}

// Volume returns the signed volume (positive for outward-facing windings).
func (m *Mesh) Volume() float64 {
	v := 0.0
	for _, t := range m.Triangles {
		v += m.Vertices[t[0]].Dot(m.Vertices[t[1]].Cross(m.Vertices[t[2]]))
	}
	return v / 6
}

// Bounds returns the axis-aligned bounding box.
func (m *Mesh) Bounds() (lo, hi Vector3) {
	if len(m.Vertices) == 0 {
		return
	}
	lo = Vector3{math.Inf(1), math.Inf(1), math.Inf(1)}
	hi = lo.Negate()
	for _, p := range m.Vertices {
		lo = Vector3{math.Min(lo.X, p.X), math.Min(lo.Y, p.Y), math.Min(lo.Z, p.Z)}
		hi = Vector3{math.Max(hi.X, p.X), math.Max(hi.Y, p.Y), math.Max(hi.Z, p.Z)}
	}
	return
}

// Clone deep-copies the mesh.
func (m *Mesh) Clone() *Mesh {
	return &Mesh{Vertices: append([]Vector3(nil), m.Vertices...), Triangles: append([]Triangle(nil), m.Triangles...)}
}

func (m *Mesh) flip() {
	for i, t := range m.Triangles {
		m.Triangles[i] = Triangle{t[0], t[2], t[1]}
	}
}

// Report summarises mesh health.
type Report struct {
	Triangles   int
	Vertices    int
	Volume      float64
	OpenEdges   int // directed edges without an opposite partner (cracks)
	NonManifold int // directed edges used more than once
	Degenerate  int
}

// Watertight is true for a closed, consistently oriented 2-manifold.
func (r Report) Watertight() bool { return r.OpenEdges == 0 && r.NonManifold == 0 && r.Degenerate == 0 }

// Validate inspects topology.
func (m *Mesh) Validate() Report {
	r := Report{Triangles: len(m.Triangles), Vertices: len(m.Vertices), Volume: m.Volume()}
	edges := map[[2]int]int{}
	for _, t := range m.Triangles {
		if t[0] == t[1] || t[1] == t[2] || t[2] == t[0] {
			r.Degenerate++
			continue
		}
		for k := 0; k < 3; k++ {
			edges[[2]int{t[k], t[(k+1)%3]}]++
		}
	}
	for e, c := range edges {
		if c > 1 {
			r.NonManifold++
		}
		if edges[[2]int{e[1], e[0]}] == 0 {
			r.OpenEdges++
		}
	}
	return r
}

// ---- cleaning: weld, T-junction repair, degenerate / internal-face removal ----

const (
	weldEpsilon      = 1e-6 // merge vertices closer than this (mm)
	tJunctionEpsilon = 1e-6 // a vertex this close to an edge is treated as lying on it
	flatEpsilon      = 5e-7 // triangles thinner than this are slivers and are dropped
)

type cellKey [3]int32

func cellKeyOf(p Vector3, cell float64) cellKey {
	return cellKey{int32(math.Floor(p.X / cell)), int32(math.Floor(p.Y / cell)), int32(math.Floor(p.Z / cell))}
}

// Weld merges vertices closer than epsilon and drops unused vertices.
func (m *Mesh) Weld(epsilon float64) {
	cell := epsilon * 4
	grid := map[cellKey][]int{}
	remap := make([]int, len(m.Vertices))
	var newVertices []Vector3
	for i, p := range m.Vertices {
		k := cellKeyOf(p, cell)
		found := -1
	search:
		for dx := int32(-1); dx <= 1; dx++ {
			for dy := int32(-1); dy <= 1; dy++ {
				for dz := int32(-1); dz <= 1; dz++ {
					for _, j := range grid[cellKey{k[0] + dx, k[1] + dy, k[2] + dz}] {
						if newVertices[j].Sub(p).Length() <= epsilon {
							found = j
							break search
						}
					}
				}
			}
		}
		if found < 0 {
			found = len(newVertices)
			newVertices = append(newVertices, p)
			grid[k] = append(grid[k], found)
		}
		remap[i] = found
	}
	for i, t := range m.Triangles {
		m.Triangles[i] = Triangle{remap[t[0]], remap[t[1]], remap[t[2]]}
	}
	m.Vertices = newVertices
}

func triangleArea(a, b, c Vector3) float64 { return b.Sub(a).Cross(c.Sub(a)).Length() / 2 }

// dropRepeated removes triangles that use the same vertex twice.
func (m *Mesh) dropRepeated() {
	out := m.Triangles[:0]
	for _, t := range m.Triangles {
		if t[0] != t[1] && t[1] != t[2] && t[2] != t[0] {
			out = append(out, t)
		}
	}
	m.Triangles = out
}

func (m *Mesh) dropDegenerate() {
	out := m.Triangles[:0]
	for _, t := range m.Triangles {
		if t[0] == t[1] || t[1] == t[2] || t[2] == t[0] {
			continue
		}
		if triangleArea(m.Vertices[t[0]], m.Vertices[t[1]], m.Vertices[t[2]]) < 1e-12 {
			continue
		}
		out = append(out, t)
	}
	m.Triangles = out
}

// dropInternal removes pairs of coincident triangles with opposite orientation
// (internal faces left behind where two solids touch).
func (m *Mesh) dropInternal() {
	type key [3]int
	canonical := func(t Triangle) key { // rotate so the smallest index comes first (keeps orientation)
		a, b, c := t[0], t[1], t[2]
		if b < a && b < c {
			a, b, c = b, c, a
		} else if c < a && c < b {
			a, b, c = c, a, b
		}
		return key{a, b, c}
	}
	seen := map[key][]int{}
	for i, t := range m.Triangles {
		k := canonical(t)
		seen[k] = append(seen[k], i)
	}
	dead := make([]bool, len(m.Triangles))
	for i, t := range m.Triangles {
		if dead[i] {
			continue
		}
		k := canonical(t)
		for _, j := range seen[key{k[0], k[2], k[1]}] { // same vertices, opposite orientation
			if j != i && !dead[j] {
				dead[i], dead[j] = true, true
				break
			}
		}
	}
	out := m.Triangles[:0]
	for i, t := range m.Triangles {
		if !dead[i] {
			out = append(out, t)
		}
	}
	m.Triangles = out
}

var debugTJunctions func(m *Mesh, original Triangle, pieces []Triangle)

// fixTJunctions splits triangle edges at vertices that lie on them (within epsilon).
func (m *Mesh) fixTJunctions(epsilon float64) {
	const cell = 2.0
	grid := map[cellKey][]int{}
	for i, p := range m.Vertices {
		k := cellKeyOf(p, cell)
		grid[k] = append(grid[k], i)
	}
	type edgeKey [2]int
	extras := map[edgeKey][]int{} // keyed by (min,max); values ordered from min to max
	done := map[edgeKey]bool{}
	for _, t := range m.Triangles {
		for k := 0; k < 3; k++ {
			a, b := t[k], t[(k+1)%3]
			if a > b {
				a, b = b, a
			}
			edge := edgeKey{a, b}
			if done[edge] {
				continue
			}
			done[edge] = true
			pointA, pointB := m.Vertices[a], m.Vertices[b]
			lo := Vector3{math.Min(pointA.X, pointB.X) - epsilon, math.Min(pointA.Y, pointB.Y) - epsilon, math.Min(pointA.Z, pointB.Z) - epsilon}
			hi := Vector3{math.Max(pointA.X, pointB.X) + epsilon, math.Max(pointA.Y, pointB.Y) + epsilon, math.Max(pointA.Z, pointB.Z) + epsilon}
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
						for _, v := range grid[cellKey{x, y, z}] {
							if v == a || v == b {
								continue
							}
							p := m.Vertices[v]
							fraction := p.Sub(pointA).Dot(edgeVector) / edgeLengthSquared
							if fraction <= 0 || fraction >= 1 {
								continue
							}
							if p.Sub(pointA.Add(edgeVector.Scale(fraction))).Length() > epsilon {
								continue
							}
							if p.Sub(pointA).Length() <= epsilon || p.Sub(pointB).Length() <= epsilon {
								continue
							}
							candidates = append(candidates, candidate{v, fraction})
						}
					}
				}
			}
			if len(candidates) == 0 {
				continue
			}
			for i := 1; i < len(candidates); i++ { // insertion sort by t
				for j := i; j > 0 && candidates[j].t < candidates[j-1].t; j-- {
					candidates[j], candidates[j-1] = candidates[j-1], candidates[j]
				}
			}
			vertexIndices := make([]int, len(candidates))
			for i, c := range candidates {
				vertexIndices[i] = c.v
			}
			extras[edge] = vertexIndices
		}
	}
	if len(extras) == 0 {
		return
	}
	edgeExtras := func(a, b int) []int {
		if a < b {
			return extras[edgeKey{a, b}]
		}
		e := extras[edgeKey{b, a}]
		r := make([]int, len(e))
		for i := range e {
			r[len(e)-1-i] = e[i]
		}
		return r
	}
	var out []Triangle
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
			out = append(out, Triangle{a, b, c})
		}
	}
	for _, t := range m.Triangles {
		startLength := len(out)
		without := func(vertexIndices []int, corner int) []int { // a triangle's own corner is never an "extra" on its edges
			for _, v := range vertexIndices {
				if v == corner {
					r := make([]int, 0, len(vertexIndices))
					for _, w := range vertexIndices {
						if w != corner {
							r = append(r, w)
						}
					}
					return r
				}
			}
			return vertexIndices
		}
		subdivide(t[0], t[1], t[2], without(edgeExtras(t[0], t[1]), t[2]), without(edgeExtras(t[1], t[2]), t[0]), without(edgeExtras(t[2], t[0]), t[1]))
		if debugTJunctions != nil {
			debugTJunctions(m, t, out[startLength:])
		}
	}
	m.Triangles = out
}

// dropFlat removes slivers: triangles whose height over their longest edge is below flatEps.
// Together with T-junction repair this is exactly right: a flat triangle's neighbours have just
// been split at its apex, so dropping it closes the surface again.
func (m *Mesh) dropFlat() {
	out := m.Triangles[:0]
	for _, t := range m.Triangles {
		a, b, c := m.Vertices[t[0]], m.Vertices[t[1]], m.Vertices[t[2]]
		longestEdge := math.Max(b.Sub(a).Length(), math.Max(c.Sub(b).Length(), a.Sub(c).Length()))
		if longestEdge == 0 || b.Sub(a).Cross(c.Sub(a)).Length()/longestEdge < flatEpsilon {
			continue
		}
		out = append(out, t)
	}
	m.Triangles = out
}

// Cleanup welds vertices, repairs T-junctions, and removes slivers and internal faces.
func (m *Mesh) Cleanup() {
	m.Weld(weldEpsilon)
	m.dropRepeated() // must precede T-junction repair: collapsed triangles would be "split" into garbage
	for pass := 0; pass < 4; pass++ {
		n := len(m.Triangles)
		m.fixTJunctions(tJunctionEpsilon)
		m.dropRepeated()
		m.dropFlat()
		m.dropInternal()
		if len(m.Triangles) == n {
			break
		}
	}
	// drop vertices no longer referenced
	used := make([]int, len(m.Vertices))
	for i := range used {
		used[i] = -1
	}
	var newVertices []Vector3
	for i, t := range m.Triangles {
		for k := 0; k < 3; k++ {
			if used[t[k]] < 0 {
				used[t[k]] = len(newVertices)
				newVertices = append(newVertices, m.Vertices[t[k]])
			}
			m.Triangles[i][k] = used[t[k]]
		}
	}
	m.Vertices = newVertices
}

// merge appends other (indices are shifted).
func (m *Mesh) merge(other *Mesh) {
	offset := len(m.Vertices)
	m.Vertices = append(m.Vertices, other.Vertices...)
	for _, t := range other.Triangles {
		m.Triangles = append(m.Triangles, Triangle{t[0] + offset, t[1] + offset, t[2] + offset})
	}
}

// Transform applies f to every vertex; flips winding when mirror is true.
func (m *Mesh) transform(transformVertex func(Vector3) Vector3, mirror bool) {
	for i, p := range m.Vertices {
		m.Vertices[i] = transformVertex(p)
	}
	if mirror {
		m.flip()
	}
}
