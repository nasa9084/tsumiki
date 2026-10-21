// guard rebuilds the Prestera90 guard (see the project doc prestera90-guard-spec.md) with the
// tsumiki kernel, one feature at a time, and checks it against the Fusion reference values.
package main

import (
	"fmt"
	"io"
	"math"
	"os"
	"time"

	tsumiki "github.com/nasa9084/tsumiki"
)

const (
	hexAcrossFlats          = 3.2
	hexPitch                = 4.0
	frame                   = 2.5
	splitX                  = 0.7
	plateRadius             = 65.0
	holeRadius              = 22.5
	plateThickness          = 1.5
	touchTolerance          = 0.05 // how far a hex may be from the reinforcement band and still count as touching
	skirtHeight             = 12.0
	rotationDegrees         = 45.0
	cornerArcCenterDistance = 33.94 // (41.3-17.3)*sqrt2 : distance of the skirt corner arc center
)

type point = tsumiki.Vector2

func hexPoly(c point) []point {
	circumradius := hexAcrossFlats / math.Sqrt(3)
	out := make([]point, 6)
	for k := range out {
		a := float64(k) * math.Pi / 3
		out[k] = point{X: c.X + circumradius*math.Cos(a), Y: c.Y + circumradius*math.Sin(a)}
	}
	return out
}

func inRounded(p point, halfSize, r float64) bool {
	a := -tsumiki.Radians(rotationDegrees)
	u := p.X*math.Cos(a) - p.Y*math.Sin(a)
	v := p.X*math.Sin(a) + p.Y*math.Cos(a)
	qx, qy := math.Abs(u)-(halfSize-r), math.Abs(v)-(halfSize-r)
	if qx > r || qy > r {
		return false
	}
	return math.Max(qx, 0)*math.Max(qx, 0)+math.Max(qy, 0)*math.Max(qy, 0) <= r*r
}

func inBand(p point, tolerance float64) bool {
	return inRounded(p, 44.3+tolerance, 20.3+tolerance) && !inRounded(p, 38.3-tolerance, 14.3-tolerance)
}

func inHex(p, c point) bool {
	for k := 0; k < 6; k++ {
		a := float64(k)*math.Pi/3 + math.Pi/6
		if (p.X-c.X)*math.Cos(a)+(p.Y-c.Y)*math.Sin(a) > hexAcrossFlats/2 {
			return false
		}
	}
	return true
}

// cells returns every hex center whose hexagon lies >= 40% inside the meshed region, and the
// subset that touches the reinforcement band.
func cells() (allCells, bandCells []point) {
	circumradius := hexAcrossFlats / math.Sqrt(3)
	basisA := point{X: hexPitch * math.Cos(math.Pi/6), Y: hexPitch * math.Sin(math.Pi/6)}
	basisB := point{X: hexPitch * math.Cos(math.Pi/2), Y: hexPitch * math.Sin(math.Pi/2)}
	const sampleCount = 24
	for i := -40; i <= 40; i++ {
		for j := -40; j <= 40; j++ {
			c := point{X: float64(i)*basisA.X + float64(j)*basisB.X, Y: float64(i)*basisA.Y + float64(j)*basisB.Y}
			if math.Hypot(c.X, c.Y) > 70 {
				continue
			}
			total, inside, band := 0, 0, 0
			for a := 0; a < sampleCount; a++ {
				for b := 0; b < sampleCount; b++ {
					p := point{X: c.X + ((float64(a)+.5)/sampleCount*2-1)*circumradius, Y: c.Y + ((float64(b)+.5)/sampleCount*2-1)*circumradius}
					if !inHex(p, c) {
						continue
					}
					total++
					if r := math.Hypot(p.X, p.Y); r >= holeRadius+frame && r <= plateRadius-frame && p.X >= splitX+frame {
						inside++
					}
					if inBand(p, touchTolerance) {
						band++
					}
				}
			}
			if float64(inside)/float64(total) >= 0.4 {
				allCells = append(allCells, c)
				if band > 0 {
					bandCells = append(bandCells, c)
				}
			}
		}
	}
	return
}

