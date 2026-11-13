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

// The lockstep: constructed matchups over the generated zonebatchclone pool.
// Kaya, Spirits' Justice is the carrier of all four abilities:
//
//   - +2: surveil 2, then exile a card from a graveyard;
//   - +1: a 1/1 Spirit token;
//   - -2: exile a creature you control and up to one creature the other
//     player controls (TargetsForEachPlayer$ over PlayerCountOther$);
//   - the exile trigger: a creature card among the exiled batch (battlefield
//     creatures and graveyard creature cards) is picked, and the target token
//     becomes a copy of it with flying until end of turn.
//
// Swords to Plowshares and Doom Blade put creatures into exile and graveyards.

var zbcDeck = []cardset.Entry{
	{Name: "Kaya, Spirits' Justice", Count: 4}, {Name: "Swords to Plowshares", Count: 3}, {Name: "Doom Blade", Count: 3},
	{Name: "Grizzly Bears", Count: 6}, {Name: "Memnite", Count: 4}, {Name: "Ornithopter", Count: 2},
	{Name: "Plains", Count: 9}, {Name: "Swamp", Count: 9},
}

const zbcKaya = "Kaya, Spirits' Justice"

func zbcSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card, *kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("zonebatchclone")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range zbcDeck {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(zbcDeck, zbcDeck)
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, decks, tbl
}

// zbcScan counts, from gorge's event log, what Kaya's abilities did: the
// latest pushed trigger or ability is Kaya's, and the event is its move.
func zbcScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	name := func(id state.ObjID) string {
		if ob := g.E.G.Obj(id); ob != nil && ob.Face() != nil {
			return ob.Face().Name
		}
		return ""
	}
	var src state.ObjID
	var kind events.Kind
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.TriggerPush, events.AbilityPush:
			if name(ev.Obj) == zbcKaya {
				src, kind = ev.Obj, ev.Kind
			} else {
				src = 0
			}
		case events.PutOnStack, events.StepChange, events.TurnChange:
			src = 0
		case events.Surveil:
			if src != 0 && kind == events.AbilityPush {
				n["kaya:surveil"]++
			}
		case events.MoveZone:
			if src == 0 || ev.To != state.ZExile {
				continue
			}
			switch {
			case kind == events.AbilityPush && ev.From == state.ZGraveyard:
				n["kaya:gravepick"]++
			case kind == events.AbilityPush && ev.From == state.ZBattlefield:
				if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Owner == g.E.G.Obj(src).Owner {
					n["kaya:exile-own"]++
				} else {
					n["kaya:exile-opp"]++
				}
			}
		case events.ClonePermanent:
			if src != 0 && kind == events.TriggerPush {
				n["kaya:copy"]++
			}
		case events.TokenCreate:
			if src != 0 && kind == events.AbilityPush {
				n["kaya:token"]++
			}
		}
	}
}

func TestZonebatchcloneLockstep(t *testing.T) {
	t.Parallel()
	reg, set, decks, tbl := zbcSetup(t)
	played := map[string]int{}
	n := map[string]int{}
	stalls := map[string]int{}
	games := 0
	for seed := uint64(1); seed <= 256; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"zbc-a", "zbc-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		res := Lockstep(m, seed, k, trackingEagerPolicy(seed, played, &gs), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls[res.Stall]++
		}
		zbcScan(gs, n)
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games, counts %v; kernel stalls %v; carrier plays %v", games, n, stalls, played)
	for _, w := range []struct {
		key string
		min int
	}{{"kaya:surveil", 10}, {"kaya:gravepick", 5}, {"kaya:token", 10}, {"kaya:exile-own", 5}, {"kaya:exile-opp", 3}, {"kaya:copy", 5}} {
		if n[w.key] < w.min {
			t.Errorf("%s = %d, want at least %d", w.key, n[w.key], w.min)
		}
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
}

// TestZonebatchcloneMutation rewrites Kaya's clone pick to a battlefield pool
// (the batch flag cleared on both the pick and the copy) and requires the
// lockstep to diverge: gorge picks among the exiled cards.
func TestZonebatchcloneMutation(t *testing.T) {
	reg, set, decks, tbl := zbcSetup(t)
	mut := *tbl
	mut.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	hit := 0
	for i := range mut.Cards {
		if mut.Cards[i].Name != zbcKaya {
			continue
		}
		abs := append([]kop.Ability(nil), mut.Cards[i].Abilities...)
		for j := range abs {
			prog := append([]kop.Instr(nil), abs[j].Prog...)
			for p := range prog {
				if prog[p].Ext() == kop.ExtCloneCopy && prog[p].B.N&kop.CloneBatch != 0 {
					prog[p].B.N &^= kop.CloneBatch
					hit++
				}
			}
			abs[j].Prog = prog
		}
		mut.Cards[i].Abilities = abs
	}
	if hit != 2 {
		t.Fatalf("%d batch clone instructions in Kaya's table, want the pick and the copy", hit)
	}
	for seed := uint64(1); seed <= 128; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"zbc-a", "zbc-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, &mut, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		res := Lockstep(m, seed, k, trackingEagerPolicy(seed, map[string]int{}, &gs), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		if res.Div != nil {
			t.Logf("diverged at seed %d decision %d: %s %s", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatal("battlefield-pool mutation did not diverge in 128 seeds")
}
