package cardsettest

import "testing"

// TestEDHPodsBuild: four pods build from the committed slice, each of four
// 100-card decks.
func TestEDHPodsBuild(t *testing.T) {
	pods := EDHPods(t, "../../oracle/testdata/slices/edh/decks.tar.gz", 4)
	if len(pods) != 4 {
		t.Fatalf("%d pods, want 4", len(pods))
	}
	for i, p := range pods {
		if p.Table == nil {
			t.Fatalf("pod %d: nil table", i)
		}
		for s, d := range p.Decks {
			if len(d) != 100 {
				t.Errorf("pod %d seat %d: %d ids, want 100", i, s, len(d))
			}
		}
	}
}
