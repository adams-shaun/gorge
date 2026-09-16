package main

import (
	"context"
	"io"
	"path/filepath"
	"sync"
)

// processPaceState is the fallback limiter state for one cache directory.
// permit protects stamp for the entire read/wait/write sequence in paceWait,
// not merely each individual access. That prevents a waiter from reading an
// old timestamp and later overwriting a newer one written by another cache.
type processPaceState struct {
	permit  chan struct{}
	stamp   [8]byte
	stamped bool
}

var processPaceStates = struct {
	sync.Mutex
	byDir map[string]*processPaceState
}{byDir: make(map[string]*processPaceState)}

// lockProcessPace takes the process-local pace lock for dir. It lives in an
// untagged file so the fallback's synchronization can be tested on Unix even
// though production Unix builds use flock instead.
func lockProcessPace(ctx context.Context, dir string) (paceLock, error) {
	key, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	key = filepath.Clean(key)
	key, err = filepath.EvalSymlinks(key)
	if err != nil {
		return nil, err
	}

	processPaceStates.Lock()
	state := processPaceStates.byDir[key]
	if state == nil {
		state = &processPaceState{permit: make(chan struct{}, 1)}
		state.permit <- struct{}{}
		processPaceStates.byDir[key] = state
	}
	processPaceStates.Unlock()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-state.permit:
		// Prefer cancellation even if it raced with an available permit.
		if err := ctx.Err(); err != nil {
			state.permit <- struct{}{}
			return nil, err
		}
		return &processPaceLock{state: state}, nil
	}
}

type processPaceLock struct {
	state *processPaceState
	once  sync.Once
}

func (l *processPaceLock) ReadAt(p []byte, off int64) (int, error) {
	if !l.state.stamped || off < 0 || off >= int64(len(l.state.stamp)) {
		return 0, io.EOF
	}
	n := copy(p, l.state.stamp[off:])
	if n != len(p) {
		return n, io.EOF
	}
	return n, nil
}

func (l *processPaceLock) WriteAt(p []byte, off int64) (int, error) {
	if off < 0 || off >= int64(len(l.state.stamp)) {
		return 0, io.ErrShortWrite
	}
	n := copy(l.state.stamp[off:], p)
	if n != len(p) {
		return n, io.ErrShortWrite
	}
	l.state.stamped = true
	return n, nil
}

func (l *processPaceLock) Close() error {
	l.once.Do(func() { l.state.permit <- struct{}{} })
	return nil
}
