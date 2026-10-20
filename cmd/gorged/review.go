package main

import (
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/testkit/testutil/feedback"
)

// feedbackReview is `gorged review`'s phase-1 loader: a feedback snapshot
// directory (match.json + log.json, the server's own feedback capture) read
// through feedback.Load, which resolves the recorded card names and token
// scripts against the checkout's corpus. It lives here, in the binary, rather
// than in server/gorged, because server/gorged never imports testkit
// (docs/agents/invariants.md).
func feedbackReview(dir string) (host.ReviewMatch, error) {
	l, cfg, meta, err := feedback.Load(dir)
	if err != nil {
		return host.ReviewMatch{}, err
	}
	return host.ReviewMatch{Config: cfg, Log: l, Label: meta.ID}, nil
}
