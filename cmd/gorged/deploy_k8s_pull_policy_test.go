package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestK8sManifestPullsAlways keeps a restarted pod on the image its tag names:
// the Deployment always re-pulls, and the README documents the dated deploy and
// the volumes the manifest actually mounts.
func TestK8sManifestPullsAlways(t *testing.T) {
	dir := filepath.Join("..", "..", "deploy", "k8s")
	manifest := readDeployFile(t, filepath.Join(dir, "deployment.yaml"))
	readme := readDeployFile(t, filepath.Join(dir, "README.md"))

	if got := strings.Count(manifest, "imagePullPolicy: Always"); got != 1 {
		t.Fatalf("deploy/k8s/deployment.yaml has %d %q lines, want exactly 1", got, "imagePullPolicy: Always")
	}

	const claim = "persistentVolumeClaim: {claimName: "
	for _, line := range strings.Split(manifest, "\n") {
		i := strings.Index(line, claim)
		if i < 0 {
			continue
		}
		rest := line[i+len(claim):]
		end := strings.IndexAny(rest, ",}")
		if end < 0 {
			t.Fatalf("cannot parse claimName in %q", line)
		}
		claimName := rest[:end]
		found := false
		for _, row := range strings.Split(readme, "\n") {
			row = strings.TrimSpace(row)
			if strings.HasPrefix(row, "|") && strings.Contains(row, "PVC") && strings.Contains(row, "`"+claimName+"`") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("deploy/k8s/README.md has no PVC row for %q (manifest: %s)", claimName, strings.TrimSpace(line))
		}
	}

	if bad := "`/art` (card art cache), `/feedback`"; strings.Contains(readme, bad) {
		t.Errorf("deploy/k8s/README.md still calls /art an emptyDir: found %q", bad)
	}
}

func readDeployFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
