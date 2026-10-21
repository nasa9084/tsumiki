package tsumiki

import (
	"math"
	"sort"
)

// triangulateRegion triangulates a region (outer CCW, holes CW).
// points is outer followed by each hole in the given order; triangles index into points
// and are counter-clockwise. Holes are bridged into the outer ring (Eberly), then ear clipped.
func triangulateRegion(outer Loop, holes []Loop) ([]Vector2, [][3]int) {
	points := append([]Vector2(nil), outer...)
	ring := make([]int, len(outer))
	for i := range ring {
		ring[i] = i
	}
	type hole struct {
		indices  []int
		maxX     float64
		maxIndex int
	}
	var sortedHoles []hole
	for _, h := range holes {
		base := len(points)
		points = append(points, h...)
		indices := make([]int, len(h))
		maxIndex := 0
		for i := range h {
			indices[i] = base + i
			if h[i].X > h[maxIndex].X {
				maxIndex = i
			}
		}
		sortedHoles = append(sortedHoles, hole{indices, h[maxIndex].X, maxIndex})
	}
	sort.SliceStable(sortedHoles, func(i, j int) bool { return sortedHoles[i].maxX > sortedHoles[j].maxX })
	for _, h := range sortedHoles {
		ring = bridgeHole(points, ring, h.indices, h.maxIndex)
	}
	return points, earClip(points, ring)
}

func twiceSignedArea(a, b, c Vector2) float64 { return b.Sub(a).Cross(c.Sub(a)) }

// inTriangleInclusive reports whether p lies in triangle abc, boundary included.
// A zero-area triangle contains nothing: without this, every point on its supporting line
// would count as inside.
func inTriangleInclusive(a, b, c, p Vector2) bool {
	if twiceSignedArea(a, b, c) == 0 {
		return false
	}
	sideAB, sideBC, sideCA := twiceSignedArea(a, b, p), twiceSignedArea(b, c, p), twiceSignedArea(c, a, p)
	neg := sideAB < 0 || sideBC < 0 || sideCA < 0
	pos := sideAB > 0 || sideBC > 0 || sideCA > 0
	return !neg || !pos
}

// locallyInside reports whether direction d from ring[j] points into the polygon interior (CCW ring).
func locallyInside(points []Vector2, ring []int, j int, target Vector2) bool {
	n := len(ring)
	v := points[ring[j]]
	p := points[ring[(j-1+n)%n]]
	nextPoint := points[ring[(j+1)%n]]
	d := target.Sub(v)
	a, b := nextPoint.Sub(v), p.Sub(v)
	if a.Cross(b) >= 0 { // convex (or straight) vertex
		return a.Cross(d) >= 0 && d.Cross(b) >= 0
	}
	return !(b.Cross(d) >= 0 && d.Cross(a) >= 0)
}

// rightwardHit finds the ring edge that a ray from holePoint towards +x hits first.
// It returns the edge's start index (-1 when none is hit) and the x of the hit.
func rightwardHit(points []Vector2, ring []int, holePoint Vector2) (edge int, hitX float64) {
	n := len(ring)
	hitX, edge = math.Inf(1), -1
	for i := range n {
		a, b := points[ring[i]], points[ring[(i+1)%n]]
		if a.Y == b.Y {
			continue
		}
		if (a.Y <= holePoint.Y && b.Y >= holePoint.Y) || (a.Y >= holePoint.Y && b.Y <= holePoint.Y) {
			x := a.X + (holePoint.Y-a.Y)*(b.X-a.X)/(b.Y-a.Y)
			if x >= holePoint.X && x < hitX {
				hitX, edge = x, i
			}
		}
	}
	return edge, hitX
}

// nearestRingVertex returns the index into ring of the vertex closest to holePoint.
func nearestRingVertex(points []Vector2, ring []int, holePoint Vector2) int {
	best, bestDistance := 0, math.Inf(1)
	for j := range ring {
		if d := points[ring[j]].Sub(holePoint).Length(); d < bestDistance {
			bestDistance, best = d, j
		}
	}
	return best
}

// visiblePartner picks the ring vertex that a hole's rightmost point can be bridged to, given
// that the ray from holePoint hits the ring edge starting at bestEdge at x = hitX.
func visiblePartner(points []Vector2, ring []int, holePoint Vector2, bestEdge int, hitX float64) int {
	n := len(ring)
	intersection := Vector2{hitX, holePoint.Y}
	startIndex, endIndex := bestEdge, (bestEdge+1)%n
	a, b := points[ring[startIndex]], points[ring[endIndex]]
	var partnerIndex int
	switch {
	case intersection.Sub(a).Length() < 1e-12:
		partnerIndex = startIndex
	case intersection.Sub(b).Length() < 1e-12:
		partnerIndex = endIndex
	case b.X > a.X:
		partnerIndex = endIndex
	default:
		partnerIndex = startIndex
	}
	partnerPoint := points[ring[partnerIndex]]
	// a reflex vertex inside triangle (M, I, P) blocks the view: pick the one with the smallest angle
	corner1, corner2, corner3 := holePoint, intersection, partnerPoint
	if twiceSignedArea(corner1, corner2, corner3) < 0 {
		corner2, corner3 = corner3, corner2
	}
	bestTangent := math.Inf(1)
	for j := range n {
		if ring[j] == ring[partnerIndex] {
			continue
		}
		q := points[ring[j]]
		if twiceSignedArea(points[ring[(j-1+n)%n]], q, points[ring[(j+1)%n]]) >= 0 { // not reflex
			continue
		}
		if !inTriangleInclusive(corner1, corner2, corner3, q) || q.X <= holePoint.X {
			continue
		}
		if tangent := math.Abs(q.Y-holePoint.Y) / (q.X - holePoint.X); tangent < bestTangent {
			bestTangent, partnerIndex = tangent, j
		}
	}
	return partnerIndex
}

