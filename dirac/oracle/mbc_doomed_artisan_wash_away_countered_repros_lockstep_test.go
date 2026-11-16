package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 701.5a: a countered spell is put into its owner's graveyard. Doomed
// Artisan is exiled by Stolen Goods and cast from exile through its free
// may-play permission ("until end of turn, you may cast that card without
// paying its mana cost"), and Wash Away's {U} cast counters it: gorge puts
// the card in its owner's graveyard -- Stolen Goods' permission governs the
// play, not the spell's destination, and its effect carries no
// exile-on-leave replacement. The kernel's play register once carried
// ReplaceGraveyard$ Exile from Storm of Memories' declined free-cast offer
// past the drain, stamped it on the unrelated may-play cast (sfFreeGY), and
// the counter sent the card back to exile.

// washAwayCounterDecks deals the scenario: p0 holds Storm of Memories,
// Stolen Goods, Galvanic Blast and lands, p1 Doomed Artisans, Islands and
// Wash Aways, so Storm's declined offer leaves the stale register, Stolen
// Goods exiles the Doomed Artisan on top of p1's library and Wash Away
// counters the may-play cast of it.
func washAwayCounterDecks() [2][]string {
	return [2][]string{
		append(append(append(repeatName("Storm of Memories", 8), repeatName("Stolen Goods", 6)...),
			repeatName("Galvanic Blast", 2)...),
			append(repeatName("Island", 6), repeatName("Mountain", 18)...)...),
		append(append(repeatName("Doomed Artisan", 26), repeatName("Island", 10)...), repeatName("Wash Away", 4)...),
	}
}

// washAwayCounterPolicy plays the scenario: p0 plays a hand land, casts
// Galvanic Blast at the opponent once while Storm of Memories is still
// uncast (Storm's random exile then finds it and offers its free cast),
// casts Storm of Memories when it is affordable (declining the offer), then
// Stolen Goods at the opponent, then the exiled Doomed Artisan through the
// may-play permission; p1 counters it with Wash Away's {U} cast (the cleave
// cast is outside the autopay surface's vocabulary). Each card is cast once,
// in story order.
func washAwayCounterPolicy(m *Matchup, played map[string]bool) PolicyFunc {
	return func(c *PickCtx) Canon {
		if c.G == nil {
			return pickKind(c, ActPass)
		}
		g := c.G.E.G
		switch c.D.Kind {
		case DChoose:
			// Storm of Memories' optional free cast: declined. The queue
			// drains empty; the kernel once kept its ReplaceGraveyard$ Exile
			// register set for every later free cast.
			for _, a := range c.D.Options {
				if a.Kind() == ActDone {
					return a
				}
			}
		case DTarget:
			for _, a := range c.D.Options {
				if o, ok := c.G.GorgeOption(a); ok && a.Kind() == ActTarget {
					if ob := g.Obj(o.Obj); ob != nil && ob.Zone == state.ZStack && sideObjName(c.G, o.Obj) == "Doomed Artisan" {
						return a // Wash Away's target: the may-play cast on the stack
					}
				}
			}
			for _, a := range c.D.Options {
				if a.Kind() == ActTarget && a.Tgt() == RefPlayer+1 {
					return a // Galvanic Blast's target: the opponent
				}
			}
			return pickKind(c, ActDone)
		case DPriority:
			switch c.D.Player {
			case 0:
				for _, a := range c.D.Options {
					if o, ok := c.G.GorgeOption(a); ok && a.Kind() == ActPlayLand {
						if ob := g.Obj(o.Obj); ob != nil && ob.Zone == state.ZHand {
							return a
						}
					}
				}
				for _, want := range []string{"Storm of Memories", "Stolen Goods", "Galvanic Blast", "Doomed Artisan"} {
					if played[want] {
						continue
					}
					switch want {
					case "Stolen Goods":
						if !played["Storm of Memories"] {
							continue
						}
					case "Galvanic Blast":
						if played["Storm of Memories"] {
							continue // a grave card for Storm's random exile only
						}
					case "Doomed Artisan":
						if !played["Stolen Goods"] {
							continue
						}
					}
					for _, a := range c.D.Options {
						if a.Kind() == ActCast && m.InstanceCard(int(a.Src())) == want {
							played[want] = true
							return a
						}
					}
				}
			case 1:
				for _, a := range c.D.Options {
					if o, ok := c.G.GorgeOption(a); ok && a.Kind() == ActPlayLand {
						if ob := g.Obj(o.Obj); ob != nil && ob.Zone == state.ZHand {
							return a
						}
					}
				}
				for _, a := range c.D.Options {
					if a.Kind() == ActCast && m.InstanceCard(int(a.Src())) == "Wash Away" {
						return a
					}
				}
			}
		}
		if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActPass }); ok {
			return a
		}
		return pickKind(c, ActPass)
	}
}

