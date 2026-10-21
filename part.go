package tsumiki

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// Part is a solid built by a timeline of features, Fusion style:
//
//	p := tsumiki.NewPart("plate")
//	p.Extrude(sketch, 1.5)                 // join (the first feature creates the body)
//	p.Extrude(hole, 1.5, tsumiki.Cut())        // cut
//
// The Go source is the history; there is no parametric recompute. Internally the body is an
// exact polygon soup (see exact.go) so a long chain of booleans never accumulates epsilon errors.
type Part struct {
	Name string
	s    Solid
	Log  []Feature
	err  error
	mesh *Mesh // cache
}

// Feature is one timeline entry with the state of the body after it.
type Feature struct {
	Name   string
	Op     BoolOp
	Report Report
}

// NewPart creates an empty part.
func NewPart(name string) *Part { return &Part{Name: name} }

// Err returns the first error recorded by a feature.
func (p *Part) Err() error { return p.err }

// Mesh returns the current body as a cleaned triangle mesh (nil when empty). Do not modify it.
func (p *Part) Mesh() *Mesh {
	if len(p.s) == 0 {
		return nil
	}
	if p.mesh == nil {
		p.mesh = p.s.Mesh()
	}
	return p.mesh
}

// Report validates the current body.
func (p *Part) Report() Report {
	if m := p.Mesh(); m != nil {
		return m.Validate()
	}
	return Report{}
}

// Timeline renders the feature log.
func (p *Part) Timeline() string {
	var sb strings.Builder
	for i, f := range p.Log {
		fmt.Fprintf(&sb, "%2d %-30s %-9s tris=%-6d vol=%11.3f open=%d nonmanifold=%d\n",
			i+1, f.Name, f.Op, f.Report.Tris, f.Report.Volume, f.Report.OpenEdges, f.Report.NonManifold)
	}
	return sb.String()
}

type featureOpt struct {
	op    BoolOp
	taper float64
	start float64
}

// FeatureOpt tweaks Extrude / Revolve.
type FeatureOpt func(*featureOpt)

// Cut subtracts the feature from the body.
func Cut() FeatureOpt { return func(o *featureOpt) { o.op = OpCut } }

// Intersect keeps only the overlap of the feature and the body.
func Intersect() FeatureOpt { return func(o *featureOpt) { o.op = OpIntersect } }

// Taper sets the taper angle in degrees; positive grows the section with distance.
func Taper(deg float64) FeatureOpt { return func(o *featureOpt) { o.taper = deg } }

// StartOffset shifts the start of the extrusion along the plane normal.
func StartOffset(d float64) FeatureOpt { return func(o *featureOpt) { o.start = d } }

func options(opts []FeatureOpt) featureOpt {
	o := featureOpt{op: OpJoin}
	for _, f := range opts {
		f(&o)
	}
	return o
}

// Extrude extrudes all regions of s by dist along the sketch normal (negative = opposite side).
func (p *Part) Extrude(s *Sketch, dist float64, opts ...FeatureOpt) *Part {
	if p.err != nil {
		return p
	}
	o := options(opts)
	regs := s.regions()
	if len(regs) == 0 {
		p.err = errors.New("extrude: sketch has no closed profile")
		return p
	}
	name := fmt.Sprintf("extrude %.4g", dist)
	if o.taper != 0 {
		name += fmt.Sprintf(" taper %.3g°", o.taper)
	}
	tool, err := extrudeMesh(s.Plane, regs, dist, o.taper, o.start)
	if err != nil {
		p.err = fmt.Errorf("extrude: %w", err)
		return p
	}
	return p.apply(name, o.op, tool)
}

// Revolve sweeps the sketch around its plane's V axis (the line u=0) by angleDeg.
func (p *Part) Revolve(s *Sketch, angleDeg float64, opts ...FeatureOpt) *Part {
	if p.err != nil {
		return p
	}
	o := options(opts)
	regs := s.regions()
	if len(regs) == 0 {
		p.err = errors.New("revolve: sketch has no closed profile")
		return p
	}
	for _, r := range regs {
		for _, q := range r.Outer {
			if q.X < -1e-9 {
				p.err = errors.New("revolve: profile crosses the axis (u < 0)")
				return p
			}
		}
	}
	return p.apply(fmt.Sprintf("revolve %.4g°", angleDeg), o.op, revolveMesh(s.Plane, regs, angleDeg, s.Tol))
}

// Combine joins/cuts/intersects another part into this one.
func (p *Part) Combine(op BoolOp, other *Part) *Part {
	if p.err != nil {
		return p
	}
	if len(other.s) == 0 {
		p.err = errors.New("combine: other part is empty")
		return p
	}
	return p.applySolid("combine "+other.Name, op, other.s)
}

func (p *Part) apply(name string, op BoolOp, tool *Mesh) *Part {
	ts, err := meshToSolid(tool)
	if err != nil {
		p.err = fmt.Errorf("%s: %w", name, err)
		return p
	}
	return p.applySolid(name, op, ts)
}

func (p *Part) applySolid(name string, op BoolOp, tool Solid) *Part {
	switch {
	case len(p.s) == 0 && op == OpJoin:
		p.s = tool
	case len(p.s) == 0:
		p.err = fmt.Errorf("%s: %s on an empty body", name, op)
		return p
	default:
		p.s = BooleanSolid(op, p.s, tool)
	}
	p.mesh = nil
	p.Log = append(p.Log, Feature{Name: name, Op: op, Report: p.Report()})
	return p
}

// Clone copies the part (polygons are immutable, so this is cheap).
func (p *Part) Clone(name string) *Part {
	return &Part{Name: name, s: append(Solid(nil), p.s...), Log: append([]Feature(nil), p.Log...), err: p.err}
}

// Translate moves the body; the offset is rounded to the 1 µm lattice so the move is exact.
func (p *Part) Translate(dx, dy, dz float64) *Part {
	if len(p.s) > 0 {
		p.s = p.s.translate([3]int64{int64(math.Round(dx * lattice)), int64(math.Round(dy * lattice)), int64(math.Round(dz * lattice))})
		p.mesh = nil
	}
	return p
}

// RotateZ rotates the body around the Z axis through the origin (re-snaps to the lattice).
func (p *Part) RotateZ(deg float64) *Part {
	m := p.Mesh()
	if m == nil {
		return p
	}
	r := m.Clone()
	s, c := math.Sincos(Rad(deg))
	r.transform(func(v V3) V3 { return V3{v.X*c - v.Y*s, v.X*s + v.Y*c, v.Z} }, false)
	ns, err := meshToSolid(r)
	if err != nil {
		p.err = err
		return p
	}
	p.s, p.mesh = ns, nil
	return p
}
