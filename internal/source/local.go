package source

import (
	"context"
	"fmt"
	"os"
)

func init() { Register("file", localAdapter{}) }

// localAdapter serves directories on the local file system (including
// anything the OS has mounted).
type localAdapter struct{}

func (localAdapter) Open(_ context.Context, s Spec) (*Conn, error) {
	fi, err := os.Stat(s.Path)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", s.Path)
	}
	return NewConn(os.DirFS(s.Path), nil), nil
}