func hexSketch(z float64, centers []point) *tsumiki.Sketch {
	s := tsumiki.NewSketch(tsumiki.XY(z))
	for _, c := range centers {
		s.Polygon(hexPoly(c)...)
	}
	return s
}

func ring(z float64, outerHalf, outerR, innerHalf, innerR float64) *tsumiki.Sketch {
	return tsumiki.NewSketch(tsumiki.XY(z)).RoundedRect(0, 0, 2*outerHalf, 2*outerHalf, outerR, rotationDegrees).
		RoundedRect(0, 0, 2*innerHalf, 2*innerHalf, innerR, rotationDegrees)
}

func discArea(innerRadius, outerRadius float64) *tsumiki.Sketch {
	s := tsumiki.NewSketch(tsumiki.XY(0)).Circle(0, 0, outerRadius)
	if innerRadius > 0 {
		s.Circle(0, 0, innerRadius)
	}
	return s
}

// cornerCutter builds the plan-view region removed by an R3 rounding of the corner between the
// end face x=splitX and a circle of radius rc (material on the side of the circle given by
// outside: true = material inside the circle, false = material outside it); sign is +1/-1 for y.
func cornerCutter(circleRadius float64, materialInside bool, sign float64) []point {
	const filletRadius = 3.0
	centerX := splitX + filletRadius
	var filletCenterDistance float64 // distance of the fillet center from the origin
	if materialInside {
		filletCenterDistance = circleRadius - filletRadius
	} else {
		filletCenterDistance = circleRadius + filletRadius
	}
	centerY := math.Sqrt(filletCenterDistance*filletCenterDistance - centerX*centerX)
	filletCenter := point{X: centerX, Y: centerY}
	tangentOnLine := point{X: splitX, Y: centerY}
	tangentOnCircle := filletCenter.Scale(circleRadius / filletCenterDistance)
	cornerPoint := point{X: splitX, Y: math.Sqrt(circleRadius*circleRadius - splitX*splitX)}
	// arc on the fillet circle from T1 to T2 (the short way), then along the big circle back to P
	arc := func(c point, radius float64, from, to point, n int) []point {
		startAngle := math.Atan2(from.Y-c.Y, from.X-c.X)
		endAngle := math.Atan2(to.Y-c.Y, to.X-c.X)
		for endAngle-startAngle > math.Pi {
			endAngle -= 2 * math.Pi
		}
		for endAngle-startAngle < -math.Pi {
			endAngle += 2 * math.Pi
		}
		out := make([]point, 0, n)
		for i := 1; i < n; i++ {
			a := startAngle + (endAngle-startAngle)*float64(i)/float64(n)
			out = append(out, point{X: c.X + radius*math.Cos(a), Y: c.Y + radius*math.Sin(a)})
		}
		return out
	}
	poly := []point{cornerPoint, tangentOnLine}
	poly = append(poly, arc(filletCenter, filletRadius, tangentOnLine, tangentOnCircle, 40)...)
	poly = append(poly, tangentOnCircle)
	poly = append(poly, arc(point{}, circleRadius, tangentOnCircle, cornerPoint, 40)...)
	for i := range poly {
		poly[i].Y *= sign
	}
	return poly
}

func step(out io.Writer, part *tsumiki.Part, tag string, stepStart *time.Time) error {
	if part.Err() != nil {
		return fmt.Errorf("%s: %w", tag, part.Err())
	}
	r := part.Report()
	lo, hi := part.Mesh().Bounds()
	_, _ = fmt.Fprintf(out, "%-34s tris=%-6d vol=%10.2f open=%d nm=%d  bbox x[%.2f,%.2f] y[%.2f,%.2f] z[%.2f,%.2f]  %.1fs\n",
		tag, r.Triangles, r.Volume, r.OpenEdges, r.NonManifold, lo.X, hi.X, lo.Y, hi.Y, lo.Z, hi.Z, time.Since(*stepStart).Seconds())
	*stepStart = time.Now()
	return nil
}

