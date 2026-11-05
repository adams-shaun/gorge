package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cli"
	"github.com/adams-shaun/gorge/draft/limited/pickbot"
)

// TestDeckpoolDefaultIsBest23: without --builder deckpool builds with
// best23, the strongest measured builder.
func TestDeckpoolDefaultIsBest23(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := cli.Execute(newRoot(), []string{"--print-config"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("--print-config: exit %d (stderr %q)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"builder": "best23"`) {
		t.Fatalf("--print-config lacks \"builder\": \"best23\":\n%s", stdout.String())
	}
	_, b, ok := pickbot.New(defaultBuilder, nil)
	if _, isBest23 := b.(*pickbot.Best23); !ok || !isBest23 {
		t.Fatalf("default builder %q resolves to %T, want *pickbot.Best23", defaultBuilder, b)
	}
}