func bridgeHole(points []Vector2, ring, hole []int, maxIndex int) []int {
	holePoint := points[hole[maxIndex]]
	var partnerIndex int
	if bestEdge, hitX := rightwardHit(points, ring, holePoint); bestEdge < 0 {
		partnerIndex = nearestRingVertex(points, ring, holePoint) // no edge in view: fall back
	} else {
		partnerIndex = visiblePartner(points, ring, holePoint, bestEdge, hitX)
	}
	// the chosen vertex may occur several times in the ring: use the occurrence that sees M
	for j := range ring {
		if ring[j] == ring[partnerIndex] && locallyInside(points, ring, j, holePoint) {
			partnerIndex = j
			break
		}
	}
	out := make([]int, 0, len(ring)+len(hole)+2)
	out = append(out, ring[:partnerIndex+1]...)
	for k := range hole {
		out = append(out, hole[(maxIndex+k)%len(hole)])
	}
	out = append(out, hole[maxIndex], ring[partnerIndex])
	out = append(out, ring[partnerIndex+1:]...)
	return out
}

// clipper holds the doubly linked ring that earClip consumes.
type clipper struct {
	points     []Vector2
	ring       []int
	prev, next []int
	alive      int
}

func newClipper(points []Vector2, ring []int) *clipper {
	n := len(ring)
	c := &clipper{points: points, ring: ring, prev: make([]int, n), next: make([]int, n), alive: n}
	for i := range ring {
		c.prev[i], c.next[i] = (i-1+n)%n, (i+1)%n
	}
	return c
}

func (c *clipper) remove(i int) {
	c.next[c.prev[i]], c.prev[c.next[i]] = c.next[i], c.prev[i]
	c.alive--
}

func (c *clipper) triangleAt(i int) [3]int {
	return [3]int{c.ring[c.prev[i]], c.ring[i], c.ring[c.next[i]]}
}

// isEar reports whether the corner at i is convex and, when strict, contains no reflex vertex.
func (c *clipper) isEar(i int, strict bool) bool {
	a, b, d := c.ring[c.prev[i]], c.ring[i], c.ring[c.next[i]]
	pointA, pointB, pointC := c.points[a], c.points[b], c.points[d]
	if twiceSignedArea(pointA, pointB, pointC) <= 1e-12 {
		return false
	}
	if !strict {
		return true
	}
	for q := c.next[c.next[i]]; q != c.prev[i]; q = c.next[q] {
		otherIndex := c.ring[q]
		if otherIndex == a || otherIndex == b || otherIndex == d {
			continue
		}
		otherPoint := c.points[otherIndex]
		if otherPoint == pointA || otherPoint == pointB || otherPoint == pointC {
			continue
		}
		if twiceSignedArea(c.points[c.ring[c.prev[q]]], otherPoint, c.points[c.ring[c.next[q]]]) > 0 {
			continue // convex vertices cannot be the only intruder
		}
		if inTriangleInclusive(pointA, pointB, pointC, otherPoint) {
			return false
		}
	}
	return true
}

// findEar scans the alive corners from start for an ear, returning -1 when there is none.
func (c *clipper) findEar(start int, strict bool) int {
	for k, i := 0, start; k < c.alive; k, i = k+1, c.next[i] {
		if c.isEar(i, strict) {
			return i
		}
	}
	return -1
}

func earClip(points []Vector2, ring []int) [][3]int {
	c := newClipper(points, ring)
	var triangles [][3]int
	current := 0
	for c.alive > 3 {
		ear := c.findEar(current, true)
		if ear < 0 {
			ear = c.findEar(current, false) // no strict ear: accept any convex corner
		}
		if ear < 0 { // otherwise drop a flat corner
			c.remove(current)
			current = c.next[current]
			continue
		}
		triangles = append(triangles, c.triangleAt(ear))
		current = c.next[ear]
		c.remove(ear)
	}
	if twiceSignedArea(points[ring[c.prev[current]]], points[ring[current]], points[ring[c.next[current]]]) > 1e-12 {
		triangles = append(triangles, c.triangleAt(current))
	}
	return triangles
}
