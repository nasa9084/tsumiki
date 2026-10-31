package tsumiki

import (
	"cmp"
	"fmt"
	"math"
	"slices"
)

// ordinate is a position to dimension, with the end of the feature nearest to where the value is
// written (both in view coordinates), so that the extension line can start there.
type ordinate struct {
	value, from float64
}

// levels returns the heights of the visible horizontal lines of the view.
func (p *panel) levels() []ordinate {
	var values []ordinate
	for _, line := range p.visible {
		if math.Abs(line[0].Y-line[1].Y) < 1e-6 && math.Abs(line[0].X-line[1].X) >= 0.2 {
			values = append(values, ordinate{value: line[0].Y, from: math.Max(line[0].X, line[1].X)})
		}
	}
	return values
}

// addDetailDimensions labels the circles and arcs seen square-on (diameters and radii, grouped by
// size), marks the centers of circles and gives their positions as ordinate dimensions measured
// from the lower left corner of the view. withLevels also gives the heights of horizontal lines.
func (p *panel) addDetailDimensions(o drawingOptions, withLevels bool) {
	arcs := p.faceOnArcs()
	var across, upward []ordinate
	if withLevels {
		upward = p.levels()
	}
	groups := map[[2]int64][]faceOnArc{}
	var order [][2]int64
	for _, a := range arcs {
		if a.full {
			across = append(across, ordinate{value: a.center.X, from: a.center.Y + a.radius})
			upward = append(upward, ordinate{value: a.center.Y, from: a.center.X + a.radius})
			p.marks = append(p.marks, p.centerLines(a, o)...)
		}
		key := [2]int64{int64(math.Round(roundedRadius(a.radius) / sameValueSpacing)), map[bool]int64{true: 1}[a.full]}
		if groups[key] == nil {
			order = append(order, key)
		}
		groups[key] = append(groups[key], a)
	}
	p.marks = append(p.marks, p.ordinates(across, false, o)...)
	p.marks = append(p.marks, p.ordinates(upward, true, o)...)
	space := p.occupancy()
	for _, key := range order {
		p.marks = append(p.marks, p.arcLabel(groups[key], space, o)...)
	}
}

// centerLines draws the two center lines of a circle, reaching a little beyond it.
func (p *panel) centerLines(a faceOnArc, o drawingOptions) []mark {
	center := p.rel(a.center)
	reach := a.radius*o.scale + 1.5
	return []mark{
		centerMark(center.Sub(Vector2{reach, 0}), center.Add(Vector2{reach, 0})),
		centerMark(center.Sub(Vector2{0, reach}), center.Add(Vector2{0, reach})),
	}
}

// arcLabel writes the size of a group of equal circles or arcs ("2× ⌀4", "R3") on a leader that
// points at the first of them, placed where it covers the least of the drawing.
func (p *panel) arcLabel(group []faceOnArc, space *occupancy, o drawingOptions) []mark {
	a := group[0]
	text := "R" + length(roundedRadius(a.radius))
	var anchors []Vector2 // directions from the center to where the leader starts (paper)
	if a.full {
		text = "⌀" + length(2*roundedRadius(a.radius))
		for _, degrees := range []float64{45, 135, -45, -135, 90, -90, 0, 180} {
			anchors = append(anchors, Vector2{math.Cos(Radians(degrees)), -math.Sin(Radians(degrees))})
		}
	} else {
		anchors = append(anchors, unit(p.rel(a.middle).Sub(p.rel(a.center))))
	}
	if len(group) > 1 {
		text = fmt.Sprintf("%d× %s", len(group), text)
	}
	var best []mark
	bestCost := math.MaxInt
	for _, anchor := range anchors {
		at := p.rel(a.center).Add(anchor.Scale(a.radius * o.scale))
		for _, outward := range []float64{1, -1} {
			for _, turn := range []float64{0, 25, -25, 50, -50} {
				for _, reach := range []float64{5, 10} {
					direction := anchor.Scale(outward).Rotate(Radians(turn))
					marks := leaderLabel(at, direction, reach, text, o)
					if cost := space.cost(marks); cost < bestCost {
						best, bestCost = marks, cost
					}
				}
			}
			if a.full {
				break // a leader into a hole would cross the circle itself
			}
		}
	}
	space.add(best)
	return best
}

// leaderLabel is a label at the end of a leader that starts at at and runs reach millimeters
// along direction, then a short way sideways.
func leaderLabel(at, direction Vector2, reach float64, text string, o drawingOptions) []mark {
	bend := at.Add(direction.Scale(reach))
	sideways, anchor := Vector2{2, 0}, "start"
	if direction.X < 0 {
		sideways, anchor = Vector2{-2, 0}, "end"
	}
	end := bend.Add(sideways)
	return []mark{
		lineMark(at, bend, end),
		arrowMark(at, direction.Scale(-1)),
		textMark(end.Add(unit(sideways)).Add(Vector2{0, o.textSize * 0.35}), text, anchor, false, o.textSize),
	}
}

// occupancy is a coarse map of what is already drawn on and around a panel (paper millimeters,
// relative to the panel's box).
type occupancy struct {
	cells map[[2]int]int
}

const occupancyCell = 1.0 // paper mm

func (space *occupancy) cellOf(point Vector2) [2]int {
	return [2]int{int(math.Floor(point.X / occupancyCell)), int(math.Floor(point.Y / occupancyCell))}
}

