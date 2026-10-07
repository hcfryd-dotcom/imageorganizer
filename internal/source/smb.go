package source

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"strings"
	"time"

	"github.com/hirochachacha/go-smb2"
)

func init() { Register("smb", smbAdapter{}) }

// smbAdapter speaks SMB2/3 directly, so NAS shares need no OS mount.
// Locations look like smb://[DOMAIN;]user@host[:port]/share/sub/dir; the
// first path element is the share.
type smbAdapter struct{}

func splitShare(p string) (share, rest string) {
	share, rest, _ = strings.Cut(strings.TrimPrefix(p, "/"), "/")
	return share, rest
}

func (smbAdapter) Open(ctx context.Context, s Spec) (*Conn, error) {
	shareName, rest := splitShare(s.Path)
	if shareName == "" {
		return nil, errors.New("smb location needs a share: smb://user@host/share")
	}
	sess, err := dialSMB(ctx, s)
	if err != nil {
		return nil, err
	}
	share, err := sess.Mount(shareName)
	if err != nil {
		sess.Logoff()
		return nil, fmt.Errorf("mount share %q: %w", shareName, err)
	}
	fsys := share.DirFS(rest)
	if _, err := fs.Stat(fsys, "."); err != nil {
		share.Umount()
		sess.Logoff()
		return nil, fmt.Errorf("open %s: %w", s.Location(), err)
	}
	return NewConn(fsys, func() error {
		share.Umount()
		return sess.Logoff()
	}), nil
}

// ListRoots lists the server's visible shares.
func (smbAdapter) ListRoots(ctx context.Context, s Spec) ([]string, error) {
	sess, err := dialSMB(ctx, s)
	if err != nil {
		return nil, err
	}
	defer sess.Logoff()
	names, err := sess.ListSharenames()
	if err != nil {
		return nil, err
	}
	out := names[:0]
	for _, n := range names {
		if !strings.HasSuffix(n, "$") { // admin/hidden shares
			out = append(out, n)
		}
	}
	return out, nil
}

func dialSMB(ctx context.Context, s Spec) (*smb2.Session, error) {
	addr := s.Host
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "445")
	}
	dctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{KeepAlive: 30 * time.Second}).DialContext(dctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	user, domain := s.User, ""
	if d, u, ok := strings.Cut(user, ";"); ok {
		domain, user = d, u
	}
	d := &smb2.Dialer{Initiator: &smb2.NTLMInitiator{User: user, Password: s.Password, Domain: domain}}
	sess, err := d.DialContext(dctx, conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("smb login to %s: %w", s.Host, err)
	}
	return sess, nil
}