// build reproduces the guard feature by feature and logs one line per step to out.
func build(out io.Writer) (*tsumiki.Part, error) {
	stepStart := time.Now()
	allCells, bandCells := cells()
	_, _ = fmt.Fprintf(out, "hex cells: %d (spec 364), recess cells: %d (spec 100)\n", len(allCells), len(bandCells))

	part := tsumiki.NewPart("prestera90-guard")

	// 1. plate
	part.Extrude(discArea(holeRadius, plateRadius), plateThickness)
	if err := step(out, part, "1 plate r65 / hole 22.5", &stepStart); err != nil {
		return nil, err
	}

	// 2. skirt: rounded-square ring, 12 below the plate, biting 0.5 into it
	part.Extrude(ring(-skirtHeight, 41.3, 17.3, 38.3, 14.3), skirtHeight+0.5)
	if err := step(out, part, "2 skirt", &stepStart); err != nil {
		return nil, err
	}

	// 3. fillet: outer profile grows 45deg for 3 mm, the cavity is cut back out
	fillet := tsumiki.NewPart("fillet")
	fillet.Extrude(tsumiki.NewSketch(tsumiki.XY(-3)).RoundedRect(0, 0, 82.6, 82.6, 17.3, rotationDegrees), 3, tsumiki.Taper(45))
	fillet.Extrude(tsumiki.NewSketch(tsumiki.XY(-4)).RoundedRect(0, 0, 76.6, 76.6, 14.3, rotationDegrees), 5, tsumiki.Cut())
	part.Combine(tsumiki.OperationJoin, fillet)
	if err := step(out, part, "3 fillet 45deg x3", &stepStart); err != nil {
		return nil, err
	}

	// 4. hexagonal through holes
	part.Extrude(hexSketch(0, allCells), plateThickness, tsumiki.Cut())
	if err := step(out, part, "4 hex through holes", &stepStart); err != nil {
		return nil, err
	}

	// 5. reinforcement band (solid) over the wall + fillet
	part.Extrude(ring(0, 44.3, 20.3, 38.3, 14.3), plateThickness)
	if err := step(out, part, "5 reinforcement band", &stepStart); err != nil {
		return nil, err
	}

	// 6. hex recesses 0.6 deep from the top, on cells touching the band
	part.Extrude(hexSketch(plateThickness-0.6, bandCells), 0.6, tsumiki.Cut())
	if err := step(out, part, "6 hex recesses", &stepStart); err != nil {
		return nil, err
	}

	// 7. frame restore: outer ring, hole ring, split-edge strip
	part.Extrude(discArea(plateRadius-frame, plateRadius), plateThickness)
	part.Extrude(discArea(holeRadius, holeRadius+frame), plateThickness)
	strip := tsumiki.NewPart("strip").Extrude(tsumiki.NewSketch(tsumiki.XY(0)).Rect((0+splitX+frame)/2, 0, splitX+frame, 2*plateRadius), plateThickness)
	strip.Combine(tsumiki.OperationIntersect, tsumiki.NewPart("annulus").Extrude(discArea(holeRadius, plateRadius), plateThickness))
	part.Combine(tsumiki.OperationJoin, strip)
	if err := step(out, part, "7 frame restore", &stepStart); err != nil {
		return nil, err
	}

	// 8. zip-tie bumps: 10 mm long triangular rib around each corner arc, 7..9 mm below the plate
	bump := tsumiki.NewPart("bump").Revolve(
		tsumiki.NewSketch(tsumiki.XZ(0)).Polygon(point{X: 16.8, Y: -9}, point{X: 17.3, Y: -9}, point{X: 18.3, Y: -8}, point{X: 17.3, Y: -7}, point{X: 16.8, Y: -7}),
		math.Round(10/17.3*180/math.Pi*100)/100)
	halfAngleDegrees := 10 / 17.3 / 2 * 180 / math.Pi
	bump.RotateZ(-halfAngleDegrees)
	for _, angleDegrees := range []float64{0, 90, -90} {
		b := bump.Clone("bump").RotateZ(angleDegrees)
		c := point{X: cornerArcCenterDistance}.Rotate(tsumiki.Radians(angleDegrees))
		b.Translate(c.X, c.Y, 0)
		part.Combine(tsumiki.OperationJoin, b)
	}
	if err := step(out, part, "8 zip-tie bumps", &stepStart); err != nil {
		return nil, err
	}

	// 9. rim slit inside the skirt: 2 mm straight, then a 45deg ceiling (1.2 mm)
	slit := tsumiki.NewPart("slit")
	slit.Extrude(tsumiki.NewSketch(tsumiki.XY(0)).RoundedRect(0, 0, 79.0, 79.0, 15.5, rotationDegrees), -2)
	slit.Extrude(tsumiki.NewSketch(tsumiki.XY(-2)).RoundedRect(0, 0, 79.0, 79.0, 15.5, rotationDegrees), -1.2, tsumiki.Taper(-45))
	part.Combine(tsumiki.OperationCut, slit)
	if err := step(out, part, "9 rim slit + 45deg ceiling", &stepStart); err != nil {
		return nil, err
	}

	// 10. split at x = 0.7
	part.Extrude(tsumiki.NewSketch(tsumiki.XY(-20)).Rect((splitX-100)/2, 0, 100+splitX, 300), 40, tsumiki.Cut())
	if err := step(out, part, "10 split x=0.7", &stepStart); err != nil {
		return nil, err
	}

	// 11. R3 on the four vertical end corners
	for _, s := range []float64{1, -1} {
		part.Extrude(tsumiki.NewSketch(tsumiki.XY(-1)).Polygon(cornerCutter(plateRadius, true, s)...), 3.5, tsumiki.Cut())
		part.Extrude(tsumiki.NewSketch(tsumiki.XY(-1)).Polygon(cornerCutter(holeRadius, false, s)...), 3.5, tsumiki.Cut())
	}
	if err := step(out, part, "11 R3 end corners", &stepStart); err != nil {
		return nil, err
	}

	// 12. R1 on the top outer rim, as a revolve cut
	const rimRadius = 1.0
	rim := tsumiki.NewSketch(tsumiki.XZ(0))
	rim.Path(plateRadius-rimRadius, plateThickness).ArcTo(plateRadius, plateThickness-rimRadius, plateRadius-rimRadius, plateThickness-rimRadius, false).
		LineTo(plateRadius+0.5, plateThickness-rimRadius).LineTo(plateRadius+0.5, plateThickness+0.5).LineTo(plateRadius-rimRadius, plateThickness+0.5).Close()
	part.Revolve(rim, 360, tsumiki.Cut())
	if err := step(out, part, "12 R1 top rim (revolve cut)", &stepStart); err != nil {
		return nil, err
	}

	// 13. wire notch: semicircle r1.5 at the +y end of the split face, plate thickness only
	part.Extrude(tsumiki.NewSketch(tsumiki.XY(0)).Circle(splitX, math.Sqrt(45.7*45.7-splitX*splitX), 1.5), plateThickness, tsumiki.Cut())
	if err := step(out, part, "13 wire notch", &stepStart); err != nil {
		return nil, err
	}

	return part, nil
}

