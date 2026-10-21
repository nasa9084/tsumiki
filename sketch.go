package tsumiki

import (
	"math"
)

// Plane is a sketch plane. U x V = N.
type Plane struct{ Origin, U, V, Normal Vector3 }

// XY is the plane z = z0 (U=X, V=Y, Normal=+Z).
func XY(z0 float64) Plane {
	return Plane{Vector3{0, 0, z0}, Vector3{1, 0, 0}, Vector3{0, 1, 0}, Vector3{0, 0, 1}}
}

// XZ is the plane y = y0 (U=X, V=Z, Normal=-Y).
func XZ(y0 float64) Plane {
	return Plane{Vector3{0, y0, 0}, Vector3{1, 0, 0}, Vector3{0, 0, 1}, Vector3{0, -1, 0}}
}

// YZ is the plane x = x0 (U=Y, V=Z, Normal=+X).
func YZ(x0 float64) Plane {
	return Plane{Vector3{x0, 0, 0}, Vector3{0, 1, 0}, Vector3{0, 0, 1}, Vector3{1, 0, 0}}
}

// Point maps plane coordinates (u,v) and an offset along the plane normal to world space.
func (p Plane) Point(u, v, normalOffset float64) Vector3 {
	return p.Origin.Add(p.U.Scale(u)).Add(p.V.Scale(v)).Add(p.Normal.Scale(normalOffset))
}

// Loop is a closed polygon (the closing edge is implicit).
type Loop []Vector2

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

// DefaultTolerance is the default chord tolerance used to tessellate arcs (mm).
const DefaultTolerance = 0.01

// Sketch is a set of closed loops on a plane. Nesting decides material:
// a loop inside one loop is a region, inside two is a hole, and so on (even-odd).
type Sketch struct {
	Plane     Plane
	Tolerance float64
	loops     []Loop
}

// NewSketch starts an empty sketch on p.
func NewSketch(p Plane) *Sketch { return &Sketch{Plane: p, Tolerance: DefaultTolerance} }

// Polygon adds a closed polygon.
func (s *Sketch) Polygon(points ...Vector2) *Sketch {
	s.loops = append(s.loops, append(Loop(nil), points...))
	return s
}

// Circle adds a circle tessellated within the sketch tolerance.
func (s *Sketch) Circle(centerX, centerY, radius float64) *Sketch {
	n := max(arcSteps(radius, 2*math.Pi, s.Tolerance), 8)
	l := make(Loop, n)
	for i := range l {
		a := 2 * math.Pi * float64(i) / float64(n)
		l[i] = Vector2{centerX + radius*math.Cos(a), centerY + radius*math.Sin(a)}
	}
	s.loops = append(s.loops, l)
	return s
}

// Rect adds an axis-aligned rectangle centered at (centerX,centerY).
func (s *Sketch) Rect(centerX, centerY, width, height float64) *Sketch {
	return s.RoundedRect(centerX, centerY, width, height, 0, 0)
}

// RoundedRect adds a width x height rectangle with corner radius, rotated by rotationDegrees around its center.
func (s *Sketch) RoundedRect(centerX, centerY, width, height, radius, rotationDegrees float64) *Sketch {
	halfWidth, halfHeight := width/2, height/2
	radius = math.Min(radius, math.Min(halfWidth, halfHeight)) // a radius beyond half the short side would self-intersect
	var l Loop
	if radius <= 0 {
		l = Loop{{halfWidth, -halfHeight}, {halfWidth, halfHeight}, {-halfWidth, halfHeight}, {-halfWidth, -halfHeight}}
	} else {
		n := arcSteps(radius, math.Pi/2, s.Tolerance)
		corners := []struct {
			c          Vector2
			startAngle float64
		}{
			{Vector2{halfWidth - radius, -halfHeight + radius}, -math.Pi / 2},
			{Vector2{halfWidth - radius, halfHeight - radius}, 0},
			{Vector2{-halfWidth + radius, halfHeight - radius}, math.Pi / 2},
			{Vector2{-halfWidth + radius, -halfHeight + radius}, math.Pi},
		}
		for _, c := range corners {
			for i := 0; i <= n; i++ {
				a := c.startAngle + (math.Pi/2)*float64(i)/float64(n)
				l = append(l, Vector2{c.c.X + radius*math.Cos(a), c.c.Y + radius*math.Sin(a)})
			}
		}
	}
	rotationRadians := Radians(rotationDegrees)
	for i := range l {
		l[i] = l[i].Rotate(rotationRadians).Add(Vector2{centerX, centerY})
	}
	s.loops = append(s.loops, l)
	return s
}

