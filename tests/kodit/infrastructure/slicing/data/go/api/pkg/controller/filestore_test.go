package controller

import "testing"

func TestGetFileStore(t *testing.T) {
	fs := GetFileStore()
	if fs == nil {
		t.Error("FileStore is nil")
	}
}
