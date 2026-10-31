package tsumiki

import (
	"cmp"
	"math"
	"slices"
)

const (
	// circleTolerance is how far from a circle a point may lie and still be on it (mm). Facets cut
	// inside a curve by up to the 0.01 mm chord tolerance, and where other features split a facet
	// edge the extra point lies on the chord.
	circleTolerance = 0.012
	maxSegmentSine  = 0.4 // sin of half the largest angle one facet of a curve may span (~47°)
	// maxSagitta is how far a facet of a curve may cut inside it (mm): sketches facet curves to
	// DefaultTolerance, with some room for the 1 µm snapping of vertices. Polygons drawn by hand
	// with coarser facets are not taken for curves.
	maxSagitta        = DefaultTolerance * 1.5
	minArcRadius      = 0.2  // smaller arcs are not dimensioned (mm)
	minArcSegments    = 3    // fewer segments than this are not recognized as an arc
	sameValueSpacing  = 0.01 // dimension values closer than this are the same (mm)
	straightTolerance = 2e-3 // a point this close to the line through its neighbors is no corner (mm)
	minArcDegrees     = 10   // runs that turn less than this are treated as straight
	minCircleCoverage = 30   // circles seen over less than this many degrees are not dimensioned
	coverageBins      = 72   // resolution of the record of which directions of a circle were seen
)

// faceOnArc is a circle or an arc seen square-on in a view, in that view's coordinates.
type faceOnArc struct {
	center, middle Vector2 // middle: a point on the arc, where its label points
	radius         float64
	full           bool
}

// circlePiece is a run of drawn edges that lies on a circle in the view.
type circlePiece struct {
	points        []Vector2
	center        Vector2
	radius, sweep float64
	depth         float64 // how far the piece reaches towards and away from the viewer
	full          bool
}

// faceOnArcs returns the visible circles and arcs of the view, one per distinct circle. Circles
// are recognized in the view's own coordinates, so an outline that zigzags in depth (where two
// faceted surfaces meet) still counts. Where other features cut into a circle it is found in
// pieces; the pieces are fitted together, and circles of which less than minCircleCoverage is seen
// are left out, since the radius of a short piece is uncertain.
func (p *panel) faceOnArcs() []faceOnArc {
	pieces := p.circlePieces()
	slices.SortStableFunc(pieces, func(a, b circlePiece) int { return cmp.Compare(b.sweep, a.sweep) })
	var clusters []*circleCluster
	for _, piece := range pieces {
		if cluster := joinable(clusters, piece); cluster != nil {
			cluster.pieces = append(cluster.pieces, piece)
		} else {
			clusters = append(clusters, &circleCluster{pieces: []circlePiece{piece}, center: piece.center,
				radius: piece.radius, middle: piece.points[len(piece.points)/2]})
		}
	}
	var result []faceOnArc
	for _, c := range clusters {
		c.refine()
		if c.full || c.coverage >= Radians(minCircleCoverage) {
			result = append(result, faceOnArc{center: c.center, middle: c.middle, radius: c.radius, full: c.full})
		}
	}
	return result
}

// circlePieces finds the visible circular runs of the drawn edges.
func (p *panel) circlePieces() []circlePiece {
	neighbors := map[int][]int{}
	for _, i := range p.drawnEdges {
		e := p.set.edges[i]
		neighbors[e.a] = append(neighbors[e.a], e.b)
		neighbors[e.b] = append(neighbors[e.b], e.a)
	}
	var pieces []circlePiece
	for _, chain := range chainsOf(neighbors) {
		points := make([]Vector2, len(chain))
		for i, index := range chain {
			points[i], _ = p.camera.project(p.set.mesh.Vertices[index])
		}
		kept := withoutStraightJoints(points)
		for _, run := range arcsAlong(points, kept) {
			corners := make([]Vector3, len(run.indices))
			for i, k := range run.indices {
				corners[i] = p.set.mesh.Vertices[chain[k]]
			}
			low, high := math.Inf(1), math.Inf(-1)
			for _, corner := range corners {
				_, depth := p.camera.project(corner)
				low, high = math.Min(low, depth), math.Max(high, depth)
			}
			run.piece.depth = high - low
			// A circle seen square-on keeps its depth, give or take an outline that zigzags between
			// two faceted surfaces; a curve that runs away from the viewer only looks like an arc.
			if run.piece.depth <= 0.05+0.05*run.piece.radius && p.mostlyVisible(corners) {
				pieces = append(pieces, run.piece)
			}
		}
	}
	return pieces
}

// mostlyVisible reports whether most of the points along a polyline are in sight.
func (p *panel) mostlyVisible(points []Vector3) bool {
	visible, total := 0, 0
	step := max(len(points)/8, 1)
	for i := 0; i+1 < len(points); i += step {
		total++
		if p.isVisible(points[i].Interpolate(points[i+1], 0.5)) {
			visible++
		}
	}
	return visible*2 >= total
}