// Translate moves every loop already in the sketch.
func (s *Sketch) Translate(dx, dy float64) *Sketch {
	for _, l := range s.loops {
		for i := range l {
			l[i] = l[i].Add(Vector2{dx, dy})
		}
	}
	return s
}

// Rotate rotates every loop already in the sketch around the sketch origin.
func (s *Sketch) Rotate(degrees float64) *Sketch {
	for _, l := range s.loops {
		for i := range l {
			l[i] = l[i].Rotate(Radians(degrees))
		}
	}
	return s
}

// Path is a line/arc chain that becomes one loop when closed.
type Path struct {
	s      *Sketch
	points []Vector2
}

// Path starts a line/arc path at (x,y).
func (s *Sketch) Path(x, y float64) *Path { return &Path{s: s, points: []Vector2{{x, y}}} }

// LineTo adds a straight segment.
func (p *Path) LineTo(x, y float64) *Path {
	p.points = append(p.points, Vector2{x, y})
	return p
}

// ArcTo adds an arc about (centerX,centerY) from the current point to (x,y).
// The radius is taken from the current point. ccw selects the sweep direction.
func (p *Path) ArcTo(x, y, centerX, centerY float64, counterClockwise bool) *Path {
	c := Vector2{centerX, centerY}
	current := p.points[len(p.points)-1]
	r := current.Sub(c).Length()
	startAngle := math.Atan2(current.Y-centerY, current.X-centerX)
	endAngle := math.Atan2(y-centerY, x-centerX)
	sweep := endAngle - startAngle
	if counterClockwise {
		for sweep <= 0 {
			sweep += 2 * math.Pi
		}
	} else {
		for sweep >= 0 {
			sweep -= 2 * math.Pi
		}
	}
	n := arcSteps(r, math.Abs(sweep), p.s.Tolerance)
	for i := 1; i < n; i++ {
		a := startAngle + sweep*float64(i)/float64(n)
		p.points = append(p.points, Vector2{centerX + r*math.Cos(a), centerY + r*math.Sin(a)})
	}
	p.points = append(p.points, Vector2{x, y})
	return p
}

// Close finishes the loop and returns the sketch.
func (p *Path) Close() *Sketch {
	p.s.loops = append(p.s.loops, Loop(p.points))
	return p.s
}

func arcSteps(radius, sweep, tolerance float64) int {
	if tolerance <= 0 {
		tolerance = DefaultTolerance
	}
	deltaAngle := math.Pi / 2
	if radius > tolerance {
		deltaAngle = 2 * math.Acos(1-tolerance/radius)
	}
	n := max(int(math.Ceil(sweep/deltaAngle)), 1)
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
		if len(out) == 0 || p.Sub(out[len(out)-1]).Length() > 1e-9 {
			out = append(out, p)
		}
	}
	for len(out) > 1 && out[0].Sub(out[len(out)-1]).Length() <= 1e-9 {
		out = out[:len(out)-1]
	}
	// collinear vertices would make the cap triangulation skip them while the side walls keep
	// them (a T-junction), so remove them up front
	for changed := true; changed && len(out) > 3; {
		changed = false
		for i := 0; i < len(out) && len(out) > 3; i++ {
			a, b, c := out[(i-1+len(out))%len(out)], out[i], out[(i+1)%len(out)]
			if l := c.Sub(a).Length(); l > 0 && math.Abs(b.Sub(a).Cross(c.Sub(a)))/l < 1e-9 && b.Sub(a).Dot(c.Sub(b)) > 0 {
				out = append(out[:i], out[i+1:]...)
				i--
				changed = true
			}
		}
	}
	return out
}

func pointInLoop(p Vector2, l Loop) bool {
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
	regionIndexByLoop := map[int]int{}
	var regions []Region
	for i, l := range loops {
		if depth[i]%2 == 0 {
			if l.Area() < 0 {
				l = l.reversed()
			}
			regionIndexByLoop[i] = len(regions)
			regions = append(regions, Region{Outer: l})
		}
	}
	for i, l := range loops {
		if depth[i]%2 == 1 {
			regionIndex, ok := regionIndexByLoop[parent[i]]
			if !ok {
				continue
			}
			if l.Area() > 0 {
				l = l.reversed()
			}
			regions[regionIndex].Holes = append(regions[regionIndex].Holes, l)
		}
	}
	return regions
}
