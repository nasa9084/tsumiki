package tsumiki_test

import (
	"testing"

	"github.com/nasa9084/tsumiki"
)

func unitBoxMesh(t *testing.T) *tsumiki.Mesh {
	t.Helper()
	return box(0, 0, 0, 1, 1, 1).Mesh().Clone()
}

func TestValidateClosedBox(t *testing.T) {
	report := unitBoxMesh(t).Validate()
	if !report.Watertight() {
		t.Errorf("box should be watertight: %+v", report)
	}
	if report.Triangles != 12 || report.Vertices != 8 {
		t.Errorf("box should have 12 triangles and 8 vertices: %+v", report)
	}
	near(t, "volume", report.Volume, 1, 1e-12)
}

func TestValidateDetectsOpenEdges(t *testing.T) {
	mesh := unitBoxMesh(t)
	mesh.Triangles = mesh.Triangles[1:] // remove one triangle: three edges lose their partner
	report := mesh.Validate()
	if report.OpenEdges == 0 || report.Watertight() {
		t.Errorf("a missing triangle must open the surface: %+v", report)
	}
}

func TestValidateDetectsNonManifoldEdges(t *testing.T) {
	mesh := unitBoxMesh(t)
	mesh.Triangles = append(mesh.Triangles, mesh.Triangles[0]) // the same directed edges used twice
	report := mesh.Validate()
	if report.NonManifold == 0 || report.Watertight() {
		t.Errorf("a duplicated triangle must be reported: %+v", report)
	}
}

func TestValidateDetectsDegenerateTriangles(t *testing.T) {
	mesh := unitBoxMesh(t)
	mesh.Triangles = append(mesh.Triangles, tsumiki.Triangle{0, 0, 1})
	if report := mesh.Validate(); report.Degenerate != 1 || report.Watertight() {
		t.Errorf("a triangle with a repeated vertex must be reported: %+v", report)
	}
}

func TestFlipNegatesVolume(t *testing.T) {
	mesh := unitBoxMesh(t)
	mesh.Flip()
	near(t, "volume", mesh.Volume(), -1, 1e-12)
}

func TestMeshBounds(t *testing.T) {
	low, high := box(-1, 2, 3, 4, 5, 9).Mesh().Bounds()
	if low != (v3(-1, 2, 3)) || high != (v3(4, 5, 9)) {
		t.Errorf("bounds %v %v", low, high)
	}
	low, high = (&tsumiki.Mesh{}).Bounds()
	if low != (tsumiki.Vector3{}) || high != (tsumiki.Vector3{}) {
		t.Errorf("an empty mesh has zero bounds, got %v %v", low, high)
	}
}

func TestMeshCloneIsDeep(t *testing.T) {
	original := unitBoxMesh(t)
	clone := original.Clone()
	clone.Vertices[0] = v3(100, 100, 100)
	clone.Triangles[0] = tsumiki.Triangle{7, 7, 7}
	if original.Vertices[0] == (v3(100, 100, 100)) || original.Triangles[0] == (tsumiki.Triangle{7, 7, 7}) {
		t.Error("modifying the clone changed the original")
	}
}

func TestMergeShiftsIndices(t *testing.T) {
	merged := unitBoxMesh(t)
	other := unitBoxMesh(t)
	other.Transform(func(v tsumiki.Vector3) tsumiki.Vector3 { return v.Add(v3(5, 0, 0)) }, false)
	merged.Merge(other)
	report := merged.Validate()
	if !report.Watertight() || report.Triangles != 24 {
		t.Errorf("merged boxes: %+v", report)
	}
	near(t, "volume", report.Volume, 2, 1e-12)
}

func TestTransformMirrorKeepsOutwardWinding(t *testing.T) {
	mesh := unitBoxMesh(t)
	mesh.Transform(func(v tsumiki.Vector3) tsumiki.Vector3 { return v3(-v.X, v.Y, v.Z) }, true)
	near(t, "volume", mesh.Volume(), 1, 1e-12)
}

// unweldedBox gives every triangle its own three vertices, the way a naive exporter would.
func unweldedBox(t *testing.T) *tsumiki.Mesh {
	t.Helper()
	source := unitBoxMesh(t)
	out := &tsumiki.Mesh{}
	for _, triangle := range source.Triangles {
		base := len(out.Vertices)
		for _, index := range triangle {
			out.Vertices = append(out.Vertices, source.Vertices[index])
		}
		out.Triangles = append(out.Triangles, tsumiki.Triangle{base, base + 1, base + 2})
	}
	return out
}

func TestWeldMergesCoincidentVertices(t *testing.T) {
	mesh := unweldedBox(t)
	mesh.Weld(1e-6)
	if len(mesh.Vertices) != 8 {
		t.Errorf("got %d vertices after welding, want 8", len(mesh.Vertices))
	}
	if report := mesh.Validate(); !report.Watertight() {
		t.Errorf("welded box should be watertight: %+v", report)
	}
}

