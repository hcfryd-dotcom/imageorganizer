// Package source gives uniform io/fs access to library folders wherever they
// live. Each protocol is an Adapter registered by URL scheme; everything else
// in the application (scanner, HTTP serving, the folder picker) works only on
// fs.FS with slash-separated relative paths. Adding SFTP, WebDAV, FTP, ... is
// a matter of writing one adapter.
package source

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"path"
	"sort"
	"strings"
	"sync"
)

// Spec is a parsed folder location.
type Spec struct {
	Scheme   string // "file", "smb", ...
	Host     string // host[:port]; empty for file
	User     string // may carry adapter-specific syntax, e.g. "DOMAIN;user" for smb
	Password string // never part of Location()
	Path     string // absolute, slash-separated, cleaned; "/" is the root
}

// Adapter opens one kind of location.
type Adapter interface {
	// Open connects and returns a file system rooted at spec.Path.
	Open(ctx context.Context, spec Spec) (*Conn, error)
}

// RootLister is implemented by adapters whose top level is not itself a
// directory, e.g. SMB, where "/" is the list of shares.
type RootLister interface {
	ListRoots(ctx context.Context, spec Spec) ([]string, error)
}

var (
	regMu    sync.RWMutex
	adapters = map[string]Adapter{}
)

// Register makes an adapter available for a URL scheme.
func Register(scheme string, a Adapter) {
	regMu.Lock()
	defer regMu.Unlock()
	adapters[scheme] = a
}

// Schemes lists registered schemes, for the UI.
func Schemes() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]string, 0, len(adapters))
	for s := range adapters {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func adapter(scheme string) (Adapter, error) {
	regMu.RLock()
	defer regMu.RUnlock()
	a, ok := adapters[scheme]
	if !ok {
		return nil, fmt.Errorf("unsupported location type %q", scheme)
	}
	return a, nil
}

// Parse accepts a bare absolute path ("/mnt/photos") or a URL such as
// "smb://user@host/share/dir".
func Parse(loc string) (Spec, error) {
	loc = strings.TrimSpace(loc)
	if strings.HasPrefix(loc, "/") {
		return Spec{Scheme: "file", Path: cleanPath(loc)}, nil
	}
	u, err := url.Parse(loc)
	if err != nil {
		return Spec{}, err
	}
	if u.Scheme == "" {
		return Spec{}, errors.New("location must be an absolute path or a URL like smb://user@host/share")
	}
	s := Spec{Scheme: strings.ToLower(u.Scheme), Host: u.Host, Path: cleanPath(u.Path)}
	if s.Scheme == "cifs" {
		s.Scheme = "smb"
	}
	if u.User != nil {
		s.User = u.User.Username()
		s.Password, _ = u.User.Password()
	}
	if _, err := adapter(s.Scheme); err != nil {
		return Spec{}, err
	}
	if s.Scheme != "file" && s.Host == "" {
		return Spec{}, fmt.Errorf("%s location needs a host", s.Scheme)
	}
	return s, nil
}

func cleanPath(p string) string { return path.Clean("/" + strings.Trim(p, "/")) }

// Location renders the spec as a canonical string without the password.
// It identifies a folder in the database and in the UI.
func (s Spec) Location() string {
	if s.Scheme == "file" {
		return s.Path
	}
	u := url.URL{Scheme: s.Scheme, Host: s.Host, Path: s.Path}
	if s.User != "" {
		u.User = url.User(s.User)
	}
	return strings.TrimSuffix(u.String(), "/")
}

// Child returns the spec of a subdirectory.
func (s Spec) Child(name string) Spec {
	s.Path = path.Join(s.Path, name)
	return s
}

// Parent returns the parent spec and false when already at the root.
func (s Spec) Parent() (Spec, bool) {
	if s.Path == "/" {
		return s, false
	}
	s.Path = path.Dir(s.Path)
	return s, true
}

// Within reports whether a is b or lies inside it (same server).
func Within(a, b Spec) bool {
	if a.Scheme != b.Scheme || !strings.EqualFold(a.Host, b.Host) {
		return false
	}
	return a.Path == b.Path || b.Path == "/" || strings.HasPrefix(a.Path, b.Path+"/")
}

// Display joins a folder location and a file's relative path for users.
func Display(location, rel string) string {
	if rel == "" || rel == "." {
		return location
	}
	return strings.TrimSuffix(location, "/") + "/" + rel
}

// Conn is an open source.
type Conn struct {
	FS      fs.FS
	closeFn func() error
}

// NewConn is for adapters.
func NewConn(fsys fs.FS, closeFn func() error) *Conn { return &Conn{FS: fsys, closeFn: closeFn} }

func (c *Conn) Close() error {
	if c == nil || c.closeFn == nil {
		return nil
	}
	return c.closeFn()
}

// Open connects to the location described by s.
func Open(ctx context.Context, s Spec) (*Conn, error) {
	a, err := adapter(s.Scheme)
	if err != nil {
		return nil, err
	}
	return a.Open(ctx, s)
}

// Listing is a directory listing for the folder picker.
type Listing struct {
	Location string   `json:"location"`
	Parent   string   `json:"parent"` // "" at the top
	Dirs     []string `json:"dirs"`
}

// Browse lists the subdirectories of s for picking a folder.
func Browse(ctx context.Context, s Spec) (Listing, error) {
	l := Listing{Location: s.Location(), Dirs: []string{}}
	if p, ok := s.Parent(); ok {
		l.Parent = p.Location()
	}
	a, err := adapter(s.Scheme)
	if err != nil {
		return l, err
	}
	var names []string
	if rl, ok := a.(RootLister); ok && s.Path == "/" {
		if names, err = rl.ListRoots(ctx, s); err != nil {
			return l, err
		}
	} else {
		c, err := a.Open(ctx, s)
		if err != nil {
			return l, err
		}
		defer c.Close()
		entries, err := fs.ReadDir(c.FS, ".")
		if err != nil {
			return l, err
		}
		for _, e := range entries {
			n := e.Name()
			if Hidden(n) {
				continue
			}
			if e.IsDir() {
				names = append(names, n)
			} else if e.Type()&fs.ModeSymlink != 0 {
				if fi, err := fs.Stat(c.FS, n); err == nil && fi.IsDir() {
					names = append(names, n)
				}
			}
		}
	}
	sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
	l.Dirs = append(l.Dirs, names...)
	return l, nil
}

// Hidden reports names that are OS or NAS housekeeping and never hold the
// user's photos (Synology keeps its own thumbnails in @eaDir, for example).
func Hidden(name string) bool {
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "@") || strings.HasPrefix(name, "#") {
		return true
	}
	switch name {
	case "$RECYCLE.BIN", "System Volume Information", "lost+found":
		return true
	}
	return false
}
