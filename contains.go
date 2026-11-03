package tsumiki

import "math"

// Contains reports whether point lies inside the closed mesh. It counts how often rays from the
// point cross the surface (an odd count means inside), along three skewed directions that vote,
// so that a ray grazing an edge or a vertex cannot decide the answer alone.
func (m *Mesh) Contains(point Vector3) bool {
	directions := []Vector3{
		{0.5773, 0.3141, 0.7532},
		{-0.2718, 0.8123, 0.5127},
		{0.1234, -0.6543, 0.7456},
	}
	votes := 0
	for _, direction := range directions {
		if m.crossings(point, direction)%2 == 1 {
			votes++
		}
	}
	return votes >= 2
}

// crossings counts the triangles the ray from origin along direction passes through
// (Möller–Trumbore).
func (m *Mesh) crossings(origin, direction Vector3) int {
	count := 0
	for _, t := range m.Triangles {
		a, b, c := m.Vertices[t[0]], m.Vertices[t[1]], m.Vertices[t[2]]
		firstEdge, secondEdge := b.Sub(a), c.Sub(a)
		h := direction.Cross(secondEdge)
		determinant := firstEdge.Dot(h)
		if math.Abs(determinant) < 1e-12 {
			continue // the ray runs along the triangle's plane
		}
		inverse := 1 / determinant
		s := origin.Sub(a)
		u := inverse * s.Dot(h)
		if u < 0 || u > 1 {
			continue
		}
		q := s.Cross(firstEdge)
		v := inverse * direction.Dot(q)
		if v < 0 || u+v > 1 {
			continue
		}
		if inverse*secondEdge.Dot(q) > 0 {
			count++
		}
	}
	return count
}
