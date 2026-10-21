package tsumiki

import (
	"math"
)

// Tri indexes three vertices, counter-clockwise seen from outside.
type Tri [3]int

// Mesh is an indexed triangle mesh.
type Mesh struct {
	V []V3
	T []Tri
}

// Volume returns the signed volume (positive for outward-facing windings).
func (m *Mesh) Volume() float64 {
	v := 0.0
	for _, t := range m.T {
		v += m.V[t[0]].Dot(m.V[t[1]].Cross(m.V[t[2]]))
	}
	return v / 6
}

// Bounds returns the axis-aligned bounding box.
func (m *Mesh) Bounds() (lo, hi V3) {
	if len(m.V) == 0 {
		return
	}
	lo = V3{math.Inf(1), math.Inf(1), math.Inf(1)}
	hi = lo.Neg()
	for _, p := range m.V {
		lo = V3{math.Min(lo.X, p.X), math.Min(lo.Y, p.Y), math.Min(lo.Z, p.Z)}
		hi = V3{math.Max(hi.X, p.X), math.Max(hi.Y, p.Y), math.Max(hi.Z, p.Z)}
	}
	return
}

// Clone deep-copies the mesh.
func (m *Mesh) Clone() *Mesh {
	return &Mesh{V: append([]V3(nil), m.V...), T: append([]Tri(nil), m.T...)}
}

func (m *Mesh) flip() {
	for i, t := range m.T {
		m.T[i] = Tri{t[0], t[2], t[1]}
	}
}

// Report summarises mesh health.
type Report struct {
	Tris        int
	Verts       int
	Volume      float64
	OpenEdges   int // directed edges without an opposite partner (cracks)
	NonManifold int // directed edges used more than once
	Degenerate  int
}

// Watertight is true for a closed, consistently oriented 2-manifold.
func (r Report) Watertight() bool { return r.OpenEdges == 0 && r.NonManifold == 0 && r.Degenerate == 0 }

