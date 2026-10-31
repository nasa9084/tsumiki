package tsumiki

import (
	"math"
)

// camera is an orthographic projection: screen coordinates along right and up, depth along forward.
type camera struct {
	forward, right, up Vector3
}

func (c camera) project(p Vector3) (screen Vector2, depth float64) {
	return Vector2{p.Dot(c.right), p.Dot(c.up)}, p.Dot(c.forward)
}

// segment2 is a line segment on a drawing view, in model millimeters.
type segment2 [2]Vector2

// meshEdge is an undirected mesh edge with up to two adjacent triangles.
type meshEdge struct {
	a, b  int
	faces [2]int
	count int // number of adjacent triangles; anything but 2 is an open or non-manifold edge
}

// edgeSet holds the edges of a mesh and the unit normals of its triangles.
type edgeSet struct {
	mesh    *Mesh
	normals []Vector3
	edges   []meshEdge
}

func newEdgeSet(m *Mesh) *edgeSet {
	set := &edgeSet{mesh: m, normals: make([]Vector3, len(m.Triangles))}
	indexOf := map[edgeKey]int{}
	for i, t := range m.Triangles {
		a, b, c := m.Vertices[t[0]], m.Vertices[t[1]], m.Vertices[t[2]]
		if n := b.Sub(a).Cross(c.Sub(a)); n.Length() > 0 {
			set.normals[i] = n.Normalize()
		}
		for k := range 3 {
			from, to := t[k], t[(k+1)%3]
			key := edgeKey{min(from, to), max(from, to)}
			index, ok := indexOf[key]
			if !ok {
				index = len(set.edges)
				indexOf[key] = index
				set.edges = append(set.edges, meshEdge{a: key[0], b: key[1], faces: [2]int{-1, -1}})
			}
			edge := &set.edges[index]
			if edge.count < 2 {
				edge.faces[edge.count] = i
			}
			edge.count++
		}
	}
	return set
}

// isFeature reports whether an edge is a crease (its faces meet at more than the feature angle,
// given as its cosine) or a border of the surface.
func (s *edgeSet) isFeature(e *meshEdge, cosFeatureAngle float64) bool {
	if e.count != 2 {
		return true
	}
	return s.normals[e.faces[0]].Dot(s.normals[e.faces[1]]) < cosFeatureAngle
}

// isSilhouette reports whether one face of the edge looks towards the viewer and the other does
// not (it looks away or is seen edge-on, like the side wall of a plate seen from above).
func (s *edgeSet) isSilhouette(e *meshEdge, forward Vector3) bool {
	if e.count != 2 {
		return false
	}
	return s.facesViewer(e.faces[0], forward) != s.facesViewer(e.faces[1], forward)
}

// facesViewer reports whether triangle i looks towards a viewer looking along forward.
func (s *edgeSet) facesViewer(i int, forward Vector3) bool {
	return s.normals[i].Dot(forward) < -edgeOnCosine
}

// projectedTriangle is a front-facing triangle on the screen, counter-clockwise, with the depth
// of its corners.
type projectedTriangle struct {
	corners [3]Vector2
	depths  [3]float64
}

// depthBuffer answers "is this point hidden behind the surface?" for one view.
type depthBuffer struct {
	triangles []projectedTriangle
	origin    Vector2
	cell      float64
	columns   int
	rows      int
	cells     [][]int32
}

const (
	// edgeOnCosine is how close to perpendicular to the view a face may be and still count as
	// seen edge-on.
	edgeOnCosine = 1e-9
	// insideMargin lets a triangle cover points on its border, up to rounding (relative to the
	// edge length), so that the seams between the triangles of one face do not let hidden
	// lines show through.
	insideMargin = 1e-12
	depthMargin  = 1e-4 // a surface must be this much nearer than a point to hide it (mm)
)

func newDepthBuffer(set *edgeSet, c camera) *depthBuffer {
	buffer := &depthBuffer{}
	low, high := Vector2{math.Inf(1), math.Inf(1)}, Vector2{math.Inf(-1), math.Inf(-1)}
	for i, t := range set.mesh.Triangles {
		if !set.facesViewer(i, c.forward) { // back-facing or edge-on: a closed surface in front hides it
			continue
		}
		var projected projectedTriangle
		for k := range 3 {
			projected.corners[k], projected.depths[k] = c.project(set.mesh.Vertices[t[k]])
			low = Vector2{math.Min(low.X, projected.corners[k].X), math.Min(low.Y, projected.corners[k].Y)}
			high = Vector2{math.Max(high.X, projected.corners[k].X), math.Max(high.Y, projected.corners[k].Y)}
		}
		if twiceSignedArea(projected.corners[0], projected.corners[1], projected.corners[2]) < 0 {
			projected.corners[1], projected.corners[2] = projected.corners[2], projected.corners[1]
			projected.depths[1], projected.depths[2] = projected.depths[2], projected.depths[1]
		}
		buffer.triangles = append(buffer.triangles, projected)
	}
	if len(buffer.triangles) == 0 {
		return buffer
	}
	buffer.buildGrid(low, high)
	return buffer
}

