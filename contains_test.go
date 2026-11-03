package tsumiki_test

import (
	"testing"

	"github.com/nasa9084/tsumiki"
)

func TestContainsBox(t *testing.T) {
	mesh := box(0, 0, 0, 2, 2, 2).Mesh()
	if !mesh.Contains(v3(1, 1, 1)) {
		t.Error("the center of the box is inside")
	}
	for _, outside := range []tsumiki.Vector3{v3(3, 1, 1), v3(1, 1, 3), v3(-0.5, 1, 1)} {
		if mesh.Contains(outside) {
			t.Errorf("%v is outside the box", outside)
		}
	}
}

func TestContainsSeesHolesAndCavities(t *testing.T) {
	ring := tsumiki.NewPart("ring").Extrude(tsumiki.NewSketch(tsumiki.XY(0)).Circle(0, 0, 10).Circle(0, 0, 4), 3)
	if ring.Mesh().Contains(v3(0, 0, 1.5)) {
		t.Error("the hole of a ring is outside")
	}
	if !ring.Mesh().Contains(v3(7, 0, 1.5)) {
		t.Error("the material of a ring is inside")
	}
	hollow := box(0, 0, 0, 10, 10, 10).Combine(tsumiki.OperationCut, box(2, 2, 2, 8, 8, 8))
	if hollow.Mesh().Contains(v3(5, 5, 5)) {
		t.Error("a closed cavity is outside")
	}
	if !hollow.Mesh().Contains(v3(1, 5, 5)) {
		t.Error("the wall around a cavity is inside")
	}
}

func TestContainsOnAGridAlignedWithEdges(t *testing.T) {
	// rays from points level with edges and vertices must not miscount
	mesh := box(0, 0, 0, 4, 4, 4).Mesh()
	for _, x := range []float64{0.5, 1, 2, 3, 3.5} {
		for _, z := range []float64{0.5, 2, 3.5} {
			if !mesh.Contains(v3(x, 2, z)) {
				t.Errorf("(%g, 2, %g) is inside the box", x, z)
			}
		}
	}
}
