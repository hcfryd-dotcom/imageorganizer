package source

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func init() { Register("file", localAdapter{}) }

// localAdapter serves directories on the local file system (including
// anything the OS has mounted).
type localAdapter struct{}

func (localAdapter) Open(_ context.Context, s Spec) (*Conn, error) {
	p := nativePath(s.Path)
	fi, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", p)
	}
	return NewConn(os.DirFS(p), nil), nil
}

// ListRoots lists the drives on Windows, where "/" is not a directory.
// Elsewhere "/" is a directory and ordinary browsing applies.
func (localAdapter) ListRoots(_ context.Context, _ Spec) ([]string, error) {
	if runtime.GOOS != "windows" {
		return nil, errNoRoots
	}
	var drives []string
	for c := 'A'; c <= 'Z'; c++ {
		d := string(c) + ":"
		if fi, err := os.Stat(d + `\`); err == nil && fi.IsDir() {
			drives = append(drives, d)
		}
	}
	return drives, nil
}

// errNoRoots tells Browse to fall back to listing "/" as a directory.
var errNoRoots = fmt.Errorf("no roots")

// isDrivePath reports whether p (slash form) is "/C:" or "/C:/...": the
// internal form of a Windows drive path.
func isDrivePath(p string) bool {
	return len(p) >= 3 && p[0] == '/' && p[2] == ':' && isDriveLetter(p[1]) &&
		(len(p) == 3 || p[3] == '/')
}

func isDriveLetter(c byte) bool { return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' }

// nativePath turns the internal slash form into what the OS opens:
// "/C:/Users/x" becomes `C:\Users\x`, and "/C:" becomes `C:\` (a bare
// "C:" would mean the current directory on that drive).
func nativePath(p string) string {
	if !isDrivePath(p) {
		return p
	}
	p = p[1:]
	if len(p) == 2 {
		p += "/"
	}
	return filepath.FromSlash(p)
}

// parseWindowsPath recognises `C:\Photos`, `C:/Photos` and `C:` and gives
// the internal form "/C:/Photos". ok is false for anything else.
func parseWindowsPath(loc string) (string, bool) {
	if len(loc) < 2 || loc[1] != ':' || !isDriveLetter(loc[0]) {
		return "", false
	}
	if len(loc) > 2 && loc[2] != '\\' && loc[2] != '/' {
		return "", false
	}
	rest := strings.ReplaceAll(loc[2:], `\`, "/")
	drive := "/" + strings.ToUpper(loc[:1]) + ":"
	if r := cleanPath(rest); r != "/" {
		return drive + r, true
	}
	return drive, true
}

// LocalPath is the path on this computer of rel inside a local folder, or
// ok false for folders of other kinds (a network share reached over SMB).
func LocalPath(s Spec, rel string) (p string, ok bool) {
	if s.Scheme != "file" {
		return "", false
	}
	return filepath.Join(nativePath(s.Path), filepath.FromSlash(rel)), true
}
