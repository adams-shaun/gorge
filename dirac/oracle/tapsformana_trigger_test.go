package oracle

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Non-static Mode$ TapsForMana triggers played decision by decision against
// gorge (dirac/kgen lower_tapsformana.go): Price of Glory destroys a land
// tapped for mana by the player whose turn it is not. Its destroy-self effect
// is a vetted post-payment consequence, so such a land is an ordinary payment
// unit in gorge's planner and the kernel (CR 603.2, 605.4a), and an autopay
// cast may tap it. Rikala's gated Island is the case that still defers.
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
	// Rikala, Homarid King casts on turn 3 off three lands; its gate (three
	// or more tide counters) holds from the third upkeep after; an Island
	// tapped for mana then brings the printed {U} and the gated extra. Opt
	// is the {U} ask the Island's gap funds; the Bears and the opponent's
	// Mountains keep both seats from doing anything else.
	rikala := func() [2][]string {
		d := deckOf(4, "Rikala, Homarid King")
		d = append(d, deckOf(4, "Opt")...)
		d = append(d, deckOf(3, "Island")...)
		d = append(d, deckOf(3, "Forest")...)
		d = append(d, deckOf(10, "Grizzly Bears")...)
		return [2][]string{d, append(deckOf(8, "Grizzly Bears"), deckOf(16, "Mountain")...)}
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
			// a land Price of Glory would destroy still pays: the opponent's
			// held Bolt is offered and cast from it on autopay
			name:    "price_of_glory_foreign_land_pays_held_bolt_autopay",
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
		{
			// the gate holds from the third tide counter: an Island tapped for
			// mana then produces two {U}, the printed one and Rikala's extra
			// (dirac/kop StatTapExtra under kop.Static.Cond2)
			name:    "rikala_gate_holds_island_taps_for_two_blue_full",
			prefer:  []string{"Rikala, Homarid King", "Opt"},
			decks:   rikala(),
			surface: "full",
			reached: rikalaTwoBlueFromOneIsland,
		},
		{
			// the gate holds with an untapped Island: gorge's planner defers
			// the Island (the gated extra is no foldable trigger,
			// pay.PaymentPlanFoldableTapMana), so the offer prices its tap at
			// the printed {U} alone while its controller holds the Opt -- the
			// kernel's unpriced static (mana_tapextra.go tapExtraReaches) must
			// agree, and its class line must defer the source
			name:    "rikala_gate_holds_deferred_island_taps_autopay",
			prefer:  []string{"Rikala, Homarid King", "Opt"},
			decks:   rikala(),
			surface: "autopay",
			reached: rikalaGateHoldsUntappedIsland,
		},
	}
}

// rikalaGateHoldsUntappedIsland: with three or more tide counters on a Rikala
// in play, its controller received priority holding an Opt with an Island of
// theirs still untapped.
func rikalaGateHoldsUntappedIsland(g *GorgeSide) bool {
	tide := map[state.ObjID]int{}
	gate := false
	name := func(id state.ObjID) string {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	optN, islands := map[state.PlayerID]int{}, map[state.ObjID]bool{} // Opt in hand by controller, Islands untapped
	for _, e := range g.E.L.Events {
		switch e.Kind {
		case events.CounterChange:
			if e.Counter == "TIDE" {
				o := g.E.G.Obj(e.Obj)
				if o != nil && o.Zone == state.ZBattlefield && name(e.Obj) == "Rikala, Homarid King" {
					tide[e.Obj] += int(e.Amount)
					gate = tide[e.Obj] >= 3
				}
			}
		case events.MoveZone, events.Draw:
			n := name(e.Obj)
			o := g.E.G.Obj(e.Obj)
			switch n {
			case "Rikala, Homarid King":
				if o == nil || e.From == state.ZBattlefield {
					delete(tide, e.Obj)
				}
			case "Opt":
				if o == nil {
					break
				}
				if e.To == state.ZHand {
					optN[o.Controller]++
				} else if e.From == state.ZHand && optN[o.Controller] > 0 {
					optN[o.Controller]--
				}
			case "Island":
				if o != nil && e.To == state.ZBattlefield {
					islands[e.Obj] = true
				} else {
					delete(islands, e.Obj)
				}
			}
		case events.Tap:
			if _, ok := islands[e.Obj]; ok {
				islands[e.Obj] = false
			}
		case events.Untap:
			if _, ok := islands[e.Obj]; ok {
				islands[e.Obj] = true
			}
		case events.Priority:
			if !gate {
				break
			}
			untapped := false
			for id, up := range islands {
				if o := g.E.G.Obj(id); o != nil && up && o.Controller == e.Player {
					untapped = true
				}
			}
			if untapped && optN[e.Player] > 0 {
				return true
			}
		}
	}
	return false
}

// rikalaTwoBlueFromOneIsland: with three or more tide counters on a Rikala in
// play (its gate), a window between two pool clears in which exactly one
// Island was tapped added two {U} to its controller's pool: the printed one
// and the gated extra. A removal at four drops the gate again, so the count
// is read live.
func rikalaTwoBlueFromOneIsland(g *GorgeSide) bool {
	tide := map[state.ObjID]int{}
	gate, blue, taps := false, 0, 0
	inPlay := func(id state.ObjID) bool {
		o := g.E.G.Obj(id)
		return o != nil && o.Zone == state.ZBattlefield
	}
	island := func(id state.ObjID) bool {
		o := g.E.G.Obj(id)
		return o != nil && o.Face() != nil && o.Face().Name == "Island" && o.Zone == state.ZBattlefield
	}
	for _, e := range g.E.L.Events {
		switch e.Kind {
		case events.ManaClear, events.TurnChange:
			blue, taps = 0, 0
		case events.CounterChange:
			if e.Counter == "TIDE" && inPlay(e.Obj) {
				tide[e.Obj] += int(e.Amount)
				gate = tide[e.Obj] >= 3
			}
		case events.MoveZone:
			if e.From == state.ZBattlefield {
				delete(tide, e.Obj)
			}
		case events.Tap:
			if island(e.Obj) {
				taps++
			}
		case events.ManaAdd:
			if e.Counter == "U" && e.Amount > 0 {
				blue += int(e.Amount)
			}
		}
		if gate && taps == 1 && blue >= 2 {
			return true
		}
	}
	return false
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
				pol := Policy(tapsForManaPolicy(m, c, []string{"Lightning Bolt"}, seed))
				if strings.HasPrefix(c.name, "rikala") {
					pol = effectLongPolicy(m, c, seed)
				}
				res, reached := effectLongPlay(t, reg, c, r, set, pol)
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
