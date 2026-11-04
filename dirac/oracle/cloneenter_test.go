package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The as-enters copy election for a clone that enters without being cast
// Zombify returns Clone from the graveyard and
// gorge asks the ETBReplacement:Copy election for the entry (CR 614.12). The
// opponent has no creatures, so the only option is "enter as itself"; the
// kernel must still pose the ask at the same decision.
var kcloneenterDecks = [2][]cardset.Entry{
	{
		{Name: "Clone", Count: 4}, {Name: "Zombify", Count: 4},
		{Name: "Island", Count: 16}, {Name: "Swamp", Count: 16},
	},
	{
		{Name: "Plains", Count: 20}, {Name: "Forest", Count: 20},
	},
}

// kcloneenterRun plays n seeds and returns the number of diverging games
// and the Clone graveyard -> battlefield entries gorge made.
func kcloneenterRun(t *testing.T, n uint64, report bool) (div, entries int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("cloneenter")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range kcloneenterDecks {
		for _, e := range d {
			card, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], card)
			}
		}
	}
	tbl, _, _, err := set.Matchup(kcloneenterDecks[0], kcloneenterDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= n; seed++ {
		m, err := NewMatchup(reg, [2]string{"ce-a", "ce-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		check := func(ref *GorgeSide, _ Side) *Divergence {
			evs := ref.E.L.Events
			if len(evs) < seen {
				seen = 0
			}
			for ; seen < len(evs); seen++ {
				ev := &evs[seen]
				if ev.Kind != events.MoveZone || ev.From != state.ZGraveyard || ev.To != state.ZBattlefield {
					continue
				}
				if o := ref.E.G.Obj(ev.Obj); o != nil && o.Face() != nil && o.Face().Name == "Clone" {
					entries++
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, castFirstPolicy(seed), RunConfig{Config: Config{}, MaxDecisions: 2000, MaxTurns: 20, Check: check})
		CloseSide(k)
		// the opponent is land-only: a game may stall on turns/decisions
		if d := res.Div; d != nil {
			div++
			if !report {
				return div, entries
			}
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return div, entries
}

// TestCloneenterLockstep: 200 games, 0 divergences, with Clone reanimated.
func TestCloneenterLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	// not parallel: TestCloneenterMutation flips dirac.DebugCloneReturnNoAsk
	div, entries := kcloneenterRun(t, 200, true)
	t.Logf("200 games: %d divergences, %d Clone graveyard->battlefield entries", div, entries)
	if entries < 100 {
		t.Errorf("%d Clone graveyard->battlefield entries; want at least 100", entries)
	}
}

// TestCloneenterMutation: with the return election dropped some seed
// diverges.
func TestCloneenterMutation(t *testing.T) {
	dirac.DebugCloneReturnNoAsk = true
	defer func() { dirac.DebugCloneReturnNoAsk = false }()
	if div, _ := kcloneenterRun(t, 200, false); div == 0 {
		t.Fatal("no seed diverged with the clone return election dropped")
	}
}

// The copy exceptions of an as-enters clone: cloneEnter
// reads them through copyExceptions. Visage Bandit and Glasspool Mimic add
// Shapeshifter and Rogue, Phyrexian Metamorph adds Artifact, Quicksilver
// Gargantuan is 7/7. The opponent is land-only; seat 0 has creatures and
// artifacts to copy.
var kcloneenterExcDecks = [2][]cardset.Entry{
	{
		{Name: "Visage Bandit", Count: 3}, {Name: "Glasspool Mimic", Count: 3},
		{Name: "Phyrexian Metamorph", Count: 3}, {Name: "Quicksilver Gargantuan", Count: 3},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Ornithopter", Count: 2}, {Name: "Juggernaut", Count: 2},
		{Name: "Island", Count: 18}, {Name: "Swamp", Count: 18},
	},
	{
		{Name: "Plains", Count: 20}, {Name: "Forest", Count: 20},
	},
}

// kcloneenterExcRun plays seeds 1..n against the set's table, with mutate
// applied to a copy of it when non-nil. It returns the first diverging seed
// (0: none), the games the kernel stalled in, and gorge's ETB-clone counts:
// [0] Visage Bandit / Glasspool Mimic copies, [1] Phyrexian Metamorph
// copying a non-artifact, [2] Quicksilver Gargantuan copies. With report
// false it stops at the first divergence.
func kcloneenterExcRun(t *testing.T, n uint64, mutate func(*kop.Table), report bool) (firstDiv uint64, stalls int, cnt [3]int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("cloneenter")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range kcloneenterExcDecks {
		for _, e := range d {
			card, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], card)
			}
		}
	}
	tbl, _, _, err := set.Matchup(kcloneenterExcDecks[0], kcloneenterExcDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mut := *tbl
		mutate(&mut)
		tbl = &mut
	}
	face := func(ref *GorgeSide, id state.ObjID) *cards.Face {
		if o := ref.E.G.Obj(id); o != nil && o.Card != nil && int(o.FaceIdx) < len(o.Card.Faces) {
			return o.Card.Faces[o.FaceIdx]
		}
		return nil
	}
	for seed := uint64(1); seed <= n; seed++ {
		m, err := NewMatchup(reg, [2]string{"cex-a", "cex-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		check := func(ref *GorgeSide, _ Side) *Divergence {
			evs := ref.E.L.Events
			if len(evs) < seen {
				seen = 0
			}
			for ; seen < len(evs); seen++ {
				ev := &evs[seen]
				if ev.Kind != events.ClonePermanent || len(ev.IDs) == 0 {
					continue
				}
				f := face(ref, ev.Obj)
				if f == nil {
					continue
				}
				switch f.Name {
				case "Visage Bandit", "Glasspool Mimic":
					cnt[0]++
				case "Phyrexian Metamorph":
					if src := face(ref, ev.IDs[0]); src != nil && !src.IsArtifact() {
						cnt[1]++
					}
				case "Quicksilver Gargantuan":
					cnt[2]++
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, castFirstPolicy(seed), RunConfig{Config: Config{}, MaxDecisions: 2000, MaxTurns: 20, Check: check})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
		}
		if d := res.Div; d != nil {
			if firstDiv == 0 {
				firstDiv = seed
			}
			if !report {
				return firstDiv, stalls, cnt
			}
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return firstDiv, stalls, cnt
}

// TestCloneenterExcLockstep: 256 games, 0 divergences, 0 kernel stalls,
// with the exceptions exercised.
func TestCloneenterExcLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	div, stalls, cnt := kcloneenterExcRun(t, 256, nil, true)
	t.Logf("256 games: first divergence seed %d, %d kernel stalls; clones: Visage/Glasspool %d, Metamorph of a non-artifact %d, Gargantuan %d", div, stalls, cnt[0], cnt[1], cnt[2])
	if stalls != 0 {
		t.Errorf("%d kernel stalls; want 0", stalls)
	}
	if cnt[0] < 5 || cnt[1] < 3 || cnt[2] < 3 {
		t.Errorf("clones %v; want at least [5 3 3]", cnt)
	}
}

// TestCloneenterExcMutation: with every OpBecomeCopy's exception override
// index cleared (C &= 1), the copies lose Shapeshifter/Rogue, 7/7 and the
// artifact type, and some seed diverges.
func TestCloneenterExcMutation(t *testing.T) {
	div, _, _ := kcloneenterExcRun(t, 64, func(tbl *kop.Table) {
		cs := append([]kop.CardDef(nil), tbl.Cards...)
		n := 0
		for i := range cs {
			abs := append([]kop.Ability(nil), cs[i].Abilities...)
			for j := range abs {
				p := append([]kop.Instr(nil), abs[j].Prog...)
				for x := range p {
					if p[x].Op == kop.OpBecomeCopy && p[x].C>>1 != 0 {
						p[x].C &= 1
						n++
					}
				}
				abs[j].Prog = p
			}
			cs[i].Abilities = abs
		}
		if n == 0 {
			t.Fatal("no OpBecomeCopy carries an exception override")
		}
		tbl.Cards = cs
	}, false)
	if div == 0 {
		t.Fatal("no seed in 1..64 diverged with the copy exceptions dropped")
	}
	t.Logf("seed %d diverges with the copy exceptions dropped", div)
}
