package service

// FileEntry holds metadata for a file found by ListFiles.
type FileEntry struct {
	Path string
	Size int64
}
