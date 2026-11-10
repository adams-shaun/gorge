package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 608.2h: Loki Laufeyson's delayed SpellCast bound ("mana value less than
// or equal to Loki's power") is his last known power once he has left the
// battlefield. The recorded game has Loki pumped to 5 by a Titan's Strength
// after he and Goblin Bombardment are out, activates his delayed trigger,
// sacrifices him to Goblin Bombardment (a non-spell removal, so the
// registration stays), and casts a mana-value-3 Divination: the bound reads 5,
// not his printed 2, so the delayed trigger copies it.
//
// TestDelaySpellSourceDepartsRecord (DELAY_SPELL_SOURCE_DEPARTS_RECORD=1)
// records the game; TestDelaySpellSourceDepartsReaches requires the recorded
// game to reach the copy, since the lockstep row alone would accept a script
// that never cast the Divination.
func delaySpellDepartsCase() effectLongCase {
	return effectLongCase{
		name: "delay_spell_source_departs",
		decks: [2][]string{
			append(append(append(append(deckOf(4, "Loki Laufeyson"), deckOf(4, "Goblin Bombardment")...),
				deckOf(4, "Titan's Strength")...), deckOf(4, "Divination")...),
				append(deckOf(12, "Mountain"), deckOf(8, "Island")...)...),
			deckOf(24, "Island"),
		},
		surface: "autopay",
		reached: delaySpellDepartsReached,
	}
}

// delaySpellDepartsReached: Loki registered the delayed trigger and left the
// battlefield, then a Divination was cast and the trigger copied it, all
// within one turn.
func delaySpellDepartsReached(g *GorgeSide) bool {
	name := func(id state.ObjID) string {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	registered, left, cast := false, false, false
	for _, e := range g.E.L.Events {
		switch {
		case e.Kind == events.TurnChange:
			registered, left, cast = false, false, false
		case e.Kind == events.DelayedRegister && name(e.Obj) == "Loki Laufeyson":
			registered = true
		case registered && e.Kind == events.MoveZone && e.From == state.ZBattlefield && e.To == state.ZGraveyard && name(e.Obj) == "Loki Laufeyson":
			left = true
		case left && e.Kind == events.PutOnStack && name(e.Obj) == "Divination":
			cast = true
		case cast && e.Kind == events.StackCopy && name(e.Obj) == "Divination":
			return true
		}
	}
	return false
}

// delaySpellDepartsPolicy plays the line: seat 0 develops Loki and Goblin
// Bombardment, then on a turn with five untapped lands casts Titan's Strength
// on Loki, activates his delayed trigger, sacrifices him to the Bombardment
// aimed at seat 1, and casts Divination. Seat 1 plays lands.
func delaySpellDepartsPolicy(m *Matchup, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.G == nil {
			return rnd.Pick(p)
		}
		g := p.G.E.G
		pl := state.PlayerID(p.D.Player)
		onBF := func(face string) state.ObjID {
			for _, id := range g.Zone(state.ZBattlefield, pl) {
				if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == face {
					return id
				}
			}
			return 0
		}
		src := func(o Canon) string { return m.InstanceCard(int(o.Src())) }
		loki, bomb := onBF("Loki Laufeyson"), onBF("Goblin Bombardment")
		registered := len(g.Delayed) > 0
		switch p.D.Kind {
		case DMulligan:
			for _, o := range p.D.Options {
				if o.Kind() == ActKeep {
					return o
				}
			}
		case DTarget:
			if pl == 0 {
				for _, o := range p.D.Options {
					if x, ok := p.G.GorgeOption(o); ok {
						if registered && x.Obj == 0 && x.Player == 1 || !registered && x.Obj != 0 && x.Obj == loki {
							return o
						}
					}
				}
			}
		case DPriority:
			if pl != 0 {
				for _, k := range []CanonKind{ActPlayLand, ActPass} {
					for _, o := range p.D.Options {
						if o.Kind() == k {
							return o
						}
					}
				}
				break
			}
			find := func(k CanonKind, card string, mode int) (Canon, bool) {
				for _, o := range p.D.Options {
					if o.Kind() == k && src(o) == card && (mode < 0 || int(o.Mode()) == mode) {
						return o, true
					}
				}
				return 0, false
			}
			pass := func() (Canon, bool) {
				for _, o := range p.D.Options {
					if o.Kind() == ActPass {
						return o, true
					}
				}
				return 0, false
			}
			untapped, islands, mountains := 0, 0, 0
			for _, id := range g.Zone(state.ZBattlefield, pl) {
				if o := g.Obj(id); o != nil && o.Face() != nil && !o.Tapped && o.Face().Name != "Loki Laufeyson" && o.Face().Name != "Goblin Bombardment" {
					untapped++
					switch o.Face().Name {
					case "Island":
						islands++
					case "Mountain":
						mountains++
					}
				}
			}
			inHand := func(face string) bool {
				for _, id := range g.Zone(state.ZHand, pl) {
					if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == face {
						return true
					}
				}
				return false
			}
			var want Canon
			var ok bool
			switch {
			case len(g.Stack) > 0:
				want, ok = pass()
			case registered && loki != 0 && bomb != 0:
				want, ok = find(ActActivate, "Goblin Bombardment", -1)
			case registered:
				want, ok = find(ActCast, "Divination", -1)
			case loki != 0 && bomb != 0 && (inHand("Titan's Strength") && inHand("Divination") || p.G.E.Power(loki) >= 5) && (untapped >= 5 && islands >= 2 && mountains >= 1 || p.G.E.Power(loki) >= 5) && !g.Obj(loki).SummonSick && !g.Obj(loki).Tapped:
				if p.G.E.Power(loki) < 5 {
					want, ok = find(ActCast, "Titan's Strength", -1)
				} else {
					want, ok = find(ActActivate, "Loki Laufeyson", 0)
				}
			}
			if !ok && len(g.Stack) == 0 {
				for _, k := range []CanonKind{ActPlayLand} {
					for _, o := range p.D.Options {
						if o.Kind() == k {
							want, ok = o, true
						}
					}
				}
			}
			if !ok && len(g.Stack) == 0 {
				switch {
				case bomb == 0:
					want, ok = find(ActCast, "Goblin Bombardment", -1)
				case loki == 0:
					want, ok = find(ActCast, "Loki Laufeyson", -1)
				}
			}
			if !ok {
				want, ok = pass()
			}
			if ok {
				return want
			}
		}
		return rnd.Pick(p)
	})
}

// TestDelaySpellSourceDepartsReaches replays the recorded game: no divergence,
// every scripted action offered, the departed source's bound read at the cast.
func TestDelaySpellSourceDepartsReaches(t *testing.T) {
	reg := synthReg(t)
	c := delaySpellDepartsCase()
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
		t.Fatalf("the recorded game never reaches the Divination copy")
	}
}

// TestDelaySpellSourceDepartsRecord plays the line over seeds until one
// reaches the copy, and writes the repro. It runs only with
// DELAY_SPELL_SOURCE_DEPARTS_RECORD=1.
func TestDelaySpellSourceDepartsRecord(t *testing.T) {
	if os.Getenv("DELAY_SPELL_SOURCE_DEPARTS_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips, since a skip here must name a missing pool (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	c := delaySpellDepartsCase()
	set := printedDelveSet(t, c.decks)
	for seed := uint64(1); seed <= 400; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		res, reached := effectLongPlay(t, reg, c, r, set, delaySpellDepartsPolicy(m, seed))
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
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
	t.Fatalf("no seed reached %s", c.name)
}
