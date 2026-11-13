package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Warp lockstep (CR 702.185): constructed matchups over the generated warp
// pool, whose every supported warp card is cheap to warp and expensive to
// cast, so the games reach all five arms:
//
//   - a hand warp cast (canonical "warped" mode) at sorcery timing;
//   - the end-step delayed exile of the warped permanent (battlefield->exile);
//   - the recast from exile on a LATER turn (canonical "warp_recast");
//   - no recast offer in the exile turn itself (the same-turn gate);
//   - no recast offer for a warp card exiled by another effect (Tormod's
//     Crypt exiles one from a graveyard).
//
// The counters come from the gorge event log and gorge's own option list, so
// the test fails if the kernel stops matching any arm. A mutation table with
// the keyword cleared must diverge, proving the counters are not vacuous.
var warpDecks = [][2][]cardset.Entry{{
	{
		{Name: "Haliya, Guided by Light", Count: 4}, {Name: "Rayblade Trooper", Count: 4},
		{Name: "All-Fates Stalker", Count: 4}, {Name: "Knight Luminary", Count: 4},
		{Name: "Codecracker Hound", Count: 4}, {Name: "Sinister Cryologist", Count: 4},
		{Name: "Starbreach Whale", Count: 4}, {Name: "Plains", Count: 6}, {Name: "Island", Count: 6},
	},
	{
		{Name: "Memorial Team Leader", Count: 4}, {Name: "Red Tiger Mechan", Count: 4},
		{Name: "Weftstalker Ardent", Count: 4}, {Name: "Nova Hellkite", Count: 4},
		{Name: "Bygone Colossus", Count: 4}, {Name: "Blaze", Count: 4},
		{Name: "Tormod's Crypt", Count: 4}, {Name: "Mountain", Count: 6}, {Name: "Forest", Count: 6},
	},
}}

// warpNames is the set of warp cards the decks use (the pool's other warp
// cards are refused by kgen for unrelated abilities).
var warpNames = map[string]bool{
	"Haliya, Guided by Light": true, "Rayblade Trooper": true, "All-Fates Stalker": true,
	"Knight Luminary": true, "Codecracker Hound": true, "Sinister Cryologist": true,
	"Starbreach Whale": true, "Memorial Team Leader": true, "Red Tiger Mechan": true,
	"Weftstalker Ardent": true, "Nova Hellkite": true, "Bygone Colossus": true,
}

// warpTally counts the five arms. The event cursor mirrors c4_test's: the log
// only grows, but resets when a side is rebuilt (a reset rewinds the cursor).
type warpTally struct {
	evAt      int
	exileTurn map[state.ObjID]int32
	nonWarp   map[state.ObjID]bool // warp cards exiled from a graveyard (not by warp)
	handWarp  int                  // warp-cost hand casts picked (canon mode "warped")
	exiles    int                  // end-step delayed exiles of a warp card
	nonExiles int                  // non-warp exiles of a warp card
	recasts   int                  // later-turn recasts picked (canon face mode "warp_recast")
	offers    int                  // gorge warp_recast options seen
	mapped    int                  // offers whose object is a counted exile
	nonOffers int                  // offers naming a non-warp exile (must stay 0)
	sameTurn  int                  // offers in the exile's own turn (must stay 0)
}

func (w *warpTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < w.evAt {
		w.evAt = 0
	}
	for j := w.evAt; j < len(evs); j++ {
		e := &evs[j]
		if e.Kind != events.MoveZone || e.To != state.ZExile {
			continue
		}
		o := g.Obj(e.Obj)
		if o == nil || o.Face() == nil || !warpNames[o.Face().Name] {
			continue
		}
		switch e.From {
		case state.ZBattlefield:
			w.exiles++
			delete(w.nonWarp, e.Obj)
			w.exileTurn[e.Obj] = g.Turn
		case state.ZGraveyard:
			w.nonExiles++
			w.nonWarp[e.Obj] = true
		}
	}
	w.evAt = len(evs)
	if fr := ref.fr; fr != nil && fr.d != nil {
		for _, o := range fr.d.Options {
			if o.Kind != "cast" || o.Mode != "warp_recast" {
				continue
			}
			w.offers++
			if w.nonWarp[o.Obj] {
				w.nonOffers++
			}
			if t, ok := w.exileTurn[o.Obj]; ok {
				w.mapped++
				if t == g.Turn {
					w.sameTurn++
				}
			}
		}
	}
	return nil
}

