package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Airbend lockstep (CR 701.65a): the eight api:Airbend
// carriers against a deck of bodies and a token maker. The games reach:
//
//   - the airbend exile of a battlefield permanent (the MoveZone gorge marks
//     Counter "airbend");
//   - its owner's recast from exile for {2} (canonical ActCastAirbend), also
//     when the owner is the airbender's opponent;
//   - a token airbent: it ceases to exist in exile and is never offered;
//   - a creature kept off the offer by its sorcery timing while the owner's
//     pool could pay {2}.
//
// The counters come from the gorge event log and gorge's own option list.
var kairbendDecks = [2][]cardset.Entry{
	{
		{Name: "Airbending Lesson", Count: 4}, {Name: "Aang, the Last Airbender", Count: 3},
		{Name: "Airbender Ascension", Count: 2},
		{Name: "Glider Staff", Count: 3}, {Name: "Airbender's Reversal", Count: 2},
		{Name: "Appa, Loyal Sky Bison", Count: 2},
		{Name: "Opt", Count: 2}, {Name: "Stroke of Genius", Count: 4},
		{Name: "Plains", Count: 9}, {Name: "Island", Count: 9},
	},
	{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Dragon Fodder", Count: 3}, {Name: "Blaze", Count: 4},
		{Name: "Mountain", Count: 8}, {Name: "Forest", Count: 9},
	},
}

type kairbendTally struct {
	evAt      int
	pending   []state.ObjID
	bender    map[state.ObjID]state.PlayerID // airbent object -> the airbender
	tokens    map[state.ObjID]bool
	bent      int // airbent permanents (cards and tokens)
	tokenBent int
	recasts   int // casts of an airbent card from exile
	oppCasts  int // ... by the airbender's opponent
	tokOffers int // airbend_cast options naming a token (must stay 0)
	badOffers int // airbend_cast options naming no live airbent card (must stay 0)
	timingOff int // frames where an airbent sorcery-speed card is not offered at instant timing with {2} floating
	offers    int
	cast      int // picks of airbend_cast
}

func (w *kairbendTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < w.evAt {
		w.evAt = 0
	}
	for j := w.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch {
		case e.Kind == events.MoveZone && e.To == state.ZExile && e.Counter == "airbend":
			w.bent++
			w.pending = append(w.pending, e.Obj)
			if o := g.Obj(e.Obj); o == nil || o.IsToken {
				w.tokens[e.Obj] = true
				w.tokenBent++
			}
			delete(w.bender, e.Obj)
		case e.Kind == events.ElementalBend && e.Text == "air":
			for _, id := range w.pending {
				w.bender[id] = e.Player
			}
			w.pending = w.pending[:0]
		case e.Kind == events.MoveZone && e.From == state.ZExile:
			delete(w.bender, e.Obj)
		}
	}
	w.evAt = len(evs)
	fr := ref.fr
	if fr == nil || fr.d == nil {
		return nil
	}
	offered := map[state.ObjID]bool{}
	for _, o := range fr.d.Options {
		if o.Kind != "cast" || o.Mode != "airbend_cast" {
			continue
		}
		w.offers++
		offered[o.Obj] = true
		if w.tokens[o.Obj] {
			w.tokOffers++
		}
		if _, ok := w.bender[o.Obj]; !ok {
			w.badOffers++
		}
	}
	p := fr.d.Player
	if int(p) < len(g.Players) && g.Players[p].Pool.Total() >= 2 {
		for _, id := range g.Zone(state.ZExile, p) {
			o := g.Obj(id)
			if _, ok := w.bender[id]; !ok || o == nil || o.IsToken || offered[id] {
				continue
			}
			if f := o.Face(); f != nil && !f.IsInstant() && !(g.Step.IsMain() && len(g.Stack) == 0 && g.Active == p) {
				w.timingOff++
			}
		}
	}
	return nil
}

// owns reports whether player p has a live airbent card in their exile.
func (w *kairbendTally) owns(g *state.Game, p state.PlayerID) bool {
	for _, id := range g.Zone(state.ZExile, p) {
		if _, ok := w.bender[id]; ok {
			return true
		}
	}
	return false
}

// kairbendPolicy casts an airbent card whenever one is offered, else plays
// at random; it counts the picks.
func kairbendPolicy(seed uint64, w *kairbendTally) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		for _, a := range c.D.Options {
			if a.Kind() != ActCastAirbend {
				continue
			}
			w.cast++
			if g := c.G; g != nil {
				for _, pe := range g.fr.prio {
					if pe.a != a || pe.opt >= len(g.fr.d.Options) {
						continue
					}
					id := g.fr.d.Options[pe.opt].Obj
					if b, ok := w.bender[id]; ok {
						w.recasts++
						if o := g.E.G.Obj(id); o != nil && o.Owner != b {
							w.oppCasts++
						}
					}
				}
			}
			return a
		}
		// an {X} spell's X=0 leaves the whole witness floating (faces.go):
		// announce 0 whenever asked, and with an airbent card of the
		// player's own in exile prefer casting an {X} spell
		for _, a := range c.D.Options {
			if a.Kind() == ActChoose && a.Mode() == 0 {
				return a
			}
		}
		if g := c.G; g != nil && w.owns(g.E.G, state.PlayerID(c.D.Player)) {
			var buf []Canon
			for _, pe := range g.fr.prio {
				if pe.a.Kind() != ActCast || pe.opt >= len(g.fr.d.Options) {
					continue
				}
				if l := g.fr.d.Options[pe.opt].Label; strings.Contains(l, "Stroke of Genius") || strings.Contains(l, "Blaze") {
					buf = append(buf, pe.a)
				}
			}
			if len(buf) > 0 {
				d := *c.D
				d.Options = buf
				return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
		}
		return rnd.Pick(c)
	})
}

func TestAirbendLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("airbend")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kairbendDecks {
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
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kairbendDecks[0], kairbendDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	w := &kairbendTally{bender: map[state.ObjID]state.PlayerID{}, tokens: map[state.ObjID]bool{}}
	games, stalls := 0, 0
	for seed := uint64(1); seed <= 256; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"airbend-a", "airbend-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		w.evAt, w.pending = 0, w.pending[:0]
		res := Lockstep(m, seed, k, kairbendPolicy(seed, w), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: w.check})
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		CloseSide(k)
		games++
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games: %d airbent (%d tokens), %d recasts (%d by the airbender's opponent), %d offers, %d picks, %d timing-forbidden frames, %d stalls",
		games, w.bent, w.tokenBent, w.recasts, w.oppCasts, w.offers, w.cast, w.timingOff, stalls)
	if stalls != 0 {
		t.Errorf("%d games stalled", stalls)
	}
	if w.bent < 40 {
		t.Errorf("%d airbent permanents, want at least 40", w.bent)
	}
	if w.recasts < 15 {
		t.Errorf("%d airbent recasts, want at least 15", w.recasts)
	}
	if w.oppCasts < 10 {
		t.Errorf("%d recasts by the airbender's opponent, want at least 10", w.oppCasts)
	}
	if w.tokenBent < 5 {
		t.Errorf("%d airbent tokens, want at least 5", w.tokenBent)
	}
	if w.timingOff < 5 {
		t.Errorf("%d timing-forbidden frames, want at least 5", w.timingOff)
	}
	if w.tokOffers != 0 {
		t.Errorf("%d airbend_cast offers named a token, want 0", w.tokOffers)
	}
	if w.badOffers != 0 {
		t.Errorf("%d airbend_cast offers named a card that is not airbent, want 0", w.badOffers)
	}
}
