// guard rebuilds the Prestera90 guard (see the project doc prestera90-guard-spec.md) with the
// gocad kernel, one feature at a time, and checks it against the Fusion reference values.
package main

import (
	"fmt"
	"math"
	"os"
	"time"

	tsumiki "github.com/nasa9084/tsumiki"
)

const (
	hexFlats  = 3.2
	hexPitch  = 4.0
	frame     = 2.5
	splitX    = 0.7
	plateR    = 65.0
	holeR     = 22.5
	plateT    = 1.5
	touchEps  = 0.05 // how far a hex may be from the reinforcement band and still count as touching
	skirtH    = 12.0
	rot45     = 45.0
	cornerOff = 33.94 // (41.3-17.3)*sqrt2 : distance of the skirt corner arc centre
)

type pt = tsumiki.V2

func hexPoly(c pt) []pt {
	R := hexFlats / math.Sqrt(3)
	out := make([]pt, 6)
	for k := range out {
		a := float64(k) * math.Pi / 3
		out[k] = pt{c.X + R*math.Cos(a), c.Y + R*math.Sin(a)}
	}
	return out
}

func inRounded(p pt, half, r float64) bool {
	a := -tsumiki.Rad(rot45)
	u := p.X*math.Cos(a) - p.Y*math.Sin(a)
	v := p.X*math.Sin(a) + p.Y*math.Cos(a)
	qx, qy := math.Abs(u)-(half-r), math.Abs(v)-(half-r)
	if qx > r || qy > r {
		return false
	}
	return math.Max(qx, 0)*math.Max(qx, 0)+math.Max(qy, 0)*math.Max(qy, 0) <= r*r
}

func inBand(p pt, eps float64) bool {
	return inRounded(p, 44.3+eps, 20.3+eps) && !inRounded(p, 38.3-eps, 14.3-eps)
}

func inHex(p, c pt) bool {
	for k := 0; k < 6; k++ {
		a := float64(k)*math.Pi/3 + math.Pi/6
		if (p.X-c.X)*math.Cos(a)+(p.Y-c.Y)*math.Sin(a) > hexFlats/2 {
			return false
		}
	}
	return true
}

// cells returns every hex centre whose hexagon lies >= 40% inside the meshed region, and the
// subset that touches the reinforcement band.
func cells() (all, nearBand []pt) {
	R := hexFlats / math.Sqrt(3)
	b1 := pt{hexPitch * math.Cos(math.Pi/6), hexPitch * math.Sin(math.Pi/6)}
	b2 := pt{hexPitch * math.Cos(math.Pi/2), hexPitch * math.Sin(math.Pi/2)}
	const n = 24
	for i := -40; i <= 40; i++ {
		for j := -40; j <= 40; j++ {
			c := pt{float64(i)*b1.X + float64(j)*b2.X, float64(i)*b1.Y + float64(j)*b2.Y}
			if math.Hypot(c.X, c.Y) > 70 {
				continue
			}
			tot, in, band := 0, 0, 0
			for a := 0; a < n; a++ {
				for b := 0; b < n; b++ {
					p := pt{c.X + ((float64(a)+.5)/n*2-1)*R, c.Y + ((float64(b)+.5)/n*2-1)*R}
					if !inHex(p, c) {
						continue
					}
					tot++
					if r := math.Hypot(p.X, p.Y); r >= holeR+frame && r <= plateR-frame && p.X >= splitX+frame {
						in++
					}
					if inBand(p, touchEps) {
						band++
					}
				}
			}
			if float64(in)/float64(tot) >= 0.4 {
				all = append(all, c)
				if band > 0 {
					nearBand = append(nearBand, c)
				}
			}
		}
	}
	return
}

func hexSketch(z float64, cs []pt) *tsumiki.Sketch {
	s := tsumiki.NewSketch(tsumiki.XY(z))
	for _, c := range cs {
		s.Polygon(hexPoly(c)...)
	}
	return s
}

func ring(z float64, outerHalf, outerR, innerHalf, innerR float64) *tsumiki.Sketch {
	return tsumiki.NewSketch(tsumiki.XY(z)).RoundedRect(0, 0, 2*outerHalf, 2*outerHalf, outerR, rot45).
		RoundedRect(0, 0, 2*innerHalf, 2*innerHalf, innerR, rot45)
}

