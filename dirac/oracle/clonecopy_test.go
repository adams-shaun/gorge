package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The DB$ Clone lockstep: Fleeting Reflection, Shuri, Deepfathom
// Echo, Oko the Ringleader, Silent Hallcreeper and Naga Fleshcrafter make a
// battlefield permanent a copy of another one (until end of turn, or for good)
// as a layer-1 copy (Ext CloneCopy); the cleanup step ends the until-end-of-turn
// ones. The expected answer at every decision is gorge's.
//
// Brokers Veteran (a dies trigger) is in both decks: a copy of it that dies has
// the copied card's trigger (CR 603.10a). Not two legendary permanents of one
// name (Oko and Shuri are one per deck): when a legend-rule ask is answered in
// the cleanup step gorge goes on to the next turn, where CR 514.3a gives the
// active player priority first (see the report's gorge findings).
var kclonecopyBase = [2][]cardset.Entry{
	{
		{Name: "Oko, the Ringleader", Count: 1}, {Name: "Deepfathom Echo", Count: 2}, {Name: "Naga Fleshcrafter", Count: 2},
		{Name: "Fleeting Reflection", Count: 2}, {Name: "Silent Hallcreeper", Count: 3}, {Name: "Shuri, Wakandan Inventor", Count: 1},
		{Name: "Mind Stone", Count: 2}, {Name: "Horn of Gondor", Count: 1}, {Name: "Brokers Veteran", Count: 3},
		{Name: "Llanowar Elves", Count: 2}, {Name: "Gnarlid Colony", Count: 1}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Pestermite", Count: 1}, {Name: "Prodigal Sorcerer", Count: 1}, {Name: "Island", Count: 9}, {Name: "Forest", Count: 7},
	},
	{
		{Name: "Silent Hallcreeper", Count: 3}, {Name: "Naga Fleshcrafter", Count: 2}, {Name: "Fleeting Reflection", Count: 2},
		{Name: "Shuri, Wakandan Inventor", Count: 1}, {Name: "Mind Stone", Count: 2}, {Name: "Horn of Gondor", Count: 1},
		{Name: "Doom Blade", Count: 2}, {Name: "Murder", Count: 1}, {Name: "Brokers Veteran", Count: 3},
		{Name: "Pestermite", Count: 3}, {Name: "Prodigal Sorcerer", Count: 4}, {Name: "Island", Count: 9}, {Name: "Swamp", Count: 7},
	},
}

// kclonecopyPoly is the base pair with 4 True Polymorph for 4 basics in each
// deck: a two-target permanent copy (any artifact or creature), which with
// Oko's -5 makes token copies of permanents that are themselves copies (gorge
// mints those on the copied permanent's printed card: cloneTokenBasis).
var kclonecopyPoly = [2][]cardset.Entry{
	append([]cardset.Entry{{Name: "True Polymorph", Count: 4}}, kclonecopySwap(kclonecopyBase[0], "Forest", 3)...),
	append([]cardset.Entry{{Name: "True Polymorph", Count: 4}}, kclonecopySwap(kclonecopyBase[1], "Swamp", 3)...),
}

// kclonecopyDecks are the lockstep's deck pairs: the base pair, the same pair
// with the seats swapped (Oko, Deepfathom Echo and Elvish Visionary on seat 1),
// and the True Polymorph mix.
var kclonecopyDecks = [][2][]cardset.Entry{
	kclonecopyBase,
	{kclonecopyBase[1], kclonecopyBase[0]},
	kclonecopyPoly,
}

// kclonecopySwap is deck d with name's count set to n.
func kclonecopySwap(d []cardset.Entry, name string, n int) []cardset.Entry {
	out := append([]cardset.Entry(nil), d...)
	for i := range out {
		if out[i].Name == name {
			out[i].Count = n
		}
	}
	return out
}

// kclonecopyStallOK: the stalls a game may end in, none of them a divergence.
// clonelayer, board and tables are kernel limits the decks reach with a creature
// the copiers can copy in play more often.
func kclonecopyStallOK(stall string) bool {
	for _, r := range []string{"clonelayer", "board", "tables"} {
		if strings.HasSuffix(stall, r) {
			return true
		}
	}
	return false
}

// kclonecopyCounts is what the run did.
type kclonecopyCounts struct {
	copies, okoAtk, deepCopies, copyLeft int
	stalls                               int // games that ended in a kernel stall (kclonecopyStallOK)
	st                                   dirac.CloneCopyStats
}

