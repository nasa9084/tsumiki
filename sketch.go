package tsumiki

import (
	"math"
)

// Plane is a sketch plane. U x V = N.
type Plane struct{ O, U, V, N V3 }

// XY is the plane z = z0 (U=X, V=Y, N=+Z).
func XY(z0 float64) Plane {
	return Plane{V3{0, 0, z0}, V3{1, 0, 0}, V3{0, 1, 0}, V3{0, 0, 1}}
}

// XZ is the plane y = y0 (U=X, V=Z, N=-Y).
func XZ(y0 float64) Plane {
	return Plane{V3{0, y0, 0}, V3{1, 0, 0}, V3{0, 0, 1}, V3{0, -1, 0}}
}

// YZ is the plane x = x0 (U=Y, V=Z, N=+X).
func YZ(x0 float64) Plane {
	return Plane{V3{x0, 0, 0}, V3{0, 1, 0}, V3{0, 0, 1}, V3{1, 0, 0}}
}

// Point maps plane coordinates (u,v) and a height w along N to world space.
func (p Plane) Point(u, v, w float64) V3 {
	return p.O.Add(p.U.Mul(u)).Add(p.V.Mul(v)).Add(p.N.Mul(w))
}

// Loop is a closed polygon (the closing edge is implicit).
type Loop []V2

// Area is the signed shoelace area (CCW positive).
func (l Loop) Area() float64 {
	a := 0.0
	for i := range l {
		a += l[i].Cross(l[(i+1)%len(l)])
	}
	return a / 2
}

func (l Loop) reversed() Loop {
	r := make(Loop, len(l))
	for i := range l {
		r[len(l)-1-i] = l[i]
	}
	return r
}

// DefaultTol is the default chord tolerance used to tessellate arcs (mm).
const DefaultTol = 0.01

// Sketch is a set of closed loops on a plane. Nesting decides material:
// a loop inside one loop is a region, inside two is a hole, and so on (even-odd).
type Sketch struct {
	Plane Plane
	Tol   float64
	loops []Loop
}

// NewSketch starts an empty sketch on p.
func NewSketch(p Plane) *Sketch { return &Sketch{Plane: p, Tol: DefaultTol} }

// Polygon adds a closed polygon.
func (s *Sketch) Polygon(pts ...V2) *Sketch {
	s.loops = append(s.loops, append(Loop(nil), pts...))
	return s
}

// Circle adds a circle tessellated within the sketch tolerance.
func (s *Sketch) Circle(cx, cy, r float64) *Sketch {
	n := arcSteps(r, 2*math.Pi, s.Tol)
	if n < 8 {
		n = 8
	}
	l := make(Loop, n)
	for i := range l {
		a := 2 * math.Pi * float64(i) / float64(n)
		l[i] = V2{cx + r*math.Cos(a), cy + r*math.Sin(a)}
	}
	s.loops = append(s.loops, l)
	return s
}

// Rect adds an axis-aligned rectangle centred at (cx,cy).
func (s *Sketch) Rect(cx, cy, w, h float64) *Sketch { return s.RoundedRect(cx, cy, w, h, 0, 0) }

// RoundedRect adds a w x h rectangle with corner radius r, rotated by rotDeg around its centre.
func (s *Sketch) RoundedRect(cx, cy, w, h, r, rotDeg float64) *Sketch {
	hx, hy := w/2, h/2
	r = math.Min(r, math.Min(hx, hy)) // a radius beyond half the short side would self-intersect
	var l Loop
	if r <= 0 {
		l = Loop{{hx, -hy}, {hx, hy}, {-hx, hy}, {-hx, -hy}}
	} else {
		n := arcSteps(r, math.Pi/2, s.Tol)
		corners := []struct {
			c  V2
			a0 float64
		}{
			{V2{hx - r, -hy + r}, -math.Pi / 2},
			{V2{hx - r, hy - r}, 0},
			{V2{-hx + r, hy - r}, math.Pi / 2},
			{V2{-hx + r, -hy + r}, math.Pi},
		}
		for _, c := range corners {
			for i := 0; i <= n; i++ {
				a := c.a0 + (math.Pi/2)*float64(i)/float64(n)
				l = append(l, V2{c.c.X + r*math.Cos(a), c.c.Y + r*math.Sin(a)})
			}
		}
	}
	rot := Rad(rotDeg)
	for i := range l {
		l[i] = l[i].Rot(rot).Add(V2{cx, cy})
	}
	s.loops = append(s.loops, l)
	return s
}

// Translate moves every loop already in the sketch.
func (s *Sketch) Translate(dx, dy float64) *Sketch {
	for _, l := range s.loops {
		for i := range l {
			l[i] = l[i].Add(V2{dx, dy})
		}
	}
	return s
}

// Rotate rotates every loop already in the sketch around the sketch origin.
func (s *Sketch) Rotate(deg float64) *Sketch {
	for _, l := range s.loops {
		for i := range l {
			l[i] = l[i].Rot(Rad(deg))
		}
	}
	return s
}

