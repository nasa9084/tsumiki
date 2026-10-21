package tsumiki

import (
	"errors"
	"fmt"
	"math"
)

// offsetLoop moves every edge of l (material on the left) to the right by d, using mitered corners.
// d > 0 grows the material region.
func offsetLoop(l Loop, d float64) Loop {
	n := len(l)
	out := make(Loop, n)
	norm := make([]V2, n) // right-hand normal of edge i -> i+1
	for i := range l {
		e := l[(i+1)%n].Sub(l[i])
		ln := e.Len()
		norm[i] = V2{e.Y / ln, -e.X / ln}
	}
	for i := range l {
		n1, n2 := norm[(i-1+n)%n], norm[i]
		den := 1 + n1.Dot(n2)
		if den < 0.05 {
			den = 0.05
		}
		out[i] = l[i].Add(n1.Add(n2).Mul(d / den))
	}
	return out
}

// extrudeMesh builds the closed solid for regs extruded by dist along the plane normal.
// taperDeg > 0 makes the section grow with distance travelled. start shifts the base along N.
func extrudeMesh(pl Plane, regs []Region, dist, taperDeg, start float64) (*Mesh, error) {
	h := math.Abs(dist)
	off := h * math.Tan(Rad(taperDeg))
	m := &Mesh{}
	for _, r := range regs {
		loops := append([]Loop{r.Outer}, r.Holes...)
		pts, tris := triangulateRegion(r.Outer, r.Holes)
		n := len(pts)
		var top []V2
		for _, l := range loops {
			if off == 0 {
				top = append(top, l...)
				continue
			}
			ol := offsetLoop(l, off)
			if err := checkOffset(l, ol); err != nil {
				return nil, fmt.Errorf("taper %.3g° over %.4g mm: %w", taperDeg, h, err)
			}
			top = append(top, ol...)
		}
		rm := &Mesh{}
		for _, p := range pts {
			rm.V = append(rm.V, V3{p.X, p.Y, 0})
		}
		for _, p := range top {
			rm.V = append(rm.V, V3{p.X, p.Y, h})
		}
		for _, t := range tris {
			rm.T = append(rm.T, Tri{t[0], t[2], t[1]})             // bottom faces -N
			rm.T = append(rm.T, Tri{t[0] + n, t[1] + n, t[2] + n}) // top faces +N
		}
		base := 0
		for _, l := range loops {
			for i := range l {
				a, b := base+i, base+(i+1)%len(l)
				rm.T = append(rm.T, Tri{a, b, b + n}, Tri{a, b + n, a + n})
			}
			base += len(l)
		}
		m.merge(rm)
	}
	mirror := dist < 0
	m.transform(func(p V3) V3 {
		w := p.Z
		if mirror {
			w = -w
		}
		return pl.Point(p.X, p.Y, w+start)
	}, mirror)
	return m, nil
}

// checkOffset rejects a taper that collapses or inverts the profile: every edge must keep its
// direction and the loop its orientation (a mitered offset cannot survive past that point).
func checkOffset(base, off Loop) error {
	if (base.Area() > 0) != (off.Area() > 0) || off.Area() == 0 {
		return errors.New("the profile collapses (reduce the taper angle or distance)")
	}
	n := len(base)
	for i := range base {
		j := (i + 1) % n
		if off[j].Sub(off[i]).Dot(base[j].Sub(base[i])) <= 0 {
			return errors.New("an edge of the profile inverts (reduce the taper angle or distance)")
		}
	}
	return nil
}

// revolveMesh sweeps regs around the plane's V axis (the line u = 0) by angleDeg.
// Profile points must have u >= 0 (u is the radius). A full turn (>= 360) has no caps.
func revolveMesh(pl Plane, regs []Region, angleDeg, tol float64) *Mesh {
	full := angleDeg >= 360-1e-9
	sweep := Rad(math.Min(angleDeg, 360))
	m := &Mesh{}
	for _, r := range regs {
		loops := append([]Loop{r.Outer}, r.Holes...)
		pts, tris := triangulateRegion(r.Outer, r.Holes)
		rmax := 0.0
		for _, p := range pts {
			rmax = math.Max(rmax, math.Abs(p.X))
		}
		steps := arcSteps(rmax, sweep, tol)
		if steps < 3 && full {
			steps = 3
		}
		rings := steps + 1
		if full {
			rings = steps
		}
		rm := &Mesh{}
		n := len(pts)
		for k := 0; k < rings; k++ {
			th := sweep * float64(k) / float64(steps)
			s, c := math.Sincos(th)
			for _, p := range pts {
				// rotate U about V by th (right-handed): U cos - N sin
				rm.V = append(rm.V, pl.O.Add(pl.U.Mul(p.X*c)).Add(pl.N.Mul(-p.X*s)).Add(pl.V.Mul(p.Y)))
			}
		}
		for k := 0; k < steps; k++ {
			k0, k1 := k, k+1
			if full && k1 == rings {
				k1 = 0
			}
			base := 0
			for _, l := range loops {
				for i := range l {
					a, b := base+i, base+(i+1)%len(l)
					A, B := k0*n+a, k0*n+b
					C, D := k1*n+b, k1*n+a
					rm.T = append(rm.T, Tri{A, C, B}, Tri{A, D, C})
				}
				base += len(l)
			}
		}
		if !full {
			last := steps * n
			for _, t := range tris {
				rm.T = append(rm.T, Tri{t[0], t[1], t[2]})
				rm.T = append(rm.T, Tri{t[0] + last, t[2] + last, t[1] + last})
			}
		}
		m.merge(rm)
	}
	m.Weld(weldEps) // collapse points on the axis
	m.dropDegenerate()
	return m
}