func kclonecopyRun(t *testing.T, decks [2][]cardset.Entry, n uint64, report bool) (div int, c kclonecopyCounts) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("clonecopy")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range decks {
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
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	dirac.DebugCloneCopyStats = &c.st
	defer func() { dirac.DebugCloneCopyStats = nil }()
	for seed := uint64(1); seed <= n; seed++ {
		m, err := NewMatchup(reg, [2]string{"cc-a", "cc-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		if seed%4 == 0 {
			cfg = Config{GorgeAutopaySurface: true}
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := castFirstPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		seen := 0
		copying := map[state.ObjID]bool{} // battlefield permanents that are copies (a ClonePermanent basis)
		orig := func(ref *GorgeSide, id events.Event) string {
			if o := ref.E.G.Obj(id.Obj); o != nil && o.Card != nil && int(o.FaceIdx) < len(o.Card.Faces) {
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
						delete(copying, ev.Obj) // a settle's clear (its re-base follows)
						break
					}
					copying[ev.Obj] = true
					c.copies++
					if orig(ref, ev) == "Deepfathom Echo" {
						c.deepCopies++
					}
				case events.MoveZone:
					if ev.From == state.ZBattlefield && copying[ev.Obj] {
						c.copyLeft++ // a copy leaves: removePerm unsubscribes the copied card too
						delete(copying, ev.Obj)
					}
				case events.DeclareAttackers:
					for _, id := range ev.IDs {
						if o := ref.E.G.Obj(id); o != nil && o.Card != nil && int(o.FaceIdx) < len(o.Card.Faces) &&
							o.Card.Faces[o.FaceIdx].Name == "Oko, the Ringleader" && o.Face() != nil && o.Face().Name != "Oko, the Ringleader" {
							c.okoAtk++
						}
					}
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 20, Check: check})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			if kclonecopyStallOK(res.Stall) {
				c.stalls++
			} else {
				t.Errorf("seed %d: kernel stalled: %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			div++
			if !report {
				return div, c
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return div, c
}

// TestClonecopyLockstep: 256 games per deck pair, 0 divergences, 0 stalls,
// with real until-end-of-turn and permanent copies.
func TestClonecopyLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	// not parallel: it sets dirac.DebugCloneCopyStats
	var tot kclonecopyCounts
	for v, d := range kclonecopyDecks {
		_, c := kclonecopyRun(t, d, 256, true)
		t.Logf("pair %d, 256 games: %d copy events, %d EOT (%d reverted), %d permanent, %d Oko attacked as a copy, %d Naga with 2+ becomers, %d Optional$ elections, %d copies left the battlefield, %d token copies of a copy",
			v, c.copies, c.st.EOT, c.st.Reverted, c.st.Perm, c.okoAtk, c.st.Multi, c.st.Asked, c.copyLeft, c.st.Minted)
		tot.copies += c.copies
		tot.okoAtk += c.okoAtk
		tot.copyLeft += c.copyLeft
		if v < 2 { // the base pairs: every Deepfathom copy is an accepted election
			tot.deepCopies += c.deepCopies
			tot.st.Asked += c.st.Asked
		}
		tot.st.Moved += c.st.Moved
		tot.st.EOT += c.st.EOT
		tot.st.Reverted += c.st.Reverted
		tot.st.Perm += c.st.Perm
		tot.st.Multi += c.st.Multi
		tot.st.Minted += c.st.Minted
		tot.stalls += c.stalls
		tot.st.Stacked += c.st.Stacked
		tot.st.Rebased += c.st.Rebased
	}
	declined := int(tot.st.Asked) - tot.deepCopies
	t.Logf("all pairs: %d reverted, %d permanent, %d Oko attacks, %d Naga multi, %d declined (base pairs), %d copies left, %d minted, %d stacked, %d re-based",
		tot.st.Reverted, tot.st.Perm, tot.okoAtk, tot.st.Multi, declined, tot.copyLeft, tot.st.Minted, tot.st.Stacked, tot.st.Rebased)
	t.Logf("%d optional triggers asked after their source became a copy", tot.st.Moved)
	t.Logf("%d of %d games ended in a stall", tot.stalls, 3*256)
	if tot.stalls > 3*256/8 {
		t.Errorf("games stalled %d (at most %d)", tot.stalls, 3*256/8)
	}
	if tot.st.Moved < 1 {
		t.Errorf("no optional trigger was asked after its source became a copy (want at least 1)")
	}
	if tot.st.Stacked < 100 || tot.st.Rebased < 50 {
		t.Errorf("stacked %d (want 100), re-based %d (50)", tot.st.Stacked, tot.st.Rebased)
	}
	if tot.st.Reverted < 40 || tot.st.Perm < 10 || tot.okoAtk < 10 || tot.st.Multi < 5 || declined < 10 || tot.copyLeft < 20 || tot.st.Minted < 3 {
		t.Errorf("reverted %d (want 40), permanent %d (10), Oko attacks %d (10), Naga multi %d (5), declined %d (10), copies left %d (20), minted %d (3)",
			tot.st.Reverted, tot.st.Perm, tot.okoAtk, tot.st.Multi, declined, tot.copyLeft, tot.st.Minted)
	}
}

// TestClonecopyMutation: an until-end-of-turn copy never ended diverges.
func TestClonecopyMutation(t *testing.T) {
	// not parallel: it flips dirac.DebugCloneCopyNoRevert
	dirac.DebugCloneCopyNoRevert = true
	defer func() { dirac.DebugCloneCopyNoRevert = false }()
	if n, _ := kclonecopyRun(t, kclonecopyBase, 64, false); n == 0 {
		t.Fatal("no seed diverged with the copies never ended")
	}
}

// TestClonecopyMutationNoRebase: a cleanup that drops the permanent copy
// layer under an ended until-end-of-turn one diverges.
func TestClonecopyMutationNoRebase(t *testing.T) {
	// not parallel: it flips dirac.DebugCloneCopyNoRebase
	dirac.DebugCloneCopyNoRebase = true
	defer func() { dirac.DebugCloneCopyNoRebase = false }()
	n := 0
	for _, d := range kclonecopyDecks {
		m, _ := kclonecopyRun(t, d, 128, false)
		n += m
	}
	if n == 0 {
		t.Fatal("no seed diverged with the permanent layer never re-based")
	}
}
