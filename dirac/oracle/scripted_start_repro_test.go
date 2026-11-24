package oracle

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/rules"
)

// scriptedStartReproSeed is the game seed every repro fixture plays.
const scriptedStartReproSeed = 1

// TestScriptedStartRepro: a game played from a scripted start is written as a
// koracle-repro-3 repro with the start inline, read back, rebuilt into the
// matchup and replayed to the same verdict, once clean and once diverging at
// decision 0 (a kernel that ignores the start). It also pins the format rule:
// koracle-repro-3 only when a start is present, koracle-repro-2 as before
// otherwise, and a recorded kernel game (koracle-game-3) replays in gorge from
// the same start.
func TestScriptedStartRepro(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join("testdata", "scenarios", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 6 {
		t.Fatalf("%d fixtures under testdata/scenarios: want at least 6", len(files))
	}
	slices.Sort(files)
	c, err := OpenCorpus(corpusDirOrSkip(t), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		t.Run(strings.TrimSuffix(filepath.Base(file), ".json"), func(t *testing.T) {
			t.Parallel()
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			sc, err := ParseScenario(raw)
			if err != nil {
				t.Fatal(err)
			}
			m, err := c.ScenarioMatchup(sc, 0)
			if err != nil {
				t.Fatal(err)
			}
			seed := sc.GameSeed(scriptedStartReproSeed)
			cfg := Config{}
			rc := RunConfig{Config: cfg, MaxDecisions: 300, MaxTurns: 3}
			dir := t.TempDir()

			// The recorded game: clean in lockstep against a kernel that
			// opens in the same start.
			k, err := GorgeKernel(cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, RandomPolicy(seed), rc)
			if !res.Clean() || len(res.Actions) == 0 {
				t.Fatalf("recorded game: clean=%v, %d actions, div %+v", res.Clean(), len(res.Actions), res.Div)
			}
			r := NewRepro(m, seed, rc, &res)
			if r.Format != reproFormat3 || r.Start == nil {
				t.Fatalf("repro of a scripted start: format %q, start %v", r.Format, r.Start)
			}
			path := filepath.Join(dir, "clean.repro.json")
			if err := r.Write(path); err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(b, []byte(`"format":"koracle-repro-3"`)) || !bytes.Contains(b, []byte(`"start":{"seats":`)) {
				t.Fatalf("file lacks the format or the inline start: %.300s", b)
			}
			if canon, err := compactReproJSON(b); err != nil || !bytes.Equal(canon, b) {
				t.Fatalf("file is not the compact encoding (%v)", err)
			}
			back, err := ReadRepro(path)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := startJSON(t, back.Start), startJSON(t, m.Start); got != want {
				t.Fatalf("start did not survive the file:\n got %s\nwant %s", got, want)
			}
			if back.Format != reproFormat3 || !slices.Equal(back.Actions, r.Actions) || back.Seed != seed {
				t.Fatalf("read back: format %q, %d/%d actions, seed %d", back.Format, len(back.Actions), len(r.Actions), back.Seed)
			}
			rm, err := back.Matchup(c.Reg)
			if err != nil {
				t.Fatal(err)
			}
			if rm.Start == nil || startJSON(t, rm.Start) != startJSON(t, m.Start) {
				t.Fatalf("rebuilt matchup start %v", rm.Start)
			}
			// the replay opens in the start (a start that did nothing would
			// replay the same clean verdict), then ends where the game did
			lead, err := NewGorgeSide(rm, seed, cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			scriptedStartPlaced(t, lead, rm, seed)
			again := back.Rerun(rm, GorgeKernel(cfg), 0)
			if !again.Clean() || !slices.Equal(again.Actions, res.Actions) {
				t.Fatalf("replay: clean=%v, %d actions vs %d, div %+v", again.Clean(), len(again.Actions), len(res.Actions), again.Div)
			}

			// The diverging game: the kernel ignores the start, so decision 0
			// differs. The repro records that, and replays to the same verdict.
			bare := func(m *Matchup, seed uint64) (Side, error) {
				b := *m
				b.Start = nil
				return GorgeKernel(cfg)(&b, seed)
			}
			bk, err := bare(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			dres := Lockstep(m, seed, bk, RandomPolicy(seed), rc)
			if dres.Div == nil || dres.Div.At != 0 {
				t.Fatalf("a kernel without the start did not diverge at decision 0: %+v", dres.Div)
			}
			dr := NewRepro(m, seed, rc, &dres)
			dpath := filepath.Join(dir, "div.repro.json")
			if err := dr.Write(dpath); err != nil {
				t.Fatal(err)
			}
			dback, err := ReadRepro(dpath)
			if err != nil {
				t.Fatal(err)
			}
			drm, err := dback.Matchup(c.Reg)
			if err != nil {
				t.Fatal(err)
			}
			dagain := dback.Rerun(drm, bare, 0)
			if dagain.Div == nil || dagain.Div.At != dres.Div.At || dagain.Div.What != dres.Div.What || dagain.Div.Field != dres.Div.Field {
				t.Fatalf("replayed verdict %+v, recorded %+v", dagain.Div, dres.Div)
			}

			// The same game without a start is koracle-repro-2, byte for byte
			// what it was: no start key, and it reads back as format 2.
			plain := *m
			plain.Start = nil
			pr := NewRepro(&plain, seed, rc, &res)
			ppath := filepath.Join(dir, "plain.repro.json")
			if err := pr.Write(ppath); err != nil {
				t.Fatal(err)
			}
			pb, err := os.ReadFile(ppath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(pb, []byte(`"format":"koracle-repro-2"`)) || bytes.Contains(pb, []byte(`"start"`)) {
				t.Fatalf("a repro without a start is not plain koracle-repro-2: %.300s", pb)
			}
			if pback, err := ReadRepro(ppath); err != nil || pback.Start != nil || pback.Format != reproFormat {
				t.Fatalf("plain repro read back: %v, %+v", err, pback)
			}
			// a format that disagrees with the start is refused, both ways
			for name, edit := range map[string]func() []byte{
				"repro-3 without a start": func() []byte { return bytes.Replace(pb, []byte("repro-2"), []byte("repro-3"), 1) },
				"repro-2 with a start":    func() []byte { return bytes.Replace(b, []byte("repro-3"), []byte("repro-2"), 1) },
			} {
				bad := filepath.Join(dir, strings.ReplaceAll(name, " ", "_")+".json")
				if err := os.WriteFile(bad, edit(), 0o644); err != nil {
					t.Fatal(err)
				}
				if _, err := ReadRepro(bad); err == nil {
					t.Fatalf("%s: read without an error", name)
				}
			}

			// The kernel game record: gorge, as the kernel, plays from the
			// start; the record is koracle-game-3 and replays in gorge.
			gk, err := GorgeKernel(cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			g := RecordKernelGame(m, seed, gk, RandomPolicy(seed), cfg, 300)
			if g.Format != gameFormat3 || g.Start == nil || len(g.Actions) == 0 {
				t.Fatalf("game record: format %q, start %v, %d actions", g.Format, g.Start, len(g.Actions))
			}
			gpath := filepath.Join(dir, "game.json")
			if err := g.Write(gpath); err != nil {
				t.Fatal(err)
			}
			g2, err := ReadKernelGame(gpath)
			if err != nil {
				t.Fatal(err)
			}
			if startJSON(t, g2.Start) != startJSON(t, m.Start) {
				t.Fatalf("game start did not survive the file")
			}
			if rr := Replay(c.Reg, g2); rr.DivergedAt != -1 || rr.Verified != nil {
				t.Fatalf("game replay: diverged at %d (%s: %s), verified %v", rr.DivergedAt, rr.Reason, rr.Detail, rr.Verified)
			}
			// the control: the same game replayed without its start is another
			// game, which the recorded hashes catch
			g3 := *g2
			g3.Start, g3.Format = nil, gameFormat
			if rr := Replay(c.Reg, &g3); rr.DivergedAt == -1 {
				t.Fatal("a game replayed without its start reproduced the recorded game")
			}
		})
	}
}

// startJSON is sp's wire form as a string, so two starts compare by what a
// recording holds (an empty slice and a nil one are the same start).
func startJSON(t *testing.T, sp *rules.StartSpec) string {
	t.Helper()
	b, err := json.Marshal(startToWire(sp))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
