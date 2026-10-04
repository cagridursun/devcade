package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"time"
)

// entry is one file in a release archive.
type entry struct {
	Name string
	Mode int64 // permission bits
	Data []byte
}

// archiveEntries returns the files of one target's archive, sorted by name:
// the binary (0755), the license notice and the README (0644).
func archiveEntries(t target, bin, readme, notice []byte) []entry {
	es := []entry{
		{Name: t.binaryName(), Mode: 0o755, Data: bin},
		{Name: noticeName, Mode: 0o644, Data: notice},
		{Name: readmeName, Mode: 0o644, Data: readme},
	}
	sort.Slice(es, func(i, j int) bool { return es[i].Name < es[j].Name })
	return es
}

// packageArchive builds the archive bytes for one target.
func packageArchive(t target, bin, readme, notice []byte, mtime time.Time) ([]byte, error) {
	var buf bytes.Buffer
	es := archiveEntries(t, bin, readme, notice)
	var err error
	if t.OS == "windows" {
		err = writeZip(&buf, es, mtime)
	} else {
		err = writeTarGz(&buf, es, mtime)
	}
	return buf.Bytes(), err
}

// writeTarGz writes a deterministic gzip-compressed ustar archive: entries
// in the given order, a fixed mtime, uid/gid 0 and no user/group names, and
// a gzip header with no name and no timestamp.
func writeTarGz(w io.Writer, es []entry, mtime time.Time) error {
	gz, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		return err
	}
	tw := tar.NewWriter(gz)
	for _, e := range es {
		hdr := &tar.Header{
			Typeflag: tar.TypeReg,
			Name:     e.Name,
			Mode:     e.Mode,
			Size:     int64(len(e.Data)),
			ModTime:  mtime.UTC().Truncate(time.Second),
			Uid:      0,
			Gid:      0,
			Format:   tar.FormatUSTAR,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("tar %s: %w", e.Name, err)
		}
		if _, err := tw.Write(e.Data); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

// writeZip writes a deterministic zip archive: entries in the given order,
// deflate at a fixed level, a fixed UTC mtime and Unix permission bits.
func writeZip(w io.Writer, es []entry, mtime time.Time) error {
	zw := zip.NewWriter(w)
	zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.BestCompression)
	})
	for _, e := range es {
		hdr := &zip.FileHeader{
			Name:     e.Name,
			Method:   zip.Deflate,
			Modified: mtime.UTC().Truncate(2 * time.Second),
		}
		hdr.SetMode(fs.FileMode(e.Mode))
		f, err := zw.CreateHeader(hdr)
		if err != nil {
			return fmt.Errorf("zip %s: %w", e.Name, err)
		}
		if _, err := f.Write(e.Data); err != nil {
			return err
		}
	}
	return zw.Close()
}
