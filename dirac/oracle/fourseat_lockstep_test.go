package oracle

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"hash/fnv"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestFourSeatGorgeLockstepRunsClean plays synthetic-pool pods of four
// distinct decks gorge against gorge: every game reaches its end with no
// divergence, and the projection carries all four seats.
func TestFourSeatGorgeLockstepRunsClean(t *testing.T) {
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-fdn")
	pool, files, err := SynthPool(reg, set, 1, SynthDecks)
	if err != nil {
		t.Fatal(err)
	}
	c := &Corpus{Reg: reg, decks: map[string][]*cards.Card{}}
	c.AddDecks(files)
	pods := PodsFrom(pool, 0, 3, 1, 4)
	if len(pods) != 3 {
		t.Fatalf("%d pods, want 3", len(pods))
	}
	for _, p := range pods {
		m, err := c.MatchupN(p.Decks(), 0)
		if err != nil {
			t.Fatal(err)
		}
		if m.NSeat() != 4 {
			t.Fatalf("pod %d: %d seats", p.Index, m.NSeat())
		}
		k, err := GorgeKernel(Config{})(m, p.Seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), RunConfig{})
		if res.Div != nil {
			t.Fatalf("pod %d %v: diverged: %v", p.Index, p.Decks(), res.Div)
		}
		if !res.Over {
			t.Fatalf("pod %d %v: not over after %d decisions", p.Index, p.Decks(), res.Decisions)
		}
		var pr Projection
		k.Project(&pr)
		if pr.Seats.NSeat != 4 {
			t.Fatalf("pod %d: projection NSeat %d, want 4", p.Index, pr.Seats.NSeat)
		}
		t.Logf("pod %d: %d decisions, winner %d", p.Index, res.Decisions, res.Winner)
	}
}

// TestProjectionHashTwoSeatStable: a two-seat projection, commander fields
// set and not, hashes over exactly the bytes of every field before Seats, as
// it did before the further seats existed.
func TestProjectionHashTwoSeatStable(t *testing.T) {
	var p Projection
	p.NInst, p.Life, p.Turn, p.Winner = 3, [2]int32{20, 7}, 9, -1
	p.Cards[1].Power, p.Pool[1][2] = 4, 2
	old := func(p *Projection) uint64 {
		var buf bytes.Buffer
		v := reflect.ValueOf(p).Elem()
		for i := 0; i < v.NumField(); i++ {
			name := v.Type().Field(i).Name
			if name == "Seats" {
				continue
			}
			zeroCmd := p.CmdCasts == [2]uint8{} && p.CmdDamage == [2][2]uint8{}
			if zeroCmd && (name == "CmdCasts" || name == "CmdDamage") {
				continue
			}
			_ = binary.Write(&buf, binary.LittleEndian, v.Field(i).Interface())
		}
		h := fnv.New64a()
		h.Write(buf.Bytes())
		return h.Sum64()
	}
	if got, want := ProjectionHash(&p), old(&p); got != want {
		t.Fatalf("plain: hash %x, want %x", got, want)
	}
	p.CmdCasts[1] = 1
	if got, want := ProjectionHash(&p), old(&p); got != want {
		t.Fatalf("commander: hash %x, want %x", got, want)
	}
	q := p
	q.Seats.NSeat = 4
	if ProjectionHash(&q) == ProjectionHash(&p) {
		t.Fatal("a four-seat projection hashes as the two-seat one")
	}
}

// TestRepro2SeatJSONStable: every two-seat repro in testdata reads into a
// matchup of two seats and writes back the same JSON keys it was read with
// (no further-seat field appears).
func TestRepro2SeatJSONStable(t *testing.T) {
	paths, err := filepath.Glob("testdata/*.repro.json")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no repros: %v", err)
	}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var r Repro
		if err := json.Unmarshal(raw, &r); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if len(r.MoreDecks)+len(r.MoreCards) != 0 {
			t.Fatalf("%s: further seats in a two-seat repro", path)
		}
		b, err := json.Marshal(&r)
		if err != nil {
			t.Fatal(err)
		}
		var in, out map[string]json.RawMessage
		if err := json.Unmarshal(raw, &in); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatal(err)
		}
		for k := range out {
			if _, ok := in[k]; !ok {
				t.Fatalf("%s: rewrite adds key %q", path, k)
			}
		}
	}
}
