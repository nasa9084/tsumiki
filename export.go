package tsumiki

import (
	"archive/zip"
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// WriteSTL writes a binary STL (millimetres).
func (m *Mesh) WriteSTL(w io.Writer) error {
	bufferedWriter := bufio.NewWriter(w)
	header := make([]byte, 80)
	copy(header, "tsumiki binary STL")
	bufferedWriter.Write(header)
	binary.Write(bufferedWriter, binary.LittleEndian, uint32(len(m.Triangles)))
	writeFloat32 := func(v float64) { binary.Write(bufferedWriter, binary.LittleEndian, float32(v)) }
	for _, t := range m.Triangles {
		a, b, c := m.Vertices[t[0]], m.Vertices[t[1]], m.Vertices[t[2]]
		n := b.Sub(a).Cross(c.Sub(a)).Normalize()
		writeFloat32(n.X)
		writeFloat32(n.Y)
		writeFloat32(n.Z)
		for _, p := range []Vector3{a, b, c} {
			writeFloat32(p.X)
			writeFloat32(p.Y)
			writeFloat32(p.Z)
		}
		bufferedWriter.Write([]byte{0, 0})
	}
	return bufferedWriter.Flush()
}

// Write3MF writes a minimal 3MF package (one object, unit = millimetre).
func (m *Mesh) Write3MF(w io.Writer, name string) error {
	zipWriter := zip.NewWriter(w)
	addFile := func(path, body string) error {
		f, err := zipWriter.Create(path)
		if err != nil {
			return err
		}
		_, err = io.WriteString(f, body)
		return err
	}
	if err := addFile("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="model" ContentType="application/vnd.ms-package.3dmanufacturing-3dmodel+xml"/></Types>`); err != nil {
		return err
	}
	if err := addFile("_rels/.rels", `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Target="/3D/3dmodel.model" Id="rel0" Type="http://schemas.microsoft.com/3dmanufacturing/2013/01/3dmodel"/></Relationships>`); err != nil {
		return err
	}
	f, err := zipWriter.Create("3D/3dmodel.model")
	if err != nil {
		return err
	}
	bufferedWriter := bufio.NewWriter(f)
	fmt.Fprint(bufferedWriter, `<?xml version="1.0" encoding="UTF-8"?>
<model unit="millimeter" xml:lang="en-US" xmlns="http://schemas.microsoft.com/3dmanufacturing/core/2015/02">
<resources><object id="1" type="model" name="`+name+`"><mesh><vertices>
`)
	for _, p := range m.Vertices {
		fmt.Fprintf(bufferedWriter, "<vertex x=\"%.6f\" y=\"%.6f\" z=\"%.6f\"/>\n", p.X, p.Y, p.Z)
	}
	fmt.Fprint(bufferedWriter, "</vertices><triangles>\n")
	for _, t := range m.Triangles {
		fmt.Fprintf(bufferedWriter, "<triangle v1=\"%d\" v2=\"%d\" v3=\"%d\"/>\n", t[0], t[1], t[2])
	}
	fmt.Fprint(bufferedWriter, "</triangles></mesh></object></resources>\n<build><item objectid=\"1\"/></build>\n</model>\n")
	if err := bufferedWriter.Flush(); err != nil {
		return err
	}
	return zipWriter.Close()
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
