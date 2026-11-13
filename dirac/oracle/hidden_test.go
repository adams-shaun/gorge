package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Hidden$ True on a Defined$ ChangeZone lockstep: Corpses
// of the Lost's end-step "pay 1 life: return it to hand" carries Hidden$ True
// beside Defined$ Self. gorge's hidden pick needs cz.Defined == "", so the
// move is the plain object path (no choose, reveal or shuffle); the kernel
// lowers it as the plain bounce. Shock kills creatures, which makes Corpses'
// "descended" true. Zuko, Conflicted charm mode 4 exiles it,
// then returns it with GainControl$ ChosenPlayer after a one-opponent
// ChoosePlayer: it enters under the opponent's control (kop.RetUnderOpponent).
var (
	khiddenBR = []cardset.Entry{
		{Name: "Corpses of the Lost", Count: 4}, {Name: "Zuko, Conflicted", Count: 4},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 4},
		{Name: "Shock", Count: 6}, {Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 8},
	}
	// the second seat plays a second list (fewer Bears, more Hill Giant) so the two orders are different games
	khiddenBR2 = []cardset.Entry{
		{Name: "Corpses of the Lost", Count: 4}, {Name: "Zuko, Conflicted", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 6},
		{Name: "Shock", Count: 6}, {Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 8},
	}
	khiddenMatchups = [][2][]cardset.Entry{{khiddenBR, khiddenBR2}, {khiddenBR2, khiddenBR}}
)

type khiddenTally struct {
	games, corpses, zuko int
}

// khiddenCheck counts, from gorge's event log, Corpses of the Lost moving
// battlefield -> hand and Zuko, Conflicted entering from exile under a
// player other than its owner.
func khiddenCheck(tal *khiddenTally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	seen := 0
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, seen = ref, 0
			tal.games++
		}
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			ev := evs[seen]
			if ev.Kind != events.MoveZone {
				continue
			}
			o := ref.E.G.Obj(ev.Obj)
			if o == nil || o.Face() == nil {
				continue
			}
			if ev.From == state.ZBattlefield && ev.To == state.ZHand && o.Face().Name == "Corpses of the Lost" {
				tal.corpses++
			}
			if ev.From == state.ZExile && ev.To == state.ZBattlefield && o.Face().Name == "Zuko, Conflicted" && o.Controller != o.Owner {
				tal.zuko++
			}
		}
		return nil
	}
}

// khiddenRun plays seeds games of every matchup in lockstep and returns the
// divergence and kernel-stall counts.
func khiddenRun(t *testing.T, seeds uint64, mutate func(*kop.Table), check func(*GorgeSide, Side) *Divergence, stopAtDiv bool) (divs, stalls int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("hidden")
	if err != nil {
		t.Fatal(err)
	}
	for mi, md := range khiddenMatchups {
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
		if mutate != nil {
			mutate(tbl)
		}
		for seed := uint64(1); seed <= seeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0, PassElision: true}
			m, err := NewMatchup(reg, [2]string{"khidden-a", "khidden-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, pkPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 40, Check: check})
			CloseSide(k)
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls++
				t.Errorf("matchup %d seed %d: kernel stall %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				divs++
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Logf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				if stopAtDiv {
					return
				}
			}
		}
	}
	return
}

func TestHiddenLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	var tal khiddenTally
	divs, stalls := khiddenRun(t, 300, nil, khiddenCheck(&tal), false)
	t.Logf("%d games: %d Corpses of the Lost returned to hand, %d Zuko exile->battlefield under the opponent; %d divergences, %d kernel stalls", tal.games, tal.corpses, tal.zuko, divs, stalls)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d kernel stalls", divs, stalls)
	}
	if tal.corpses < 10 {
		t.Errorf("Corpses of the Lost battlefield->hand moves %d; want >= 10", tal.corpses)
	}
	if tal.zuko < 3 {
		t.Errorf("Zuko, Conflicted exile->battlefield under the opponent %d; want >= 3", tal.zuko)
	}
}

// TestHiddenMutation: deleting the move-to-hand instruction from Corpses of
// the Lost's triggered ability makes some seed diverge.
func TestHiddenMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	mut := func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			if cards[i].Types&kop.TEnchantment == 0 {
				continue // Corpses of the Lost is the only enchantment
			}
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				if abs[j].Kind != kop.AbTriggered {
					continue
				}
				kept := make([]kop.Instr, 0, len(abs[j].Prog))
				for _, in := range abs[j].Prog {
					if in.Op == kop.OpBounce {
						continue
					}
					kept = append(kept, in)
				}
				abs[j].Prog = kept
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	}
	if divs, _ := khiddenRun(t, 128, mut, nil, true); divs == 0 {
		t.Error("mutation (Corpses of the Lost move to hand deleted): no seed diverged")
	}
}

// With nothing elided gorge still poses Zuko's "choose an opponent" over its one
// opponent, and the kernel asks nothing: the oracle answers the forced choice.
func TestForcedPlayerChoiceLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	deck := []cardset.Entry{
		{Name: "Zuko, Conflicted", Count: 16}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Swamp", Count: 10}, {Name: "Mountain", Count: 10},
	}
	lockstepRun(t, "hidden", [2][]cardset.Entry{deck, deck}, 40, true, nil)
}

// TestHiddenZukoMutation: dropping RetUnderOpponent from Zuko, Conflicted's
// return (it then enters under its owner) makes some seed diverge.
func TestHiddenZukoMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	mut := func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				prog := append([]kop.Instr(nil), abs[j].Prog...)
				for k := range prog {
					if prog[k].Op == kop.OpReturnEx && prog[k].B.N&kop.RetUnderOpponent != 0 {
						prog[k].B = kop.Const(0)
					}
				}
				abs[j].Prog = prog
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	}
	if divs, _ := khiddenRun(t, 128, mut, nil, true); divs == 0 {
		t.Error("mutation (Zuko returns under its owner): no seed diverged")
	}
}
