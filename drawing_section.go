package tsumiki

import (
	"errors"
	"fmt"
	"slices"
)

// sectionSpec is a cutting plane perpendicular to an axis (0 = X, 1 = Y, 2 = Z).
type sectionSpec struct {
	axis   int
	offset float64
}

// camera looks at the cut face: the part in front of the plane is removed, as if the viewer
// stood on the plane.
func (spec sectionSpec) camera() camera {
	switch spec.axis {
	case 0:
		return ViewRight.camera()
	case 1:
		return ViewFront.camera()
	default:
		return ViewTop.camera()
	}
}

func component(v Vector3, axis int) float64 { return [3]float64{v.X, v.Y, v.Z}[axis] }

func withComponent(v Vector3, axis int, value float64) Vector3 {
	c := [3]float64{v.X, v.Y, v.Z}
	c[axis] = value
	return Vector3{c[0], c[1], c[2]}
}

// makeSections builds the section views and marks their cutting planes on the top view (cuts
// along X and Y) or the front view (cuts along Z).
func makeSections(m *Mesh, o drawingOptions, top, front *panel) ([]*panel, error) {
	var panels []*panel
	for i, spec := range o.sections {
		letter := string(rune('A' + i%26))
		if i >= 26 {
			letter += fmt.Sprint(i / 26)
		}
		p, err := sectionPanel(m, spec, letter, o)
		if err != nil {
			return nil, err
		}
		panels = append(panels, p)
		parent := top
		if spec.axis == 2 {
			parent = front
		}
		parent.marks = append(parent.marks, cuttingPlaneMarks(parent, spec, letter, o)...)
	}
	return panels, nil
}

func sectionPanel(m *Mesh, spec sectionSpec, letter string, o drawingOptions) (*panel, error) {
	c := spec.camera()
	clipped, cutFaces, err := clipMesh(m, spec.axis, spec.offset, c)
	if err != nil {
		return nil, fmt.Errorf("tsumiki: section %s-%s: %w", letter, letter, err)
	}
	p := newPanel(newEdgeSet(clipped), c, fmt.Sprintf("SECTION %s-%s", letter, letter), o, false)
	for _, region := range cutFaces {
		p.hatch = append(p.hatch, region.Outer)
		for _, hole := range region.Holes {
			p.hatch = append(p.hatch, hole)
		}
	}
	return p, nil
}

// sectionNudge moves the cutting plane a hair towards the removed side, so that no vertex lies
// exactly on it and faces lying in the plane are kept whole (mm).
const sectionNudge = 1.234e-5

// clipMesh keeps the part of a closed mesh beyond the plane (component axis = cut) as seen by a
// viewer looking along c.forward, and closes it with the cut faces. It returns the clipped mesh
// and the cut faces in view coordinates.
func clipMesh(m *Mesh, axis int, cut float64, c camera) (*Mesh, []Region, error) {
	sign := component(c.forward, axis)
	cut -= sign * sectionNudge
	clipper := meshClipper{source: m, axis: axis, cut: cut, sign: sign, out: &Mesh{},
		kept: map[int]int{}, crossings: map[edgeKey]int{}, links: map[int][]int{}}
	for _, t := range m.Triangles {
		clipper.clipTriangle(t)
	}
	if len(clipper.out.Triangles) == 0 || len(clipper.links) == 0 {
		return nil, nil, errors.New("the cutting plane misses the part")
	}
	// The cut faces lie in a plane whose U and V are the view's right and up, so their sketch
	// coordinates are the view coordinates.
	plane := Plane{Origin: withComponent(Vector3{}, axis, cut), U: c.right, V: c.up, Normal: c.forward.Negate()}
	sketch := &Sketch{Plane: plane, Tolerance: DefaultTolerance}
	for _, loop := range clipper.loops() {
		points := make(Loop, len(loop))
		for i, index := range loop {
			points[i], _ = c.project(clipper.out.Vertices[index])
		}
		sketch.loops = append(sketch.loops, points)
	}
	regions := sketch.regions()
	for _, region := range regions {
		points, triangles := triangulateRegion(region.Outer, region.Holes)
		base := len(clipper.out.Vertices)
		for _, point := range points {
			clipper.out.Vertices = append(clipper.out.Vertices, plane.Point(point.X, point.Y, 0))
		}
		for _, t := range triangles {
			clipper.out.Triangles = append(clipper.out.Triangles, Triangle{base + t[0], base + t[1], base + t[2]})
		}
	}
	return clipper.out, regions, nil
}

// meshClipper cuts triangles by a plane, keeping the side where sign * (coordinate - cut) > 0.
type meshClipper struct {
	source    *Mesh
	axis      int
	cut, sign float64
	out       *Mesh
	kept      map[int]int     // source vertex -> output vertex
	crossings map[edgeKey]int // source edge -> output vertex where the plane crosses it
	links     map[int][]int   // the cut outline: crossing vertex -> its neighbors along the outline
}

