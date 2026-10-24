package tsumiki

import (
	"archive/zip"
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// WriteSTL writes a binary STL (millimeters).
func (m *Mesh) WriteSTL(w io.Writer) error {
	bufferedWriter := bufio.NewWriter(w)
	header := make([]byte, 80)
	copy(header, "tsumiki binary STL")
	_, _ = bufferedWriter.Write(header)
	// A bufio.Writer keeps its first error and reports it from Flush, so the individual writes
	// below need no checks of their own.
	_ = binary.Write(bufferedWriter, binary.LittleEndian, uint32(len(m.Triangles)))
	writeFloat32 := func(v float64) { _ = binary.Write(bufferedWriter, binary.LittleEndian, float32(v)) }
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
		_, _ = bufferedWriter.Write([]byte{0, 0}) // attribute byte count
	}
	return bufferedWriter.Flush()
}

// Write3MF writes a minimal 3MF package (one object, unit = millimeter).
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
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`+
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>`+
		`<Default Extension="model" ContentType="application/vnd.ms-package.3dmanufacturing-3dmodel+xml"/>`+
		`</Types>`); err != nil {
		return err
	}
	if err := addFile("_rels/.rels", `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+
		`<Relationship Target="/3D/3dmodel.model" Id="rel0" `+
		`Type="http://schemas.microsoft.com/3dmanufacturing/2013/01/3dmodel"/>`+
		`</Relationships>`); err != nil {
		return err
	}
	f, err := zipWriter.Create("3D/3dmodel.model")
	if err != nil {
		return err
	}
	bufferedWriter := bufio.NewWriter(f)
	_, _ = fmt.Fprint(bufferedWriter, `<?xml version="1.0" encoding="UTF-8"?>
<model unit="millimeter" xml:lang="en-US" xmlns="http://schemas.microsoft.com/3dmanufacturing/core/2015/02">
<resources><object id="1" type="model" name="`+name+`"><mesh><vertices>
`)
	for _, p := range m.Vertices {
		_, _ = fmt.Fprintf(bufferedWriter, "<vertex x=\"%.6f\" y=\"%.6f\" z=\"%.6f\"/>\n", p.X, p.Y, p.Z)
	}
	_, _ = fmt.Fprint(bufferedWriter, "</vertices><triangles>\n")
	for _, t := range m.Triangles {
		_, _ = fmt.Fprintf(bufferedWriter, "<triangle v1=\"%d\" v2=\"%d\" v3=\"%d\"/>\n", t[0], t[1], t[2])
	}
	_, _ = fmt.Fprint(bufferedWriter,
		"</triangles></mesh></object></resources>\n<build><item objectid=\"1\"/></build>\n</model>\n")
	if err := bufferedWriter.Flush(); err != nil {
		return err
	}
	return zipWriter.Close()
}

// SaveSTL writes the mesh to path as a binary STL.
func (m *Mesh) SaveSTL(path string) error {
	return saveFile(path, m.WriteSTL)
}

// Save3MF writes the mesh to path as a 3MF package whose single object is called name.
func (m *Mesh) Save3MF(path, name string) error {
	return saveFile(path, func(w io.Writer) error { return m.Write3MF(w, name) })
}

// saveFile creates path, fills it with write and reports the first error, including the one
// from closing the file (a failed close can mean the data never reached the disk).
func saveFile(path string, write func(io.Writer) error) (err error) {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}()
	return write(file)
}
