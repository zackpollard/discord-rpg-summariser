package diarize

import (
	"archive/tar"
	"compress/bzip2"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// extractTarBz2 extracts a tar.bz2 archive to the destination directory.
func extractTarBz2(r io.Reader, destDir string) error {
	bzReader := bzip2.NewReader(r)
	tarReader := tar.NewReader(bzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}

		// Sanitize path to prevent directory traversal.
		name := filepath.Clean(header.Name)
		if strings.Contains(name, "..") {
			continue
		}
		target := filepath.Join(destDir, name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("create dir %s: %w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.Create(target)
			if err != nil {
				return fmt.Errorf("create file %s: %w", target, err)
			}
			if _, err := io.Copy(f, tarReader); err != nil {
				f.Close()
				return fmt.Errorf("write file %s: %w", target, err)
			}
			f.Close()
		}
	}
	return nil
}
