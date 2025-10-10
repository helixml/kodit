package controller

type FileStore struct {
}

// File structure
type File struct {
	Path string
}

type FileList []*File

func GetFileStore() *FileStore {
	return &FileStore{}
}

// GetFile returns a file by path
func (fs *FileStore) GetFile(path string) *File {
	return &File{Path: path}
}

func (fs *FileStore) GetFileList(filter string) ([]*File, error) {
	return []*File{}, nil
}
