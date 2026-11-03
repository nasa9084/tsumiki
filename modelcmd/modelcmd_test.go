package modelcmd_test

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nasa9084/tsumiki"
	"github.com/nasa9084/tsumiki/modelcmd"
)

func cube() modelcmd.Model {
	return modelcmd.Model{
		Name: "cube",
		Build: func(log io.Writer) (*tsumiki.Part, error) {
			_, _ = io.WriteString(log, "building the cube\n")
			return tsumiki.NewPart("cube").Extrude(tsumiki.NewSketch(tsumiki.XY(0)).Rect(0, 0, 10, 10), 10), nil
		},
		Drawing: []tsumiki.DrawingOption{tsumiki.DrawingTitle("Cube")},
	}
}

func run(t *testing.T, m modelcmd.Model, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := modelcmd.Run(m, args, &out)
	return out.String(), err
}

func requireFiles(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, name := range names {
		if info, err := os.Stat(filepath.Join(dir, name)); err != nil || info.Size() == 0 {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestRunReportsWithoutSaving(t *testing.T) {
	out, err := run(t, cube())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"building the cube", "extrude 10", "volume  1000.00 mm³"} {
		if !strings.Contains(out, want) {
			t.Errorf("the report lacks %q:\n%s", want, out)
		}
	}
}

func TestRunSavesModelsAndDrawings(t *testing.T) {
	models, drawings := t.TempDir(), t.TempDir()
	if _, err := run(t, cube(), "-models", models, "-drawings", drawings); err != nil {
		t.Fatal(err)
	}
	requireFiles(t, models, "cube.3mf", "cube.stl")
	requireFiles(t, drawings, "cube.svg", "cube.pdf")
	if entries, _ := os.ReadDir(models); len(entries) != 2 {
		t.Errorf("only the models go to the models directory, got %d files", len(entries))
	}
}

func TestRunQuiet(t *testing.T) {
	out, err := run(t, cube(), "-quiet")
	if err != nil || out != "" {
		t.Errorf("a quiet run prints nothing, got %q (%v)", out, err)
	}
}

func TestRunPassesOnBuildErrors(t *testing.T) {
	broken := cube()
	broken.Build = func(io.Writer) (*tsumiki.Part, error) { return nil, errors.New("no such hole") }
	if _, err := run(t, broken); err == nil || !strings.Contains(err.Error(), "cube: no such hole") {
		t.Errorf("got %v", err)
	}
	failing := cube()
	failing.Build = func(io.Writer) (*tsumiki.Part, error) {
		return tsumiki.NewPart("empty").Extrude(tsumiki.NewSketch(tsumiki.XY(0)), 1), nil
	}
	if _, err := run(t, failing); err == nil || !strings.Contains(err.Error(), "no closed profile") {
		t.Errorf("a feature's error must be reported, got %v", err)
	}
}

func TestRunRefusesAnOpenSurface(t *testing.T) {
	open := cube()
	open.Build = func(io.Writer) (*tsumiki.Part, error) {
		part := tsumiki.NewPart("open").Extrude(tsumiki.NewSketch(tsumiki.XY(0)).Rect(0, 0, 10, 10), 10)
		mesh := part.Mesh()
		mesh.Triangles = mesh.Triangles[1:] // a hole in the surface
		return part, nil
	}
	dir := t.TempDir()
	if _, err := run(t, open, "-models", dir); err == nil || !strings.Contains(err.Error(), "not watertight") {
		t.Errorf("got %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Error("nothing is saved for a part that cannot be printed")
	}
}

func TestRunRejectsBadCommandLines(t *testing.T) {
	if _, err := run(t, cube(), "-no-such-flag"); err == nil {
		t.Error("an unknown flag must be an error")
	}
	if _, err := run(t, cube(), "stray"); err == nil {
		t.Error("a stray argument must be an error")
	}
	if _, err := run(t, cube(), "-h"); !errors.Is(err, flag.ErrHelp) {
		t.Errorf("-h asks for help, got %v", err)
	}
	if _, err := run(t, modelcmd.Model{Name: "nothing"}); err == nil {
		t.Error("a model without a Build function must be an error")
	}
}
