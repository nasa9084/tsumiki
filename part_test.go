package tsumiki

import (
	"strings"
	"testing"
)

func TestNewPartIsEmpty(t *testing.T) {
	part := NewPart("empty")
	if part.Mesh() != nil || part.Err() != nil {
		t.Errorf("a new part has no body and no error: %v %v", part.Mesh(), part.Err())
	}
	if report := part.Report(); report != (Report{}) {
		t.Errorf("report of an empty part: %+v", report)
	}
	if part.Timeline() != "" {
		t.Errorf("timeline of an empty part: %q", part.Timeline())
	}
}

func TestTimelineRecordsEveryFeature(t *testing.T) {
	part := NewPart("timeline")
	part.Extrude(NewSketch(XY(0)).Rect(0, 0, 10, 10), 5)
	part.Extrude(NewSketch(XY(0)).Circle(0, 0, 2), 5, Cut())
	part.Extrude(NewSketch(XY(5)).Rect(0, 0, 4, 4), 2, Taper(10))
	if len(part.Log) != 3 {
		t.Fatalf("got %d features, want 3", len(part.Log))
	}
	if part.Log[0].Operation != OperationJoin || part.Log[1].Operation != OperationCut {
		t.Errorf("operations %v %v", part.Log[0].Operation, part.Log[1].Operation)
	}
	timeline := part.Timeline()
	for _, want := range []string{"extrude 5", "cut", "join", "taper 10"} {
		if !strings.Contains(timeline, want) {
			t.Errorf("timeline lacks %q:\n%s", want, timeline)
		}
	}
	if lines := strings.Count(timeline, "\n"); lines != 3 {
		t.Errorf("timeline has %d lines, want 3", lines)
	}
	if part.Log[2].Report.Volume <= part.Log[1].Report.Volume {
		t.Error("joining a tapered block must add volume")
	}
}

func TestExtrudeWithoutProfileIsAnError(t *testing.T) {
	part := NewPart("bad").Extrude(NewSketch(XY(0)), 5)
	if part.Err() == nil {
		t.Error("an empty sketch cannot be extruded")
	}
	// the first error sticks and later features are skipped
	part.Extrude(NewSketch(XY(0)).Rect(0, 0, 1, 1), 1)
	if part.Mesh() != nil || len(part.Log) != 0 {
		t.Error("features after an error must be ignored")
	}
}

func TestCutOnEmptyBodyIsAnError(t *testing.T) {
	part := NewPart("empty").Extrude(NewSketch(XY(0)).Rect(0, 0, 1, 1), 1, Cut())
	if part.Err() == nil || !strings.Contains(part.Err().Error(), "empty body") {
		t.Errorf("got %v", part.Err())
	}
}

func TestIntersectOption(t *testing.T) {
	part := box(0, 0, 0, 4, 4, 4).Clone("intersect")
	part.Extrude(NewSketch(XY(2)).Rect(4, 4, 4, 4), 4, Intersect()) // overlaps one 2x2x2 corner
	report := mustWatertight(t, "intersect", part)
	near(t, "volume", report.Volume, 8, 1e-9)
}

func TestStartOffsetShiftsTheFeature(t *testing.T) {
	part := NewPart("offset").Extrude(NewSketch(XY(0)).Rect(0, 0, 2, 2), 3, StartOffset(10))
	low, high := part.Mesh().Bounds()
	near(t, "bottom", low.Z, 10, 1e-9)
	near(t, "top", high.Z, 13, 1e-9)
}

func TestExtrudeOnVerticalPlanes(t *testing.T) {
	// XZ has its normal along -Y and YZ along +X
	front := NewPart("xz").Extrude(NewSketch(XZ(0)).Rect(0, 0, 2, 2), 5)
	low, high := front.Mesh().Bounds()
	near(t, "XZ ymin", low.Y, -5, 1e-9)
	near(t, "XZ ymax", high.Y, 0, 1e-9)
	side := NewPart("yz").Extrude(NewSketch(YZ(1)).Rect(0, 0, 2, 2), 5)
	low, high = side.Mesh().Bounds()
	near(t, "YZ xmin", low.X, 1, 1e-9)
	near(t, "YZ xmax", high.X, 6, 1e-9)
}

func TestExtrudeRejectsCollapsingTaper(t *testing.T) {
	part := NewPart("collapse").Extrude(NewSketch(XY(0)).Rect(0, 0, 2, 2), 10, Taper(-30))
	if part.Err() == nil {
		t.Error("a shrinking taper that passes through zero size must be rejected")
	}
}

func TestRevolveRejectsProfileAcrossTheAxis(t *testing.T) {
	part := NewPart("axis").Revolve(
		NewSketch(XY(0)).Polygon(Vector2{-1, 0}, Vector2{1, 0}, Vector2{1, 2}, Vector2{-1, 2}), 360)
	if part.Err() == nil {
		t.Error("a profile on both sides of the axis cannot be revolved")
	}
}

