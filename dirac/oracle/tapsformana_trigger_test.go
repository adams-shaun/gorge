package oracle

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Non-static Mode$ TapsForMana triggers played decision by decision against
// gorge (dirac/kgen lower_tapsformana.go): Price of Glory destroys a land
// tapped for mana by the player whose turn it is not, and its deferral keeps
// such a land out of gorge's planner (the kernel's tapsWatchClass), so an
// autopay cast taps another source or none.
//
// TestTapsForManaTriggerLockstep replays the recorded games (no divergence,
// every scripted action offered, the effect reached); TestTapsForManaTriggerRecord
// (PINS_RECORD=1) searches seeds and writes them.
func tapsForManaCases() []effectLongCase {
	pog := func(opp string) [2][]string {
		return [2][]string{
			append(deckOf(8, "Price of Glory"), deckOf(16, "Mountain")...),
			append(deckOf(8, opp), deckOf(16, "Mountain")...),
		}
	}
	return []effectLongCase{
		{
			name:    "price_of_glory_destroys_land_tapped_on_foreign_turn_full",
			prefer:  []string{"Price of Glory"},
			decks:   pog("Lightning Bolt"),
			surface: "full",
			reached: func(g *GorgeSide) bool { return priceOfGloryDestroyed(g, true) },
		},
		{
			// gorge's planner defers a land Price of Glory would destroy, so
			// the opponent's held Bolt is not offered from it on autopay
			name:    "price_of_glory_defers_foreign_land_so_held_bolt_is_not_offered_autopay",
			prefer:  []string{"Price of Glory"},
			decks:   pog("Lightning Bolt"),
			surface: "autopay",
			reached: priceOfGloryHeldBolt,
		},
		{
			// the active player's own taps trigger nothing: a land tapped on
			// its controller's turn, with Price of Glory in play, survives
			name:    "price_of_glory_land_tapped_on_own_turn_stays",
			prefer:  []string{"Price of Glory", "Grizzly Bears"},
			decks:   pog("Grizzly Bears"),
			surface: "autopay",
			reached: func(g *GorgeSide) bool { return priceOfGloryDestroyed(g, false) },
		},
	}
}

// priceOfGloryDestroyed scans the finished log with Price of Glory in play.
// With destroyed: a land was tapped by a player on a turn that was not theirs
// and left the battlefield before the turn ended. Without: a land was tapped
// by the active player on their own turn (Price of Glory in play) and was
// still on the battlefield when the turn ended.
func priceOfGloryDestroyed(g *GorgeSide, destroyed bool) bool {
	var active state.PlayerID
	inPlay := 0
	tappedOn := map[state.ObjID]bool{} // land -> tapped this turn with the effect's precondition
	isLand := func(id state.ObjID) bool {
		o := g.E.G.Obj(id)
		return o != nil && o.Face() != nil && o.Face().Name == "Mountain"
	}
	isPoG := func(id state.ObjID) bool {
		o := g.E.G.Obj(id)
		return o != nil && o.Face() != nil && o.Face().Name == "Price of Glory"
	}
	for _, e := range g.E.L.Events {
		switch e.Kind {
		case events.TurnChange:
			if !destroyed && len(tappedOn) > 0 {
				return true // a land tapped on its controller's turn is still in play
			}
			active = e.Player
			clear(tappedOn)
		case events.MoveZone:
			if isPoG(e.Obj) {
				if e.To == state.ZBattlefield {
					inPlay++
				} else if e.From == state.ZBattlefield {
					inPlay--
				}
			}
			if e.From == state.ZBattlefield && tappedOn[e.Obj] {
				if destroyed {
					return true
				}
				delete(tappedOn, e.Obj)
			}
		case events.Tap:
			o := g.E.G.Obj(e.Obj)
			if inPlay <= 0 || o == nil || !isLand(e.Obj) {
				break
			}
			if destroyed && o.Controller != active || !destroyed && o.Controller == active {
				tappedOn[e.Obj] = true
			}
		}
	}
	return false
}

