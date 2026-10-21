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

func inTriangleInclusive(a, b, c, p Vector2) bool {
	sideAB, sideBC, sideCA := twiceSignedArea(a, b, p), twiceSignedArea(b, c, p), twiceSignedArea(c, a, p)
	neg := sideAB < 0 || sideBC < 0 || sideCA < 0
	pos := sideAB > 0 || sideBC > 0 || sideCA > 0
	return !(neg && pos)
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

func bridgeHole(points []Vector2, ring, hole []int, maxIndex int) []int {
	holePoint := points[hole[maxIndex]]
	n := len(ring)
	bestX, bestEdge := math.Inf(1), -1
	for i := 0; i < n; i++ {
		a, b := points[ring[i]], points[ring[(i+1)%n]]
		if a.Y == b.Y {
			continue
		}
		if (a.Y <= holePoint.Y && b.Y >= holePoint.Y) || (a.Y >= holePoint.Y && b.Y <= holePoint.Y) {
			x := a.X + (holePoint.Y-a.Y)*(b.X-a.X)/(b.Y-a.Y)
			if x >= holePoint.X && x < bestX {
				bestX, bestEdge = x, i
			}
		}
	}
	partnerIndex := 0
	if bestEdge < 0 {
		// fall back to the nearest ring vertex
		bestDistance := math.Inf(1)
		for j := 0; j < n; j++ {
			if d := points[ring[j]].Sub(holePoint).Length(); d < bestDistance {
				bestDistance, partnerIndex = d, j
			}
		}
	} else {
		intersection := Vector2{bestX, holePoint.Y}
		startIndex, endIndex := bestEdge, (bestEdge+1)%n
		a, b := points[ring[startIndex]], points[ring[endIndex]]
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
		bestTangent := math.Inf(1)
		for j := 0; j < n; j++ {
			if ring[j] == ring[partnerIndex] {
				continue
			}
			q := points[ring[j]]
			prev, next := points[ring[(j-1+n)%n]], points[ring[(j+1)%n]]
			if twiceSignedArea(prev, q, next) >= 0 { // not reflex
				continue
			}
			corner1, corner2, corner3 := holePoint, intersection, partnerPoint
			if twiceSignedArea(corner1, corner2, corner3) < 0 {
				corner2, corner3 = corner3, corner2
			}
			if !inTriangleInclusive(corner1, corner2, corner3, q) || q.X <= holePoint.X {
				continue
			}
			tangent := math.Abs(q.Y-holePoint.Y) / (q.X - holePoint.X)
			if tangent < bestTangent {
				bestTangent, partnerIndex = tangent, j
			}
		}
	}
	// the chosen vertex may occur several times in the ring: use the occurrence that sees M
	for j := 0; j < n; j++ {
		if ring[j] == ring[partnerIndex] && locallyInside(points, ring, j, holePoint) {
			partnerIndex = j
			break
		}
	}
	out := make([]int, 0, n+len(hole)+2)
	out = append(out, ring[:partnerIndex+1]...)
	for k := 0; k < len(hole); k++ {
		out = append(out, hole[(maxIndex+k)%len(hole)])
	}
	out = append(out, hole[maxIndex], ring[partnerIndex])
	out = append(out, ring[partnerIndex+1:]...)
	return out
}

func earClip(points []Vector2, ring []int) [][3]int {
	n := len(ring)
	prev := make([]int, n)
	next := make([]int, n)
	for i := range ring {
		prev[i], next[i] = (i-1+n)%n, (i+1)%n
	}
	alive := n
	var triangles [][3]int
	isEar := func(c int, strict bool) bool {
		a, b, d := ring[prev[c]], ring[c], ring[next[c]]
		pointA, pointB, pointC := points[a], points[b], points[d]
		if twiceSignedArea(pointA, pointB, pointC) <= 1e-12 {
			return false
		}
		if !strict {
			return true
		}
		for q := next[next[c]]; q != prev[c]; q = next[q] {
			otherIndex := ring[q]
			if otherIndex == a || otherIndex == b || otherIndex == d {
				continue
			}
			otherPoint := points[otherIndex]
			if otherPoint == pointA || otherPoint == pointB || otherPoint == pointC {
				continue
			}
			if twiceSignedArea(points[ring[prev[q]]], otherPoint, points[ring[next[q]]]) > 0 {
				continue // convex vertices cannot be the only intruder
			}
			if inTriangleInclusive(pointA, pointB, pointC, otherPoint) {
				return false
			}
		}
		return true
	}
	remove := func(c int) {
		next[prev[c]], prev[next[c]] = next[c], prev[c]
		alive--
	}
	current := 0
	for alive > 3 {
		clipped := false
		for k, c := 0, current; k < alive; k, c = k+1, next[c] {
			if isEar(c, true) {
				triangles = append(triangles, [3]int{ring[prev[c]], ring[c], ring[next[c]]})
				current = next[c]
				remove(c)
				clipped = true
				break
			}
		}
		if clipped {
			continue
		}
		// no strict ear: accept any convex corner, otherwise drop a flat one
		for k, c := 0, current; k < alive; k, c = k+1, next[c] {
			if isEar(c, false) {
				triangles = append(triangles, [3]int{ring[prev[c]], ring[c], ring[next[c]]})
				current = next[c]
				remove(c)
				clipped = true
				break
			}
		}
		if !clipped {
			remove(current)
			current = next[current]
		}
	}
	c := current
	if twiceSignedArea(points[ring[prev[c]]], points[ring[c]], points[ring[next[c]]]) > 1e-12 {
		triangles = append(triangles, [3]int{ring[prev[c]], ring[c], ring[next[c]]})
	}
	return triangles
}
