package tsumiki

import (
	"errors"
	"fmt"
	"math"
)

// offsetLoop moves every edge of l (material on the left) to the right by d, using mitered corners.
// d > 0 grows the material region.
func offsetLoop(l Loop, distance float64) Loop {
	n := len(l)
	out := make(Loop, n)
	normals := make([]Vector2, n) // right-hand normal of edge i -> i+1
	for i := range l {
		e := l[(i+1)%n].Sub(l[i])
		edgeLength := e.Length()
		normals[i] = Vector2{e.Y / edgeLength, -e.X / edgeLength}
	}
	for i := range l {
		previousNormal, currentNormal := normals[(i-1+n)%n], normals[i]
		denominator := 1 + previousNormal.Dot(currentNormal)
		if denominator < 0.05 {
			denominator = 0.05
		}
		out[i] = l[i].Add(previousNormal.Add(currentNormal).Scale(distance / denominator))
	}
	return out
}

// extrudeMesh builds the closed solid for regions extruded by distance along the plane normal.
// taperDegrees > 0 makes the section grow with distance traveled. start shifts the base along N.
func extrudeMesh(plane Plane, regions []Region, distance, taperDegrees, start float64) (*Mesh, error) {
	height := math.Abs(distance)
	offsetDistance := height * math.Tan(Radians(taperDegrees))
	m := &Mesh{}
	for _, r := range regions {
		loops := append([]Loop{r.Outer}, r.Holes...)
		points, triangles := triangulateRegion(r.Outer, r.Holes)
		n := len(points)
		var top []Vector2
		for _, l := range loops {
			if offsetDistance == 0 {
				top = append(top, l...)
				continue
			}
			shiftedLoop := offsetLoop(l, offsetDistance)
			if err := checkOffset(l, shiftedLoop); err != nil {
				return nil, fmt.Errorf("taper %.3g° over %.4g mm: %w", taperDegrees, height, err)
			}
			top = append(top, shiftedLoop...)
		}
		regionMesh := &Mesh{}
		for _, p := range points {
			regionMesh.Vertices = append(regionMesh.Vertices, Vector3{p.X, p.Y, 0})
		}
		for _, p := range top {
			regionMesh.Vertices = append(regionMesh.Vertices, Vector3{p.X, p.Y, height})
		}
		for _, t := range triangles {
			regionMesh.Triangles = append(regionMesh.Triangles, Triangle{t[0], t[2], t[1]})             // bottom faces -N
			regionMesh.Triangles = append(regionMesh.Triangles, Triangle{t[0] + n, t[1] + n, t[2] + n}) // top faces +N
		}
		base := 0
		for _, l := range loops {
			for i := range l {
				a, b := base+i, base+(i+1)%len(l)
				regionMesh.Triangles = append(regionMesh.Triangles, Triangle{a, b, b + n}, Triangle{a, b + n, a + n})
			}
			base += len(l)
		}
		m.merge(regionMesh)
	}
	mirror := distance < 0
	m.transform(func(p Vector3) Vector3 {
		w := p.Z
		if mirror {
			w = -w
		}
		return plane.Point(p.X, p.Y, w+start)
	}, mirror)
	return m, nil
}

// checkOffset rejects a taper that collapses or inverts the profile: every edge must keep its
// direction and the loop its orientation (a mitered offset cannot survive past that point).
func checkOffset(base, shifted Loop) error {
	if (base.Area() > 0) != (shifted.Area() > 0) || shifted.Area() == 0 {
		return errors.New("the profile collapses (reduce the taper angle or distance)")
	}
	n := len(base)
	for i := range base {
		j := (i + 1) % n
		if shifted[j].Sub(shifted[i]).Dot(base[j].Sub(base[i])) <= 0 {
			return errors.New("an edge of the profile inverts (reduce the taper angle or distance)")
		}
	}
	return nil
}

// revolveMesh sweeps regions around the plane's V axis (the line u = 0) by angleDegrees.
// Profile points must have u >= 0 (u is the radius). A full turn (>= 360) has no caps.
func revolveMesh(plane Plane, regions []Region, angleDegrees, tolerance float64) *Mesh {
	full := angleDegrees >= 360-1e-9
	sweep := Radians(math.Min(angleDegrees, 360))
	m := &Mesh{}
	for _, r := range regions {
		m.merge(revolveRegion(plane, r, sweep, full, tolerance))
	}
	m.Weld(weldEpsilon) // collapse points on the axis
	m.dropDegenerate()
	return m
}

// revolveRegion sweeps one region by sweep radians; a full turn closes on itself instead of capping.
func revolveRegion(plane Plane, r Region, sweep float64, full bool, tolerance float64) *Mesh {
	points, triangles := triangulateRegion(r.Outer, r.Holes)
	maxRadius := 0.0
	for _, p := range points {
		maxRadius = math.Max(maxRadius, math.Abs(p.X))
	}
	steps := arcSteps(maxRadius, sweep, tolerance)
	if steps < 3 && full {
		steps = 3
	}
	rings := steps + 1
	if full {
		rings = steps
	}
	regionMesh := &Mesh{}
	for k := range rings {
		s, c := math.Sincos(sweep * float64(k) / float64(steps))
		for _, p := range points {
			// rotate U about V by angle (right-handed): U cos - Normal sin
			regionMesh.Vertices = append(regionMesh.Vertices,
				plane.Origin.Add(plane.U.Scale(p.X*c)).Add(plane.Normal.Scale(-p.X*s)).Add(plane.V.Scale(p.Y)))
		}
	}
	loops := append([]Loop{r.Outer}, r.Holes...)
	n := len(points)
	for k := range steps {
		nextRing := k + 1
		if full && nextRing == rings {
			nextRing = 0
		}
		regionMesh.addSweptWalls(loops, k, nextRing, n)
	}
	if !full {
		last := steps * n
		for _, t := range triangles {
			regionMesh.Triangles = append(regionMesh.Triangles,
				Triangle{t[0], t[1], t[2]}, Triangle{t[0] + last, t[2] + last, t[1] + last})
		}
	}
	return regionMesh
}

// addSweptWalls joins ring currentRing to ring nextRing along every loop edge; each ring has n points.
func (m *Mesh) addSweptWalls(loops []Loop, currentRing, nextRing, n int) {
	base := 0
	for _, l := range loops {
		for i := range l {
			a, b := base+i, base+(i+1)%len(l)
			currentA, currentB := currentRing*n+a, currentRing*n+b
			nextB, nextA := nextRing*n+b, nextRing*n+a
			m.Triangles = append(m.Triangles, Triangle{currentA, nextB, currentB}, Triangle{currentA, nextA, nextB})
		}
		base += len(l)
	}
}
