package tsumiki

import (
	"archive/zip"
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
)

// WriteSTL writes a binary STL (millimetres).
func (m *Mesh) WriteSTL(w io.Writer) error {
	bw := bufio.NewWriter(w)
	hdr := make([]byte, 80)
	copy(hdr, "gocad binary STL")
	bw.Write(hdr)
	binary.Write(bw, binary.LittleEndian, uint32(len(m.T)))
	f32 := func(v float64) { binary.Write(bw, binary.LittleEndian, float32(v)) }
	for _, t := range m.T {
		a, b, c := m.V[t[0]], m.V[t[1]], m.V[t[2]]
		n := b.Sub(a).Cross(c.Sub(a)).Norm()
		f32(n.X)
		f32(n.Y)
		f32(n.Z)
		for _, p := range []V3{a, b, c} {
			f32(p.X)
			f32(p.Y)
			f32(p.Z)
		}
		bw.Write([]byte{0, 0})
	}
	return bw.Flush()
}

// Write3MF writes a minimal 3MF package (one object, unit = millimetre).
func (m *Mesh) Write3MF(w io.Writer, name string) error {
	zw := zip.NewWriter(w)
	add := func(path, body string) error {
		f, err := zw.Create(path)
		if err != nil {
			return err
		}
		_, err = io.WriteString(f, body)
		return err
	}
	if err := add("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="model" ContentType="application/vnd.ms-package.3dmanufacturing-3dmodel+xml"/></Types>`); err != nil {
		return err
	}
	if err := add("_rels/.rels", `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Target="/3D/3dmodel.model" Id="rel0" Type="http://schemas.microsoft.com/3dmanufacturing/2013/01/3dmodel"/></Relationships>`); err != nil {
		return err
	}
	f, err := zw.Create("3D/3dmodel.model")
	if err != nil {
		return err
	}
	bw := bufio.NewWriter(f)
	fmt.Fprint(bw, `<?xml version="1.0" encoding="UTF-8"?>
<model unit="millimeter" xml:lang="en-US" xmlns="http://schemas.microsoft.com/3dmanufacturing/core/2015/02">
<resources><object id="1" type="model" name="`+name+`"><mesh><vertices>
`)
	for _, p := range m.V {
		fmt.Fprintf(bw, "<vertex x=\"%.6f\" y=\"%.6f\" z=\"%.6f\"/>\n", p.X, p.Y, p.Z)
	}
	fmt.Fprint(bw, "</vertices><triangles>\n")
	for _, t := range m.T {
		fmt.Fprintf(bw, "<triangle v1=\"%d\" v2=\"%d\" v3=\"%d\"/>\n", t[0], t[1], t[2])
	}
	fmt.Fprint(bw, "</triangles></mesh></object></resources>\n<build><item objectid=\"1\"/></build>\n</model>\n")
	if err := bw.Flush(); err != nil {
		return err
	}
	return zw.Close()
}

// SaveSTL / Save3MF are file helpers.
func (m *Mesh) SaveSTL(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return m.WriteSTL(f)
}

func (m *Mesh) Save3MF(path, name string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return m.Write3MF(f, name)
}

var _ = math.Pi