// chainsOf splits a graph into chains running between vertices that do not have exactly two
// neighbors, plus closed loops; a closed loop repeats its first vertex at the end.
func chainsOf(neighbors map[int][]int) [][]int {
	vertices := make([]int, 0, len(neighbors))
	for v := range neighbors {
		vertices = append(vertices, v)
	}
	slices.Sort(vertices)
	used := map[edgeKey]bool{}
	walk := func(from, to int) []int {
		chain := []int{from}
		for {
			used[edgeKey{min(from, to), max(from, to)}] = true
			chain = append(chain, to)
			if len(neighbors[to]) != 2 || to == chain[0] {
				return chain
			}
			following := neighbors[to][0]
			if following == from || used[edgeKey{min(to, following), max(to, following)}] {
				following = neighbors[to][1]
			}
			from, to = to, following
		}
	}
	var chains [][]int
	for _, onlyEnds := range []bool{true, false} { // open chains first, then the remaining loops
		for _, v := range vertices {
			if onlyEnds && len(neighbors[v]) == 2 {
				continue
			}
			for _, u := range neighbors[v] {
				if !used[edgeKey{min(u, v), max(u, v)}] {
					chains = append(chains, walk(v, u))
				}
			}
		}
	}
	return chains
}

// withoutStraightJoints returns the indices of the points that are real corners: points on the
// straight line between their neighbors (where booleans split an edge) and points on top of
// their predecessor (an edge seen end-on) are left out. A loop starts at a real corner and
// repeats it at the end.
func withoutStraightJoints(points []Vector2) []int {
	n := len(points) - 1
	order := make([]int, len(points))
	for i := range order {
		order[i] = i
	}
	if n > 2 && points[0] == points[n] {
		for i := range n {
			if !onSegmentLine(points[(i+n-1)%n], points[i], points[(i+1)%n]) {
				for k := range n {
					order[k] = (i + k) % n
				}
				order[n] = i
				break
			}
		}
	}
	kept := []int{order[0]}
	for k := 1; k+1 < len(order); k++ {
		previous, point, next := points[kept[len(kept)-1]], points[order[k]], points[order[k+1]]
		if point.Sub(previous).Length() < straightTolerance || onSegmentLine(previous, point, next) {
			continue
		}
		kept = append(kept, order[k])
	}
	return append(kept, order[len(order)-1])
}

// onSegmentLine reports whether point lies on the straight line from previous to next.
func onSegmentLine(previous, point, next Vector2) bool {
	chord := next.Sub(previous)
	return chord.Length() > 0 && math.Abs(chord.Cross(point.Sub(previous)))/chord.Length() < straightTolerance
}

// circleRun is a circular run along a chain: the piece and the chain indices of its points.
type circleRun struct {
	piece   circlePiece
	indices []int
}

// arcsAlong splits a polyline (points, reduced to the corners in kept) into its longest circular
// runs. One run all the way round a loop is a full circle.
func arcsAlong(points []Vector2, kept []int) []circleRun {
	closed := len(kept) > 3 && kept[0] == kept[len(kept)-1]
	corners := make([]Vector2, len(kept))
	for i, k := range kept {
		corners[i] = points[k]
	}
	var runs []circleRun
	for start := 0; start+minArcSegments < len(corners); {
		end := -1
		var best circlePiece
		for candidate := start + minArcSegments; candidate < len(corners); candidate++ {
			fit, ok := fitPiece(corners[start : candidate+1])
			if !ok {
				break
			}
			end, best = candidate, fit
		}
		if end < 0 || best.sweep < Radians(minArcDegrees) {
			start++
			continue
		}
		best.full = closed && start == 0 && end == len(corners)-1
		runs = append(runs, circleRun{piece: best, indices: kept[start : end+1]})
		start = end
	}
	return runs
}

// fitPiece fits a circle to a polyline and reports whether every point lies on it and every
// segment is short enough to be a facet of a curve rather than a side of a polygon.
func fitPiece(points []Vector2) (circlePiece, bool) {
	center, radius, ok := fitCircle(points)
	if !ok || radius > 1e4 || radius < minArcRadius {
		return circlePiece{}, false
	}
	sweep := 0.0
	for i, point := range points {
		if math.Abs(point.Sub(center).Length()-radius) > circleTolerance {
			return circlePiece{}, false
		}
		if i > 0 {
			chord := point.Sub(points[i-1]).Length()
			if chord > 2*radius*maxSegmentSine || radius-math.Sqrt(math.Max(radius*radius-chord*chord/4, 0)) > maxSagitta {
				return circlePiece{}, false
			}
			sweep += 2 * math.Asin(math.Min(chord/(2*radius), 1))
		}
	}
	return circlePiece{points: points, center: center, radius: radius, sweep: sweep}, true
}

// circleCluster gathers the pieces of one circle.
type circleCluster struct {
	pieces         []circlePiece
	center, middle Vector2
	radius         float64
	coverage       float64 // the angle the pieces cover, in radians
	full           bool
}

