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
front, top and right views in third-angle projection, an isometric view, and the overall dimensions in
millimeters. Edges where faces meet at more than 30° and the outlines of curved surfaces are drawn; edges hidden
behind the part are dashed in the three orthographic views.

```go
p.Mesh().SaveSVG("bracket.svg",
	tsumiki.DrawingTitle("Bracket"),
	tsumiki.DrawingScale(2),           // paper mm per model mm (2:1)
	tsumiki.WithoutHiddenLines(),      // optional
	tsumiki.FeatureAngle(15),          // also draw edges between faces meeting at 15° or more
)
```

The SVG is sized in millimeters, so it prints at the chosen scale. Hidden-line removal samples each edge, so
features much narrower than about 1/2000 of the view's diagonal may hide a line without being noticed.

## Booleans

Booleans use an exact plane-based BSP. Inputs are snapped to a 1 µm lattice, and polygons are represented by
integer planes plus edge planes so that all side tests are exact. Coplanar and touching cases, and long chains of
operations, produce no cracks. The set-algebra volume identities are checked on randomly generated, coplanar-heavy
shapes (`stress_test.go`).

Limits: coordinates must stay within ±500 mm and are rounded to 1 µm. One boolean on a large mesh takes a few hundred ms.

## Example

`cmd/guard` rebuilds a half-split plate-with-skirt plant guard (364 hex through-holes and 100 recesses) in 13 steps.

```sh
go run ./cmd/guard ./out   # writes a 3MF, an STL and an SVG drawing to ./out
```

## Development

```sh
go test ./...                 # unit tests, including the full guard build (about 30 s)
go test -short ./...          # skips the slow guard build
STRESS_N=400 go test -run TestBooleanRandom -v
golangci-lint run ./...       # lint (golangci-lint v2, see .golangci.yml)
```

Pull requests run the unit tests and golangci-lint through GitHub Actions (`.github/workflows/ci.yml`).

## License

MIT