// tapsForManaPolicy casts the case's preferred cards at every priority, and
// on a turn that is not the deciding player's (gorge leading, so its active
// player is readable) also the foreign card names: an instant held for the
// opponent's turn, the foreign-turn mana tap Price of Glory punishes.
func tapsForManaPolicy(m *Matchup, c effectLongCase, foreign []string, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	cast := func(p *PickCtx, names []string) (Canon, bool) {
		for _, want := range names {
			for _, o := range p.D.Options {
				if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == want {
					return o, true
				}
			}
		}
		return 0, false
	}
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DPriority {
			if p.G != nil && int(p.G.E.G.Active) != int(p.D.Player) {
				if o, ok := cast(p, foreign); ok {
					return o
				}
			}
			if o, ok := cast(p, c.prefer); ok {
				return o
			}
			for _, o := range p.D.Options {
				if o.Kind() == ActPlayLand {
					return o
				}
			}
		}
		if p.D.Kind == DPriority && p.G != nil && int(p.G.E.G.Active) == int(p.D.Player) {
			// hold the instants for the opponent's turn
			rest := *p.D
			rest.Options = nil
			for _, o := range p.D.Options {
				if o.Kind() == ActCast && slices.Contains(foreign, m.InstanceCard(int(o.Src()))) {
					continue
				}
				rest.Options = append(rest.Options, o)
			}
			return rnd.Pick(&PickCtx{D: &rest, G: p.G, Seq: p.Seq})
		}
		return rnd.Pick(p)
	})
}

// priceOfGloryHeldBolt: with Price of Glory in play, the player whose turn it
// is not received priority holding a Lightning Bolt and an untapped Mountain.
func priceOfGloryHeldBolt(g *GorgeSide) bool {
	var active state.PlayerID
	pog := 0
	hand := map[state.ObjID]bool{} // Lightning Bolts in hand
	land := map[state.ObjID]bool{} // Mountains on the battlefield: untapped?
	name := func(id state.ObjID) string {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	for _, e := range g.E.L.Events {
		switch e.Kind {
		case events.TurnChange:
			active = e.Player
		case events.MoveZone, events.Draw:
			delete(hand, e.Obj)
			delete(land, e.Obj)
			switch name(e.Obj) {
			case "Price of Glory":
				if e.To == state.ZBattlefield {
					pog++
				} else if e.From == state.ZBattlefield {
					pog--
				}
			case "Lightning Bolt":
				hand[e.Obj] = e.To == state.ZHand
			case "Mountain":
				if e.To == state.ZBattlefield {
					land[e.Obj] = true
				}
			}
		case events.Tap:
			if _, ok := land[e.Obj]; ok {
				land[e.Obj] = false
			}
		case events.Untap:
			if _, ok := land[e.Obj]; ok {
				land[e.Obj] = true
			}
		case events.Priority:
			if pog <= 0 || e.Player == active {
				break
			}
			bolt, untapped := false, false
			for id, in := range hand {
				bolt = bolt || in && g.E.G.Obj(id).Controller == e.Player
			}
			for id, up := range land {
				untapped = untapped || up && g.E.G.Obj(id).Controller == e.Player
			}
			if bolt && untapped {
				return true
			}
		}
	}
	return false
}

func TestTapsForManaTriggerLockstep(t *testing.T) {
	reg := synthReg(t)
	for _, c := range tapsForManaCases() {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			script := &ScriptPolicy{Actions: r.Actions}
			res, reached := effectLongPlay(t, reg, c, r, printedDelveSet(t, r.Cards), script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if !reached {
				t.Fatalf("the recorded game never reaches the effect of %s", c.name)
			}
		})
	}
}

// TestTapsForManaTriggerRecord plays the cases' policies over seeds until one
// reaches the effect and writes the repro. It runs only with
// PINS_RECORD=1; the games it writes are the committed repros.
func TestTapsForManaTriggerRecord(t *testing.T) {
	if os.Getenv("PINS_RECORD") != "1" {
		return // a recording tool, not coverage: it returns rather than skips (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range tapsForManaCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom", Pool: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c, r, set, tapsForManaPolicy(m, c, []string{"Lightning Bolt"}, seed))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v\nref %v\nker %v\n%s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail)
				}
				if !reached {
					continue
				}
				r.setActions(res.Actions)
				if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
					t.Fatal(err)
				}
				t.Logf("seed %d: %d decisions", seed, len(res.Actions))
				return
			}
			t.Fatalf("no seed reached the effect of %s", c.name)
		})
	}
}

// TestTapsForManaTriggerDeferralMutation: with the Taps-watcher classification
// dropped (dirac.DebugNoTapsDefer) the recorded autopay game diverges from
// gorge, proof the repro exercises Price of Glory's deferral of the
// opponent's land.
func TestTapsForManaTriggerDeferralMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoTapsDefer = true
	defer func() { dirac.DebugNoTapsDefer = false }()
	reg := synthReg(t)
	c := tapsForManaCases()[1]
	r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	res, _ := effectLongPlay(t, reg, c, r, printedDelveSet(t, r.Cards), &ScriptPolicy{Actions: r.Actions})
	if res.Div == nil {
		t.Fatal("no divergence with the Taps-watcher classification dropped: the recorded game does not exercise the deferral")
	}
}
