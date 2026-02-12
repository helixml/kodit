package service

import "errors"

// ErrClientClosed indicates the client has been closed.
var ErrClientClosed = errors.New("kodit: client is closed")
