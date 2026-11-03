// Package modelcmd turns a tsumiki model into a command: it builds the part, reports on it and
// saves its models (3MF, STL) and drawings (SVG, PDF) where the flags say.
//
// A model's main package only describes the part:
//
//	//go:generate go run . -drawings .
//
//	func main() {
//		modelcmd.Main(modelcmd.Model{
//			Name:    "bracket",
//			Build:   build, // func(log io.Writer) (*tsumiki.Part, error)
//			Drawing: []tsumiki.DrawingOption{tsumiki.DrawingTitle("Bracket")},
//		})
//	}
//
// Flags:
//
//	-models dir    write NAME.3mf and NAME.stl to dir
//	-drawings dir  write NAME.svg and NAME.pdf to dir
//	-quiet         print nothing but errors
package modelcmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/nasa9084/tsumiki"
)

// Model is a part that a command builds and saves.
type Model struct {
	// Name is the base name of the files written (NAME.3mf, NAME.pdf, …) and the object name in
	// the 3MF file.
	Name string
	// Build makes the part. It may log its progress to log.
	Build func(log io.Writer) (*tsumiki.Part, error)
	// Drawing configures the SVG and PDF drawings (title, number, sections, …). Keep it free of
	// anything that changes from run to run, such as today's date, so that committed drawings can
	// be checked for being up to date.
	Drawing []tsumiki.DrawingOption
}

// Main runs the command for the model with the program's arguments and exits with status 1 on
// failure, or 2 on a bad command line.
func Main(m Model) {
	err := Run(m, os.Args[1:], os.Stdout)
	switch {
	case err == nil:
	case errors.Is(err, flag.ErrHelp):
		os.Exit(0)
	case errors.Is(err, errUsage):
		os.Exit(2)
	default:
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var errUsage = errors.New("modelcmd: bad command line")

// Run builds the model and saves what args ask for, reporting to stdout.
func Run(m Model, args []string, stdout io.Writer) error {
	if m.Name == "" || m.Build == nil {
		return errors.New("modelcmd: a model needs a Name and a Build function")
	}
	flags := flag.NewFlagSet(m.Name, flag.ContinueOnError)
	flags.SetOutput(stdout)
	models := flags.String("models", "", "write `dir`/"+m.Name+".3mf and .stl")
	drawings := flags.String("drawings", "", "write `dir`/"+m.Name+".svg and .pdf")
	quiet := flags.Bool("quiet", false, "print nothing but errors")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return fmt.Errorf("%w: %w", errUsage, err)
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("%w: unexpected arguments %q", errUsage, flags.Args())
	}
	log := stdout
	if *quiet {
		log = io.Discard
	}
	part, err := m.Build(log)
	if err != nil {
		return fmt.Errorf("%s: %w", m.Name, err)
	}
	if err := report(log, m.Name, part); err != nil {
		return err
	}
	if *models != "" {
		if err := saveModels(log, m, part.Mesh(), *models); err != nil {
			return err
		}
	}
	if *drawings != "" {
		if err := saveDrawings(log, m, part.Mesh(), *drawings); err != nil {
			return err
		}
	}
	return nil
}

// report prints the part's timeline and health, and refuses a part that is not a closed solid:
// a slicer cannot print it.
func report(log io.Writer, name string, part *tsumiki.Part) error {
	if part.Err() != nil {
		return fmt.Errorf("%s: %w", name, part.Err())
	}
	if part.Mesh() == nil {
		return fmt.Errorf("%s: the part is empty", name)
	}
	r := part.Report()
	lo, hi := part.Mesh().Bounds()
	_, _ = fmt.Fprintf(log, "\n%s", part.Timeline())
	_, _ = fmt.Fprintf(log, "\nbounds  x %.3f..%.3f  y %.3f..%.3f  z %.3f..%.3f\n", lo.X, hi.X, lo.Y, hi.Y, lo.Z, hi.Z)
	_, _ = fmt.Fprintf(log, "volume  %.2f mm³  (%d triangles, %d vertices)\n", r.Volume, r.Triangles, r.Vertices)
	if !r.Watertight() {
		return fmt.Errorf("%s: the part is not watertight (%d open edges, %d non-manifold edges, %d degenerate triangles)",
			name, r.OpenEdges, r.NonManifold, r.Degenerate)
	}
	return nil
}

func saveModels(log io.Writer, m Model, mesh *tsumiki.Mesh, dir string) error {
	if err := mesh.Save3MF(filepath.Join(dir, m.Name+".3mf"), m.Name); err != nil {
		return err
	}
	if err := mesh.SaveSTL(filepath.Join(dir, m.Name+".stl")); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(log, "models saved to", dir)
	return nil
}

func saveDrawings(log io.Writer, m Model, mesh *tsumiki.Mesh, dir string) error {
	if err := mesh.SaveSVG(filepath.Join(dir, m.Name+".svg"), m.Drawing...); err != nil {
		return err
	}
	if err := mesh.SavePDF(filepath.Join(dir, m.Name+".pdf"), m.Drawing...); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(log, "drawings saved to", dir)
	return nil
}
