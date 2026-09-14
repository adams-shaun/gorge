//go:build !(linux || darwin || dragonfly || freebsd || netbsd || openbsd)

package main

import "context"

// lockPace is the fallback on platforms without syscall.Flock, including
// Windows, AIX and Solaris. It degrades the limiter to per-process sharing:
// every artCache for the same directory uses one synchronized in-memory stamp,
// so their complete read/wait/write sequences remain serialized and a stale
// waiter can never overwrite a newer request start. Separate processes do not
// share that state and may each send up to 10 req/s. Production deployments of
// gorged are Unix; the fallback exists so other targets compile and retain a
// conservative, race-free per-process limiter.
func (a *artCache) lockPace(ctx context.Context) (paceLock, error) {
	return lockProcessPace(ctx, a.dir)
}
