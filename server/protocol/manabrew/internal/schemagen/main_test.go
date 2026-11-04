//go:build manabrew

package main

import (
	"os"
	"testing"
)

// TestSchemaGenIsFresh fails when server/protocol/manabrew/schema_gen.go no longer
// matches the package source: run `go generate ./server/protocol/manabrew`.
func TestSchemaGenIsFresh(t *testing.T) {
	want, err := generate("../..")
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("../../" + genFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatal("server/protocol/manabrew/schema_gen.go is stale: run go generate ./server/protocol/manabrew")
	}
}