// joinable returns the cluster whose circle is roughly the piece's: short pieces give rough
// circles, so the match is loose and refine sorts out the stragglers.
func joinable(clusters []*circleCluster, piece circlePiece) *circleCluster {
	for _, c := range clusters {
		slack := 0.05 + 0.05*c.radius
		if piece.center.Sub(c.center).Length() < slack && math.Abs(piece.radius-c.radius) < slack {
			return c
		}
	}
	return nil
}

// refine fits the circle to all pieces, drops the pieces that do not lie on it, fits again and
// works out how much of the circle the pieces cover.
func (c *circleCluster) refine() {
	c.fit(c.pieces)
	var kept []circlePiece
	for _, piece := range c.pieces {
		if piece.fits(c.center, c.radius) {
			kept = append(kept, piece)
		}
	}
	if len(kept) == 0 {
		return
	}
	c.fit(kept)
	var seen [coverageBins]bool
	for _, piece := range kept {
		c.full = c.full || piece.full
		for i := 1; i < len(piece.points); i++ {
			markDirections(&seen, piece.points[i-1].Sub(c.center), piece.points[i].Sub(c.center))
		}
	}
	covered := 0
	for _, bin := range seen {
		if bin {
			covered++
		}
	}
	c.coverage = 2 * math.Pi * float64(covered) / coverageBins
	c.full = c.full || covered == coverageBins
}

// fit centers the circle by least squares and takes the farthest point as its radius: vertices
// lie on the circle, while points where other features cut it lie on chords inside it.
func (c *circleCluster) fit(pieces []circlePiece) {
	var points []Vector2
	for _, piece := range pieces {
		points = append(points, fitPoints(piece.points)...)
	}
	center, _, ok := fitCircle(points)
	if !ok {
		return
	}
	c.center, c.radius = center, 0
	for _, point := range points {
		c.radius = math.Max(c.radius, point.Sub(center).Length())
	}
}

// markDirections records the directions from the center swept from a to b (less than half a
// turn apart).
func markDirections(seen *[coverageBins]bool, a, b Vector2) {
	bin := func(v Vector2) int {
		angle := math.Atan2(v.Y, v.X)
		return int(math.Floor((angle+math.Pi)/(2*math.Pi)*coverageBins)) % coverageBins
	}
	from, to := bin(a), bin(b)
	if a.Cross(b) < 0 {
		from, to = to, from
	}
	for k := from; ; k = (k + 1) % coverageBins {
		seen[k] = true
		if k == to {
			return
		}
	}
}

// fits reports whether the piece lies on the circle.
func (piece circlePiece) fits(center Vector2, radius float64) bool {
	for _, point := range fitPoints(piece.points) {
		if math.Abs(point.Sub(center).Length()-radius) > 2*circleTolerance {
			return false
		}
	}
	return true
}

// fitPoints leaves out the ends of a piece when it has enough points: an end can be a point where
// another feature cuts the circle, which lies on a chord rather than on the circle.
func fitPoints(points []Vector2) []Vector2 {
	if len(points) >= 5 {
		return points[1 : len(points)-1]
	}
	return points
}

// fitCircle fits a circle to points by least squares (Kåsa's method).
func fitCircle(points []Vector2) (center Vector2, radius float64, ok bool) {
	var mean Vector2
	for _, point := range points {
		mean = mean.Add(point)
	}
	mean = mean.Scale(1 / float64(len(points)))
	// minimize the sum of (x² + y² + d·x + e·y + f)² over the points shifted to their mean
	var sxx, sxy, syy, sx, sy, sxz, syz, sz float64
	for _, point := range points {
		x, y := point.X-mean.X, point.Y-mean.Y
		z := x*x + y*y
		sxx, sxy, syy, sx, sy = sxx+x*x, sxy+x*y, syy+y*y, sx+x, sy+y
		sxz, syz, sz = sxz+x*z, syz+y*z, sz+z
	}
	n := float64(len(points))
	d, e, f, ok := solve3([3][3]float64{{sxx, sxy, sx}, {sxy, syy, sy}, {sx, sy, n}}, [3]float64{-sxz, -syz, -sz})
	if !ok {
		return mean, 0, false
	}
	center = Vector2{-d / 2, -e / 2}
	return center.Add(mean), math.Sqrt(math.Max(center.Dot(center)-f, 0)), true
}

// solve3 solves a 3×3 linear system by Cramer's rule.
func solve3(a [3][3]float64, b [3]float64) (x, y, z float64, ok bool) {
	determinant := func(m [3][3]float64) float64 {
		return m[0][0]*(m[1][1]*m[2][2]-m[1][2]*m[2][1]) - m[0][1]*(m[1][0]*m[2][2]-m[1][2]*m[2][0]) +
			m[0][2]*(m[1][0]*m[2][1]-m[1][1]*m[2][0])
	}
	d := determinant(a)
	if math.Abs(d) < 1e-18 {
		return 0, 0, 0, false
	}
	var result [3]float64
	for column := range 3 {
		m := a
		for row := range 3 {
			m[row][column] = b[row]
		}
		result[column] = determinant(m) / d
	}
	return result[0], result[1], result[2], true
}
