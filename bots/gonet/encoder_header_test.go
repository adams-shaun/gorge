package gonet_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots/gonet"
	"github.com/adams-shaun/gorge/bots/gonet/gonettest"
	"github.com/adams-shaun/gorge/bots/policynet"
)

// writePodNamed writes a pod-geometry network whose header names encoderName
// and carries hash.
func writePodNamed(t *testing.T, dir, name, encoderName, hash string) string {
	t.Helper()
	p, err := gonettest.WriteRaw(dir, name, policynet.EntityRawWidthPod, map[string]any{
		"encoder":      encoderName,
		"encoder_hash": hash,
		"DENSE_W":      policynet.DenseWidthPod,
		"GROUPS":       policynet.EntityGroupsPod,
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestLoadResolvesHeaderEncoder: a .dzgw header's encoder name picks the
// registered encoder the loader checks the hash and geometry against. A pod
// net naming entity-pod loads; a name and hash that disagree are refused; a
// hidden encoder is refused because a .dzgw is a playing (seat) net.
func TestLoadResolvesHeaderEncoder(t *testing.T) {
	dir := t.TempDir()
	podHash := fmt.Sprintf("%016x", policynet.EncoderHashFor(policynet.FeaturesPod))

	t.Run("entity-pod", func(t *testing.T) {
		g, err := gonet.Load(writePodNamed(t, dir, "pod.dzgw", "entity-pod", podHash))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if g.RawWidth() != policynet.EntityRawWidthPod || !g.EncoderHashChecked() {
			t.Fatalf("loaded pod net: raw %d checked %v", g.RawWidth(), g.EncoderHashChecked())
		}
	})

	t.Run("name-hash-mismatch", func(t *testing.T) {
		// The geometry is the entity encoder's, but the hash names the pod
		// encoder: resolving the name alone, without checking its hash, would
		// accept this.
		p, err := gonettest.WriteWith(dir, "mismatch.dzgw", map[string]any{
			"encoder":      "entity",
			"encoder_hash": podHash,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := gonet.Load(p); err == nil {
			t.Fatal("a net naming entity with the pod encoder's hash loaded")
		}
	})

	t.Run("hidden-in-seat", func(t *testing.T) {
		oracleHash := fmt.Sprintf("%016x", policynet.EncoderHashFor(policynet.FeaturesMZOracle))
		p, err := gonettest.WriteWith(dir, "oracle.dzgw", map[string]any{
			"encoder":      "mz-oracle",
			"encoder_hash": oracleHash,
		})
		if err != nil {
			t.Fatal(err)
		}
		_, err = gonet.Load(p)
		if err == nil {
			t.Fatal("a net naming mz-oracle loaded for a seat")
		}
		if !strings.Contains(err.Error(), "hidden") {
			t.Fatalf("refused for the wrong reason: %v", err)
		}
	})
}
