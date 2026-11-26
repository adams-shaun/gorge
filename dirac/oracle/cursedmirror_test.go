package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The as-enters clone with a duration: Cursed Mirror's
// K:ETBReplacement:Copy:DBCopy:Optional (DB$ Clone | Choices$ Creature |
// AddKeywords$ Haste | Duration$ UntilEndOfTurn) asks the optional election as
// the artifact is cast, enters as a copy of a battlefield creature except it
// has haste, and the cleanup step (CR 514.2) ends the copy — the mirror is an
// artifact again. The kernel bounds the copy with OpBecomeCopy's BecomeCopyEOT
// bit (an fxCopyEOT record, cloneCleanup); gorge bounds the copy unit with
// UntilEOT. The expected answer at every decision is gorge's.
var kcursedmirrorDecks = [2][]cardset.Entry{
	{
		{Name: "Cursed Mirror", Count: 8}, {Name: "Hill Giant", Count: 8},
		{Name: "Mountain", Count: 24},
	},
	{
		{Name: "Cursed Mirror", Count: 8}, {Name: "Hill Giant", Count: 8},
		{Name: "Mountain", Count: 24},
	},
}

// kcursedmirrorRun plays n seeds and returns the number of diverging games
// and what the runs did: copies made, until-end-of-turn copies that the
// cleanup step ended, and copied mirrors that attacked (they entered this
// turn, so only the haste grant lets them).
func kcursedmirrorRun(t *testing.T, n uint64, report bool) (div, copies, reverts, hasteAtk int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("cursedmirror")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range kcursedmirrorDecks {
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
	tbl, _, _, err := set.Matchup(kcursedmirrorDecks[0], kcursedmirrorDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= n; seed++ {
		m, err := NewMatchup(reg, [2]string{"cm-a", "cm-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		copying := map[state.ObjID]bool{} // battlefield permanents that are copies (a ClonePermanent basis)
		orig := func(o *state.Object) string {
			if o != nil && o.Card != nil && int(o.FaceIdx) < len(o.Card.Faces) {
				return o.Card.Faces[o.FaceIdx].Name
			}
			return ""
		}
		check := func(ref *GorgeSide, _ Side) *Divergence {
			evs := ref.E.L.Events
			for ; seen < len(evs); seen++ {
				switch ev := evs[seen]; ev.Kind {
				case events.ClonePermanent:
					if len(ev.IDs) == 0 {
						if copying[ev.Obj] {
							delete(copying, ev.Obj) // the until-end-of-turn copy ended (its settle's clear)
							reverts++
						}
						break
					}
					if !copying[ev.Obj] {
						copies++
					}
					copying[ev.Obj] = true
				case events.DeclareAttackers:
					for _, id := range ev.IDs {
						o := ref.E.G.Obj(id)
						if orig(o) == "Cursed Mirror" && o.Face() != nil && o.Face().Name != "Cursed Mirror" {
							hasteAtk++ // a copied mirror attacked the turn it entered: the haste grant
						}
					}
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, castFirstPolicy(seed), RunConfig{Config: Config{}, MaxDecisions: 2000, MaxTurns: 20, Check: check})
		CloseSide(k)
		if d := res.Div; d != nil {
			div++
			if !report {
				return div, copies, reverts, hasteAtk
			}
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("seed %d: kernel stalled: %s", seed, res.Stall)
			div++
		}
	}
	return div, copies, reverts, hasteAtk
}

// TestCursedmirrorLockstep: 200 games, 0 divergences, with the optional
// election answered, the haste grant in play and the copies ended at cleanup.
func TestCursedmirrorLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	div, copies, reverts, hasteAtk := kcursedmirrorRun(t, 200, true)
	t.Logf("200 games: %d divergences, %d copies, %d ended at cleanup, %d hasted-mirror attacks", div, copies, reverts, hasteAtk)
	if copies < 100 {
		t.Errorf("%d Cursed Mirror copies; want at least 100", copies)
	}
	if reverts < 100 {
		t.Errorf("%d until-end-of-turn copies ended at cleanup; want at least 100", reverts)
	}
	if hasteAtk == 0 {
		t.Errorf("no copied Cursed Mirror ever attacked; the haste grant never showed")
	}
}

// TestCursedmirrorMutation: with the duration bound dropped some seed
// diverges (the copy outlives the turn in the kernel, gorge ends it).
func TestCursedmirrorMutation(t *testing.T) {
	dirac.DebugCloneCopyNoRevert = true
	defer func() { dirac.DebugCloneCopyNoRevert = false }()
	if div, _, _, _ := kcursedmirrorRun(t, 200, false); div == 0 {
		t.Fatal("no seed diverged with the until-end-of-turn copy never ending")
	}
}
