package tsumiki_test

import (
	"math"
	"math/big"
	"testing"

	"github.com/nasa9084/tsumiki"
)

func TestNormalizePlaneDividesCommonFactor(t *testing.T) {
	if got := tsumiki.NormalizePlane(2, 4, 6, 8); got != (tsumiki.NewPlane(1, 2, 3, 4)) {
		t.Errorf("got %v", got)
	}
	if got := tsumiki.NormalizePlane(0, 0, 5, -10); got != (tsumiki.NewPlane(0, 0, 1, -2)) {
		t.Errorf("got %v", got)
	}
}

func TestGCD(t *testing.T) {
	for _, test := range []struct{ a, b, want int64 }{{12, 18, 6}, {0, 7, 7}, {7, 0, 7}, {-4, 6, 2}, {17, 5, 1}} {
		if got := tsumiki.GCD64(test.a, test.b); got != test.want {
			t.Errorf("gcd(%d,%d) = %d, want %d", test.a, test.b, got, test.want)
		}
	}
}

func TestIntegerPlaneNegate(t *testing.T) {
	if got := (tsumiki.NewPlane(1, -2, 3, -4)).Neg(); got != (tsumiki.NewPlane(-1, 2, -3, 4)) {
		t.Errorf("got %v", got)
	}
}

func TestSnapRoundsToLattice(t *testing.T) {
	got, err := tsumiki.Snap(v3(1.0004, -2.0006, 0))
	if err != nil {
		t.Fatal(err)
	}
	if got != [3]int64{1000, -2001, 0} {
		t.Errorf("got %v", got)
	}
}

func TestSnapRejectsOutOfRangeAndNaN(t *testing.T) {
	if _, err := tsumiki.Snap(v3(501, 0, 0)); err == nil {
		t.Error("501 mm is beyond the supported build volume")
	}
	if _, err := tsumiki.Snap(v3(0, math.NaN(), 0)); err == nil {
		t.Error("NaN must be rejected")
	}
	if _, err := tsumiki.Snap(v3(500, -500, 500)); err != nil {
		t.Errorf("the ±500 mm limit itself is allowed: %v", err)
	}
}

func TestVertexOfThreePlanes(t *testing.T) {
	x, y, z := tsumiki.NewPlane(1, 0, 0, 2000), tsumiki.NewPlane(0, 1, 0, 3000), tsumiki.NewPlane(0, 0, 1, 4000)
	vertex, ok := tsumiki.VertexOf(&x, &y, &z)
	if !ok {
		t.Fatal("three orthogonal planes meet in a point")
	}
	if vertex.FloatX() != 2000 || vertex.FloatY() != 3000 || vertex.FloatZ() != 4000 {
		t.Errorf("vertex at (%g, %g, %g)", vertex.FloatX(), vertex.FloatY(), vertex.FloatZ())
	}
}

func TestVertexOfRationalPoint(t *testing.T) {
	// 3x = 1, 3y = 2, 3z = 4 -> (1/3, 2/3, 4/3) is not representable on the lattice
	x, y, z := tsumiki.NewPlane(3, 0, 0, 1), tsumiki.NewPlane(0, 3, 0, 2), tsumiki.NewPlane(0, 0, 3, 4)
	vertex, ok := tsumiki.VertexOf(&x, &y, &z)
	if !ok {
		t.Fatal("expected a vertex")
	}
	near(t, "x", vertex.FloatX(), 1.0/3, 1e-15)
	near(t, "y", vertex.FloatY(), 2.0/3, 1e-15)
	near(t, "z", vertex.FloatZ(), 4.0/3, 1e-15)
	if vertex.Weight().Cmp(big.NewInt(0)) <= 0 {
		t.Error("the homogeneous weight must be positive")
	}
}