func TestRevolveWithoutProfileIsAnError(t *testing.T) {
	if NewPart("none").Revolve(NewSketch(XY(0)), 360).Err() == nil {
		t.Error("an empty sketch cannot be revolved")
	}
}

func TestRevolveCutMakesAGroove(t *testing.T) {
	part := NewPart("groove").Extrude(NewSketch(XY(0)).Circle(0, 0, 10), 10)
	before := part.Report().Volume
	// a rectangular notch taken out of the rim, revolved around the Z axis
	part.Revolve(NewSketch(XZ(0)).Polygon(Vector2{9, 4}, Vector2{11, 4}, Vector2{11, 6}, Vector2{9, 6}), 360, Cut())
	report := mustWatertight(t, "groove", part)
	near(t, "removed ring", before-report.Volume, 3.14159265*(100-81)*2, 1.5)
}

func TestCombineRejectsEmptyOther(t *testing.T) {
	part := box(0, 0, 0, 1, 1, 1).Clone("body").Combine(OperationJoin, NewPart("empty"))
	if part.Err() == nil {
		t.Error("combining with an empty part must fail")
	}
}

func TestCombineUsesTheOtherPartAsIs(t *testing.T) {
	left, right := box(0, 0, 0, 2, 2, 2), box(1, 0, 0, 3, 2, 2)
	joined := left.Clone("joined").Combine(OperationJoin, right)
	near(t, "union", mustWatertight(t, "union", joined).Volume, 12, 1e-9)
	near(t, "the other part is untouched", right.Report().Volume, 8, 1e-9)
}

func TestCloneIsIndependent(t *testing.T) {
	original := box(0, 0, 0, 2, 2, 2)
	clone := original.Clone("copy")
	clone.Extrude(NewSketch(XY(0)).Circle(1, 1, 0.5), 2, Cut())
	near(t, "original", original.Report().Volume, 8, 1e-9)
	if clone.Report().Volume >= 8 || clone.Name != "copy" {
		t.Errorf("clone %q has volume %g", clone.Name, clone.Report().Volume)
	}
	if len(original.Log) != 1 || len(clone.Log) != 2 {
		t.Errorf("logs: original %d, clone %d", len(original.Log), len(clone.Log))
	}
}

func TestTranslateIsExactOnTheLattice(t *testing.T) {
	part := box(0, 0, 0, 1, 1, 1).Clone("moved").Translate(10.5, -3.25, 0.001)
	low, high := part.Mesh().Bounds()
	if low != (Vector3{10.5, -3.25, 0.001}) || high != (Vector3{11.5, -2.25, 1.001}) {
		t.Errorf("bounds %v %v", low, high)
	}
	near(t, "volume", part.Report().Volume, 1, 1e-12)
}

func TestTranslateOfAnEmptyPartDoesNothing(t *testing.T) {
	if part := NewPart("empty").Translate(1, 2, 3); part.Mesh() != nil || part.Err() != nil {
		t.Error("translating an empty part must be harmless")
	}
}

func TestRotateZQuarterTurn(t *testing.T) {
	part := box(1, 0, 0, 3, 1, 1).Clone("rotated").RotateZ(90)
	low, high := part.Mesh().Bounds()
	near(t, "xmin", low.X, -1, 1e-9)
	near(t, "xmax", high.X, 0, 1e-9)
	near(t, "ymin", low.Y, 1, 1e-9)
	near(t, "ymax", high.Y, 3, 1e-9)
	mustWatertight(t, "rotated", part)
}

func TestRotateZOfAnEmptyPartDoesNothing(t *testing.T) {
	if part := NewPart("empty").RotateZ(45); part.Mesh() != nil || part.Err() != nil {
		t.Error("rotating an empty part must be harmless")
	}
}

func TestBuildingBeyondTheBuildVolumeFails(t *testing.T) {
	part := NewPart("huge").Extrude(NewSketch(XY(0)).Rect(0, 0, 2000, 2000), 1)
	if part.Err() == nil {
		t.Error("a part larger than the ±500 mm lattice limit must be rejected, not silently clipped")
	}
}

func TestMeshIsCachedUntilTheBodyChanges(t *testing.T) {
	part := box(0, 0, 0, 1, 1, 1).Clone("cache")
	first := part.Mesh()
	if part.Mesh() != first {
		t.Error("Mesh should return the cached mesh while nothing changed")
	}
	part.Extrude(NewSketch(XY(0)).Circle(0.5, 0.5, 0.2), 1, Cut())
	if part.Mesh() == first {
		t.Error("changing the body must invalidate the cache")
	}
}
