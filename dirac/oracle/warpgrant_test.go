package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// kwarpGrantDeck: Tannuk, Steadfast Second's hand warp grant ({2}{R} for
// artifact cards and red creature cards you own) over Kavaron Harrier and
// Galvanizing Sawship (artifacts), Kavaron Skywarden and Nebula Dragon (red
// creatures), Mechanozoa and Pinnacle Emissary (printed warps the grant's cost
// neither covers nor is covered by: two warp rows), Nova Hellkite (a printed
// warp equal to the grant's: one row), Lightning Strike to take Tannuk off
// the battlefield, and Blaze, whose {X} witness announced at X=0 leaves mana
// floating (a recast and a second warp row are offered only while the pool
// pays them).
var kwarpGrantDeck = []cardset.Entry{
	{Name: "Tannuk, Steadfast Second", Count: 7}, {Name: "Kavaron Harrier", Count: 3}, {Name: "Galvanizing Sawship", Count: 1},
	{Name: "Kavaron Skywarden", Count: 2}, {Name: "Nebula Dragon", Count: 1}, {Name: "Mechanozoa", Count: 3},
	{Name: "Pinnacle Emissary", Count: 3}, {Name: "Nova Hellkite", Count: 1}, {Name: "Lightning Strike", Count: 2},
	{Name: "Blaze", Count: 3}, {Name: "Mountain", Count: 9}, {Name: "Island", Count: 5},
}

// kwarpGrantStats counts what the hand warp grant lockstep saw on gorge's side.
type kwarpGrantStats struct {
	evAt         int
	warped       map[state.ObjID]bool // grant-only cards a warp cast was picked for
	exileTurn    map[state.ObjID]int32
	grantOffers  int  // "warped" rows for a card that prints no warp
	secondRows   int  // "warped" rows at AltCostIndex 1 (the grant's beside a printed warp)
	noTannuk     int  // "warped" rows for a card that prints no warp with no Tannuk under the caster's control (must stay 0)
	bareChances  int  // priority decisions with a grant-type card in hand and no Tannuk out
	grantCasts   int  // grant-only warp casts picked
	exiles       int  // end-step exiles of a grant-warped card
	recastOffers int  // warp_recast rows for a card that prints no warp
	recasts      int  // those recasts picked
	sameTurn     int  // recast offers in the exile's own turn (must stay 0)
	blazing      bool // the policy cast Blaze to float mana: announce X = 0
}

// grantType reports whether face f is one Tannuk grants warp to and prints no
// warp of its own.
func grantType(f *cards.Face) bool {
	if f == nil || f.HasKeyword("Warp") {
		return false
	}
	if f.IsCreature() && strings.Contains(f.Colors, "R") {
		return true
	}
	for _, t := range f.Types {
		if t == "Artifact" {
			return true
		}
	}
	return false
}

// controlsTannuk reports whether p controls a Tannuk on the battlefield.
func controlsTannuk(g *state.Game, p state.PlayerID) bool {
	for q := state.PlayerID(0); q < 2; q++ {
		for _, id := range g.Zone(state.ZBattlefield, q) {
			if o := g.Obj(id); o != nil && o.Controller == p && o.Face() != nil && o.Face().Name == "Tannuk, Steadfast Second" {
				return true
			}
		}
	}
	return false
}

func (st *kwarpGrantStats) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < st.evAt {
		st.evAt = 0
	}
	for j := st.evAt; j < len(evs); j++ {
		if e := &evs[j]; e.Kind == events.MoveZone && e.From == state.ZBattlefield && e.To == state.ZExile && st.warped[e.Obj] {
			st.exiles++
			st.exileTurn[e.Obj] = g.Turn
		}
	}
	st.evAt = len(evs)
	return nil
}

