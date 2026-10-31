package draftlab

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// updateFingerprints regenerates the fingerprint goldens. Run it when an
// agent's behaviour deliberately changed (bump its version in the table
// first) or the corpus pin moved:
//
//	go test ./draft/draftlab -run TestBuiltinFingerprints -update-fingerprints
var updateFingerprints = flag.Bool("update-fingerprints", false, "regenerate draftlab's fingerprint goldens")

// The fingerprint run's fixed inputs.
const (
	fingerprintSet      = "FDN"
	fingerprintCollate  = setdata.Arena
	fingerprintPackSeed = 20261014
	fingerprintPodSeats = 8
	fingerprintPodRnds  = 3
	fingerprintDraft    = 7
	fingerprintAgent    = "rating-colour"
	fingerprintBuilder  = "best23"
)

// packsGolden is testdata/fingerprint_packs.json's shape: the pod the
// agent fingerprints draft. The packs are pinned so a corpus pin bump
// (renumbered card IDs) shows up as a data change, not as a silent agent
// change.
type packsGolden struct {
	Format    string                `json:"format"`
	Set       string                `json:"set"`
	Collation string                `json:"collation"`
	PackSeed  uint64                `json:"pack_seed"`
	Seats     int                   `json:"seats"`
	Rounds    int                   `json:"rounds"`
	Packs     [][][]limited.CardRef `json:"packs"`
}

// agentGolden is testdata/agent_fingerprints.json's shape: the two
// fingerprints of the fingerprint draft (picks, built decks), keyed
// agent@version.
type agentGolden struct {
	Format    string            `json:"format"`
	DraftSeed uint64            `json:"draft_seed"`
	Hashes    map[string]string `json:"hashes"`
}

func TestBuiltinFingerprints(t *testing.T) {
	l := testLab(t)
	podPath := filepath.Join("testdata", "fingerprint_packs.json")
	agentPath := filepath.Join("testdata", "agent_fingerprints.json")

	if *updateFingerprints {
		packs, err := l.OpenPod(fingerprintSet, fingerprintCollate, fingerprintPackSeed, fingerprintPodSeats, fingerprintPodRnds)
		if err != nil {
			t.Fatalf("OpenPod: %v", err)
		}
		writeJSON(t, podPath, packsGolden{
			Format: "draftlab fingerprint packs v1", Set: fingerprintSet,
			Collation: string(fingerprintCollate), PackSeed: fingerprintPackSeed,
			Seats: fingerprintPodSeats, Rounds: fingerprintPodRnds, Packs: packs,
		})
		writeJSON(t, agentPath, agentGolden{
			Format: "draftlab agent fingerprints v1", DraftSeed: fingerprintDraft,
			Hashes: agentFingerprints(t, l, packs, fingerprintDraft),
		})
		t.Log("updated", podPath, "and", agentPath)
		return
	}

	var pg packsGolden
	readJSON(t, podPath, &pg)
	if pg.Set != fingerprintSet || pg.Seats != fingerprintPodSeats || pg.Rounds != fingerprintPodRnds || pg.PackSeed != fingerprintPackSeed {
		t.Fatalf("%s is not the v1 fingerprint pod: set %q %d×%d seed %d", podPath, pg.Set, pg.Seats, pg.Rounds, pg.PackSeed)
	}
	// Precondition: the pod is intact (unique IDs pod-wide).
	seen := make(map[string]bool)
	for s := range pg.Packs {
		for r := range pg.Packs[s] {
			for _, ref := range pg.Packs[s][r] {
				if seen[ref.ID] {
					t.Fatalf("%s: card ID %s appears twice", podPath, ref.ID)
				}
				seen[ref.ID] = true
			}
		}
	}

	var ag agentGolden
	readJSON(t, agentPath, &ag)
	if ag.DraftSeed != fingerprintDraft {
		t.Fatalf("%s is not the v1 fingerprint draft (seed %d)", agentPath, ag.DraftSeed)
	}
	for _, key := range []string{"drafter:" + fingerprintAgent + "@1", "builder:" + fingerprintBuilder + "@1"} {
		if _, ok := ag.Hashes[key]; !ok {
			t.Fatalf("%s: no %s entry", agentPath, key)
		}
	}
	got := agentFingerprints(t, l, pg.Packs, fingerprintDraft)
	for key, want := range ag.Hashes {
		if got[key] != want {
			t.Errorf("%s: %s fingerprint changed: got %s, want %s — behaviour changed: if the agent's code or ratings changed, bump its version in the table and regenerate with -update-fingerprints; if only the corpus pin moved, regenerate and say so in the commit", agentPath, key, got[key], want)
		}
	}
}

// agentFingerprints runs the fingerprint draft (every seat its own fresh
// rating-colour@1) and hashes the observable behaviour: the Picked lines
// of the log, and the eight best23 decks the pools build to.
func agentFingerprints(t *testing.T, l *Lab, packs [][][]limited.CardRef, seed uint64) map[string]string {
	t.Helper()
	drafters := newDrafters(t, l, fingerprintAgent, fingerprintPodSeats)
	res, err := l.Draft(packs, drafters, seed)
	if err != nil {
		t.Fatalf("fingerprint draft: %v", err)
	}

	// The picks: every Picked entry, JSON-marshalled, newline-joined.
	entries, err := event.ParseLog(res.Log)
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}
	var picked bytes.Buffer
	for _, en := range entries {
		if en.Kind != event.KindPicked {
			continue
		}
		line, err := json.Marshal(en)
		if err != nil {
			t.Fatal(err)
		}
		picked.Write(line)
		picked.WriteByte('\n')
	}
	if picked.Len() == 0 {
		t.Fatal("fingerprint draft produced no Picked lines")
	}
	out := map[string]string{
		"drafter:" + fingerprintAgent + "@1": hash16(picked.Bytes()),
	}

	// The builds: the eight best23 decks as one JSON document.
	builder := testBuilder(t, l, fingerprintBuilder)
	decks := make([]build.Deck, 0, len(res.Pools))
	for s := range res.Pools {
		d := builder.Builder.Build(res.Set, res.Pools[s])
		if err := build.Validate(res.Pools[s], d); err != nil {
			t.Fatalf("fingerprint build seat %d: %v", s, err)
		}
		if len(build.ToDeckFile(res.Pools[s], d, "x").Cards) == 0 {
			t.Fatalf("fingerprint build seat %d produced an empty deck", s)
		}
		decks = append(decks, d)
	}
	jb, err := json.Marshal(decks)
	if err != nil {
		t.Fatal(err)
	}
	out["builder:"+fingerprintBuilder+"@1"] = hash16(jb)
	return out
}

func hash16(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}

func writeJSON(t *testing.T, path string, v interface{}) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readJSON(t *testing.T, path string, v interface{}) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v (run `go test ./draft/draftlab -run TestBuiltinFingerprints -update-fingerprints` once with the corpus present)", path, err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
}