func TestVertexOfParallelPlanes(t *testing.T) {
	a, b, c := tsumiki.NewPlane(1, 0, 0, 0), tsumiki.NewPlane(1, 0, 0, 5), tsumiki.NewPlane(0, 1, 0, 0)
	if _, ok := tsumiki.VertexOf(&a, &b, &c); ok {
		t.Error("two parallel planes never meet in a point")
	}
}

func TestSideIsExact(t *testing.T) {
	vertex := tsumiki.LatticeVertex(10, 20, 30)
	tests := []struct {
		plane tsumiki.IntegerPlane
		want  int
	}{
		{tsumiki.NewPlane(1, 0, 0, 5), 1},
		{tsumiki.NewPlane(1, 0, 0, 10), 0},
		{tsumiki.NewPlane(1, 0, 0, 15), -1},
		{tsumiki.NewPlane(1, 1, 1, 60), 0},
		{tsumiki.NewPlane(1, 1, 1, 61), -1},
		{tsumiki.NewPlane(1, 1, 1, 59), 1},
	}
	for _, test := range tests {
		if got := vertex.Side(&test.plane); got != test.want {
			t.Errorf("side of %v = %d, want %d", test.plane, got, test.want)
		}
	}
}

func TestSideDecidesNearlyCoplanarCasesExactly(t *testing.T) {
	// the vertex is 1/3 of a lattice unit off a plane: far below float noise at this magnitude
	x, y, z := tsumiki.NewPlane(3, 0, 0, 1), tsumiki.NewPlane(0, 3, 0, 2), tsumiki.NewPlane(0, 0, 3, 4)
	vertex, _ := tsumiki.VertexOf(&x, &y, &z)
	on := tsumiki.NewPlane(3, 0, 0, 1)
	if vertex.Side(&on) != 0 {
		t.Error("the vertex lies exactly on its defining plane")
	}
	above := tsumiki.NewPlane(3, 0, 0, 0)
	if vertex.Side(&above) != 1 {
		t.Error("the vertex lies strictly in front of 3x = 0")
	}
}

func TestNewTrianglePolygonRejectsCollinearPoints(t *testing.T) {
	if polygon := tsumiki.NewTrianglePolygon([3][3]int64{{0, 0, 0}, {1, 0, 0}, {2, 0, 0}}); polygon != nil {
		t.Error("collinear corners do not span a plane")
	}
}

func TestNewTrianglePolygonPlaneAndEdges(t *testing.T) {
	polygon := tsumiki.NewTrianglePolygon([3][3]int64{{0, 0, 0}, {1000, 0, 0}, {0, 1000, 0}})
	if polygon == nil {
		t.Fatal("expected a polygon")
	}
	if polygon.Plane() != (tsumiki.NewPlane(0, 0, 1, 0)) {
		t.Errorf("plane %v, want +Z through the origin", polygon.Plane())
	}
	if len(polygon.Edges()) != 3 || len(polygon.Corners()) != 3 {
		t.Fatalf("%d edges, %d vertices", len(polygon.Edges()), len(polygon.Corners()))
	}
	// every polygon vertex lies on its own plane and on the planes of both adjacent edges
	for i, vertex := range polygon.Corners() {
		previous := polygon.Edges()[(i+len(polygon.Edges())-1)%len(polygon.Edges())]
		for _, plane := range []tsumiki.IntegerPlane{polygon.Plane(), previous, polygon.Edges()[i]} {
			if vertex.Side(&plane) != 0 {
				t.Errorf("vertex %d is off plane %v", i, plane)
			}
		}
	}
}

func TestFlippedPolygonReversesOrientation(t *testing.T) {
	polygon := tsumiki.NewTrianglePolygon([3][3]int64{{0, 0, 0}, {1000, 0, 0}, {0, 1000, 0}})
	flipped := polygon.Flipped()
	if flipped.Plane() != polygon.Plane().Neg() {
		t.Errorf("plane %v", flipped.Plane())
	}
	for i, vertex := range flipped.Corners() {
		previous := flipped.Edges()[(i+len(flipped.Edges())-1)%len(flipped.Edges())]
		for _, plane := range []tsumiki.IntegerPlane{flipped.Plane(), previous, flipped.Edges()[i]} {
			if vertex.Side(&plane) != 0 {
				t.Errorf("flipped vertex %d is off plane %v", i, plane)
			}
		}
	}
}

