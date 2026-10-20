package gorged

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/bots"
)

func TestParseSBAgentSpec(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "agent")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	spec, err := parseSBAgentSpec("sb-one=" + exe)
	if err != nil {
		t.Fatal(err)
	}
	if spec.name != "sb-one" || len(spec.argv) != 1 || spec.argv[0] != exe {
		t.Fatalf("spec = %+v", spec)
	}
	for _, bad := range []string{
		"sb-one",
		"SB-one=" + exe,
		"sb one=" + exe,
		"sb-one=" + filepath.Join(t.TempDir(), "missing"),
	} {
		if _, err := parseSBAgentSpec(bad); err == nil {
			t.Errorf("parseSBAgentSpec(%q) accepted", bad)
		}
	}
}

func TestSbAgentFlagRegistersPolicy(t *testing.T) {
	fs, c := serveFlags()
	exe := filepath.Join(t.TempDir(), "agent")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fs.Parse([]string{"-sb-agent", "sb-gorged-test=" + exe}); err != nil {
		t.Fatal(err)
	}
	if err := registerSBAgents(c.sbAgents); err != nil {
		t.Fatal(err)
	}
	entry, ok := bots.Lookup("sb-gorged-test")
	if !ok {
		t.Fatal("registered -sb-agent is not in the bots registry")
	}
	if entry.MaxSeats != 2 || entry.Tier != bots.Experimental {
		t.Fatalf("entry = %+v", entry)
	}
	def, offered, err := validateBotPolicyFlags("", "")
	if err != nil {
		t.Fatal(err)
	}
	list := buildBotPolicyList(def, offered)
	found := false
	for _, p := range list.Policies {
		if p.Name == "sb-gorged-test" {
			found = true
		}
	}
	if !found {
		t.Fatalf("-sb-agent policy missing from /api/bot-policies: %+v", list.Policies)
	}
}

func TestSBAgentDuplicateAndCollisionAreRefused(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "agent")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := registerSBAgents([]sbAgentSpec{
		{name: "sb-dupe", argv: []string{exe}},
		{name: "sb-dupe", argv: []string{exe}},
	}); err == nil {
		t.Fatal("duplicate -sb-agent name accepted")
	}
	if _, ok := bots.Lookup("sb-dupe"); !ok {
		t.Fatal("first registration did not happen before the duplicate was refused")
	}
	if err := registerSBAgents([]sbAgentSpec{{name: "sb-dupe", argv: []string{exe}}}); err == nil {
		t.Fatal("collision with an existing policy accepted")
	}
}
