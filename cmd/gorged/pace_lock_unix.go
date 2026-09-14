//go:build linux || darwin || dragonfly || freebsd || netbsd || openbsd

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// pacePollInterval is how long a contended paceWait sleeps between two
// non-blocking flock attempts on the stamp file. It is an order of magnitude
// under the real 100ms pace, so the polling adds at most a tenth of a pace of
// acquisition latency — well under the gap the limiter enforces anyway.
const pacePollInterval = 10 * time.Millisecond

// lockPace opens the pace stamp file and takes its exclusive flock, giving up
// when ctx is done. This file is limited to the GOOS targets whose syscall
// package provides Flock; pace_lock_other.go carries the fallback everywhere
// else, which locks nothing and degrades the limiter to per-process.
//
// The wait is a non-blocking LOCK_EX|LOCK_NB attempt every pacePollInterval,
// not one blocking flock(2) in a goroutine: a blocking flock cannot be
// interrupted by a context, so a waiter cancelled while contended used to
// strand an OS thread inside the flock call until the holder happened to
// release — fifty cancelled browser requests against a holder stopped
// mid-hold meant fifty parked threads. Polling through a.sleep (the same
// context-aware pause every other wait in this cache uses) makes cancellation
// exact: the poll loop returns on the first iteration after ctx is done and
// closes the file, and since it never held the lock there is nothing to
// release and nobody to wait for.
//
// flock locks belong to the open file description, so two caches in one
// process exclude each other exactly as two processes do.
func (a *artCache) lockPace(ctx context.Context) (paceLock, error) {
	f, err := os.OpenFile(filepath.Join(a.dir, paceFile), os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	fd := int(f.Fd())
	for {
		if err := ctx.Err(); err != nil {
			_ = f.Close()
			return nil, err
		}
		err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return f, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			_ = f.Close()
			return nil, err
		}
		if err := a.sleep(ctx, pacePollInterval); err != nil {
			_ = f.Close()
			return nil, err
		}
	}
}