func TestSplitPolygonClassification(t *testing.T) {
	polygon := tsumiki.NewTrianglePolygon([3][3]int64{{0, 0, 0}, {2000, 0, 0}, {0, 2000, 0}})
	classify := func(plane tsumiki.IntegerPlane) (coplanarFront, coplanarBack, front, back int) {
		var cf, cb, f, b []*tsumiki.ExactPolygon
		tsumiki.SplitPolygon(&plane, polygon, &cf, &cb, &f, &b)
		return len(cf), len(cb), len(f), len(b)
	}
	if cf, cb, f, b := classify(tsumiki.NewPlane(0, 0, 1, 0)); [4]int{cf, cb, f, b} != [4]int{1, 0, 0, 0} {
		t.Errorf("same plane, same direction: %d %d %d %d", cf, cb, f, b)
	}
	if cf, cb, f, b := classify(tsumiki.NewPlane(0, 0, -1, 0)); [4]int{cf, cb, f, b} != [4]int{0, 1, 0, 0} {
		t.Errorf("same plane, opposite direction: %d %d %d %d", cf, cb, f, b)
	}
	if cf, cb, f, b := classify(tsumiki.NewPlane(0, 0, 1, -5)); [4]int{cf, cb, f, b} != [4]int{0, 0, 1, 0} {
		t.Errorf("entirely in front: %d %d %d %d", cf, cb, f, b)
	}
	if cf, cb, f, b := classify(tsumiki.NewPlane(0, 0, 1, 5)); [4]int{cf, cb, f, b} != [4]int{0, 0, 0, 1} {
		t.Errorf("entirely behind: %d %d %d %d", cf, cb, f, b)
	}
	if cf, cb, f, b := classify(tsumiki.NewPlane(1, 0, 0, 500)); [4]int{cf, cb, f, b} != [4]int{0, 0, 1, 1} {
		t.Errorf("a spanning plane must split into a front and a back piece: %d %d %d %d", cf, cb, f, b)
	}
}

func TestSplitPolygonPiecesAddUpToTheOriginalArea(t *testing.T) {
	polygon := tsumiki.NewTrianglePolygon([3][3]int64{{0, 0, 0}, {4000, 0, 0}, {0, 4000, 0}})
	plane := tsumiki.NewPlane(1, 1, 0, 3000)
	var coplanarFront, coplanarBack, front, back []*tsumiki.ExactPolygon
	tsumiki.SplitPolygon(&plane, polygon, &coplanarFront, &coplanarBack, &front, &back)
	if len(front) != 1 || len(back) != 1 {
		t.Fatalf("front %d, back %d", len(front), len(back))
	}
	area := func(p *tsumiki.ExactPolygon) float64 {
		corners := make([]tsumiki.Vector3, len(p.Corners()))
		for i, vertex := range p.Corners() {
			corners[i] = v3(vertex.FloatX(), vertex.FloatY(), vertex.FloatZ())
		}
		total := 0.0
		for i := 1; i+1 < len(corners); i++ {
			total += tsumiki.TriangleArea(corners[0], corners[i], corners[i+1])
		}
		return total
	}
	near(t, "sum of the pieces", area(front[0])+area(back[0]), area(polygon), 1e-6)
}

func TestMeshToSolidSkipsDegenerateTriangles(t *testing.T) {
	mesh := &tsumiki.Mesh{
		Vertices:  []tsumiki.Vector3{{0, 0, 0}, {1, 0, 0}, {2, 0, 0}, {0, 1, 0}},
		Triangles: []tsumiki.Triangle{{0, 1, 2}, {0, 1, 3}},
	}
	solid, err := tsumiki.MeshToSolid(mesh)
	if err != nil {
		t.Fatal(err)
	}
	if len(solid) != 1 {
		t.Errorf("got %d polygons, want 1", len(solid))
	}
}