// occupancy maps the panel's lines and the texts of its annotations.
func (p *panel) occupancy() *occupancy {
	space := &occupancy{cells: map[[2]int]int{}}
	for _, lines := range [][]segment2{p.visible, p.hidden} {
		for _, line := range lines {
			a, b := p.rel(line[0]), p.rel(line[1])
			steps := int(b.Sub(a).Length()/(occupancyCell/2)) + 1
			for k := 0; k <= steps; k++ {
				space.cells[space.cellOf(a.Add(b.Sub(a).Scale(float64(k)/float64(steps))))] |= 1
			}
		}
	}
	space.add(p.marks)
	return space
}

// add records the texts of marks as taken.
func (space *occupancy) add(marks []mark) {
	for _, m := range marks {
		if m.kind != markText {
			continue
		}
		low, high := m.bounds()
		first, last := space.cellOf(low), space.cellOf(high)
		for x := first[0]; x <= last[0]; x++ {
			for y := first[1]; y <= last[1]; y++ {
				space.cells[[2]int{x, y}] |= 2
			}
		}
	}
}

// cost scores where a label would go: lines under its text count, other texts count far more,
// and so do texts its leader would cross.
func (space *occupancy) cost(marks []mark) int {
	cost := 0
	for _, m := range marks {
		switch m.kind {
		case markText:
			low, high := m.bounds()
			first, last := space.cellOf(low), space.cellOf(high)
			for x := first[0]; x <= last[0]; x++ {
				for y := first[1]; y <= last[1]; y++ {
					cost += map[int]int{0: 0, 1: 1, 2: 50, 3: 51}[space.cells[[2]int{x, y}]]
				}
			}
		case markLine:
			for i := 1; i < len(m.points); i++ {
				for k := range 8 {
					if space.cells[space.cellOf(m.points[i-1].Add(m.points[i].Sub(m.points[i-1]).Scale(float64(k)/8)))]&2 != 0 {
						cost += 20
					}
				}
			}
		case markCenter, markThick, markArrow:
		}
	}
	return cost
}

// ordinates gives positions measured from the lower left corner of the view. Values along the
// view's vertical axis are written to the right of the view, values along its horizontal axis
// above it. Labels that would overlap are spread apart and joined to their position by a jog.
func (p *panel) ordinates(values []ordinate, vertical bool, o drawingOptions) []mark {
	type entry struct {
		at, from float64 // paper position along the edge, and where the extension line starts
		text     string
	}
	var entries []entry
	for _, v := range distinct(values) {
		if vertical {
			entries = append(entries, entry{p.rel(Vector2{0, v.value}).Y, p.rel(Vector2{v.from, 0}).X + 1,
				length(v.value - p.low.Y)})
		} else {
			entries = append(entries, entry{p.rel(Vector2{v.value, 0}).X, p.rel(Vector2{0, v.from}).Y - 1,
				length(v.value - p.low.X)})
		}
	}
	if len(entries) == 0 {
		return nil
	}
	slices.SortFunc(entries, func(a, b entry) int { return cmp.Compare(a.at, b.at) })
	labels := spreadLabels(len(entries), func(i int) float64 { return entries[i].at }, o.textSize*1.1)
	size := p.boxSize()
	var marks []mark
	for i, e := range entries {
		if vertical {
			edge := size.X
			marks = append(marks,
				lineMark(Vector2{e.from, e.at}, Vector2{edge + 4, e.at}, Vector2{edge + 6, labels[i]}),
				textMark(Vector2{edge + 7, labels[i] + o.textSize*0.35}, e.text, "start", false, o.textSize))
		} else {
			marks = append(marks,
				lineMark(Vector2{e.at, e.from}, Vector2{e.at, -4}, Vector2{labels[i], -6}),
				textMark(Vector2{labels[i] + o.textSize*0.35, -7}, e.text, "start", true, o.textSize))
		}
	}
	return marks
}

// spreadLabels places n labels in order along an edge, each as near its feature (at(i), sorted
// ascending) as possible but at least spacing apart, and centers the spread on the features.
func spreadLabels(n int, at func(int) float64, spacing float64) []float64 {
	labels := make([]float64, n)
	for i := range n {
		labels[i] = at(i)
		if i > 0 {
			labels[i] = math.Max(at(i), labels[i-1]+spacing)
		}
	}
	shift := (labels[n-1] - at(n-1)) / 2
	for i := range labels {
		labels[i] -= shift
	}
	return labels
}

// distinct sorts the ordinates by value and merges those within sameValueSpacing, keeping the
// feature end nearest to the labels.
func distinct(values []ordinate) []ordinate {
	sorted := slices.Clone(values)
	slices.SortFunc(sorted, func(a, b ordinate) int { return cmp.Compare(a.value, b.value) })
	var result []ordinate
	for _, v := range sorted {
		if n := len(result); n > 0 && v.value-result[n-1].value <= sameValueSpacing {
			result[n-1].from = math.Max(result[n-1].from, v.from)
			continue
		}
		result = append(result, v)
	}
	return result
}

// roundedRadius rounds a measured radius to a multiple of 0.05 mm when it is that close to one:
// a faceted curve only pins its radius down to the chord tolerance.
func roundedRadius(radius float64) float64 {
	if rounded := math.Round(radius/0.05) * 0.05; math.Abs(rounded-radius) <= circleTolerance {
		return rounded
	}
	return radius
}