func TestWeldKeepsDistantVertices(t *testing.T) {
	mesh := &tsumiki.Mesh{
		Vertices:  []tsumiki.Vector3{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {0.0000001, 0, 0}},
		Triangles: []tsumiki.Triangle{{0, 1, 2}},
	}
	mesh.Weld(1e-6)
	if len(mesh.Vertices) != 3 {
		t.Errorf("only the unused near-duplicate should go: got %d vertices", len(mesh.Vertices))
	}
}

func TestDropRepeated(t *testing.T) {
	mesh := unitBoxMesh(t)
	mesh.Triangles = append(mesh.Triangles, tsumiki.Triangle{0, 0, 1}, tsumiki.Triangle{2, 3, 3})
	mesh.DropRepeated()
	if len(mesh.Triangles) != 12 {
		t.Errorf("got %d triangles, want the original 12", len(mesh.Triangles))
	}
}

func TestDropDegenerateRemovesZeroAreaTriangles(t *testing.T) {
	mesh := &tsumiki.Mesh{
		Vertices:  []tsumiki.Vector3{{0, 0, 0}, {1, 0, 0}, {2, 0, 0}, {0, 1, 0}},
		Triangles: []tsumiki.Triangle{{0, 1, 2}, {0, 1, 3}},
	}
	mesh.DropDegenerate()
	if len(mesh.Triangles) != 1 || mesh.Triangles[0] != (tsumiki.Triangle{0, 1, 3}) {
		t.Errorf("got %v", mesh.Triangles)
	}
}

func TestDropInternalRemovesFacingPairs(t *testing.T) {
	mesh := &tsumiki.Mesh{
		Vertices:  []tsumiki.Vector3{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {0, 0, 1}},
		Triangles: []tsumiki.Triangle{{0, 1, 2}, {0, 2, 1}, {0, 1, 3}},
	}
	mesh.DropInternal()
	if len(mesh.Triangles) != 1 || mesh.Triangles[0] != (tsumiki.Triangle{0, 1, 3}) {
		t.Errorf("an internal pair of coincident, opposite triangles must vanish: %v", mesh.Triangles)
	}
}

func TestDropFlatRemovesSlivers(t *testing.T) {
	mesh := &tsumiki.Mesh{
		Vertices:  []tsumiki.Vector3{{0, 0, 0}, {10, 0, 0}, {5, 1e-9, 0}, {5, 5, 0}},
		Triangles: []tsumiki.Triangle{{0, 1, 2}, {0, 1, 3}},
	}
	mesh.DropFlat()
	if len(mesh.Triangles) != 1 || mesh.Triangles[0] != (tsumiki.Triangle{0, 1, 3}) {
		t.Errorf("got %v", mesh.Triangles)
	}
}

func TestFixTJunctionsSplitsEdgeAtVertexOnIt(t *testing.T) {
	// the apex vertex of the second triangle lies in the middle of the first triangle's edge 0-1
	mesh := &tsumiki.Mesh{
		Vertices:  []tsumiki.Vector3{{0, 0, 0}, {2, 0, 0}, {0, 2, 0}, {1, 0, 0}, {1, -1, 0}, {2, -1, 0}},
		Triangles: []tsumiki.Triangle{{0, 1, 2}, {3, 5, 4}},
	}
	before := tsumiki.TriangleArea(mesh.Vertices[0], mesh.Vertices[1], mesh.Vertices[2])
	mesh.FixTJunctions(1e-9)
	if len(mesh.Triangles) != 3 {
		t.Fatalf("the long edge must be split into two triangles, got %v", mesh.Triangles)
	}
	after := 0.0
	for _, triangle := range mesh.Triangles[:2] {
		after += tsumiki.TriangleArea(mesh.Vertices[triangle[0]], mesh.Vertices[triangle[1]], mesh.Vertices[triangle[2]])
	}
	near(t, "area preserved", after, before, 1e-12)
}

func TestCleanupRepairsUnweldedBox(t *testing.T) {
	mesh := unweldedBox(t)
	mesh.Cleanup()
	report := mesh.Validate()
	if !report.Watertight() {
		t.Errorf("cleanup should leave a watertight box: %+v", report)
	}
	near(t, "volume", report.Volume, 1, 1e-9)
	if report.Vertices != 8 {
		t.Errorf("got %d vertices, want 8", report.Vertices)
	}
}

func TestCellKeyOf(t *testing.T) {
	if a, b := tsumiki.CellKeyOf(v3(0.5, 0.5, 0.5), 1), tsumiki.CellKeyOf(v3(0.9, 0.1, 0.2), 1); a != b {
		t.Errorf("points in the same cell got different keys %v %v", a, b)
	}
	if a, b := tsumiki.CellKeyOf(v3(-0.1, 0, 0), 1), tsumiki.CellKeyOf(v3(0.1, 0, 0), 1); a == b {
		t.Error("points on either side of the origin must not share a cell")
	}
}
