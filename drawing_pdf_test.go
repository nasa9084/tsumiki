package tsumiki_test

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/nasa9084/tsumiki"
)

func pdfOf(t *testing.T, part *tsumiki.Part, options ...tsumiki.DrawingOption) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := part.Mesh().WritePDF(&buffer, options...); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// pdfScale reads the scale the drawing was made at from the document's subject.
func pdfScale(t *testing.T, document []byte) string {
	t.Helper()
	match := regexp.MustCompile(`/Subject \(A3 part drawing, scale ([0-9.:]+),`).FindSubmatch(document)
	if match == nil {
		t.Fatal("no scale in the document information")
	}
	return string(match[1])
}

func TestPDFIsAWellFormedA3Page(t *testing.T) {
	document := pdfOf(t, plateWithHoles(), tsumiki.DrawingTitle("plate"))
	if !bytes.HasPrefix(document, []byte("%PDF-1.4")) || !bytes.HasSuffix(document, []byte("%%EOF\n")) {
		t.Fatal("not a PDF file")
	}
	if !bytes.Contains(document, []byte("/MediaBox [0 0 1190.5512 841.8898]")) {
		t.Error("the page is not A3 landscape (420 × 297 mm)")
	}
	// startxref must point at the cross-reference table, whose entries point at the objects
	match := regexp.MustCompile(`startxref\n(\d+)\n`).FindSubmatch(document)
	if match == nil {
		t.Fatal("no startxref")
	}
	offset, _ := strconv.Atoi(string(match[1]))
	if !bytes.HasPrefix(document[offset:], []byte("xref\n")) {
		t.Fatal("startxref does not point at the cross-reference table")
	}
	entries := regexp.MustCompile(`(\d{10}) 00000 n `).FindAllSubmatch(document[offset:], -1)
	for i, entry := range entries {
		at, _ := strconv.Atoi(string(entry[1]))
		if !bytes.HasPrefix(document[at:], []byte(strconv.Itoa(i+1)+" 0 obj")) {
			t.Errorf("cross-reference entry %d does not point at its object", i+1)
		}
	}
}

func TestPDFIsReproducible(t *testing.T) {
	first := pdfOf(t, pocketedBlock(), tsumiki.SectionY(5), tsumiki.DrawingDate("2026-11-01"))
	second := pdfOf(t, pocketedBlock(), tsumiki.SectionY(5), tsumiki.DrawingDate("2026-11-01"))
	if !bytes.Equal(first, second) {
		t.Error("the same part must give the same bytes")
	}
	if bytes.Contains(first, []byte("CreationDate")) || bytes.Contains(first, []byte("/ID")) {
		t.Error("timestamps and random identifiers would make every file different")
	}
}

func TestPDFPicksTheLargestStandardScaleThatFits(t *testing.T) {
	tests := []struct {
		name string
		part *tsumiki.Part
		want string
	}{
		{"small block", box(0, 0, 0, 20, 10, 5), "5:1"},
		{"hand-sized plate", plateWithHoles(), "2:1"},
		{"large plate", box(0, 0, 0, 200, 120, 10), "1:2"},
	}
	for _, test := range tests {
		if got := pdfScale(t, pdfOf(t, test.part)); got != test.want {
			t.Errorf("%s: scale %s, want %s", test.name, got, test.want)
		}
	}
}

func TestPDFKeepsAChosenScale(t *testing.T) {
	if got := pdfScale(t, pdfOf(t, box(0, 0, 0, 20, 10, 5), tsumiki.DrawingScale(1))); got != "1:1" {
		t.Errorf("scale %s, want the chosen 1:1", got)
	}
	var buffer bytes.Buffer
	err := box(0, 0, 0, 300, 200, 10).Mesh().WritePDF(&buffer, tsumiki.DrawingScale(1))
	if err == nil || !strings.Contains(err.Error(), "does not fit") {
		t.Errorf("a part too large for A3 at the chosen scale must be reported, got %v", err)
	}
}

func TestPDFRejectsTextItCannotDraw(t *testing.T) {
	var buffer bytes.Buffer
	err := box(0, 0, 0, 10, 10, 10).Mesh().WritePDF(&buffer, tsumiki.DrawingTitle("ガード"))
	if err == nil || !strings.Contains(err.Error(), "no glyph") {
		t.Errorf("characters without a glyph must be reported, got %v", err)
	}
}

func TestSavePDF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cube.pdf")
	if err := box(0, 0, 0, 10, 10, 10).Mesh().SavePDF(path); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() == 0 {
		t.Errorf("%s: %v, %v", path, info, err)
	}
	if err := box(0, 0, 0, 10, 10, 10).Mesh().SavePDF(filepath.Join(t.TempDir(), "missing", "cube.pdf")); err == nil {
		t.Error("SavePDF into a missing directory should fail")
	}
}
