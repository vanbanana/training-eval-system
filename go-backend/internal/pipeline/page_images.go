package pipeline

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/smartedu/training-eval-system/internal/model"
	"github.com/smartedu/training-eval-system/internal/vision"
)

// pageDir returns the cache directory for an upload's rendered page images.
func (o *Orchestrator) pageDir(uploadID int64) string {
	return filepath.Join(o.pageImageDir, fmt.Sprintf("%d", uploadID))
}

// savePageImages persists rendered document pages to disk so the original
// document can be shown faithfully (图文原貌) in the report viewer.
func (o *Orchestrator) savePageImages(uploadID int64, pages []vision.PageImage) error {
	if o.pageImageDir == "" || len(pages) == 0 {
		return nil
	}
	dir := o.pageDir(uploadID)
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for i, p := range pages {
		data, ext, err := decodeDataURI(p.DataURI)
		if err != nil {
			continue
		}
		name := fmt.Sprintf("p%d%s", i+1, ext)
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// decodeDataURI splits a "data:<mime>;base64,<data>" URI into raw bytes and a
// file extension.
func decodeDataURI(uri string) ([]byte, string, error) {
	const marker = "base64,"
	idx := strings.Index(uri, marker)
	if idx < 0 {
		return nil, "", fmt.Errorf("invalid data uri")
	}
	meta := uri[:idx]
	data, err := base64.StdEncoding.DecodeString(uri[idx+len(marker):])
	if err != nil {
		return nil, "", err
	}
	ext := ".png"
	if strings.Contains(meta, "jpeg") || strings.Contains(meta, "jpg") {
		ext = ".jpg"
	}
	return data, ext, nil
}

// EnsurePageImages returns the number of cached page images for an upload,
// rendering them on demand from the original file when the cache is empty.
func (o *Orchestrator) EnsurePageImages(ctx context.Context, upload *model.Upload) (int, error) {
	if o.pageImageDir == "" {
		return 0, fmt.Errorf("page images not configured")
	}
	dir := o.pageDir(upload.ID)
	if n := countPageFiles(dir); n > 0 {
		return n, nil
	}
	if _, err := os.Stat(upload.StoragePath); err != nil {
		return 0, fmt.Errorf("original file unavailable")
	}
	pages, err := vision.ConvertFile(upload.StoragePath, "."+upload.FileType)
	if err != nil {
		return 0, err
	}
	if err := o.savePageImages(upload.ID, pages); err != nil {
		return 0, err
	}
	return countPageFiles(dir), nil
}

// PageImageFile resolves the path and content type of a cached page image.
func (o *Orchestrator) PageImageFile(uploadID int64, page int) (string, string, error) {
	dir := o.pageDir(uploadID)
	for _, ext := range []string{".png", ".jpg", ".jpeg"} {
		p := filepath.Join(dir, fmt.Sprintf("p%d%s", page, ext))
		if _, err := os.Stat(p); err == nil {
			ct := "image/png"
			if ext != ".png" {
				ct = "image/jpeg"
			}
			return p, ct, nil
		}
	}
	return "", "", fmt.Errorf("page not found")
}

func countPageFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "p") {
			n++
		}
	}
	return n
}