func TestMeshToSolidRejectsOutOfRangeMesh(t *testing.T) {
	mesh := &tsumiki.Mesh{
		Vertices:  []tsumiki.Vector3{{0, 0, 0}, {1000, 0, 0}, {0, 1, 0}},
		Triangles: []tsumiki.Triangle{{0, 1, 2}},
	}
	if _, err := tsumiki.MeshToSolid(mesh); err == nil {
		t.Error("a vertex 1000 mm from the origin is outside the build volume")
	}
}

func TestBoolOperationString(t *testing.T) {
	names := map[tsumiki.BoolOperation]string{
		tsumiki.OperationJoin:      "join",
		tsumiki.OperationCut:       "cut",
		tsumiki.OperationIntersect: "intersect",
	}
	for operation, want := range names {
		if got := operation.String(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

func solidOf(t *testing.T, part *tsumiki.Part) tsumiki.Solid {
	t.Helper()
	solid, err := tsumiki.MeshToSolid(part.Mesh())
	if err != nil {
		t.Fatal(err)
	}
	return solid
}

func TestBoolSolidDisjointOperands(t *testing.T) {
	a, b := solidOf(t, box(0, 0, 0, 1, 1, 1)), solidOf(t, box(5, 5, 5, 6, 6, 6))
	join := tsumiki.BoolSolid(tsumiki.OperationJoin, a, b).Mesh()
	near(t, "join of disjoint boxes", join.Volume(), 2, 1e-9)
	if got := tsumiki.BoolSolid(tsumiki.OperationIntersect, a, b); len(got) != 0 {
		t.Errorf("disjoint boxes have an empty intersection, got %d polygons", len(got))
	}
	cut := tsumiki.BoolSolid(tsumiki.OperationCut, a, b).Mesh()
	near(t, "cutting a distant box changes nothing", cut.Volume(), 1, 1e-9)
}

func TestBoolSolidDoesNotModifyOperands(t *testing.T) {
	a, b := solidOf(t, box(0, 0, 0, 2, 2, 2)), solidOf(t, box(1, 1, 1, 3, 3, 3))
	before := len(a)
	tsumiki.BoolSolid(tsumiki.OperationCut, a, b)
	if len(a) != before {
		t.Error("the first operand was modified")
	}
	near(t, "a still has its volume", a.Mesh().Volume(), 8, 1e-9)
	near(t, "b still has its volume", b.Mesh().Volume(), 8, 1e-9)
}

func TestSolidTranslateIsExact(t *testing.T) {
	solid := solidOf(t, box(0, 0, 0, 1, 2, 3))
	moved := solid.Translate([3]int64{5000, -2000, 250}).Mesh()
	low, high := moved.Bounds()
	if low != (v3(5, -2, 0.25)) || high != (v3(6, 0, 3.25)) {
		t.Errorf("bounds %v %v", low, high)
	}
	near(t, "volume is unchanged", moved.Volume(), 6, 1e-12)
}

func TestSolidMeshRepairsTJunctionsExactly(t *testing.T) {
	// a small box on top of a large one makes the big top face a polygon with a hole-like
	// boundary; the shared edges only close up if vertices are inserted on the neighbors' edges
	part := box(0, 0, 0, 10, 10, 2).Clone("base")
	part.Combine(tsumiki.OperationJoin, box(3, 3, 2, 5, 5, 4))
	part.Combine(tsumiki.OperationJoin, box(5, 3, 2, 8, 5, 3))
	report := mustWatertight(t, "stacked boxes", part)
	near(t, "volume", report.Volume, 200+8+6, 1e-9)
}