// warpPolicy prefers the warp cast and the recast whenever either is offered
// (so the games reach every arm), else plays or casts; it counts the two picks
// the tallies cannot see from the gorge side alone.
func warpPolicy(seed uint64, w *warpTally) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		// Blaze's {X} cast is lowered by a witness that taps for the
		// largest X; announcing X=0 leaves the rest floating, which pays
		// the warp recast on a later turn (faces_test's float).
		for _, a := range c.D.Options {
			if a.Kind() == ActChoose && a.Mode() == 0 {
				return a
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast:
				if a.Mode()&0x0f == 8 { // canonical "warped"
					buf = append(buf, a)
				}
			case ActCastFace:
				if a.Mode()&0x0f == 7 { // canonical "warp_recast", face 0
					buf = append(buf, a)
				}
			}
		}
		if len(buf) == 0 {
			for _, a := range c.D.Options {
				if k := a.Kind(); k == ActCast || k == ActActivate || k == ActPlayLand {
					buf = append(buf, a)
				}
			}
		}
		var a Canon
		if len(buf) == 0 {
			a = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		switch a.Kind() {
		case ActCast:
			if a.Mode()&0x0f == 8 {
				w.handWarp++
			}
		case ActCastFace:
			if a.Mode()&0x0f == 7 {
				w.recasts++
			}
		}
		return a
	})
}

// warpIgnored is the mutation: the kernel behaves as if no card had Warp.
func warpIgnored(t *kop.Table) *kop.Table {
	m := *t
	m.Cards = append([]kop.CardDef(nil), t.Cards...)
	for i := range m.Cards {
		m.Cards[i].HasWarp = false
	}
	return &m
}

func TestWarpLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("warp")
	if err != nil {
		t.Fatal(err)
	}
	games, mutationDiverged := 0, false
	var w warpTally
	w.exileTurn = map[state.ObjID]int32{}
	w.nonWarp = map[state.ObjID]bool{}
	for mi, md := range warpDecks {
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
		for seed := uint64(1); seed <= 48; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"warp-a", "warp-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := warpPolicy(seed, &w)
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: w.check})
			CloseSide(k)
			games++
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			if seed <= 8 {
				m2, err := NewMatchup(reg, [2]string{"warp-a", "warp-b"}, decks, 1)
				if err != nil {
					t.Fatal(err)
				}
				k2, err := SetSide(set, warpIgnored(tbl), cfg)(m2, seed)
				if err != nil {
					t.Fatal(err)
				}
				res2 := Lockstep(m2, seed, k2, warpPolicy(seed, &warpTally{exileTurn: map[state.ObjID]int32{}, nonWarp: map[state.ObjID]bool{}}), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
				CloseSide(k2)
				if res2.Div != nil {
					mutationDiverged = true
				}
			}
		}
	}
	t.Logf("%d games: %d warp casts, %d warp exiles, %d non-warp exiles, %d recasts, %d recast offers (%d same-turn)",
		games, w.handWarp, w.exiles, w.nonExiles, w.recasts, w.offers, w.sameTurn)
	if w.handWarp == 0 {
		t.Error("no hand warp cast was picked")
	}
	if w.exiles == 0 {
		t.Error("no end-step warp exile was logged")
	}
	if w.recasts == 0 {
		t.Error("no recast from warp exile was picked")
	}
	if w.offers == 0 {
		t.Error("gorge never offered a warp_recast")
	}
	if w.nonExiles == 0 {
		t.Error("no non-warp exile of a warp card was logged")
	}
	if w.mapped != w.offers {
		t.Errorf("%d of %d warp_recast offers name no counted exile", w.offers-w.mapped, w.offers)
	}
	if w.nonOffers != 0 {
		t.Errorf("%d recast offers named a non-warp exile, want 0", w.nonOffers)
	}
	if w.sameTurn != 0 {
		t.Errorf("%d recast offers in the exile's own turn, want 0", w.sameTurn)
	}
	if !mutationDiverged {
		t.Error("mutation 'warp cleared' never diverged")
	}
}
