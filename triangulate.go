package tsumiki

import (
	"math"
	"sort"
)

// triangulateRegion triangulates a region (outer CCW, holes CW).
// pts is outer followed by each hole in the given order; triangles index into pts
// and are counter-clockwise. Holes are bridged into the outer ring (Eberly), then ear clipped.
func triangulateRegion(outer Loop, holes []Loop) ([]V2, [][3]int) {
	pts := append([]V2(nil), outer...)
	ring := make([]int, len(outer))
	for i := range ring {
		ring[i] = i
	}
	type hole struct {
		idx  []int
		maxX float64
		mi   int
	}
	var hs []hole
	for _, h := range holes {
		base := len(pts)
		pts = append(pts, h...)
		idx := make([]int, len(h))
		mi := 0
		for i := range h {
			idx[i] = base + i
			if h[i].X > h[mi].X {
				mi = i
			}
		}
		hs = append(hs, hole{idx, h[mi].X, mi})
	}
	sort.SliceStable(hs, func(i, j int) bool { return hs[i].maxX > hs[j].maxX })
	for _, h := range hs {
		ring = bridgeHole(pts, ring, h.idx, h.mi)
	}
	return pts, earClip(pts, ring)
}

func tri2(a, b, c V2) float64 { return b.Sub(a).Cross(c.Sub(a)) }

func inTriInclusive(a, b, c, p V2) bool {
	d1, d2, d3 := tri2(a, b, p), tri2(b, c, p), tri2(c, a, p)
	neg := d1 < 0 || d2 < 0 || d3 < 0
	pos := d1 > 0 || d2 > 0 || d3 > 0
	return !(neg && pos)
}

// locallyInside reports whether direction d from ring[j] points into the polygon interior (CCW ring).
func locallyInside(pts []V2, ring []int, j int, target V2) bool {
	n := len(ring)
	v := pts[ring[j]]
	p := pts[ring[(j-1+n)%n]]
	nx := pts[ring[(j+1)%n]]
	d := target.Sub(v)
	a, b := nx.Sub(v), p.Sub(v)
	if a.Cross(b) >= 0 { // convex (or straight) vertex
		return a.Cross(d) >= 0 && d.Cross(b) >= 0
	}
	return !(b.Cross(d) >= 0 && d.Cross(a) >= 0)
}

func bridgeHole(pts []V2, ring, hole []int, mi int) []int {
	M := pts[hole[mi]]
	n := len(ring)
	bestX, bestEdge := math.Inf(1), -1
	for i := 0; i < n; i++ {
		a, b := pts[ring[i]], pts[ring[(i+1)%n]]
		if a.Y == b.Y {
			continue
		}
		if (a.Y <= M.Y && b.Y >= M.Y) || (a.Y >= M.Y && b.Y <= M.Y) {
			x := a.X + (M.Y-a.Y)*(b.X-a.X)/(b.Y-a.Y)
			if x >= M.X && x < bestX {
				bestX, bestEdge = x, i
			}
		}
	}
	pi := 0
	if bestEdge < 0 {
		// fall back to the nearest ring vertex
		bd := math.Inf(1)
		for j := 0; j < n; j++ {
			if d := pts[ring[j]].Sub(M).Len(); d < bd {
				bd, pi = d, j
			}
		}
	} else {
		I := V2{bestX, M.Y}
		i0, i1 := bestEdge, (bestEdge+1)%n
		a, b := pts[ring[i0]], pts[ring[i1]]
		switch {
		case I.Sub(a).Len() < 1e-12:
			pi = i0
		case I.Sub(b).Len() < 1e-12:
			pi = i1
		case b.X > a.X:
			pi = i1
		default:
			pi = i0
		}
		P := pts[ring[pi]]
		// a reflex vertex inside triangle (M, I, P) blocks the view: pick the one with the smallest angle
		bestTan := math.Inf(1)
		for j := 0; j < n; j++ {
			if ring[j] == ring[pi] {
				continue
			}
			q := pts[ring[j]]
			prev, next := pts[ring[(j-1+n)%n]], pts[ring[(j+1)%n]]
			if tri2(prev, q, next) >= 0 { // not reflex
				continue
			}
			t1, t2, t3 := M, I, P
			if tri2(t1, t2, t3) < 0 {
				t2, t3 = t3, t2
			}
			if !inTriInclusive(t1, t2, t3, q) || q.X <= M.X {
				continue
			}
			tan := math.Abs(q.Y-M.Y) / (q.X - M.X)
			if tan < bestTan {
				bestTan, pi = tan, j
			}
		}
	}
	// the chosen vertex may occur several times in the ring: use the occurrence that sees M
	for j := 0; j < n; j++ {
		if ring[j] == ring[pi] && locallyInside(pts, ring, j, M) {
			pi = j
			break
		}
	}
	out := make([]int, 0, n+len(hole)+2)
	out = append(out, ring[:pi+1]...)
	for k := 0; k < len(hole); k++ {
		out = append(out, hole[(mi+k)%len(hole)])
	}
	out = append(out, hole[mi], ring[pi])
	out = append(out, ring[pi+1:]...)
	return out
}

func earClip(pts []V2, ring []int) [][3]int {
	n := len(ring)
	prev := make([]int, n)
	next := make([]int, n)
	for i := range ring {
		prev[i], next[i] = (i-1+n)%n, (i+1)%n
	}
	alive := n
	var tris [][3]int
	isEar := func(c int, strict bool) bool {
		a, b, d := ring[prev[c]], ring[c], ring[next[c]]
		A, B, C := pts[a], pts[b], pts[d]
		if tri2(A, B, C) <= 1e-12 {
			return false
		}
		if !strict {
			return true
		}
		for q := next[next[c]]; q != prev[c]; q = next[q] {
			qi := ring[q]
			if qi == a || qi == b || qi == d {
				continue
			}
			Q := pts[qi]
			if Q == A || Q == B || Q == C {
				continue
			}
			if tri2(pts[ring[prev[q]]], Q, pts[ring[next[q]]]) > 0 {
				continue // convex vertices cannot be the only intruder
			}
			if inTriInclusive(A, B, C, Q) {
				return false
			}
		}
		return true
	}
	remove := func(c int) {
		next[prev[c]], prev[next[c]] = next[c], prev[c]
		alive--
	}
	cur := 0
	for alive > 3 {
		clipped := false
		for k, c := 0, cur; k < alive; k, c = k+1, next[c] {
			if isEar(c, true) {
				tris = append(tris, [3]int{ring[prev[c]], ring[c], ring[next[c]]})
				cur = next[c]
				remove(c)
				clipped = true
				break
			}
		}
		if clipped {
			continue
		}
		// no strict ear: accept any convex corner, otherwise drop a flat one
		for k, c := 0, cur; k < alive; k, c = k+1, next[c] {
			if isEar(c, false) {
				tris = append(tris, [3]int{ring[prev[c]], ring[c], ring[next[c]]})
				cur = next[c]
				remove(c)
				clipped = true
				break
			}
		}
		if !clipped {
			remove(cur)
			cur = next[cur]
		}
	}
	c := cur
	if tri2(pts[ring[prev[c]]], pts[ring[c]], pts[ring[next[c]]]) > 1e-12 {
		tris = append(tris, [3]int{ring[prev[c]], ring[c], ring[next[c]]})
	}
	return tris
}