// Path is a line/arc chain that becomes one loop when closed.
type Path struct {
	s   *Sketch
	pts []V2
}

// Path starts a line/arc path at (x,y).
func (s *Sketch) Path(x, y float64) *Path { return &Path{s: s, pts: []V2{{x, y}}} }

// LineTo adds a straight segment.
func (p *Path) LineTo(x, y float64) *Path {
	p.pts = append(p.pts, V2{x, y})
	return p
}

// ArcTo adds an arc about (cx,cy) from the current point to (x,y).
// The radius is taken from the current point. ccw selects the sweep direction.
func (p *Path) ArcTo(x, y, cx, cy float64, ccw bool) *Path {
	c := V2{cx, cy}
	cur := p.pts[len(p.pts)-1]
	r := cur.Sub(c).Len()
	a0 := math.Atan2(cur.Y-cy, cur.X-cx)
	a1 := math.Atan2(y-cy, x-cx)
	sweep := a1 - a0
	if ccw {
		for sweep <= 0 {
			sweep += 2 * math.Pi
		}
	} else {
		for sweep >= 0 {
			sweep -= 2 * math.Pi
		}
	}
	n := arcSteps(r, math.Abs(sweep), p.s.Tol)
	for i := 1; i < n; i++ {
		a := a0 + sweep*float64(i)/float64(n)
		p.pts = append(p.pts, V2{cx + r*math.Cos(a), cy + r*math.Sin(a)})
	}
	p.pts = append(p.pts, V2{x, y})
	return p
}

// Close finishes the loop and returns the sketch.
func (p *Path) Close() *Sketch {
	p.s.loops = append(p.s.loops, Loop(p.pts))
	return p.s
}

func arcSteps(r, sweep, tol float64) int {
	if tol <= 0 {
		tol = DefaultTol
	}
	dth := math.Pi / 2
	if r > tol {
		dth = 2 * math.Acos(1-tol/r)
	}
	n := int(math.Ceil(sweep / dth))
	if n < 1 {
		n = 1
	}
	return n
}

// Region is an outer boundary (CCW) with holes (CW): material is on the left of every edge.
type Region struct {
	Outer Loop
	Holes []Loop
}

func cleanLoop(l Loop) Loop {
	var out Loop
	for _, p := range l {
		if len(out) == 0 || p.Sub(out[len(out)-1]).Len() > 1e-9 {
			out = append(out, p)
		}
	}
	for len(out) > 1 && out[0].Sub(out[len(out)-1]).Len() <= 1e-9 {
		out = out[:len(out)-1]
	}
	// collinear vertices would make the cap triangulation skip them while the side walls keep
	// them (a T-junction), so remove them up front
	for changed := true; changed && len(out) > 3; {
		changed = false
		for i := 0; i < len(out) && len(out) > 3; i++ {
			a, b, c := out[(i-1+len(out))%len(out)], out[i], out[(i+1)%len(out)]
			if l := c.Sub(a).Len(); l > 0 && math.Abs(b.Sub(a).Cross(c.Sub(a)))/l < 1e-9 && b.Sub(a).Dot(c.Sub(b)) > 0 {
				out = append(out[:i], out[i+1:]...)
				i--
				changed = true
			}
		}
	}
	return out
}

func pointInLoop(p V2, l Loop) bool {
	in := false
	for i, j := 0, len(l)-1; i < len(l); j, i = i, i+1 {
		a, b := l[i], l[j]
		if (a.Y > p.Y) != (b.Y > p.Y) && p.X < (b.X-a.X)*(p.Y-a.Y)/(b.Y-a.Y)+a.X {
			in = !in
		}
	}
	return in
}

// regions resolves loop nesting into regions with oriented boundaries.
func (s *Sketch) regions() []Region {
	var loops []Loop
	for _, l := range s.loops {
		l = cleanLoop(l)
		if len(l) >= 3 && math.Abs(l.Area()) > 1e-12 {
			loops = append(loops, l)
		}
	}
	n := len(loops)
	depth := make([]int, n)
	parent := make([]int, n)
	for i := range loops {
		parent[i] = -1
		best := math.Inf(1)
		for j := range loops {
			if i == j || !pointInLoop(loops[i][0], loops[j]) {
				continue
			}
			depth[i]++
			if a := math.Abs(loops[j].Area()); a < best {
				best, parent[i] = a, j
			}
		}
	}
	idx := map[int]int{}
	var regs []Region
	for i, l := range loops {
		if depth[i]%2 == 0 {
			if l.Area() < 0 {
				l = l.reversed()
			}
			idx[i] = len(regs)
			regs = append(regs, Region{Outer: l})
		}
	}
	for i, l := range loops {
		if depth[i]%2 == 1 {
			ri, ok := idx[parent[i]]
			if !ok {
				continue
			}
			if l.Area() > 0 {
				l = l.reversed()
			}
			regs[ri].Holes = append(regs[ri].Holes, l)
		}
	}
	return regs
}