// kwarpGrantPolicy takes a warp cast or a warp recast whenever one is offered,
// else any cast, play or activation, else answers at random, and tallies
// gorge's warp rows.
func kwarpGrantPolicy(seed uint64, st *kwarpGrantStats) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return rnd.Pick(c)
		}
		d := c.G.E.Pending()
		if d != nil && d.Kind != decision.KPriority && st.blazing {
			for _, a := range c.D.Options {
				if a.Kind() == ActChoose && a.Mode() == 0 {
					st.blazing = false
					return a // X = 0: the witness's other mana stays floating
				}
			}
		}
		if d == nil || d.Kind != decision.KPriority {
			return rnd.Pick(c)
		}
		g := c.G.E.G
		tannuk := controlsTannuk(g, d.Player)
		if !tannuk {
			for _, id := range g.Zone(state.ZHand, d.Player) {
				if o := g.Obj(id); o != nil && grantType(o.Face()) {
					st.bareChances++
					break
				}
			}
		}
		// the canonical options name the warp rows on both surfaces (an
		// option the pool pays, or a potential play's witness): map each
		// row's source back to the hand or exile card
		objOf := map[Ref]state.ObjID{}
		for _, z := range []state.Zone{state.ZHand, state.ZExile} {
			for _, id := range g.Zone(z, d.Player) {
				objOf[c.G.InstRef(id)] = id
			}
		}
		grantRefs := map[Ref]state.ObjID{}
		recastRefs := map[Ref]state.ObjID{}
		for _, a := range c.D.Options {
			id, ok := objOf[a.Src()]
			if !ok {
				continue
			}
			f := g.Obj(id).Face()
			switch {
			case a.Kind() == ActCast && a.Mode()&0x0f == 8: // canonical "warped"
				if a.Mode()>>4 == 1 {
					st.secondRows++
				}
				if grantType(f) {
					st.grantOffers++
					grantRefs[a.Src()] = id
					if !tannuk {
						st.noTannuk++
					}
				}
			case a.Kind() == ActCastFace && a.Mode()&0x0f == 7: // canonical "warp_recast"
				if grantType(f) {
					st.recastOffers++
					recastRefs[a.Src()] = id
					if t, ok := st.exileTurn[id]; ok && t == g.Turn {
						st.sameTurn++
					}
				}
			}
		}
		// a recast from exile and a second warp row are priority options
		// only while the floating pool pays them (the real-pool walk): float
		// land mana while one is in reach, then take it
		float := false
		for _, id := range g.Zone(state.ZExile, d.Player) {
			if t, ok := st.exileTurn[id]; ok && t < g.Turn && d.Player == g.Active && len(g.Stack) == 0 {
				float = true
			}
		}
		if tannuk && d.Player == g.Active && len(g.Stack) == 0 {
			for _, id := range g.Zone(state.ZHand, d.Player) {
				if f := g.Obj(id).Face(); f != nil && f.HasKeyword("Warp") && !grantType(f) && (f.Name == "Mechanozoa" || f.Name == "Pinnacle Emissary") {
					float = true
				}
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActCastFace && a.Mode()&0x0f == 7 || a.Kind() == ActCast && a.Mode() == 8|1<<4 {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 && float {
			for _, a := range c.D.Options {
				if id, ok := objOf[a.Src()]; ok && a.Kind() == ActCast && a.Mode() == 0 && g.Obj(id).Face().Name == "Blaze" {
					buf = append(buf, a)
				}
			}
		}
		if len(buf) == 0 {
			for _, a := range c.D.Options {
				if a.Kind() == ActCast && a.Mode()&0x0f == 8 {
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
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		dd := *c.D
		dd.Options = buf
		a := rnd.Pick(&PickCtx{D: &dd, G: c.G, Seq: c.Seq})
		if id, ok := grantRefs[a.Src()]; ok && a.Kind() == ActCast && a.Mode()&0x0f == 8 {
			st.grantCasts++
			st.warped[id] = true
		}
		if id, ok := objOf[a.Src()]; ok && a.Kind() == ActCast && a.Mode() == 0 && g.Obj(id).Face().Name == "Blaze" {
			st.blazing = true
		}
		if _, ok := recastRefs[a.Src()]; ok && a.Kind() == ActCastFace && a.Mode()&0x0f == 7 {
			st.recasts++
		}
		return a
	})
}

// TestWarpGrantLockstep: Tannuk's grant offers a warp cast at {2}{R} for an
// artifact card or a red creature card in its controller's hand only while
// Tannuk is on the battlefield; the warped permanent is exiled at the next end
// step and recast from exile on a later turn; a printed warp the grant's cost
// does not cover is a second row beside it. Both engines agree decision by
// decision (CR 702.185a).
func TestWarpGrantLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("warpgrant")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range kwarpGrantDeck {
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
	tbl, _, _, err := set.Matchup(kwarpGrantDeck, kwarpGrantDeck)
	if err != nil {
		t.Fatal(err)
	}
	st := kwarpGrantStats{warped: map[state.ObjID]bool{}, exileTurn: map[state.ObjID]int32{}}
	capped := 0
	for seed := uint64(1); seed <= 64; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kwg-a", "kwg-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		st.evAt = 0
		clear(st.warped)
		clear(st.exileTurn)
		res := LockstepLed(m, seed, g, k, kwarpGrantPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 40, Check: st.check})
		CloseSide(k)
		if res.StallUnmodelled {
			t.Fatalf("seed %d: unmodelled stall %s after %d decisions", seed, res.Stall, res.Decisions)
		}
		if res.Stall != "" {
			capped++ // a run cap or a declared kernel capacity: the game agreed up to there
		}
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games ended at a cap", capped)
	t.Logf("%+v", struct {
		GrantOffers, SecondRows, NoTannuk, BareChances, GrantCasts, Exiles, RecastOffers, Recasts, SameTurn int
	}{st.grantOffers, st.secondRows, st.noTannuk, st.bareChances, st.grantCasts, st.exiles, st.recastOffers, st.recasts, st.sameTurn})
	if st.noTannuk != 0 {
		t.Errorf("%d grant-only warp rows with no Tannuk out, want none", st.noTannuk)
	}
	if st.sameTurn != 0 {
		t.Errorf("%d warp_recast rows in the exile's own turn, want none", st.sameTurn)
	}
	if st.grantOffers < 5 || st.grantCasts < 5 || st.exiles == 0 || st.recasts == 0 || st.secondRows == 0 || st.bareChances == 0 {
		t.Errorf("thin coverage: %+v", st)
	}
}
