package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/server/gorged"
)

// k8sArgs returns the gorged container's args list from
// deploy/k8s/deployment.yaml: the "- value" lines after "          args:".
func k8sArgs(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "deploy", "k8s", "deployment.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	item := regexp.MustCompile(`^\s+- (\S+)$`)
	var args []string
	in := false
	for _, line := range strings.Split(string(b), "\n") {
		if !in {
			in = line == "          args:"
			continue
		}
		m := item.FindStringSubmatch(line)
		if m == nil {
			break
		}
		args = append(args, m[1])
	}
	if len(args) < 8 {
		t.Fatalf("found %d args in deploy/k8s/deployment.yaml (%q); want the full gorged command", len(args), args)
	}
	return args
}

// TestK8sManifestArgs keeps the Kubernetes Deployment running four
// four-seat Commander tables: its args parse with gorged's flag set and
// carry the values the site depends on.
func TestK8sManifestArgs(t *testing.T) {
	args := k8sArgs(t)
	if args[0] != "gorged" {
		t.Fatalf("args[0] = %q, want gorged (entrypoint.sh execs its arguments)", args[0])
	}
	fs := gorged.ServeFlagSetFor(extensions())
	if err := fs.Parse(args[1:]); err != nil {
		t.Fatalf("parse %q: %v", args[1:], err)
	}
	if rest := fs.Args(); len(rest) != 0 {
		t.Fatalf("args left unparsed: %q", rest)
	}
	for name, want := range map[string]string{
		"format":          "commander",
		"tables":          "4",
		"seats":           "4",
		"commander-seats": "4",
		"vsbot":           "true",
		"events":          "true",
		"dir":             "/data",
		"cards":           "/cards",
		// Bot tables prune old finished matches: without it the local pod kept
		// 9.8 GB of them and ran out of its 2 GiB at startup (11-28).
		"keep-matches": "200",
	} {
		f := fs.Lookup(name)
		if f == nil {
			t.Errorf("gorged registers no -%s", name)
			continue
		}
		if got := f.Value.String(); got != want {
			t.Errorf("-%s = %q, want %q", name, got, want)
		}
	}
}
