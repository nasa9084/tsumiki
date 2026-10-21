package tsumiki

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"math"
	"testing"
)

func near(t *testing.T, name string, got, want, tolerance float64) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Errorf("%s: got %.6f want %.6f (tol %g)", name, got, want, tolerance)
	}
}

func mustWatertight(t *testing.T, name string, p *Part) Report {
	t.Helper()
	if p.Err() != nil {
		t.Fatalf("%s: %v", name, p.Err())
	}
	r := p.Report()
	if !r.Watertight() {
		t.Errorf("%s: not watertight: %+v", name, r)
	}
	return r
}

func box(x0, y0, z0, x1, y1, z1 float64) *Part {
	s := NewSketch(XY(z0)).Rect((x0+x1)/2, (y0+y1)/2, x1-x0, y1-y0)
	return NewPart("box").Extrude(s, z1-z0)
}

// ---------- M1: sketch + extrude + export ----------

func TestExtrudeBox(t *testing.T) {
	p := box(0, 0, 0, 2, 3, 4)
	r := mustWatertight(t, "box", p)
	near(t, "volume", r.Volume, 24, 1e-9)
	lo, hi := p.Mesh().Bounds()
	if lo != (Vector3{0, 0, 0}) || hi != (Vector3{2, 3, 4}) {
		t.Errorf("bounds %v %v", lo, hi)
	}
}

func TestExtrudeNegativeAndOffset(t *testing.T) {
	s := NewSketch(XY(0)).Rect(0, 0, 2, 2)
	p := NewPart("neg").Extrude(s, -3, StartOffset(1))
	r := mustWatertight(t, "neg", p)
	near(t, "volume", r.Volume, 12, 1e-9)
	lo, hi := p.Mesh().Bounds()
	near(t, "zmin", lo.Z, -2, 1e-9)
	near(t, "zmax", hi.Z, 1, 1e-9)
}

func TestExtrudeDiscWithHole(t *testing.T) {
	s := NewSketch(XY(0)).Circle(0, 0, 65).Circle(0, 0, 22.5)
	regions := s.regions()
	if len(regions) != 1 || len(regions[0].Holes) != 1 {
		t.Fatalf("regions: %+v", len(regions))
	}
	want := (regions[0].Outer.Area() + regions[0].Holes[0].Area()) * 1.5 // hole area is negative
	p := NewPart("disc").Extrude(s, 1.5)
	r := mustWatertight(t, "disc", p)
	near(t, "volume (1µm lattice snap)", r.Volume, want, 0.05)
	near(t, "analytic", r.Volume, math.Pi*(65*65-22.5*22.5)*1.5, 12) // chord tolerance 0.01
}

func TestRoundedRectRing(t *testing.T) {
	s := NewSketch(XY(0)).RoundedRect(0, 0, 82.6, 82.6, 17.3, 45).RoundedRect(0, 0, 76.6, 76.6, 14.3, 45)
	p := NewPart("ring").Extrude(s, 12, StartOffset(-12))
	r := mustWatertight(t, "ring", p)
	// area of a rounded square: w*h - (4-pi) r^2
	outerArea := 82.6*82.6 - (4-math.Pi)*17.3*17.3
	innerArea := 76.6*76.6 - (4-math.Pi)*14.3*14.3
	near(t, "volume", r.Volume, (outerArea-innerArea)*12, 6)
}

func TestTaperedExtrude(t *testing.T) {
	// frustum: radius 10 -> 10 + h*tan(a)
	h, a := 5.0, 30.0
	s := NewSketch(XY(0)).Circle(0, 0, 10)
	p := NewPart("frustum").Extrude(s, h, Taper(a))
	r := mustWatertight(t, "frustum", p)
	grownRadius := 10 + h*math.Tan(Radians(a))
	// inscribed polygon, chord tolerance 0.01
	near(t, "volume", r.Volume, math.Pi*h/3*(100+10*grownRadius+grownRadius*grownRadius), 4)
	_, hi := p.Mesh().Bounds()
	near(t, "xmax", hi.X, grownRadius, 0.02)
}

func TestTaperedRingWithHole(t *testing.T) {
	// growing ring: outer grows, hole shrinks
	s := NewSketch(XY(0)).Circle(0, 0, 20).Circle(0, 0, 10)
	p := NewPart("tring").Extrude(s, 4, Taper(45))
	r := mustWatertight(t, "tring", p)
	outerRadius, innerRadius := 24.0, 6.0
	want := math.Pi * 4 / 3 *
		((400 + 20*outerRadius + outerRadius*outerRadius) - (100 + 10*innerRadius + innerRadius*innerRadius))
	near(t, "volume", r.Volume, want, 3)
}

func TestRevolveCylinderShell(t *testing.T) {
	// profile in XY plane: u = x (radius), v = y (axis)
	s := NewSketch(XY(0)).Polygon(Vector2{5, 0}, Vector2{10, 0}, Vector2{10, 7}, Vector2{5, 7})
	p := NewPart("shell").Revolve(s, 360)
	r := mustWatertight(t, "shell", p)
	near(t, "volume", r.Volume, math.Pi*(100-25)*7, 4)
}

func TestRevolveHalf(t *testing.T) {
	s := NewSketch(XY(0)).Polygon(Vector2{0, 0}, Vector2{10, 0}, Vector2{10, 7}, Vector2{0, 7})
	p := NewPart("halfcyl").Revolve(s, 180)
	r := mustWatertight(t, "halfcyl", p)
	near(t, "volume", r.Volume, math.Pi*100*7/2, 1.5)
}

