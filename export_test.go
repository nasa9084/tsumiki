package tsumiki

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteSTLLayout(t *testing.T) {
	mesh := box(0, 0, 0, 2, 3, 4).Mesh()
	var buffer bytes.Buffer
	if err := mesh.WriteSTL(&buffer); err != nil {
		t.Fatal(err)
	}
	data := buffer.Bytes()
	if !strings.HasPrefix(string(data[:80]), "tsumiki binary STL") {
		t.Errorf("header = %q", data[:20])
	}
	if count := binary.LittleEndian.Uint32(data[80:84]); int(count) != len(mesh.Triangles) {
		t.Errorf("facet count %d, want %d", count, len(mesh.Triangles))
	}
	if want := 84 + 50*len(mesh.Triangles); len(data) != want {
		t.Errorf("size %d, want %d", len(data), want)
	}
}

func TestWriteSTLNormalsPointOutward(t *testing.T) {
	mesh := box(0, 0, 0, 2, 3, 4).Mesh()
	var buffer bytes.Buffer
	if err := mesh.WriteSTL(&buffer); err != nil {
		t.Fatal(err)
	}
	data := buffer.Bytes()
	center := Vector3{1, 1.5, 2}
	float := func(offset int) float64 {
		return float64(math.Float32frombits(binary.LittleEndian.Uint32(data[offset:])))
	}
	for facet := 0; facet < len(mesh.Triangles); facet++ {
		start := 84 + facet*50
		normal := Vector3{float(start), float(start + 4), float(start + 8)}
		vertex := Vector3{float(start + 12), float(start + 16), float(start + 20)}
		near(t, "unit normal", normal.Length(), 1, 1e-6)
		if normal.Dot(vertex.Sub(center)) <= 0 {
			t.Errorf("facet %d: normal %v points inward", facet, normal)
		}
	}
}

func TestWrite3MFPackage(t *testing.T) {
	var buffer bytes.Buffer
	if err := box(0, 0, 0, 1, 1, 1).Mesh().Write3MF(&buffer, "cube"); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, file := range archive.File {
		names[file.Name] = true
	}
	for _, required := range []string{"[Content_Types].xml", "_rels/.rels", "3D/3dmodel.model"} {
		if !names[required] {
			t.Errorf("package is missing %s (has %v)", required, names)
		}
	}
}

func TestSaveWritesFiles(t *testing.T) {
	directory := t.TempDir()
	mesh := box(0, 0, 0, 1, 1, 1).Mesh()
	stlPath, threeMFPath := filepath.Join(directory, "cube.stl"), filepath.Join(directory, "cube.3mf")
	if err := mesh.SaveSTL(stlPath); err != nil {
		t.Fatal(err)
	}
	if err := mesh.Save3MF(threeMFPath, "cube"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{stlPath, threeMFPath} {
		if info, err := os.Stat(path); err != nil || info.Size() == 0 {
			t.Errorf("%s: %v, %v", path, info, err)
		}
	}
}

func TestSaveReportsUnwritablePath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-directory", "cube.stl")
	mesh := box(0, 0, 0, 1, 1, 1).Mesh()
	if err := mesh.SaveSTL(missing); err == nil {
		t.Error("SaveSTL into a missing directory should fail")
	}
	if err := mesh.Save3MF(missing, "cube"); err == nil {
		t.Error("Save3MF into a missing directory should fail")
	}
}
