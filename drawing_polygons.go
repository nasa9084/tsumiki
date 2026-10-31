package tsumiki

import (
	"fmt"
	"math"
)

// regularPolygon is a regular polygon seen square-on, such as a hexagonal hole.
type regularPolygon struct {
	center  Vector2
	corners []Vector2
	near    float64 // the depth of the nearest outline of this shape
	rim     int     // a mesh vertex on that outline
}

const maxPolygonSides = 8 // regular polygons with more sides are left to circle recognition

// regularPolygons returns the visible regular polygons of the view (closed outlines with equal
// sides and equal corners, too coarse to be circles), one per distinct outline.
func (p *panel) regularPolygons() []regularPolygon {
	neighbors := map[int][]int{}
	for _, i := range p.drawnEdges {
		e := p.set.edges[i]
		neighbors[e.a] = append(neighbors[e.a], e.b)
		neighbors[e.b] = append(neighbors[e.b], e.a)
	}
	seen := map[[3]int64]int{}
	var polygons []regularPolygon
	for _, chain := range chainsOf(neighbors) {
		if chain[0] != chain[len(chain)-1] {
			continue
		}
		points := make([]Vector2, len(chain))
		corners3 := make([]Vector3, len(chain))
		for i, index := range chain {
			corners3[i] = p.set.mesh.Vertices[index]
			points[i], _ = p.camera.project(corners3[i])
		}
		kept := withoutStraightJoints(points)
		corners := make([]Vector2, len(kept)-1)
		for i, k := range kept[:len(kept)-1] {
			corners[i] = points[k]
		}
		polygon, ok := asRegularPolygon(corners)
		if !ok || p.isOutline(corners) || !p.mostlyVisible(corners3) {
			continue
		}
		_, depth := p.camera.project(corners3[0])
		radius := polygon.corners[0].Sub(polygon.center).Length()
		key := [3]int64{int64(math.Round(polygon.center.X / sameValueSpacing)),
			int64(math.Round(polygon.center.Y / sameValueSpacing)), int64(math.Round(radius / sameValueSpacing))}
		if i, ok := seen[key]; ok {
			if depth < polygons[i].near {
				polygons[i].near, polygons[i].rim = depth, chain[0]
			}
			continue
		}
		seen[key] = len(polygons)
		polygon.near, polygon.rim = depth, chain[0]
		polygons = append(polygons, polygon)
	}
	return polygons
}

// asRegularPolygon reports whether corners form a regular polygon.
func asRegularPolygon(corners []Vector2) (regularPolygon, bool) {
	n := len(corners)
	if n < 3 || n > maxPolygonSides {
		return regularPolygon{}, false
	}
	var center Vector2
	for _, corner := range corners {
		center = center.Add(corner)
	}
	center = center.Scale(1 / float64(n))
	radius := corners[0].Sub(center).Length()
	side := 2 * radius * math.Sin(math.Pi/float64(n))
	if radius-radius*math.Cos(math.Pi/float64(n)) <= maxSagitta { // fine enough to be a circle
		return regularPolygon{}, false
	}
	for i, corner := range corners {
		next := corners[(i+1)%n]
		if math.Abs(corner.Sub(center).Length()-radius) > circleTolerance ||
			math.Abs(next.Sub(corner).Length()-side) > 2*circleTolerance {
			return regularPolygon{}, false
		}
	}
	return regularPolygon{center: center, corners: corners}, true
}

// size names the polygon's size: across flats for even polygons, across corners for odd ones.
func (polygon regularPolygon) size() string {
	n := len(polygon.corners)
	radius := polygon.corners[0].Sub(polygon.center).Length()
	acrossFlats := 2 * radius * math.Cos(math.Pi/float64(n))
	switch {
	case n == 4:
		return "□" + length(roundedRadius(acrossFlats))
	case n == 6:
		return "HEX " + length(roundedRadius(acrossFlats)) + " A/F"
	case n%2 == 0:
		return fmt.Sprintf("%d-GON %s A/F", n, length(roundedRadius(acrossFlats)))
	default:
		return fmt.Sprintf("%d-GON ⌀%s", n, length(roundedRadius(2*radius)))
	}
}

// feature describes the polygon for labeling; leaders point at the middles of its sides.
func (polygon regularPolygon) feature(p *panel) sizedFeature {
	f := sizedFeature{center: polygon.center, size: polygon.size() + p.blindDepth(polygon.center, polygon.rim),
		repeated: true}
	for i, corner := range polygon.corners {
		middle := corner.Add(polygon.corners[(i+1)%len(polygon.corners)]).Scale(0.5)
		f.anchors = append(f.anchors, leaderAt{point: middle, outward: unit(middle.Sub(polygon.center))})
	}
	return f
}

// isOutline reports whether a closed outline spans the whole view: the overall dimensions already
// give its size.
func (p *panel) isOutline(corners []Vector2) bool {
	low, high := corners[0], corners[0]
	for _, corner := range corners {
		low = Vector2{math.Min(low.X, corner.X), math.Min(low.Y, corner.Y)}
		high = Vector2{math.Max(high.X, corner.X), math.Max(high.Y, corner.Y)}
	}
	return low.Sub(p.low).Length() < sameValueSpacing && high.Sub(p.high).Length() < sameValueSpacing
}

// blindDepth describes how deep a hole whose outline nearest to the viewer passes through the
// mesh vertex rim goes, when it has a floor: ", depth 0.6". The wall is followed from rim straight
// away from the viewer; the hole is blind when the surface seen at its center lies where the wall
// ends. Through holes, and outlines that are not holes, get nothing.
func (p *panel) blindDepth(center Vector2, rim int) string {
	if rim < 0 {
		return ""
	}
	_, top := p.camera.project(p.set.mesh.Vertices[rim])
	wall := p.wallLength(rim)
	floor, ok := p.buffer.nearest(center)
	if !ok || wall < sameValueSpacing || math.Abs(floor-(top+wall)) > sameValueSpacing {
		return ""
	}
	return ", depth " + length(wall)
}

// wallLength follows mesh edges from a vertex straight away from the viewer for as long as they
// go, and returns how far that is.
func (p *panel) wallLength(start int) float64 {
	if p.neighbors == nil {
		p.neighbors = map[int][]int{}
		for _, e := range p.set.edges {
			p.neighbors[e.a] = append(p.neighbors[e.a], e.b)
			p.neighbors[e.b] = append(p.neighbors[e.b], e.a)
		}
	}
	vertices := p.set.mesh.Vertices
	total := 0.0
	for current := start; ; {
		next := -1
		for _, candidate := range p.neighbors[current] {
			step := vertices[candidate].Sub(vertices[current])
			if along := step.Dot(p.camera.forward); along > 1e-9 && step.Cross(p.camera.forward).Length() < 1e-6 {
				next, total = candidate, total+along
				break
			}
		}
		if next < 0 {
			return total
		}
		current = next
	}
}