// probe is one point of the spec's in/out table (section 5).
type probe struct {
	x, y, z     float64
	inside      bool
	description string
}

var referenceProbes = []probe{
	{0.9, 35, 0.7, true, "split-edge frame"},
	{0.4, 35, 0.7, false, "before split plane"},
	{cornerArcCenterDistance + 17.5, 0, -7.5, true, "bump inside"},
	{cornerArcCenterDistance + 18.1, 0, -7.5, false, "bump outside"},
	{cornerArcCenterDistance + 18.5, 0, -1.5, true, "fillet inside"},
	{cornerArcCenterDistance + 19.2, 0, -1.5, false, "fillet outside"},
	{49.0, 0, -1.0, false, "slit space"},
	{48.8, 0, -3.0, true, "under slit ceiling slope"},
	{1.2, 45.695, 0.7, false, "wire notch"},
	{1.2, -45.695, 0.7, true, "-y side, no notch"},
	{64.8, 0, 1.4, false, "top rim R1"},
	{64.0, 0, 1.4, true, "near rim"},
	{64.9, 0, 0.2, true, "rim lower"},
	{63.0, 0, 0.7, true, "outer frame"},
}

// checkProbes tests the reference points against mesh and returns how many disagree.
func checkProbes(out io.Writer, mesh *tsumiki.Mesh) int {
	mismatchCount := 0
	for _, q := range referenceProbes {
		got := contains(mesh, tsumiki.Vector3{X: q.x, Y: q.y, Z: q.z})
		mark := "ok"
		if got != q.inside {
			mark = "MISMATCH"
			mismatchCount++
		}
		_, _ = fmt.Fprintf(out, "  (%7.3f,%7.3f,%6.2f) want %-5v got %-5v %-9s %s\n", q.x, q.y, q.z, q.inside, got, mark, q.description)
	}
	return mismatchCount
}

