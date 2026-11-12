package oracle

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The emit-fixes lockstep: five kgen emission shapes.
// Counters from gorge's event log and board:
//
//  1. a Warden of the Inner Sky with three or more counters (Count$
//     CardCounters.ALL: flying and vigilance),
//  2. End the Festivities resolving, 3. its damage to a planeswalker
//     (Ajani; the B.N bit-4 permanents),
//  4. a Repurposing Bay search putting an artifact onto the battlefield,
//  5. Sneaky Snacker returning from the graveyard on a third draw,
//  6. Tear Asunder cast kicked, 7. a kicked Tear Asunder exiling a
//     creature (its mid-resolution nonland slot).
//
// The enchantment is Pacifism, not Journey to Nowhere: gorge's Defined$
// Remembered falls back to the trigger's event capture when the source
// remembered nothing (the defect reported with
// rempersist_test.go), so a Journey exiled by Tear Asunder returns ITSELF
// from exile to the battlefield; the kernel and the rules leave it exiled.
var efDecks = map[string][]cardset.Entry{
	"art": { // W/U artifacts: Warden, Bay, a planeswalker and an enchantment to hit
		{Name: "Warden of the Inner Sky", Count: 4}, {Name: "Repurposing Bay", Count: 4}, {Name: "Ornithopter", Count: 3},
		{Name: "Memnite", Count: 1}, {Name: "Wayfarer's Bauble", Count: 2},
		{Name: "Mind Stone", Count: 2}, {Name: "Guardian Idol", Count: 2}, {Name: "Bottle Gnomes", Count: 2},
		{Name: "Pacifism", Count: 2}, {Name: "Ajani, Caller of the Pride", Count: 2},
		{Name: "Plains", Count: 9}, {Name: "Island", Count: 7},
	},
	"brg": { // the sweeper and the kicked exile
		{Name: "End the Festivities", Count: 6}, {Name: "Tear Asunder", Count: 6}, {Name: "Hill Giant", Count: 6},
		{Name: "Walking Corpse", Count: 4}, {Name: "Mountain", Count: 6}, {Name: "Forest", Count: 6}, {Name: "Swamp", Count: 6},
	},
	"snack": { // Snacker and the draws that bring it back
		{Name: "Sneaky Snacker", Count: 6}, {Name: "Divination", Count: 4}, {Name: "Opt", Count: 4}, {Name: "Think Twice", Count: 4},
		{Name: "Walking Corpse", Count: 4}, {Name: "Island", Count: 9}, {Name: "Swamp", Count: 9},
	},
}

type efTally struct {
	evAt                         int
	warden, fest, festPW, bay    int
	snack, tearKicked, tearCreat int
	festOn, tearOn               bool
}

func efName(g *state.Game, id state.ObjID) string {
	if o := g.Obj(id); o != nil && o.Face() != nil {
		return o.Face().Name
	}
	return ""
}

func (p *efTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.CastInfo:
			if efName(g, e.Obj) == "Tear Asunder" {
				for _, f := range strings.Split(e.Counter, ",") {
					if strings.TrimSpace(f) == "kicked" {
						p.tearKicked++
						p.tearOn = true
					}
				}
			}
		case events.PutOnStack:
			if efName(g, e.Obj) == "End the Festivities" {
				p.festOn = true
			}
		case events.Damage:
			if o := g.Obj(e.Obj); p.festOn && o != nil && o.Face() != nil && o.Face().Name == "Ajani, Caller of the Pride" {
				p.festPW++
			}
		case events.CounterChange:
			if efName(g, e.Obj) == "Warden of the Inner Sky" {
				if o := g.Obj(e.Obj); o != nil && o.Zone == state.ZBattlefield && o.Counter("ALL") >= 3 {
					p.warden++
				}
			}
		case events.MoveZone:
			n := efName(g, e.Obj)
			switch {
			case n == "End the Festivities" && e.From == state.ZStack:
				p.festOn = false
				p.fest++
			case n == "Tear Asunder" && e.From == state.ZStack:
				p.tearOn = false
			case e.From == state.ZLibrary && e.To == state.ZBattlefield:
				p.bay++ // the pool's only library-to-battlefield move is the Bay's
			case n == "Sneaky Snacker" && e.From == state.ZGraveyard && e.To == state.ZBattlefield:
				p.snack++
			case p.tearOn && e.From == state.ZBattlefield && e.To == state.ZExile:
				if o := g.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().IsCreature() {
					p.tearCreat++
				}
			}
		}
	}
	p.evAt = len(evs)
	return nil
}

func TestEmitfixLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("emitfix")
	if err != nil {
		t.Fatal(err)
	}
	var p efTally
	games := 0
	var matchups [][2][]cardset.Entry
	for _, d := range [][2]string{{"art", "brg"}, {"snack", "brg"}, {"art", "snack"}} {
		a, b := efDecks[d[0]], efDecks[d[1]]
		matchups = append(matchups, [2][]cardset.Entry{a, b}, [2][]cardset.Entry{b, a})
	}
	for mi, md := range matchups {
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
		var tbl *kop.Table
		if tbl, _, _, err = set.Matchup(md[0], md[1]); err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 40; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"ef-a", "ef-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := fbPolicy(seed) // plays at random among its plays
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
			p.evAt, p.festOn, p.tearOn = 0, false, false
			lead, err := NewGorgeSide(m, seed, cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			res := LockstepLed(m, seed, lead, k, pol, RunConfig{Config: cfg, MaxDecisions: 8000, MaxTurns: 80, Check: p.check})
			CloseSide(k)
			games++
			if res.Stall != "" {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				if os.Getenv("KEMIT_DEBUG") != "" {
					evs := lead.E.L.Events
					var b strings.Builder
					for j := max(len(evs)-30, 0); j < len(evs); j++ {
						e := &evs[j]
						fmt.Fprintf(&b, "%d %s p%d %d(%s) %d->%d amt=%d c=%q t=%q ids=%v\n", j, e.Kind, e.Player, e.Obj, efName(lead.E.G, e.Obj), e.From, e.To, e.Amount, e.Counter, e.Text, e.IDs)
					}
					t.Logf("gorge log tail:\n%s", b.String())
				}
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games: Warden at 3+ counters %d; Festivities %d (planeswalker hits %d); Bay searches %d; Snacker returns %d; Tear kicked %d (creatures exiled %d)",
		games, p.warden, p.fest, p.festPW, p.bay, p.snack, p.tearKicked, p.tearCreat)
	for _, c := range []struct {
		name string
		n    int
	}{{"Warden 3+ counters", p.warden}, {"Festivities", p.fest}, {"Festivities planeswalker hits", p.festPW}, {"Bay searches", p.bay},
		{"Snacker returns", p.snack}, {"kicked Tears", p.tearKicked}, {"kicked creature exiles", p.tearCreat}} {
		if c.n < 5 {
			t.Errorf("%d %s, want at least 5", c.n, c.name)
		}
	}
}