func (c *meshClipper) distance(index int) float64 {
	return c.sign * (component(c.source.Vertices[index], c.axis) - c.cut)
}

func (c *meshClipper) keptVertex(index int) int {
	if out, ok := c.kept[index]; ok {
		return out
	}
	c.kept[index] = len(c.out.Vertices)
	c.out.Vertices = append(c.out.Vertices, c.source.Vertices[index])
	return c.kept[index]
}

func (c *meshClipper) crossing(a, b int) int {
	key := edgeKey{min(a, b), max(a, b)}
	if out, ok := c.crossings[key]; ok {
		return out
	}
	t := c.distance(a) / (c.distance(a) - c.distance(b))
	point := c.source.Vertices[a].Interpolate(c.source.Vertices[b], t)
	c.crossings[key] = len(c.out.Vertices)
	c.out.Vertices = append(c.out.Vertices, withComponent(point, c.axis, c.cut))
	return c.crossings[key]
}

// clipTriangle keeps the part of t on the kept side (a triangle or a quadrilateral, split into
// triangles) and records where the plane crosses it.
func (c *meshClipper) clipTriangle(t Triangle) {
	var polygon, crossed []int
	for k := range 3 {
		current, next := t[k], t[(k+1)%3]
		if c.distance(current) > 0 {
			polygon = append(polygon, c.keptVertex(current))
		}
		if (c.distance(current) > 0) != (c.distance(next) > 0) {
			crossing := c.crossing(current, next)
			polygon, crossed = append(polygon, crossing), append(crossed, crossing)
		}
	}
	for i := 1; i+1 < len(polygon); i++ {
		c.out.Triangles = append(c.out.Triangles, Triangle{polygon[0], polygon[i], polygon[i+1]})
	}
	if len(crossed) == 2 {
		c.links[crossed[0]] = append(c.links[crossed[0]], crossed[1])
		c.links[crossed[1]] = append(c.links[crossed[1]], crossed[0])
	}
}

// loops chains the cut outline into closed loops of output vertices.
func (c *meshClipper) loops() [][]int {
	starts := make([]int, 0, len(c.links))
	for vertex := range c.links {
		starts = append(starts, vertex)
	}
	slices.Sort(starts)
	visited := map[int]bool{}
	var loops [][]int
	for _, start := range starts {
		if visited[start] {
			continue
		}
		loop := []int{start}
		visited[start] = true
		for previous, current := -1, start; ; {
			next := -1
			for _, candidate := range c.links[current] {
				if candidate != previous && !visited[candidate] {
					next = candidate
					break
				}
			}
			if next < 0 {
				break
			}
			loop = append(loop, next)
			visited[next] = true
			previous, current = current, next
		}
		if len(loop) >= 3 {
			loops = append(loops, loop)
		}
	}
	return loops
}

// cuttingPlaneMarks draws the cutting plane of a section on its parent view: a chain line across
// the view with thick ends, and arrows in the direction of sight labeled with the letter.
func cuttingPlaneMarks(parent *panel, spec sectionSpec, letter string, o drawingOptions) []mark {
	size := parent.boxSize()
	var a, b, sight Vector2
	switch spec.axis {
	case 0: // a vertical line x = offset on the top view; the section looks along -X
		x := parent.rel(Vector2{spec.offset, 0}).X
		a, b, sight = Vector2{x, size.Y}, Vector2{x, 0}, Vector2{-1, 0}
	case 1: // a horizontal line y = offset on the top view; the section looks along +Y (up the paper)
		y := parent.rel(Vector2{0, spec.offset}).Y
		a, b, sight = Vector2{0, y}, Vector2{size.X, y}, Vector2{0, -1}
	default: // a horizontal line z = offset on the front view; the section looks down
		y := parent.rel(Vector2{0, spec.offset}).Y
		a, b, sight = Vector2{0, y}, Vector2{size.X, y}, Vector2{0, 1}
	}
	along := unit(b.Sub(a))
	const overhang, thickLength, arrowLength = 4.0, 4.0, 6.0
	start, end := a.Sub(along.Scale(overhang)), b.Add(along.Scale(overhang))
	marks := []mark{
		centerMark(start, end),
		thickMark(start, start.Add(along.Scale(thickLength))),
		thickMark(end, end.Sub(along.Scale(thickLength))),
	}
	for _, at := range []Vector2{start, end} {
		tip := at.Add(sight.Scale(arrowLength))
		marks = append(marks, lineMark(at, tip), arrowMark(tip, sight),
			textMark(tip.Add(sight.Scale(o.textSize*0.9)).Add(Vector2{0, o.textSize * 0.35}), letter, "middle", false,
				o.textSize))
	}
	return marks
}