func main() {
	part, err := build(os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println()
	fmt.Print(part.Timeline())
	mesh := part.Mesh()
	lo, hi := mesh.Bounds()
	report := part.Report()
	fmt.Printf("\nbbox  x %.3f..%.3f (0.7..65)  y %.3f..%.3f (±64.89)  z %.3f..%.3f (-12..1.5)\n", lo.X, hi.X, lo.Y, hi.Y, lo.Z, hi.Z)
	fmt.Printf("volume %.2f (spec ≈9979.6)   watertight=%v  %+v\n", report.Volume, report.Watertight(), report)
	mismatchCount := checkProbes(os.Stdout, mesh)
	fmt.Printf("probe mismatches: %d / %d\n", mismatchCount, len(referenceProbes))

	if len(os.Args) > 1 {
		dir := os.Args[1]
		if err := mesh.Save3MF(dir+"/prestera90_guard.3mf", "prestera90-guard"); err != nil {
			fmt.Println(err)
		}
		if err := mesh.SaveSTL(dir + "/prestera90_guard.stl"); err != nil {
			fmt.Println(err)
		}
		fmt.Println("saved to", dir)
	}
}

// contains is a parity ray test; three skew directions vote to avoid hitting edges exactly.
func contains(m *tsumiki.Mesh, q tsumiki.Vector3) bool {
	directions := []tsumiki.Vector3{{X: 0.5773, Y: 0.3141, Z: 0.7532}, {X: -0.2718, Y: 0.8123, Z: 0.5127}, {X: 0.1234, Y: -0.6543, Z: 0.7456}}
	votes := 0
	for _, d := range directions {
		hits := 0
		for _, t := range m.Triangles {
			a, b, c := m.Vertices[t[0]], m.Vertices[t[1]], m.Vertices[t[2]]
			firstEdge, secondEdge := b.Sub(a), c.Sub(a)
			h := d.Cross(secondEdge)
			determinant := firstEdge.Dot(h)
			if math.Abs(determinant) < 1e-12 {
				continue
			}
			f := 1 / determinant
			s := q.Sub(a)
			u := f * s.Dot(h)
			if u < 0 || u > 1 {
				continue
			}
			qv := s.Cross(firstEdge)
			v := f * d.Dot(qv)
			if v < 0 || u+v > 1 {
				continue
			}
			if f*secondEdge.Dot(qv) > 0 {
				hits++
			}
		}
		if hits%2 == 1 {
			votes++
		}
	}
	return votes >= 2
}