func TestPathArcs(t *testing.T) {
	// D-shape: half disc of radius 10 closed by a chord
	s := NewSketch(XY(0))
	s.Path(10, 0).ArcTo(-10, 0, 0, 0, true).Close()
	p := NewPart("D").Extrude(s, 2)
	r := mustWatertight(t, "D", p)
	near(t, "volume", r.Volume, math.Pi*100/2*2, 1)
}

func TestExport(t *testing.T) {
	p := box(0, 0, 0, 2, 3, 4)
	var buf bytes.Buffer
	if err := p.Mesh().Write3MF(&buf, "box"); err != nil {
		t.Fatal(err)
	}
	zipReader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var model []byte
	for _, f := range zipReader.File {
		if f.Name == "3D/3dmodel.model" {
			entryReader, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			model, err = io.ReadAll(entryReader)
			_ = entryReader.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	var doc struct {
		Vertices  []struct{} `xml:"resources>object>mesh>vertices>vertex"`
		Triangles []struct{} `xml:"resources>object>mesh>triangles>triangle"`
		Unit      string     `xml:"unit,attr"`
	}
	if err := xml.Unmarshal(model, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Vertices) != len(p.Mesh().Vertices) || len(doc.Triangles) != len(p.Mesh().Triangles) ||
		doc.Unit != "millimeter" {
		t.Errorf("3mf mismatch: %d/%d %q", len(doc.Vertices), len(doc.Triangles), doc.Unit)
	}
	var stl bytes.Buffer
	if err := p.Mesh().WriteSTL(&stl); err != nil {
		t.Fatal(err)
	}
	if stl.Len() != 84+50*len(p.Mesh().Triangles) {
		t.Errorf("stl size %d", stl.Len())
	}
}

// ---------- M2: boolean robustness ----------

func TestBooleanOverlap(t *testing.T) {
	a := box(0, 0, 0, 2, 2, 2)
	b := box(1, 1, 1, 3, 3, 3)
	u := a.Clone("u").Combine(OperationJoin, b)
	near(t, "union", mustWatertight(t, "union", u).Volume, 8+8-1, 1e-6)
	c := a.Clone("c").Combine(OperationCut, b)
	near(t, "cut", mustWatertight(t, "cut", c).Volume, 7, 1e-6)
	i := a.Clone("i").Combine(OperationIntersect, b)
	near(t, "intersect", mustWatertight(t, "intersect", i).Volume, 1, 1e-6)
}

func TestBooleanTouchingFaces(t *testing.T) {
	a := box(0, 0, 0, 1, 1, 1)
	b := box(1, 0, 0, 2, 1, 1) // shares the x=1 face exactly
	u := a.Clone("u").Combine(OperationJoin, b)
	near(t, "union", mustWatertight(t, "touch-union", u).Volume, 2, 1e-6)
}

func TestBooleanFlushCut(t *testing.T) {
	// the classic: cutter shares the top and bottom faces with the target
	a := box(0, 0, 0, 10, 10, 2)
	b := box(3, 3, 0, 7, 7, 2)
	c := a.Clone("c").Combine(OperationCut, b)
	near(t, "flush cut", mustWatertight(t, "flush", c).Volume, 200-32, 1e-6)
}

func TestBooleanFlushCutSideFace(t *testing.T) {
	// cutter shares one side wall as well (notch at the edge)
	a := box(0, 0, 0, 10, 10, 2)
	b := box(0, 3, 0, 4, 7, 2)
	c := a.Clone("c").Combine(OperationCut, b)
	near(t, "notch", mustWatertight(t, "notch", c).Volume, 200-32, 1e-6)
}

func TestBooleanSelf(t *testing.T) {
	a := box(0, 0, 0, 2, 2, 2)
	u := a.Clone("u").Combine(OperationJoin, box(0, 0, 0, 2, 2, 2))
	near(t, "A∪A", mustWatertight(t, "A∪A", u).Volume, 8, 1e-6)
	i := a.Clone("i").Combine(OperationIntersect, box(0, 0, 0, 2, 2, 2))
	near(t, "A∩A", mustWatertight(t, "A∩A", i).Volume, 8, 1e-6)
}

func TestBooleanCylinderHole(t *testing.T) {
	plate := NewPart("plate").Extrude(NewSketch(XY(0)).Rect(0, 0, 40, 40), 3)
	pin := NewSketch(XY(0)).Circle(5, 5, 4)
	cut := plate.Clone("cut").Extrude(pin, 3, Cut()) // same height: coplanar caps
	area := (&Mesh{}).Volume()
	_ = area
	hole := NewPart("h").Extrude(pin, 3).Mesh().Volume()
	near(t, "plate-hole", mustWatertight(t, "cyl hole", cut).Volume, 4800-hole, 1e-6)
}

func TestBooleanChain(t *testing.T) {
	// join, then cut a through hole, then cut the top with a tilted tapered pocket
	p := NewPart("chain")
	p.Extrude(NewSketch(XY(0)).Rect(0, 0, 30, 30), 5)
	p.Extrude(NewSketch(XY(5)).Circle(0, 0, 8), 4)
	p.Extrude(NewSketch(XY(0)).Circle(0, 0, 3), 9, Cut())
	p.Extrude(NewSketch(XY(9)).Circle(0, 0, 5), -2, Cut(), Taper(20))
	mustWatertight(t, "chain", p)
	if len(p.Log) != 4 {
		t.Errorf("log %d", len(p.Log))
	}
}

func TestPartRotateTranslate(t *testing.T) {
	p := box(0, 0, 0, 2, 1, 1).RotateZ(90).Translate(5, 0, 0)
	lo, hi := p.Mesh().Bounds()
	near(t, "xmin", lo.X, 4, 1e-9)
	near(t, "xmax", hi.X, 5, 1e-9)
	near(t, "ymax", hi.Y, 2, 1e-9)
	near(t, "volume", p.Report().Volume, 2, 1e-9)
}
