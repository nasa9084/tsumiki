package tsumiki

import (
	"math"
	"testing"
)

func TestVector2Operations(t *testing.T) {
	a, b := Vector2{3, 4}, Vector2{-1, 2}
	if got := a.Add(b); got != (Vector2{2, 6}) {
		t.Errorf("Add = %v", got)
	}
	if got := a.Sub(b); got != (Vector2{4, 2}) {
		t.Errorf("Sub = %v", got)
	}
	if got := a.Scale(2); got != (Vector2{6, 8}) {
		t.Errorf("Scale = %v", got)
	}
	if got := a.Dot(b); got != 5 {
		t.Errorf("Dot = %v", got)
	}
	if got := a.Cross(b); got != 10 {
		t.Errorf("Cross = %v", got)
	}
	if got := a.Length(); got != 5 {
		t.Errorf("Length = %v", got)
	}
}

func TestVector2RotateCounterClockwise(t *testing.T) {
	got := Vector2{1, 0}.Rotate(math.Pi / 2)
	near(t, "x", got.X, 0, 1e-12)
	near(t, "y", got.Y, 1, 1e-12)
}

func TestVector3Operations(t *testing.T) {
	a, b := Vector3{1, 2, 3}, Vector3{4, 5, 6}
	if got := a.Add(b); got != (Vector3{5, 7, 9}) {
		t.Errorf("Add = %v", got)
	}
	if got := b.Sub(a); got != (Vector3{3, 3, 3}) {
		t.Errorf("Sub = %v", got)
	}
	if got := a.Scale(-2); got != (Vector3{-2, -4, -6}) {
		t.Errorf("Scale = %v", got)
	}
	if got := a.Negate(); got != (Vector3{-1, -2, -3}) {
		t.Errorf("Negate = %v", got)
	}
	if got := a.Dot(b); got != 32 {
		t.Errorf("Dot = %v", got)
	}
	if got := a.Cross(b); got != (Vector3{-3, 6, -3}) {
		t.Errorf("Cross = %v", got)
	}
	if got := (Vector3{2, 3, 6}).Length(); got != 7 {
		t.Errorf("Length = %v", got)
	}
}

func TestVector3CrossIsRightHanded(t *testing.T) {
	if got := (Vector3{1, 0, 0}).Cross(Vector3{0, 1, 0}); got != (Vector3{0, 0, 1}) {
		t.Errorf("X x Y = %v, want +Z", got)
	}
}

func TestVector3Normalize(t *testing.T) {
	got := Vector3{0, 3, 4}.Normalize()
	near(t, "length", got.Length(), 1, 1e-12)
	near(t, "y", got.Y, 0.6, 1e-12)
	if zero := (Vector3{}).Normalize(); zero != (Vector3{}) {
		t.Errorf("normalizing the zero vector should return it unchanged, got %v", zero)
	}
}

func TestVector3Interpolate(t *testing.T) {
	a, b := Vector3{0, 0, 0}, Vector3{10, 20, 30}
	if got := a.Interpolate(b, 0); got != a {
		t.Errorf("t=0: %v", got)
	}
	if got := a.Interpolate(b, 1); got != b {
		t.Errorf("t=1: %v", got)
	}
	if got := a.Interpolate(b, 0.5); got != (Vector3{5, 10, 15}) {
		t.Errorf("t=0.5: %v", got)
	}
}

func TestRadians(t *testing.T) {
	near(t, "180", Radians(180), math.Pi, 1e-12)
	near(t, "90", Radians(90), math.Pi/2, 1e-12)
	near(t, "0", Radians(0), 0, 0)
	near(t, "-45", Radians(-45), -math.Pi/4, 1e-12)
}
