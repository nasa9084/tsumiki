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

// ipl is the plane a*x + b*y + c*z = d with integer coefficients (lattice units).
type ipl struct{ a, b, c, d int64 }

func (p ipl) neg() ipl { return ipl{-p.a, -p.b, -p.c, -p.d} }

func gcd64(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	if a < 0 {
		a = -a
	}
	return a
}

func normPlane(a, b, c, d int64) ipl {
	g := gcd64(gcd64(a, b), gcd64(c, d))
	if g > 1 {
		a, b, c, d = a/g, b/g, c/g, d/g
	}
	return ipl{a, b, c, d}
}

// hvert is a point (x/w, y/w, z/w) with w > 0, plus its correctly rounded float coordinates.
type hvert struct {
	x, y, z, w *big.Int
	fx, fy, fz float64 // lattice units
}

func bi(x int64) *big.Int { return big.NewInt(x) }

func ratio(x, w *big.Int) float64 {
	f, _ := new(big.Rat).SetFrac(x, w).Float64()
	return f
}

func latticeVert(x, y, z int64) hvert {
	return hvert{bi(x), bi(y), bi(z), bi(1), float64(x), float64(y), float64(z)}
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
func vertexOf(p1, p2, p3 *ipl) (hvert, bool) {
	n1 := [3]*big.Int{bi(p1.a), bi(p1.b), bi(p1.c)}
	n2 := [3]*big.Int{bi(p2.a), bi(p2.b), bi(p2.c)}
	n3 := [3]*big.Int{bi(p3.a), bi(p3.b), bi(p3.c)}
	c23, c31, c12 := cross3(n2, n3), cross3(n3, n1), cross3(n1, n2)
	t := new(big.Int)
	w := new(big.Int)
	for i := 0; i < 3; i++ {
		w.Add(w, t.Mul(n1[i], c23[i]))
	}
	if w.Sign() == 0 {
		return hvert{}, false
	}
	d1, d2, d3 := bi(p1.d), bi(p2.d), bi(p3.d)
	var c [3]*big.Int
	for i := 0; i < 3; i++ {
		c[i] = new(big.Int).Mul(d1, c23[i])
		c[i].Add(c[i], t.Mul(d2, c31[i]))
		c[i].Add(c[i], t.Mul(d3, c12[i]))
	}
	if w.Sign() < 0 {
		w.Neg(w)
		for i := range c {
			c[i].Neg(c[i])
		}
	}
	return hvert{c[0], c[1], c[2], w, ratio(c[0], w), ratio(c[1], w), ratio(c[2], w)}, true
}

// side returns the sign of (a*x + b*y + c*z - d) at the vertex: +1 front, -1 back, 0 on the plane.
func (h *hvert) side(s *ipl) int {
	a, b, c, d := float64(s.a), float64(s.b), float64(s.c), float64(s.d)
	v := a*h.fx + b*h.fy + c*h.fz - d
	bound := (math.Abs(a*h.fx) + math.Abs(b*h.fy) + math.Abs(c*h.fz) + math.Abs(d)) * 1e-13
	if v > bound {
		return 1
	}
	if v < -bound {
		return -1
	}
	t := new(big.Int).Mul(h.x, bi(s.a))
	u := new(big.Int)
	t.Add(t, u.Mul(h.y, bi(s.b)))
	t.Add(t, u.Mul(h.z, bi(s.c)))
	t.Sub(t, u.Mul(h.w, bi(s.d)))
	return t.Sign()
}

// epoly is a convex polygon on plane p. Edge i runs from vertex i to vertex i+1 and lies on
// plane e[i]; vertex i is the intersection of p, e[i-1] and e[i].
type epoly struct {
	p ipl
	e []ipl
	v []hvert
}

// Solid is a closed polyhedral surface as a soup of exact convex polygons.
type Solid []*epoly

func snap(p V3) ([3]int64, error) {
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

func newTriPoly(q [3][3]int64) *epoly {
	sub := func(a, b [3]int64) [3]int64 { return [3]int64{a[0] - b[0], a[1] - b[1], a[2] - b[2]} }
	e1, e2 := sub(q[1], q[0]), sub(q[2], q[0])
	n := [3]int64{e1[1]*e2[2] - e1[2]*e2[1], e1[2]*e2[0] - e1[0]*e2[2], e1[0]*e2[1] - e1[1]*e2[0]}
	if n == [3]int64{} {
		return nil
	}
	d := n[0]*q[0][0] + n[1]*q[0][1] + n[2]*q[0][2]
	p := &epoly{p: normPlane(n[0], n[1], n[2], d)}
	ax := 0
	for k := 1; k < 3; k++ {
		if abs64(n[k]) > abs64(n[ax]) {
			ax = k
		}
	}
	for i := 0; i < 3; i++ {
		a, b := q[i], q[(i+1)%3]
		ev := sub(b, a)
		var ne [3]int64
		switch ax {
		case 2: // e x z
			ne = [3]int64{ev[1], -ev[0], 0}
		case 0: // e x x
			ne = [3]int64{0, ev[2], -ev[1]}
		default: // e x y
			ne = [3]int64{-ev[2], 0, ev[0]}
		}
		p.e = append(p.e, normPlane(ne[0], ne[1], ne[2], ne[0]*a[0]+ne[1]*a[1]+ne[2]*a[2]))
		p.v = append(p.v, latticeVert(a[0], a[1], a[2]))
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
	pts := make([][3]int64, len(m.V))
	for i, v := range m.V {
		p, err := snap(v)
		if err != nil {
			return nil, err
		}
		pts[i] = p
	}
	s := make(Solid, 0, len(m.T))
	for _, t := range m.T {
		if p := newTriPoly([3][3]int64{pts[t[0]], pts[t[1]], pts[t[2]]}); p != nil {
			s = append(s, p)
		}
	}
	return s, nil
}

func (p *epoly) flipped() *epoly {
	n := len(p.e)
	q := &epoly{p: p.p.neg(), e: make([]ipl, n), v: make([]hvert, n)}
	for j := 0; j < n; j++ {
		q.e[j] = p.e[n-1-j]
		q.v[j] = p.v[(n-j)%n]
	}
	return q
}

func sameDir(a, b ipl) bool {
	// planes may be exactly coplanar only when normals are parallel: compare along a nonzero axis
	switch {
	case a.a != 0:
		return (a.a > 0) == (b.a > 0)
	case a.b != 0:
		return (a.b > 0) == (b.b > 0)
	}
	return (a.c > 0) == (b.c > 0)
}

// splitPoly sorts p against plane s into coplanar-front, coplanar-back, front and back lists.
func splitPoly(s *ipl, p *epoly, cf, cb, f, b *[]*epoly) {
	n := len(p.v)
	var buf [16]int8
	tags := buf[:0]
	nf, nb := 0, 0
	for i := range p.v {
		t := p.v[i].side(s)
		tags = append(tags, int8(t))
		if t > 0 {
			nf++
		} else if t < 0 {
			nb++
		}
	}
	switch {
	case nf == 0 && nb == 0:
		if sameDir(p.p, *s) {
			*cf = append(*cf, p)
		} else {
			*cb = append(*cb, p)
		}
	case nb == 0:
		*f = append(*f, p)
	case nf == 0:
		*b = append(*b, p)
	default:
		if q := p.piece(s, tags, n, true); q != nil {
			*f = append(*f, q)
		}
		if q := p.piece(s, tags, n, false); q != nil {
			*b = append(*b, q)
		}
	}
}

// piece cuts out the part of p on the front (or back) of s. p must span s.
//
// The part keeps a contiguous run of p's edges (the first and last are truncated by s) plus
// s itself as the closing edge. The run starts at the edge that crosses (or leaves) s into the
// wanted side and ends at the edge that crosses back.
func (p *epoly) piece(s *ipl, tags []int8, n int, wantFront bool) *epoly {
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
	q := &epoly{p: p.p}
	var v0 hvert
	if tags[start] == 0 {
		v0 = p.v[start]
	} else {
		var ok bool
		if v0, ok = vertexOf(&p.p, s, &p.e[start]); !ok {
			return nil
		}
	}
	q.e = append(q.e, p.e[start])
	q.v = append(q.v, v0)
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
	var vout hvert
	if end := (last + 1) % n; tags[end] == 0 {
		vout = p.v[end]
	} else {
		var ok bool
		if vout, ok = vertexOf(&p.p, &p.e[last], s); !ok {
			return nil
		}
	}
	q.e = append(q.e, *s)
	q.v = append(q.v, vout)
	if len(q.e) < 3 {
		return nil
	}
	return q
}

type node struct {
	pl          *ipl
	front, back *node
	polys       []*epoly
}

func (n *node) invert() {
	for i := range n.polys {
		n.polys[i] = n.polys[i].flipped()
	}
	if n.pl != nil {
		f := n.pl.neg()
		n.pl = &f
	}
	if n.front != nil {
		n.front.invert()
	}
	if n.back != nil {
		n.back.invert()
	}
	n.front, n.back = n.back, n.front
}

func (n *node) clipPolys(ps []*epoly) []*epoly {
	if n.pl == nil {
		return append([]*epoly(nil), ps...)
	}
	var f, b []*epoly
	for _, p := range ps {
		splitPoly(n.pl, p, &f, &b, &f, &b)
	}
	if n.front != nil {
		f = n.front.clipPolys(f)
	}
	if n.back != nil {
		b = n.back.clipPolys(b)
	} else {
		b = nil
	}
	return append(f, b...)
}

func (n *node) clipTo(o *node) {
	n.polys = o.clipPolys(n.polys)
	if n.front != nil {
		n.front.clipTo(o)
	}
	if n.back != nil {
		n.back.clipTo(o)
	}
}

func (n *node) allPolys() []*epoly {
	ps := append([]*epoly(nil), n.polys...)
	if n.front != nil {
		ps = append(ps, n.front.allPolys()...)
	}
	if n.back != nil {
		ps = append(ps, n.back.allPolys()...)
	}
	return ps
}

func (n *node) build(ps []*epoly) {
	if len(ps) == 0 {
		return
	}
	if n.pl == nil {
		pl := ps[pickSplitter(ps)].p
		n.pl = &pl
	}
	var f, b []*epoly
	for _, p := range ps {
		splitPoly(n.pl, p, &n.polys, &n.polys, &f, &b)
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
func pickSplitter(ps []*epoly) int {
	if len(ps) <= 4 {
		return 0
	}
	const cands = 8
	cstep := max(1, len(ps)/cands)
	sstep := max(1, len(ps)/48)
	best, bestScore := 0, 1<<60
	for c := 0; c < len(ps); c += cstep {
		pl := &ps[c].p
		nf, nb, ns := 0, 0, 0
		for i := 0; i < len(ps); i += sstep {
			hasF, hasB := false, false
			for k := range ps[i].v {
				switch ps[i].v[k].side(pl) {
				case 1:
					hasF = true
				case -1:
					hasB = true
				}
			}
			switch {
			case hasF && hasB:
				ns++
			case hasF:
				nf++
			case hasB:
				nb++
			}
		}
		d := nf - nb
		if d < 0 {
			d = -d
		}
		if score := ns*8 + d; score < bestScore {
			best, bestScore = c, score
		}
	}
	return best
}

// BoolOp selects a boolean operation.
type BoolOp int

const (
	OpJoin BoolOp = iota
	OpCut
	OpIntersect
)

func (o BoolOp) String() string { return [...]string{"join", "cut", "intersect"}[o] }

// BooleanSolid combines a and b exactly (neither is modified).
func BooleanSolid(op BoolOp, a, b Solid) Solid {
	A, B := &node{}, &node{}
	A.build(append([]*epoly(nil), a...))
	B.build(append([]*epoly(nil), b...))
	switch op {
	case OpJoin:
		A.clipTo(B)
		B.clipTo(A)
		B.invert()
		B.clipTo(A)
		B.invert()
		A.build(B.allPolys())
	case OpCut:
		A.invert()
		A.clipTo(B)
		B.clipTo(A)
		B.invert()
		B.clipTo(A)
		B.invert()
		A.build(B.allPolys())
		A.invert()
	case OpIntersect:
		A.invert()
		B.clipTo(A)
		B.invert()
		A.clipTo(B)
		B.clipTo(A)
		A.build(B.allPolys())
		A.invert()
	}
	return Solid(A.allPolys())
}

// translate shifts the solid by whole lattice units, exactly.
func (s Solid) translate(t [3]int64) Solid {
	shiftPlane := func(p ipl) ipl { return ipl{p.a, p.b, p.c, p.d + p.a*t[0] + p.b*t[1] + p.c*t[2]} }
	out := make(Solid, len(s))
	for i, p := range s {
		q := &epoly{p: shiftPlane(p.p), e: make([]ipl, len(p.e)), v: make([]hvert, len(p.v))}
		for k := range p.e {
			q.e[k] = shiftPlane(p.e[k])
		}
		for k, v := range p.v {
			nx := new(big.Int).Add(v.x, new(big.Int).Mul(bi(t[0]), v.w))
			ny := new(big.Int).Add(v.y, new(big.Int).Mul(bi(t[1]), v.w))
			nz := new(big.Int).Add(v.z, new(big.Int).Mul(bi(t[2]), v.w))
			q.v[k] = hvert{nx, ny, nz, v.w, v.fx + float64(t[0]), v.fy + float64(t[1]), v.fz + float64(t[2])}
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
	var hv []hvert
	vid := map[[3]float64]int{}
	get := func(h hvert) int {
		k := [3]float64{h.fx / lattice, h.fy / lattice, h.fz / lattice}
		if id, ok := vid[k]; ok {
			return id
		}
		id := len(m.V)
		vid[k] = id
		m.V = append(m.V, V3{k[0], k[1], k[2]})
		hv = append(hv, h)
		return id
	}
	ids := make([][]int, len(s))
	for pi, p := range s {
		for _, h := range p.v {
			id := get(h)
			if n := len(ids[pi]); n == 0 || ids[pi][n-1] != id {
				ids[pi] = append(ids[pi], id)
			}
		}
		for n := len(ids[pi]); n > 1 && ids[pi][0] == ids[pi][n-1]; n = len(ids[pi]) {
			ids[pi] = ids[pi][:n-1]
		}
	}
	const cell = 2.0
	grid := map[cellKey][]int{}
	for i, p := range m.V {
		k := keyOf(p, cell)
		grid[k] = append(grid[k], i)
	}
	const pad = 1e-6
	extrasOf := func(p *epoly, vs []int, i int) []int {
		n := len(vs)
		if len(p.v) != n { // consecutive duplicates were merged: edge planes no longer line up
			return nil
		}
		a, b := vs[i], vs[(i+1)%n]
		pa, pb := m.V[a], m.V[b]
		lo := V3{math.Min(pa.X, pb.X) - pad, math.Min(pa.Y, pb.Y) - pad, math.Min(pa.Z, pb.Z) - pad}
		hi := V3{math.Max(pa.X, pb.X) + pad, math.Max(pa.Y, pb.Y) + pad, math.Max(pa.Z, pb.Z) + pad}
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
					for _, c := range grid[cellKey{x, y, z}] {
						if c == a || c == b {
							continue
						}
						q := m.V[c]
						t := q.Sub(pa).Dot(d) / dd
						if t <= 0 || t >= 1 || q.Sub(pa.Add(d.Mul(t))).Len() > pad {
							continue // pre-filter only
						}
						if hv[c].side(&p.e[i]) != 0 || hv[c].side(&p.p) != 0 {
							continue // exact test
						}
						cs = append(cs, cand{c, t})
					}
				}
			}
		}
		for i := 1; i < len(cs); i++ {
			for j := i; j > 0 && cs[j].t < cs[j-1].t; j-- {
				cs[j], cs[j-1] = cs[j-1], cs[j]
			}
		}
		out := make([]int, len(cs))
		for i, c := range cs {
			out[i] = c.v
		}
		return out
	}
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
			m.T = append(m.T, Tri{a, b, c})
		}
	}
	for pi, p := range s {
		vs := ids[pi]
		n := len(vs)
		if n < 3 {
			continue
		}
		ex := make([][]int, n)
		for i := range ex {
			ex[i] = extrasOf(p, vs, i)
		}
		for i := 1; i+1 < n; i++ {
			var eab, eca []int
			if i == 1 {
				eab = ex[0]
			}
			if i+1 == n-1 {
				eca = ex[n-1]
			}
			sub(vs[0], vs[i], vs[i+1], eab, ex[i], eca)
		}
	}
	return m
}
