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
	g := func(lo, hi int) float64 { return float64(lo+r.Intn(hi-lo+1)) * 0.5 }
	p := NewPart("rand")
	for i := 0; i < parts; i++ {
		z0 := g(0, 6)
		h := g(1, 6)
		var s *Sketch
		switch r.Intn(3) {
		case 0:
			x0, y0 := g(-8, 4), g(-8, 4)
			cx, cy, w, hh := x0+g(1, 6)/2, y0+g(1, 6)/2, g(1, 12), g(1, 12)
			s = NewSketch(XY(z0)).Rect(cx, cy, w, hh)
			p.Name += fmt.Sprintf(" rect(%g,%g,%g,%g)@z%g", cx, cy, w, hh, z0)
		case 1:
			cx, cy, rr := g(-4, 4), g(-4, 4), g(1, 5)
			s = NewSketch(XY(z0)).Circle(cx, cy, rr)
			p.Name += fmt.Sprintf(" circle(%g,%g,%g)@z%g", cx, cy, rr, z0)
		default:
			cx, cy, w, hh, rr, rot := g(-4, 4), g(-4, 4), g(4, 12), g(4, 12), g(1, 3), float64(r.Intn(4))*15
			s = NewSketch(XY(z0)).RoundedRect(cx, cy, w, hh, rr, rot)
			p.Name += fmt.Sprintf(" rrect(%g,%g,%g,%g,r%g,rot%g)@z%g", cx, cy, w, hh, rr, rot, z0)
		}
		var opts []FeatureOpt
		if r.Intn(3) == 0 {
			opts = append(opts, Taper(float64(r.Intn(41)-10)))
		}
		if i > 0 && r.Intn(3) == 0 {
			opts = append(opts, Cut())
		}
		p.Extrude(s, h, opts...)
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
		va, vb := a.Report().Volume, b.Report().Volume
		u := a.Clone("u").Combine(OpJoin, b)
		i := a.Clone("i").Combine(OpIntersect, b)
		c := a.Clone("c").Combine(OpCut, b)
		tag := fmt.Sprintf("seed %d (%s | %s)", seed, a.Name, b.Name)
		for name, p := range map[string]*Part{"union": u, "intersect": i, "cut": c} {
			if p.Err() != nil || p.Mesh() == nil {
				continue // empty result (disjoint intersection, or B swallowing A) is a legal outcome
			}
			rep := p.Report()
			if rep.OpenEdges > 0 {
				t.Errorf("%s %s: cracks %+v", tag, name, rep)
			}
			if rep.NonManifold > 0 {
				edgeContact++
			}
		}
		vi := 0.0
		if i.Err() == nil && i.Mesh() != nil {
			vi = i.Report().Volume
		}
		vc := 0.0
		if c.Err() == nil && c.Mesh() != nil {
			vc = c.Report().Volume
		}
		if math.Abs(u.Report().Volume+vi-va-vb) > 1e-3 {
			t.Errorf("%s: |A∪B|+|A∩B| = %.6f, |A|+|B| = %.6f", tag, u.Report().Volume+vi, va+vb)
		}
		if math.Abs(vc-(va-vi)) > 1e-3 {
			t.Errorf("%s: |A−B| = %.6f, |A|−|A∩B| = %.6f", tag, vc, va-vi)
		}
	}
	t.Logf("checked %d pairs, skipped %d invalid inputs, %d results with edge-contact non-manifold edges", checked, skipped, edgeContact)
	if checked < n/2 {
		t.Errorf("too few valid pairs (%d of %d): generator is not exercising the kernel", checked, n)
	}
}

func TestBooleanManyHoles(t *testing.T) {
	// 4x4 grid of cylinder cuts through a plate, each coplanar with both plate faces
	plate := NewPart("plate").Extrude(NewSketch(XY(0)).Rect(0, 0, 40, 40), 3)
	want := plate.Report().Volume
	hole := NewPart("hole").Extrude(NewSketch(XY(0)).Circle(0, 0, 1.2), 3).Report().Volume
	for ix := 0; ix < 4; ix++ {
		for iy := 0; iy < 4; iy++ {
			plate.Extrude(NewSketch(XY(0)).Circle(float64(ix)*5-7.5, float64(iy)*5-7.5, 1.2), 3, Cut())
		}
	}
	r := mustWatertight(t, "grid of holes", plate)
	near(t, "volume", r.Volume, want-16*hole, 0.1)
}