// buildGrid buckets the triangles into square cells covering low..high.
func (b *depthBuffer) buildGrid(low, high Vector2) {
	size := high.Sub(low)
	b.cell = math.Max(math.Sqrt(size.X*size.Y/float64(len(b.triangles)))*2, 1e-3)
	b.origin = low
	b.columns = int(size.X/b.cell) + 1
	b.rows = int(size.Y/b.cell) + 1
	b.cells = make([][]int32, b.columns*b.rows)
	for i, t := range b.triangles {
		triangleLow, triangleHigh := t.corners[0], t.corners[0]
		for _, corner := range t.corners[1:] {
			triangleLow = Vector2{math.Min(triangleLow.X, corner.X), math.Min(triangleLow.Y, corner.Y)}
			triangleHigh = Vector2{math.Max(triangleHigh.X, corner.X), math.Max(triangleHigh.Y, corner.Y)}
		}
		firstColumn, firstRow := b.cellOf(triangleLow)
		lastColumn, lastRow := b.cellOf(triangleHigh)
		for column := firstColumn; column <= lastColumn; column++ {
			for row := firstRow; row <= lastRow; row++ {
				b.cells[row*b.columns+column] = append(b.cells[row*b.columns+column], int32(i))
			}
		}
	}
}

func (b *depthBuffer) cellOf(p Vector2) (column, row int) {
	column = min(max(int((p.X-b.origin.X)/b.cell), 0), b.columns-1)
	row = min(max(int((p.Y-b.origin.Y)/b.cell), 0), b.rows-1)
	return column, row
}

// hidden reports whether a surface lies in front of the point at screen position s and depth.
func (b *depthBuffer) hidden(s Vector2, depth float64) bool {
	if b.cells == nil {
		return false
	}
	column, row := b.cellOf(s)
	for _, index := range b.cells[row*b.columns+column] {
		t := &b.triangles[index]
		if surfaceDepth, ok := t.depthAt(s); ok && surfaceDepth < depth-depthMargin {
			return true
		}
	}
	return false
}

// nearest returns the depth of the surface nearest to the viewer at s, if any.
func (b *depthBuffer) nearest(s Vector2) (float64, bool) {
	if b.cells == nil {
		return 0, false
	}
	column, row := b.cellOf(s)
	best, found := math.Inf(1), false
	for _, index := range b.cells[row*b.columns+column] {
		if depth, ok := b.triangles[index].depthAt(s); ok && depth < best {
			best, found = depth, true
		}
	}
	return best, found
}

// depthAt returns the triangle's depth at s when s lies inside it or on its border.
func (t *projectedTriangle) depthAt(s Vector2) (float64, bool) {
	var weights [3]float64
	area := twiceSignedArea(t.corners[0], t.corners[1], t.corners[2])
	if area <= 0 {
		return 0, false
	}
	for k := range 3 {
		from, to := t.corners[(k+1)%3], t.corners[(k+2)%3]
		edgeLength := to.Sub(from).Length()
		twice := twiceSignedArea(from, to, s)
		if twice < -insideMargin*edgeLength {
			return 0, false
		}
		weights[k] = max(twice/area, 0) // a point just outside by rounding counts as on the border
	}
	total := weights[0] + weights[1] + weights[2]
	return (weights[0]*t.depths[0] + weights[1]*t.depths[1] + weights[2]*t.depths[2]) / total, true
}

// splitByVisibility cuts the 3D segment a-b into the pieces that are visible and hidden in the
// view, sampling it every step millimeters.
func (b *depthBuffer) splitByVisibility(c camera, a, bEnd Vector3, step float64) (visible, hidden []segment2) {
	pointAt := func(t float64) Vector2 { s, _ := c.project(a.Interpolate(bEnd, t)); return s }
	isHidden := func(t float64) bool { return b.hidden(c.project(a.Interpolate(bEnd, t))) }
	splitRuns(samplesFor(a.Sub(bEnd).Length(), step), isHidden, func(start, end float64, hiddenRun bool) {
		piece := segment2{pointAt(start), pointAt(end)}
		if hiddenRun {
			hidden = append(hidden, piece)
		} else {
			visible = append(visible, piece)
		}
	})
	return visible, hidden
}

// samplesFor returns how many samples a line of the given length needs at the given spacing.
func samplesFor(length, step float64) int {
	return min(max(int(math.Ceil(length/step)), 1), 2000)
}

// splitRuns divides [0, 1] into the longest runs over which predicate keeps its value. It samples
// n evenly spaced points and locates every change of value by bisection; emit receives the runs
// in order.
func splitRuns(n int, predicate func(t float64) bool, emit func(start, end float64, value bool)) {
	start, state := 0.0, predicate(0.5/float64(n))
	for i := 1; i < n; i++ {
		middle := (float64(i) + 0.5) / float64(n)
		next := predicate(middle)
		if next == state {
			continue
		}
		low, high := middle-1/float64(n), middle
		for range 12 {
			if probe := (low + high) / 2; predicate(probe) == state {
				low = probe
			} else {
				high = probe
			}
		}
		emit(start, (low+high)/2, state)
		start, state = (low+high)/2, next
	}
	emit(start, 1, state)
}