// washAwayCounterRun plays one seed of the scenario and reports the story:
// countered is set when a Doomed Artisan that was on the stack (the may-play
// cast) reaches p1's graveyard, recalled when one reaches p1's exile. The
// stack hop keeps p1's clean-up discards out of the tally.
func washAwayCounterRun(t *testing.T, reg *cards.Registry, set cardset.Set, r *Repro, policy Policy) (res Result, countered, recalled bool) {
	t.Helper()
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	tbl, _, _, err := set.Matchup(oneOfEachEntries(r.Cards[0]), oneOfEachEntries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	cast := map[state.ObjID]bool{}
	rc := r.RunConfig()
	rc.MaxDecisions = 3000
	rc.Check = func(g *GorgeSide, k Side) *Divergence {
		for _, id := range g.E.G.Zone(state.ZStack, 0) {
			if sideObjName(g, id) == "Doomed Artisan" {
				cast[id] = true
			}
		}
		for _, id := range g.E.G.Zone(state.ZGraveyard, 1) {
			if sideObjName(g, id) == "Doomed Artisan" && cast[id] {
				countered = true
			}
		}
		for _, id := range g.E.G.Zone(state.ZExile, 1) {
			if sideObjName(g, id) == "Doomed Artisan" && cast[id] {
				recalled = true
			}
		}
		return nil
	}
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	res = Lockstep(m, r.Seed, k, policy, rc)
	return res, countered, recalled
}

// TestMBCDoomedArtisanWashAwayRecord records the scenario: it runs only with
// PINS_RECORD=1; the game it writes is the committed repro. It looks for a
// seed whose game counters the may-play cast of the exiled Doomed Artisan.
func TestMBCDoomedArtisanWashAwayRecord(t *testing.T) {
	if os.Getenv("PINS_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	c := washAwayCounterDecks()
	for seed := uint64(1); seed <= 120; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{"wash_away_countered_0", "wash_away_countered_1"},
			Cards: c, Seed: seed, PassElision: true, Surface: "autopay", Kernel: "custom", Pool: "custom", MaxTurns: 12}
		set := printedDelveSet(t, r.Cards)
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		res, countered, recalled := washAwayCounterRun(t, reg, set, r, washAwayCounterPolicy(m, map[string]bool{}))
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			continue // the kernel stopped: not a game to record
		}
		if res.Div != nil {
			t.Logf("seed %d: diverged at %d: %s %s", seed, res.Div.At, res.Div.What, res.Div.Field)
			continue
		}
		if !countered {
			continue // the may-play cast was never countered
		}
		if recalled {
			t.Fatalf("seed %d: the countered spell was recalled to exile", seed)
		}
		r.setActions(res.Actions)
		if err := r.Write(filepath.Join("testdata", "repro", "doomed_artisan_wash_away_countered.repro.json")); err != nil {
			t.Fatal(err)
		}
		t.Logf("seed %d: recorded, %d actions", seed, len(res.Actions))
		return
	}
	t.Fatalf("no seed reached the countered may-play cast")
}

// TestMBCDoomedArtisanWashAwayCounteredReprosLockstep replays the recorded
// game: no divergence, every scripted action offered, and the countered spell
// in its owner's graveyard (CR 701.5a), never recalled to exile.
func TestMBCDoomedArtisanWashAwayCounteredReprosLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	r, err := ReadRepro(filepath.Join("testdata", "repro", "doomed_artisan_wash_away_countered.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	set := reproSet(t, "custom", r)
	script := &ScriptPolicy{Actions: r.Actions}
	res, countered, recalled := washAwayCounterRun(t, reg, set, r, script)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Fatalf("the kernel stopped: %q", res.Stall)
	}
	if !countered {
		t.Fatalf("the recorded game never counters the may-play cast")
	}
	if recalled {
		t.Fatalf("the countered spell was recalled to exile, not its owner's graveyard (CR 701.5a)")
	}
}