func discArea(z, r0, r1 float64) *tsumiki.Sketch {
	s := tsumiki.NewSketch(tsumiki.XY(z)).Circle(0, 0, r1)
	if r0 > 0 {
		s.Circle(0, 0, r0)
	}
	return s
}

// cornerCutter builds the plan-view region removed by an R3 rounding of the corner between the
// end face x=splitX and a circle of radius rc (material on the side of the circle given by
// outside: true = material inside the circle, false = material outside it); sign is +1/-1 for y.
func cornerCutter(rc float64, materialInside bool, sign float64) []pt {
	const rr = 3.0
	cx := splitX + rr
	var d float64 // distance of the fillet centre from the origin
	if materialInside {
		d = rc - rr
	} else {
		d = rc + rr
	}
	cy := math.Sqrt(d*d - cx*cx)
	C := pt{cx, cy}
	T1 := pt{splitX, cy}
	T2 := C.Mul(rc / d)
	P := pt{splitX, math.Sqrt(rc*rc - splitX*splitX)}
	// arc on the fillet circle from T1 to T2 (the short way), then along the big circle back to P
	arc := func(c pt, rad float64, from, to pt, n int) []pt {
		a0 := math.Atan2(from.Y-c.Y, from.X-c.X)
		a1 := math.Atan2(to.Y-c.Y, to.X-c.X)
		for a1-a0 > math.Pi {
			a1 -= 2 * math.Pi
		}
		for a1-a0 < -math.Pi {
			a1 += 2 * math.Pi
		}
		out := make([]pt, 0, n)
		for i := 1; i < n; i++ {
			a := a0 + (a1-a0)*float64(i)/float64(n)
			out = append(out, pt{c.X + rad*math.Cos(a), c.Y + rad*math.Sin(a)})
		}
		return out
	}
	poly := []pt{P, T1}
	poly = append(poly, arc(C, rr, T1, T2, 40)...)
	poly = append(poly, T2)
	poly = append(poly, arc(pt{}, rc, T2, P, 40)...)
	for i := range poly {
		poly[i].Y *= sign
	}
	return poly
}

func step(p *tsumiki.Part, tag string, t0 *time.Time) {
	if p.Err() != nil {
		fmt.Printf("!! %s: %v\n", tag, p.Err())
		os.Exit(1)
	}
	r := p.Report()
	lo, hi := p.Mesh().Bounds()
	fmt.Printf("%-34s tris=%-6d vol=%10.2f open=%d nm=%d  bbox x[%.2f,%.2f] y[%.2f,%.2f] z[%.2f,%.2f]  %.1fs\n",
		tag, r.Tris, r.Volume, r.OpenEdges, r.NonManifold, lo.X, hi.X, lo.Y, hi.Y, lo.Z, hi.Z, time.Since(*t0).Seconds())
	*t0 = time.Now()
}

