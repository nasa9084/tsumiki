# tsumiki

A code CAD library written in Go. Like Fusion, you stack operations in order
(sketch → extrude/revolve → join/cut) through a history-style API, and export 3MF/STL for 3D printing.
The geometry kernel is written from scratch (no STEP support; the internal representation is a mesh).

```go
p := tsumiki.NewPart("plate")
p.Extrude(tsumiki.NewSketch(tsumiki.XY(0)).Circle(0, 0, 65), 1.5)
p.Extrude(tsumiki.NewSketch(tsumiki.XY(0)).Circle(0, 0, 22.5), 1.5, tsumiki.Cut())
p.Mesh().Save3MF("plate.3mf", "plate")
p.Mesh().SaveSVG("plate.svg", tsumiki.DrawingTitle("plate")) // a drawing to check the shape at a glance
fmt.Print(p.Timeline())
```

## Features

- 2D sketches: polygons, circles, rounded rectangles, line/arc paths (nesting is resolved even-odd, so inner loops become holes)
- Extrude (with taper) and revolve; join, cut and intersect
- 3MF and binary STL export; watertightness and volume checks
- SVG drawings (see below)

## Drawings

`Mesh.WriteSVG` / `Mesh.SaveSVG` draw the part so you can see what the code builds without opening a slicer:
front, top and right views in third-angle projection, an isometric view and any section views you ask for.
Edges where faces meet at more than 30° and the outlines of curved surfaces are drawn; edges hidden behind the
part are dashed in the orthographic views.

```go
p.Mesh().SaveSVG("bracket.svg",
	tsumiki.DrawingTitle("Bracket"),
	tsumiki.DrawingScale(2), // paper mm per model mm (2:1)
	tsumiki.SectionY(0),     // section A-A through y = 0, seen from the front
	tsumiki.SectionZ(2),     // section B-B through z = 2, seen from above
)
```

Dimensions (all in millimeters):

- overall width, depth and height;
- diameters of circles and radii of arcs seen square-on, grouped by size (`2× ⌀4`, `4× R5`), with center lines;
- regular polygons such as hexagonal holes, across flats (`HEX 3.2 A/F`, `□5`);
- the depth of blind holes (`⌀4, depth 2`) and the pitch of repeated holes (`274× HEX 3.2 A/F, pitch 4`);
- positions of circle centers and heights of horizontal edges, as ordinate dimensions measured from the lower
  left corner of each view (a pattern with more than 12 positions along an edge is described by its pitch).

Section views (`SectionX`, `SectionY`, `SectionZ`) remove the part in front of the cutting plane, hatch the cut
faces and mark the plane, lettered A, B, …, on the top view (or, for `SectionZ`, the front view).

Other options: `WithoutHiddenLines`, `WithoutDimensions`, `WithoutDetailDimensions`, `FeatureAngle`.

### PDF (A3, JIS)

`Mesh.WritePDF` / `Mesh.SavePDF` put the same views on an A3 landscape sheet laid out like a JIS part drawing:

- a frame with a 20 mm filing margin, and center marks;
- view labels in Japanese and English;
- a title block holding:
  - the title (`DrawingTitle`), drawing number (`DrawingNumber`), author (`DrawingAuthor`) and date
    (`DrawingDate`);
  - the scale and units;
  - the third-angle projection symbol.

Unless `DrawingScale` is given, the largest JIS standard scale (10:1, 5:1, 2:1, 1:1, 1:2, 1:5, …) at which the views
fit is used.

The PDF has no fonts. Text is drawn as outlines of built-in glyphs (printable ASCII, the drawing symbols and the
Japanese of the labels), derived from Noto Sans CJK JP by `internal/fontgen` (SIL Open Font License, see
`third_party/noto-cjk/LICENSE`). Other characters are an error.

The file holds no timestamps, so the same part always gives the same bytes. Coordinates are written to 0.01 mm,
coarse enough that the output does not depend on the processor architecture.

The SVG is sized in millimeters, so it prints at the chosen scale. A few things to keep in mind:

- Hidden-line removal samples each edge, so features much narrower than about 1/2000 of the view's diagonal
  may hide a line without being noticed.
- Circles are recognized from the facets of the mesh, so sizes are good to the chord tolerance of the sketch
  (0.01 mm by default); a size within that of a multiple of 0.05 mm is shown rounded to it. Curves faceted more
  coarsely than the default tolerance (for example polygons built by hand) are not dimensioned.

## Booleans

Booleans use an exact plane-based BSP. Inputs are snapped to a 1 µm lattice, and polygons are represented by
integer planes plus edge planes so that all side tests are exact. Coplanar and touching cases, and long chains of
operations, produce no cracks. The set-algebra volume identities are checked on randomly generated, coplanar-heavy
shapes (`stress_test.go`).

Limits: coordinates must stay within ±500 mm and are rounded to 1 µm. One boolean on a large mesh takes a few hundred ms.

## Example

`cmd/guard` rebuilds a half-split plate-with-skirt plant guard (364 hex through-holes and 100 recesses) in 13 steps.

```sh
go run ./cmd/guard -models ./out   # writes a 3MF and an STL to ./out
go generate ./cmd/...              # regenerates the drawings next to the code (cmd/guard/prestera90_guard.{pdf,svg})
```

The drawings are committed: the SVG shows changes to the part as an image diff in pull requests, and the PDF is
the A3 sheet. CI regenerates them on amd64 and arm64 and fails when they differ from the committed ones.

## Development

```sh
go test ./...                 # unit tests, including the full guard build (about 30 s)
go test -short ./...          # skips the slow guard build
STRESS_N=400 go test -run TestBooleanRandom -v
golangci-lint run ./...       # lint (golangci-lint v2, see .golangci.yml)
go generate ./cmd/...         # regenerate the example drawings after changing the part or the drawing code
(cd internal/fontgen && go run .)  # regenerate glyphs_data.go (needs Noto Sans CJK, e.g. the fonts-noto-cjk package)
```

Pull requests run the unit tests and golangci-lint through GitHub Actions (`.github/workflows/ci.yml`).

## License

MIT
