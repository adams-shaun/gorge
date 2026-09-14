//go:build !(linux || darwin || dragonfly || freebsd || netbsd || openbsd)

package main

import (
	"context"
	"os"
	"path/filepath"
)

// lockPace is the fallback on platforms without syscall.Flock: it opens the
// stamp file but takes NO lock on it. This includes Windows, AIX and Solaris;
// pace_lock_unix.go lists the Flock-capable GOOS targets exactly. The limiter
// degrades to PER-PROCESS: the pacing semaphore still serializes one
// process's outbound requests at exactly a.pace, but two gorged processes
// sharing one cache directory no longer pace each other and can each send up
// to 10 req/s. Nothing downstream depends on more: a stamp written by a
// concurrent process can only make this one wait an extra gap, never less,
// and the deploy's fill budget bounds the window. Production deployments of
// gorged are unix; this fallback exists so GOOS=windows builds — and anyone
// porting further — compile and behave conservatively rather than not at all.
func (a *artCache) lockPace(ctx context.Context) (*os.File, error) {
	_ = ctx // no lock to wait for; the context governs the requests, not the open
	return os.OpenFile(filepath.Join(a.dir, paceFile), os.O_RDWR|os.O_CREATE, 0o644)
}