func main() {
	t0 := time.Now()
	all, nearBand := cells()
	fmt.Printf("hex cells: %d (spec 364), recess cells: %d (spec 100)\n", len(all), len(nearBand))

	p := tsumiki.NewPart("prestera90-guard")

	// 1. plate
	p.Extrude(discArea(0, holeR, plateR), plateT)
	step(p, "1 plate r65 / hole 22.5", &t0)

	// 2. skirt: rounded-square ring, 12 below the plate, biting 0.5 into it
	p.Extrude(ring(-skirtH, 41.3, 17.3, 38.3, 14.3), skirtH+0.5)
	step(p, "2 skirt", &t0)

	// 3. fillet: outer profile grows 45deg for 3 mm, the cavity is cut back out
	fil := tsumiki.NewPart("fillet")
	fil.Extrude(tsumiki.NewSketch(tsumiki.XY(-3)).RoundedRect(0, 0, 82.6, 82.6, 17.3, rot45), 3, tsumiki.Taper(45))
	fil.Extrude(tsumiki.NewSketch(tsumiki.XY(-4)).RoundedRect(0, 0, 76.6, 76.6, 14.3, rot45), 5, tsumiki.Cut())
	p.Combine(tsumiki.OpJoin, fil)
	step(p, "3 fillet 45deg x3", &t0)

	// 4. hexagonal through holes
	p.Extrude(hexSketch(0, all), plateT, tsumiki.Cut())
	step(p, "4 hex through holes", &t0)

	// 5. reinforcement band (solid) over the wall + fillet
	p.Extrude(ring(0, 44.3, 20.3, 38.3, 14.3), plateT)
	step(p, "5 reinforcement band", &t0)

	// 6. hex recesses 0.6 deep from the top, on cells touching the band
	p.Extrude(hexSketch(plateT-0.6, nearBand), 0.6, tsumiki.Cut())
	step(p, "6 hex recesses", &t0)

	// 7. frame restore: outer ring, hole ring, split-edge strip
	p.Extrude(discArea(0, plateR-frame, plateR), plateT)
	p.Extrude(discArea(0, holeR, holeR+frame), plateT)
	strip := tsumiki.NewPart("strip").Extrude(tsumiki.NewSketch(tsumiki.XY(0)).Rect((0+splitX+frame)/2, 0, splitX+frame, 2*plateR), plateT)
	strip.Combine(tsumiki.OpIntersect, tsumiki.NewPart("annulus").Extrude(discArea(0, holeR, plateR), plateT))
	p.Combine(tsumiki.OpJoin, strip)
	step(p, "7 frame restore", &t0)

	// 8. zip-tie bumps: 10 mm long triangular rib around each corner arc, 7..9 mm below the plate
	bump := tsumiki.NewPart("bump").Revolve(
		tsumiki.NewSketch(tsumiki.XZ(0)).Polygon(pt{16.8, -9}, pt{17.3, -9}, pt{18.3, -8}, pt{17.3, -7}, pt{16.8, -7}),
		math.Round(10/17.3*180/math.Pi*100)/100)
	half := 10 / 17.3 / 2 * 180 / math.Pi
	bump.RotateZ(-half)
	for _, ang := range []float64{0, 90, -90} {
		b := bump.Clone("bump").RotateZ(ang)
		c := pt{cornerOff, 0}.Rot(tsumiki.Rad(ang))
		b.Translate(c.X, c.Y, 0)
		p.Combine(tsumiki.OpJoin, b)
	}
	step(p, "8 zip-tie bumps", &t0)

	// 9. rim slit inside the skirt: 2 mm straight, then a 45deg ceiling (1.2 mm)
	slit := tsumiki.NewPart("slit")
	slit.Extrude(tsumiki.NewSketch(tsumiki.XY(0)).RoundedRect(0, 0, 79.0, 79.0, 15.5, rot45), -2)
	slit.Extrude(tsumiki.NewSketch(tsumiki.XY(-2)).RoundedRect(0, 0, 79.0, 79.0, 15.5, rot45), -1.2, tsumiki.Taper(-45))
	p.Combine(tsumiki.OpCut, slit)
	step(p, "9 rim slit + 45deg ceiling", &t0)

	// 10. split at x = 0.7
	p.Extrude(tsumiki.NewSketch(tsumiki.XY(-20)).Rect((splitX-100)/2, 0, 100+splitX, 300), 40, tsumiki.Cut())
	step(p, "10 split x=0.7", &t0)

	// 11. R3 on the four vertical end corners
	for _, s := range []float64{1, -1} {
		p.Extrude(tsumiki.NewSketch(tsumiki.XY(-1)).Polygon(cornerCutter(plateR, true, s)...), 3.5, tsumiki.Cut())
		p.Extrude(tsumiki.NewSketch(tsumiki.XY(-1)).Polygon(cornerCutter(holeR, false, s)...), 3.5, tsumiki.Cut())
	}
	step(p, "11 R3 end corners", &t0)

	// 12. R1 on the top outer rim, as a revolve cut
	const rr = 1.0
	rim := tsumiki.NewSketch(tsumiki.XZ(0))
	rim.Path(plateR-rr, plateT).ArcTo(plateR, plateT-rr, plateR-rr, plateT-rr, false).
		LineTo(plateR+0.5, plateT-rr).LineTo(plateR+0.5, plateT+0.5).LineTo(plateR-rr, plateT+0.5).Close()
	p.Revolve(rim, 360, tsumiki.Cut())
	step(p, "12 R1 top rim (revolve cut)", &t0)

	// 13. wire notch: semicircle r1.5 at the +y end of the split face, plate thickness only
	p.Extrude(tsumiki.NewSketch(tsumiki.XY(0)).Circle(splitX, math.Sqrt(45.7*45.7-splitX*splitX), 1.5), plateT, tsumiki.Cut())
	step(p, "13 wire notch", &t0)

	fmt.Println()
	fmt.Print(p.Timeline())
	m := p.Mesh()
	lo, hi := m.Bounds()
	rep := p.Report()
	fmt.Printf("\nbbox  x %.3f..%.3f (0.7..65)  y %.3f..%.3f (±64.89)  z %.3f..%.3f (-12..1.5)\n", lo.X, hi.X, lo.Y, hi.Y, lo.Z, hi.Z)
	fmt.Printf("volume %.2f (spec ≈9979.6)   watertight=%v  %+v\n", rep.Volume, rep.Watertight(), rep)

	// point-in/out table of spec section 5
	type probe struct {
		x, y, z float64
		in      bool
		what    string
	}
	probes := []probe{
		{0.9, 35, 0.7, true, "split-edge frame"},
		{0.4, 35, 0.7, false, "before split plane"},
		{cornerOff + 17.5, 0, -7.5, true, "bump inside"},
		{cornerOff + 18.1, 0, -7.5, false, "bump outside"},
		{cornerOff + 18.5, 0, -1.5, true, "fillet inside"},
		{cornerOff + 19.2, 0, -1.5, false, "fillet outside"},
		{49.0, 0, -1.0, false, "slit space"},
		{48.8, 0, -3.0, true, "under slit ceiling slope"},
		{1.2, 45.695, 0.7, false, "wire notch"},
		{1.2, -45.695, 0.7, true, "-y side, no notch"},
		{64.8, 0, 1.4, false, "top rim R1"},
		{64.0, 0, 1.4, true, "near rim"},
		{64.9, 0, 0.2, true, "rim lower"},
		{63.0, 0, 0.7, true, "outer frame"},
	}
	bad := 0
	for _, q := range probes {
		got := contains(m, tsumiki.V3{X: q.x, Y: q.y, Z: q.z})
		mark := "ok"
		if got != q.in {
			mark = "MISMATCH"
			bad++
		}
		fmt.Printf("  (%7.3f,%7.3f,%6.2f) want %-5v got %-5v %-9s %s\n", q.x, q.y, q.z, q.in, got, mark, q.what)
	}
	fmt.Printf("probe mismatches: %d / %d\n", bad, len(probes))

	if len(os.Args) > 1 {
		dir := os.Args[1]
		if err := m.Save3MF(dir+"/prestera90_guard.3mf", "prestera90-guard"); err != nil {
			fmt.Println(err)
		}
		if err := m.SaveSTL(dir + "/prestera90_guard.stl"); err != nil {
			fmt.Println(err)
		}
		fmt.Println("saved to", dir)
	}
}

// contains is a parity ray test; three skew directions vote to avoid hitting edges exactly.
func contains(m *tsumiki.Mesh, q tsumiki.V3) bool {
	dirs := []tsumiki.V3{{X: 0.5773, Y: 0.3141, Z: 0.7532}, {X: -0.2718, Y: 0.8123, Z: 0.5127}, {X: 0.1234, Y: -0.6543, Z: 0.7456}}
	votes := 0
	for _, d := range dirs {
		hits := 0
		for _, t := range m.T {
			a, b, c := m.V[t[0]], m.V[t[1]], m.V[t[2]]
			e1, e2 := b.Sub(a), c.Sub(a)
			h := d.Cross(e2)
			det := e1.Dot(h)
			if math.Abs(det) < 1e-12 {
				continue
			}
			f := 1 / det
			s := q.Sub(a)
			u := f * s.Dot(h)
			if u < 0 || u > 1 {
				continue
			}
			qv := s.Cross(e1)
			v := f * d.Dot(qv)
			if v < 0 || u+v > 1 {
				continue
			}
			if f*e2.Dot(qv) > 0 {
				hits++
			}
		}
		if hits%2 == 1 {
			votes++
		}
	}
	return votes >= 2
}