// Validate inspects topology.
func (m *Mesh) Validate() Report {
	r := Report{Tris: len(m.T), Verts: len(m.V), Volume: m.Volume()}
	edges := map[[2]int]int{}
	for _, t := range m.T {
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
	weldEps  = 1e-6 // merge vertices closer than this (mm)
	tjuncEps = 1e-6 // a vertex this close to an edge is treated as lying on it
	flatEps  = 5e-7 // triangles thinner than this are slivers and are dropped
)

type cellKey [3]int32

func keyOf(p V3, cell float64) cellKey {
	return cellKey{int32(math.Floor(p.X / cell)), int32(math.Floor(p.Y / cell)), int32(math.Floor(p.Z / cell))}
}

// Weld merges vertices closer than eps and drops unused vertices.
func (m *Mesh) Weld(eps float64) {
	cell := eps * 4
	grid := map[cellKey][]int{}
	remap := make([]int, len(m.V))
	var nv []V3
	for i, p := range m.V {
		k := keyOf(p, cell)
		found := -1
	search:
		for dx := int32(-1); dx <= 1; dx++ {
			for dy := int32(-1); dy <= 1; dy++ {
				for dz := int32(-1); dz <= 1; dz++ {
					for _, j := range grid[cellKey{k[0] + dx, k[1] + dy, k[2] + dz}] {
						if nv[j].Sub(p).Len() <= eps {
							found = j
							break search
						}
					}
				}
			}
		}
		if found < 0 {
			found = len(nv)
			nv = append(nv, p)
			grid[k] = append(grid[k], found)
		}
		remap[i] = found
	}
	for i, t := range m.T {
		m.T[i] = Tri{remap[t[0]], remap[t[1]], remap[t[2]]}
	}
	m.V = nv
}

func triArea(a, b, c V3) float64 { return b.Sub(a).Cross(c.Sub(a)).Len() / 2 }

// dropRepeated removes triangles that use the same vertex twice.
func (m *Mesh) dropRepeated() {
	out := m.T[:0]
	for _, t := range m.T {
		if t[0] != t[1] && t[1] != t[2] && t[2] != t[0] {
			out = append(out, t)
		}
	}
	m.T = out
}

func (m *Mesh) dropDegenerate() {
	out := m.T[:0]
	for _, t := range m.T {
		if t[0] == t[1] || t[1] == t[2] || t[2] == t[0] {
			continue
		}
		if triArea(m.V[t[0]], m.V[t[1]], m.V[t[2]]) < 1e-12 {
			continue
		}
		out = append(out, t)
	}
	m.T = out
}

// dropInternal removes pairs of coincident triangles with opposite orientation
// (internal faces left behind where two solids touch).
func (m *Mesh) dropInternal() {
	type key [3]int
	canon := func(t Tri) key { // rotate so the smallest index comes first (keeps orientation)
		a, b, c := t[0], t[1], t[2]
		if b < a && b < c {
			a, b, c = b, c, a
		} else if c < a && c < b {
			a, b, c = c, a, b
		}
		return key{a, b, c}
	}
	seen := map[key][]int{}
	for i, t := range m.T {
		k := canon(t)
		seen[k] = append(seen[k], i)
	}
	dead := make([]bool, len(m.T))
	for i, t := range m.T {
		if dead[i] {
			continue
		}
		k := canon(t)
		for _, j := range seen[key{k[0], k[2], k[1]}] { // same vertices, opposite orientation
			if j != i && !dead[j] {
				dead[i], dead[j] = true, true
				break
			}
		}
	}
	out := m.T[:0]
	for i, t := range m.T {
		if !dead[i] {
			out = append(out, t)
		}
	}
	m.T = out
}

var debugTJ func(m *Mesh, orig Tri, subs []Tri)

// fixTJunctions splits triangle edges at vertices that lie on them (within eps).
func (m *Mesh) fixTJunctions(eps float64) {
	const cell = 2.0
	grid := map[cellKey][]int{}
	for i, p := range m.V {
		k := keyOf(p, cell)
		grid[k] = append(grid[k], i)
	}
	type ekey [2]int
	extras := map[ekey][]int{} // keyed by (min,max); values ordered from min to max
	done := map[ekey]bool{}
	for _, t := range m.T {
		for k := 0; k < 3; k++ {
			a, b := t[k], t[(k+1)%3]
			if a > b {
				a, b = b, a
			}
			ek := ekey{a, b}
			if done[ek] {
				continue
			}
			done[ek] = true
			pa, pb := m.V[a], m.V[b]
			lo := V3{math.Min(pa.X, pb.X) - eps, math.Min(pa.Y, pb.Y) - eps, math.Min(pa.Z, pb.Z) - eps}
			hi := V3{math.Max(pa.X, pb.X) + eps, math.Max(pa.Y, pb.Y) + eps, math.Max(pa.Z, pb.Z) + eps}
			klo, khi := keyOf(lo, cell), keyOf(hi, cell)
			d := pb.Sub(pa)
			dd := d.Dot(d)
			type cand struct {
				v int
				t float64
			}
			var cs []cand
			for x := klo[0]; x <= khi[0]; x++ {
				for y := klo[1]; y <= khi[1]; y++ {
					for z := klo[2]; z <= khi[2]; z++ {
						for _, v := range grid[cellKey{x, y, z}] {
							if v == a || v == b {
								continue
							}
							p := m.V[v]
							tt := p.Sub(pa).Dot(d) / dd
							if tt <= 0 || tt >= 1 {
								continue
							}
							if p.Sub(pa.Add(d.Mul(tt))).Len() > eps {
								continue
							}
							if p.Sub(pa).Len() <= eps || p.Sub(pb).Len() <= eps {
								continue
							}
							cs = append(cs, cand{v, tt})
						}
					}
				}
			}
			if len(cs) == 0 {
				continue
			}
			for i := 1; i < len(cs); i++ { // insertion sort by t
				for j := i; j > 0 && cs[j].t < cs[j-1].t; j-- {
					cs[j], cs[j-1] = cs[j-1], cs[j]
				}
			}
			vs := make([]int, len(cs))
			for i, c := range cs {
				vs[i] = c.v
			}
			extras[ek] = vs
		}
	}
	if len(extras) == 0 {
		return
	}
	edgeExtras := func(a, b int) []int {
		if a < b {
			return extras[ekey{a, b}]
		}
		e := extras[ekey{b, a}]
		r := make([]int, len(e))
		for i := range e {
			r[len(e)-1-i] = e[i]
		}
		return r
	}
	var out []Tri
	var sub func(a, b, c int, eab, ebc, eca []int)
	sub = func(a, b, c int, eab, ebc, eca []int) {
		switch {
		case len(eab) > 0:
			e := eab[0]
			sub(a, e, c, nil, nil, eca)
			sub(e, b, c, eab[1:], ebc, nil)
		case len(ebc) > 0:
			sub(b, c, a, ebc, eca, eab)
		case len(eca) > 0:
			sub(c, a, b, eca, eab, ebc)
		default:
			out = append(out, Tri{a, b, c})
		}
	}
	for _, t := range m.T {
		n0 := len(out)
		without := func(vs []int, corner int) []int { // a triangle's own corner is never an "extra" on its edges
			for _, v := range vs {
				if v == corner {
					r := make([]int, 0, len(vs))
					for _, w := range vs {
						if w != corner {
							r = append(r, w)
						}
					}
					return r
				}
			}
			return vs
		}
		sub(t[0], t[1], t[2], without(edgeExtras(t[0], t[1]), t[2]), without(edgeExtras(t[1], t[2]), t[0]), without(edgeExtras(t[2], t[0]), t[1]))
		if debugTJ != nil {
			debugTJ(m, t, out[n0:])
		}
	}
	m.T = out
}

// dropFlat removes slivers: triangles whose height over their longest edge is below flatEps.
// Together with T-junction repair this is exactly right: a flat triangle's neighbours have just
// been split at its apex, so dropping it closes the surface again.
func (m *Mesh) dropFlat() {
	out := m.T[:0]
	for _, t := range m.T {
		a, b, c := m.V[t[0]], m.V[t[1]], m.V[t[2]]
		l := math.Max(b.Sub(a).Len(), math.Max(c.Sub(b).Len(), a.Sub(c).Len()))
		if l == 0 || b.Sub(a).Cross(c.Sub(a)).Len()/l < flatEps {
			continue
		}
		out = append(out, t)
	}
	m.T = out
}

// Cleanup welds vertices, repairs T-junctions, and removes slivers and internal faces.
func (m *Mesh) Cleanup() {
	m.Weld(weldEps)
	m.dropRepeated() // must precede T-junction repair: collapsed triangles would be "split" into garbage
	for pass := 0; pass < 4; pass++ {
		n := len(m.T)
		m.fixTJunctions(tjuncEps)
		m.dropRepeated()
		m.dropFlat()
		m.dropInternal()
		if len(m.T) == n {
			break
		}
	}
	// drop vertices no longer referenced
	used := make([]int, len(m.V))
	for i := range used {
		used[i] = -1
	}
	var nv []V3
	for i, t := range m.T {
		for k := 0; k < 3; k++ {
			if used[t[k]] < 0 {
				used[t[k]] = len(nv)
				nv = append(nv, m.V[t[k]])
			}
			m.T[i][k] = used[t[k]]
		}
	}
	m.V = nv
}

// merge appends other (indices are shifted).
func (m *Mesh) merge(o *Mesh) {
	off := len(m.V)
	m.V = append(m.V, o.V...)
	for _, t := range o.T {
		m.T = append(m.T, Tri{t[0] + off, t[1] + off, t[2] + off})
	}
}

// Transform applies f to every vertex; flips winding when mirror is true.
func (m *Mesh) transform(f func(V3) V3, mirror bool) {
	for i, p := range m.V {
		m.V[i] = f(p)
	}
	if mirror {
		m.flip()
	}
}
