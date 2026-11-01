package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// gorgeBraidedFixTime is the commit time (UTC, a Go pseudo-version stamp) of
// gorge (e10ad8b0d registers an Effect-delivered
// CantBeActivated; 026d67928 makes its Duration$ UntilTargetedUntaps outlive
// the turn and the Net). Before it the lock is a Note, or ends early.
const gorgeBraidedFixTime = "20261026220850"

// The Braided Net // Braided Quipu lockstep: the Net
// enters with three net counters; {T}, remove a net counter taps another
// target nonland permanent and, for as long as it stays tapped, its activated
// abilities can't be activated (gorge Effect Mode$ CantBeActivated, Duration$
// UntilTargetedUntaps, CR 602.5, 611.2b: the lock ends at its first untap or
// departure and does not follow the source). Craft with an artifact turns it
// into Braided Quipu, whose {3}{U},{T} draws a card per artifact and then
// puts it third from the top of its owner's library (LibraryPosition$ 2).
// The expected answer at every decision is gorge's.
var kbraidedDecks = [][2][]cardset.Entry{{
	{
		{Name: "Braided Net", Count: 8}, {Name: "Ornithopter", Count: 6}, {Name: "Bonesplitter", Count: 2},
		{Name: "Hill Giant", Count: 4}, {Name: "Island", Count: 14}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Braided Net", Count: 6}, {Name: "Ornithopter", Count: 3}, {Name: "Spikeshot Elder", Count: 5},
		{Name: "Flame Spirit", Count: 4}, {Name: "Prodigal Sorcerer", Count: 4}, {Name: "Shock", Count: 2},
		{Name: "Mountain", Count: 8}, {Name: "Island", Count: 8},
	},
}, {
	{
		{Name: "Braided Net", Count: 8}, {Name: "Ornithopter", Count: 4}, {Name: "Shivan Dragon", Count: 2},
		{Name: "Flame Spirit", Count: 6}, {Name: "Island", Count: 12}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "Braided Net", Count: 8}, {Name: "Ornithopter", Count: 4}, {Name: "Spikeshot Elder", Count: 6},
		{Name: "Prodigal Sorcerer", Count: 4}, {Name: "Island", Count: 10}, {Name: "Mountain", Count: 8},
	},
}}

// kbraidedTally counts, on gorge's side, the locks the Net applied, the
// decisions at which a locked permanent held a non-mana activated ability the
// lock withheld, the crafts and the Quipu activations.
type kbraidedTally struct {
	locks, held, flips, quipu, games int
}

func kbraidedRun(t *testing.T, n uint64, report bool, tal *kbraidedTally) int {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("braided")
	if err != nil {
		t.Fatal(err)
	}
	div := 0
	for mi, md := range kbraidedDecks {
		var decks [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], c)
				}
			}
			if len(decks[s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= n; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"bn-a", "bn-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			seen := 0
			armed := false
			locked := map[state.ObjID]bool{}
			check := func(ref *GorgeSide, _ Side) *Divergence {
				if tal == nil {
					return nil
				}
				evs := ref.E.L.Events
				for ; seen < len(evs); seen++ {
					switch ev := evs[seen]; ev.Kind {
					case events.AbilityPush:
						armed = false
						if o := ref.E.G.Obj(ev.Obj); o != nil && o.Face() != nil {
							switch o.Face().Name {
							case "Braided Net":
								armed = true
							case "Braided Quipu":
								tal.quipu++
							}
						}
					case events.Tap:
						if armed {
							armed = false
							locked[ev.Obj] = true
							tal.locks++
						}
					case events.Untap:
						delete(locked, ev.Obj)
					case events.MoveZone:
						if ev.From == state.ZBattlefield {
							delete(locked, ev.Obj)
						}
					case events.FlipFace:
						tal.flips++
					}
				}
				for id := range locked {
					o := ref.E.G.Obj(id)
					if o == nil || o.Zone != state.ZBattlefield || !o.Tapped || o.Face() == nil {
						continue
					}
					for _, ab := range o.Face().Abilities {
						if ab.Kind == "AB" && ab.API != "Mana" {
							tal.held++
							break
						}
					}
				}
				return nil
			}
			res := Lockstep(m, seed, k, pkPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 40, Check: check})
			CloseSide(k)
			if tal != nil {
				tal.games++
			}
			if res.Stall != "" && res.Stall != "turns" {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				div++
				if !report {
					t.Logf("matchup %d seed %d diverges (decision %d: %s)", mi, seed, d.At, d.What)
					return div
				}
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return div
}

func TestBraidedLockstep(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeBraidedFixTime) {
		t.Skip("gorge before the Effect CantBeActivated fix (86cc3f752)")
	}
	var tal kbraidedTally
	kbraidedRun(t, 300, true, &tal)
	t.Logf("%d games: locks %d, held-out decisions %d, crafts %d, Quipu activations %d", tal.games, tal.locks, tal.held, tal.flips, tal.quipu)
	if tal.locks < 100 || tal.held < 100 || tal.flips < 30 || tal.quipu < 20 {
		t.Errorf("locks %d, held-out decisions %d, crafts %d, Quipu activations %d; want at least 100, 100, 30 and 20", tal.locks, tal.held, tal.flips, tal.quipu)
	}
}

// TestBraidedLockstepMutation: with nothing ever locked, with the lock
// surviving an untap, and with Quipu going second from the top, some seed
// diverges.
func TestBraidedLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	if !gorgePinAtLeast(t, gorgeBraidedFixTime) {
		t.Skip("gorge before the Effect CantBeActivated fix (86cc3f752)")
	}
	for _, m := range []uint8{1, 2, 3} {
		dirac.SetDebugNetMutate(m)
		n := kbraidedRun(t, 100, false, nil)
		dirac.SetDebugNetMutate(0)
		if n == 0 {
			t.Errorf("mutation %d: no seed diverged", m)
		}
	}
}
