package rasterization

import (
	"archive/zip"
	"fmt"
	"image"
	// Register standard image decoders.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"path/filepath"
	"sort"
	"strings"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
)

// mediaPrefixes are the ZIP directory prefixes where Office Open XML
// formats store embedded media files.
var mediaPrefixes = []string{
	"word/media/",
	"ppt/media/",
	"xl/media/",
}

// supportedImageExts lists extensions that Go's image package can decode
// (with the registered imports above).
var supportedImageExts = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".bmp":  true,
	".tiff": true,
	".tif":  true,
}

// OfficeImageExtractor extracts embedded images from Office Open XML
// documents (docx, pptx, xlsx). These formats are ZIP archives with
// media files at predictable paths.
type OfficeImageExtractor struct{}

// NewOfficeImageExtractor creates an OfficeImageExtractor.
func NewOfficeImageExtractor() *OfficeImageExtractor {
	return &OfficeImageExtractor{}
}

// PageCount returns the number of extractable images in the document.
func (o *OfficeImageExtractor) PageCount(path string) (int, error) {
	names, err := mediaImageNames(path)
	if err != nil {
		return 0, err
	}
	return len(names), nil
}

// Render returns the Nth (1-based) embedded image as an image.Image.
func (o *OfficeImageExtractor) Render(path string, page int) (image.Image, error) {
	names, err := mediaImageNames(path)
	if err != nil {
		return nil, err
	}

	if page < 1 || page > len(names) {
		return nil, fmt.Errorf("page %d out of range [1, %d]", page, len(names))
	}

	target := names[page-1]

	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open zip %s: %w", path, err)
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name != target {
			continue
		}
		rc, openErr := f.Open()
		if openErr != nil {
			return nil, fmt.Errorf("open %s: %w", f.Name, openErr)
		}
		defer rc.Close()

		img, _, decodeErr := image.Decode(rc)
		if decodeErr != nil {
			return nil, fmt.Errorf("decode %s: %w", f.Name, decodeErr)
		}
		return img, nil
	}

	return nil, fmt.Errorf("entry %s not found in %s", target, path)
}

// Close is a no-op — OfficeImageExtractor holds no persistent state.
func (o *OfficeImageExtractor) Close() error { return nil }

// mediaImageNames opens the ZIP archive and returns the sorted names
// of image entries found under Office media directories.
func mediaImageNames(path string) ([]string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open zip %s: %w", path, err)
	}
	defer r.Close()

	var names []string
	for _, f := range r.File {
		if isMediaImage(f.Name) {
			names = append(names, f.Name)
		}
	}

	sort.Strings(names)
	return names, nil
}

// isMediaImage returns true if the ZIP entry path is an image file
// inside one of the standard Office media directories.
func isMediaImage(name string) bool {
	lower := strings.ToLower(name)
	for _, prefix := range mediaPrefixes {
		if strings.HasPrefix(lower, prefix) {
			ext := strings.ToLower(filepath.Ext(name))
			return supportedImageExts[ext]
		}
	}
	return false
}
