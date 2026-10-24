package tsumiki_test

import (
	"testing"

	"github.com/nasa9084/tsumiki"
)

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

func TestCombineDisjointParts(t *testing.T) {
	far := func() *tsumiki.Part { return box(5, 5, 5, 6, 6, 6) }
	join := box(0, 0, 0, 1, 1, 1).Combine(tsumiki.OperationJoin, far())
	near(t, "join of disjoint boxes", join.Report().Volume, 2, 1e-9)
	cut := box(0, 0, 0, 1, 1, 1).Combine(tsumiki.OperationCut, far())
	near(t, "cutting a distant box changes nothing", mustWatertight(t, "cut", cut).Volume, 1, 1e-9)
	intersect := box(0, 0, 0, 1, 1, 1).Combine(tsumiki.OperationIntersect, far())
	if intersect.Mesh() != nil || intersect.Report().Triangles != 0 {
		t.Errorf("disjoint boxes have an empty intersection, got %+v", intersect.Report())
	}
}

func TestCombineDoesNotModifyTheOtherPart(t *testing.T) {
	other := box(1, 1, 1, 3, 3, 3)
	box(0, 0, 0, 2, 2, 2).Combine(tsumiki.OperationCut, other)
	near(t, "the other part keeps its volume", mustWatertight(t, "other", other).Volume, 8, 1e-9)
	lo, hi := other.Mesh().Bounds()
	if lo != v3(1, 1, 1) || hi != v3(3, 3, 3) {
		t.Errorf("the other part moved: %v %v", lo, hi)
	}
}

func TestJoinCutIntersectVolumesAddUp(t *testing.T) {
	a, b := func() *tsumiki.Part { return box(0, 0, 0, 2, 2, 2) }, func() *tsumiki.Part { return box(1, 1, 1, 3, 3, 3) }
	join := mustWatertight(t, "join", a().Combine(tsumiki.OperationJoin, b())).Volume
	cut := mustWatertight(t, "cut", a().Combine(tsumiki.OperationCut, b())).Volume
	both := mustWatertight(t, "intersect", a().Combine(tsumiki.OperationIntersect, b())).Volume
	near(t, "intersection of the overlapping cube", both, 1, 1e-9)
	near(t, "a minus b", cut, 7, 1e-9)
	near(t, "union", join, 15, 1e-9)
}

func TestTranslatedPartKeepsItsShapeExactly(t *testing.T) {
	moved := box(0, 0, 0, 1, 2, 3).Translate(5, -2, 0.25)
	lo, hi := moved.Mesh().Bounds()
	if lo != v3(5, -2, 0.25) || hi != v3(6, 0, 3.25) {
		t.Errorf("bounds %v %v", lo, hi)
	}
	near(t, "volume is unchanged", mustWatertight(t, "moved", moved).Volume, 6, 1e-12)
}
