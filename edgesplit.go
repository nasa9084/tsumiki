package tsumiki

import (
	"cmp"
	"math"
	"slices"
)

// vertexGrid buckets vertex indices by the cell they fall in, for fast neighborhood queries.
func vertexGrid(vertices []Vector3, cell float64) map[cellKey][]int {
	grid := map[cellKey][]int{}
	for i, vertex := range vertices {
		key := cellKeyOf(vertex, cell)
		grid[key] = append(grid[key], i)
	}
	return grid
}

// verticesAlongEdge returns the vertices of the grid that lie within margin of the segment from
// vertex a to vertex b, strictly between its ends and accepted by accept, ordered from a to b.
func verticesAlongEdge(vertices []Vector3, grid map[cellKey][]int, cell float64, a, b int, margin float64,
	accept func(index int) bool) []int {
	pointA, pointB := vertices[a], vertices[b]
	low := Vector3{
		math.Min(pointA.X, pointB.X) - margin, math.Min(pointA.Y, pointB.Y) - margin, math.Min(pointA.Z, pointB.Z) - margin,
	}
	high := Vector3{
		math.Max(pointA.X, pointB.X) + margin, math.Max(pointA.Y, pointB.Y) + margin, math.Max(pointA.Z, pointB.Z) + margin,
	}
	lowKey, highKey := cellKeyOf(low, cell), cellKeyOf(high, cell)
	edge := pointB.Sub(pointA)
	lengthSquared := edge.Dot(edge)

	type candidate struct {
		index    int
		fraction float64 // position along the edge: 0 at a, 1 at b
	}
	var candidates []candidate
	for x := lowKey[0]; x <= highKey[0]; x++ {
		for y := lowKey[1]; y <= highKey[1]; y++ {
			for z := lowKey[2]; z <= highKey[2]; z++ {
				for _, index := range grid[cellKey{x, y, z}] {
					if index == a || index == b {
						continue
					}
					point := vertices[index]
					fraction := point.Sub(pointA).Dot(edge) / lengthSquared
					if fraction <= 0 || fraction >= 1 || point.Sub(pointA.Add(edge.Scale(fraction))).Length() > margin {
						continue
					}
					if accept(index) {
						candidates = append(candidates, candidate{index, fraction})
					}
				}
			}
		}
	}
	slices.SortFunc(candidates, func(l, r candidate) int { return cmp.Compare(l.fraction, r.fraction) })
	indices := make([]int, len(candidates))
	for i, c := range candidates {
		indices[i] = c.index
	}
	return indices
}

// subdivideTriangle splits the triangle abc at the extra vertices on its edges (extrasAB lies on
// a-b, extrasBC on b-c, extrasCA on c-a, each ordered along its edge) and passes every piece to emit.
func subdivideTriangle(a, b, c int, extrasAB, extrasBC, extrasCA []int, emit func(Triangle)) {
	switch {
	case len(extrasAB) > 0:
		e := extrasAB[0]
		subdivideTriangle(a, e, c, nil, nil, extrasCA, emit)
		subdivideTriangle(e, b, c, extrasAB[1:], extrasBC, nil, emit)
	case len(extrasBC) > 0:
		subdivideTriangle(b, c, a, extrasBC, extrasCA, extrasAB, emit)
	case len(extrasCA) > 0:
		subdivideTriangle(c, a, b, extrasCA, extrasAB, extrasBC, emit)
	default:
		emit(Triangle{a, b, c})
	}
}
