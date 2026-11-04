package main

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/server/gorged"
)

var dockerDir = filepath.Join("..", "..", "deploy", "docker")

func readDocker(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dockerDir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestDockerComposeUsesRealFlags keeps the container demo's gorged command
// in step with the flag set: every "- -flag" entry names a flag the binary
// registers, and the Dockerfile's default command does too.
func TestDockerComposeUsesRealFlags(t *testing.T) {
	fs := gorged.ServeFlagSetFor(extensions())
	var names []string
	for _, m := range regexp.MustCompile(`(?m)^\s+- (-[a-z][a-z0-9-]*)(?:=.*)?$`).FindAllStringSubmatch(readDocker(t, "compose.yaml"), -1) {
		names = append(names, strings.TrimPrefix(m[1], "-"))
	}
	for _, m := range regexp.MustCompile(`"(-[a-z][a-z0-9-]*)"`).FindAllStringSubmatch(readDocker(t, "Dockerfile"), -1) {
		names = append(names, strings.TrimPrefix(m[1], "-"))
	}
	if len(names) < 10 {
		t.Fatalf("found only %d flags in compose.yaml and the Dockerfile; the pattern has drifted", len(names))
	}
	for _, n := range names {
		if fs.Lookup(n) == nil {
			t.Errorf("deploy/docker names -%s, which gorged does not register", n)
		}
	}
}

// TestDockerEnvExampleDocumentsEveryVariable keeps .env.example and the
// ${VAR} references in compose.yaml in step, in both directions.
func TestDockerEnvExampleDocumentsEveryVariable(t *testing.T) {
	// FORGE_REF is set by the Makefile from the corpus pin, not by .env.
	used := map[string]bool{}
	for _, m := range regexp.MustCompile(`\$\{([A-Z_]+)[:}]`).FindAllStringSubmatch(readDocker(t, "compose.yaml"), -1) {
		if m[1] != "FORGE_REF" {
			used[m[1]] = true
		}
	}
	documented := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^#?([A-Z][A-Z_]+)=`).FindAllStringSubmatch(readDocker(t, ".env.example"), -1) {
		documented[m[1]] = true
	}
	for v := range used {
		if !documented[v] {
			t.Errorf("deploy/docker/.env.example does not document %s", v)
		}
	}
	for v := range documented {
		if !used[v] {
			t.Errorf("deploy/docker/.env.example documents %s, which compose.yaml does not read", v)
		}
	}
	if !slices.Contains(strings.Fields(readDocker(t, "../../.gitignore")), "/deploy/docker/.env") {
		t.Error(".gitignore does not ignore /deploy/docker/.env")
	}
}
