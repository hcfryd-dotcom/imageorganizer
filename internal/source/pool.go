package source

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sync"
	"sync/atomic"
	"time"
)

// Pool keeps open connections per folder for reading files, reconnecting
// when a remote drops an idle session. Remote protocols serialise a lot on
// one session, so each folder gets several connections used round-robin.
type Pool struct {
	size int

	mu    sync.Mutex
	slots map[int64]*slots
}

type slots struct {
	spec  Spec
	conns []*pooled // fixed length = pool size; nil until opened
	next  atomic.Uint32
}

type pooled struct {
	conn *Conn
	used time.Time
}

// NewPool creates a pool with up to size connections per folder.
func NewPool(size int) *Pool {
	return &Pool{size: max(1, size), slots: map[int64]*slots{}}
}

// Do runs fn against the folder's file system. If fn fails with anything
// other than "not found", that connection is assumed stale and fn is retried
// once on a fresh one.
func (p *Pool) Do(ctx context.Context, folderID int64, spec Spec, fn func(fs.FS) error) error {
	i, c, err := p.get(ctx, folderID, spec)
	if err != nil {
		return err
	}
	err = safely(fn, c.FS)
	if err == nil || errors.Is(err, fs.ErrNotExist) || errors.Is(err, context.Canceled) {
		return err
	}
	p.dropSlot(folderID, i, c)
	if _, c, err = p.get(ctx, folderID, spec); err != nil {
		return err
	}
	return safely(fn, c.FS)
}

// safely runs fn, turning a panic inside a protocol library into an error.
// go-smb2 panics on a truncated reply when its connection is closed under
// an in-flight read (shutdown, or the NAS dropping the session); that must
// fail one read, not the whole program.
func safely(fn func(fs.FS) error, fsys fs.FS) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("connection failed mid-read: %v", r)
		}
	}()
	return fn(fsys)
}

func (p *Pool) get(ctx context.Context, id int64, spec Spec) (int, *Conn, error) {
	p.mu.Lock()
	s, ok := p.slots[id]
	if !ok || s.spec != spec {
		if ok {
			s.closeAll()
		}
		s = &slots{spec: spec, conns: make([]*pooled, p.size)}
		p.slots[id] = s
	}
	i := int(s.next.Add(1)) % p.size
	if e := s.conns[i]; e != nil {
		e.used = time.Now()
		p.mu.Unlock()
		return i, e.conn, nil
	}
	p.mu.Unlock()

	c, err := Open(ctx, spec)
	if err != nil {
		return 0, nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if cur, ok := p.slots[id]; ok && cur == s {
		if e := s.conns[i]; e != nil { // lost a race for this slot
			c.Close()
			return i, e.conn, nil
		}
		s.conns[i] = &pooled{conn: c, used: time.Now()}
	}
	return i, c, nil
}

// dropSlot closes one connection if it is still the one in slot i.
func (p *Pool) dropSlot(id int64, i int, c *Conn) {
	p.mu.Lock()
	var stale *Conn
	if s, ok := p.slots[id]; ok && s.conns[i] != nil && s.conns[i].conn == c {
		stale = c
		s.conns[i] = nil
	}
	p.mu.Unlock()
	stale.Close()
}

func (s *slots) closeAll() {
	for i, e := range s.conns {
		if e != nil {
			e.conn.Close()
			s.conns[i] = nil
		}
	}
}

// Drop closes all of a folder's connections, e.g. after it was removed.
func (p *Pool) Drop(id int64) {
	p.mu.Lock()
	s, ok := p.slots[id]
	delete(p.slots, id)
	p.mu.Unlock()
	if ok {
		s.closeAll()
	}
}

// Reap closes connections idle for longer than maxIdle; call periodically.
func (p *Pool) Reap(maxIdle time.Duration) {
	var stale []*Conn
	p.mu.Lock()
	for _, s := range p.slots {
		for i, e := range s.conns {
			if e != nil && time.Since(e.used) > maxIdle {
				stale = append(stale, e.conn)
				s.conns[i] = nil
			}
		}
	}
	p.mu.Unlock()
	for _, c := range stale {
		c.Close()
	}
}

// Close closes every connection.
func (p *Pool) Close() {
	p.mu.Lock()
	all := p.slots
	p.slots = map[int64]*slots{}
	p.mu.Unlock()
	for _, s := range all {
		s.closeAll()
	}
}
