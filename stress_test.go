package tsumiki

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"testing"
)

// randSolid builds a random body from boxes and cylinders placed on a coarse grid so that
// faces are frequently coplanar or touching (the cases that break epsilon-based kernels).
func randSolid(r *rand.Rand, parts int) *Part {
	gridValue := func(lo, hi int) float64 { return float64(lo+r.Intn(hi-lo+1)) * 0.5 }
	p := NewPart("rand")
	for i := range parts {
		z0 := gridValue(0, 6)
		height := gridValue(1, 6)
		var s *Sketch
		switch r.Intn(3) {
		case 0:
			x0, y0 := gridValue(-8, 4), gridValue(-8, 4)
			centerX, centerY, width, height := x0+gridValue(1, 6)/2, y0+gridValue(1, 6)/2, gridValue(1, 12), gridValue(1, 12)
			s = NewSketch(XY(z0)).Rect(centerX, centerY, width, height)
			p.Name += fmt.Sprintf(" rect(%g,%g,%g,%g)@z%g", centerX, centerY, width, height, z0)
		case 1:
			centerX, centerY, cornerRadius := gridValue(-4, 4), gridValue(-4, 4), gridValue(1, 5)
			s = NewSketch(XY(z0)).Circle(centerX, centerY, cornerRadius)
			p.Name += fmt.Sprintf(" circle(%g,%g,%g)@z%g", centerX, centerY, cornerRadius, z0)
		default:
			centerX, centerY := gridValue(-4, 4), gridValue(-4, 4)
			width, height := gridValue(4, 12), gridValue(4, 12)
			cornerRadius, rotationDegrees := gridValue(1, 3), float64(r.Intn(4))*15
			s = NewSketch(XY(z0)).RoundedRect(centerX, centerY, width, height, cornerRadius, rotationDegrees)
			p.Name += fmt.Sprintf(" rrect(%g,%g,%g,%g,r%g,rot%g)@z%g",
				centerX, centerY, width, height, cornerRadius, rotationDegrees, z0)
		}
		var opts []FeatureOption
		if r.Intn(3) == 0 {
			opts = append(opts, Taper(float64(r.Intn(41)-10)))
		}
		if i > 0 && r.Intn(3) == 0 {
			opts = append(opts, Cut())
		}
		p.Extrude(s, height, opts...)
		if p.Err() != nil {
			return nil
		}
	}
	return p
}

// A result may legitimately have non-manifold edges when two bodies touch along an edge only
// (the grid placement makes that common), so the pass criteria are: no cracks (open edges) and
// the volume identities of set algebra, for every pair of valid inputs.
func TestBooleanRandomIdentities(t *testing.T) {
	n := 80
	if testing.Short() {
		n = 12
	}
	if v, err := strconv.Atoi(os.Getenv("STRESS_N")); err == nil {
		n = v
	}
	checked, skipped, edgeContact := 0, 0, 0
	for seed := 0; seed < n; seed++ {
		r := rand.New(rand.NewSource(int64(seed)))
		a := randSolid(r, 1+r.Intn(3))
		b := randSolid(r, 1+r.Intn(3))
		if a == nil || b == nil || a.Mesh() == nil || b.Mesh() == nil ||
			a.Report().OpenEdges > 0 || b.Report().OpenEdges > 0 {
			skipped++ // e.g. a negative taper that collapses the profile: invalid input, not a boolean bug
			continue
		}
		checked++
		volumeA, volumeB := a.Report().Volume, b.Report().Volume
		u := a.Clone("u").Combine(OperationJoin, b)
		i := a.Clone("i").Combine(OperationIntersect, b)
		c := a.Clone("c").Combine(OperationCut, b)
		tag := fmt.Sprintf("seed %d (%s | %s)", seed, a.Name, b.Name)
		for name, p := range map[string]*Part{"union": u, "intersect": i, "cut": c} {
			if p.Err() != nil || p.Mesh() == nil {
				continue // empty result (disjoint intersection, or B swallowing A) is a legal outcome
			}
			report := p.Report()
			if report.OpenEdges > 0 {
				t.Errorf("%s %s: cracks %+v", tag, name, report)
			}
			if report.NonManifold > 0 {
				edgeContact++
			}
		}
		volumeIntersection := 0.0
		if i.Err() == nil && i.Mesh() != nil {
			volumeIntersection = i.Report().Volume
		}
		volumeCut := 0.0
		if c.Err() == nil && c.Mesh() != nil {
			volumeCut = c.Report().Volume
		}
		if math.Abs(u.Report().Volume+volumeIntersection-volumeA-volumeB) > 1e-3 {
			t.Errorf("%s: |A∪B|+|A∩B| = %.6f, |A|+|B| = %.6f", tag, u.Report().Volume+volumeIntersection, volumeA+volumeB)
		}
		if math.Abs(volumeCut-(volumeA-volumeIntersection)) > 1e-3 {
			t.Errorf("%s: |A−B| = %.6f, |A|−|A∩B| = %.6f", tag, volumeCut, volumeA-volumeIntersection)
		}
	}
	t.Logf("checked %d pairs, skipped %d invalid inputs, %d results with edge-contact non-manifold edges",
		checked, skipped, edgeContact)
	if checked < n/2 {
		t.Errorf("too few valid pairs (%d of %d): generator is not exercising the kernel", checked, n)
	}
}

func TestBooleanManyHoles(t *testing.T) {
	// 4x4 grid of cylinder cuts through a plate, each coplanar with both plate faces
	plate := NewPart("plate").Extrude(NewSketch(XY(0)).Rect(0, 0, 40, 40), 3)
	want := plate.Report().Volume
	hole := NewPart("hole").Extrude(NewSketch(XY(0)).Circle(0, 0, 1.2), 3).Report().Volume
	for column := range 4 {
		for row := range 4 {
			plate.Extrude(NewSketch(XY(0)).Circle(float64(column)*5-7.5, float64(row)*5-7.5, 1.2), 3, Cut())
		}
	}
	r := mustWatertight(t, "grid of holes", plate)
	near(t, "volume", r.Volume, want-16*hole, 0.1)
}
