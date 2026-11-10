package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Say Its Name's graveyard ability: "Exile this card and two other cards named
// Say Its Name from your graveyard: search your graveyard, hand, and/or library
// for a card named Altanak, the Thrice-Called and put it onto the battlefield".
// gorge settles the CARDNAME part first, unasked, then asks the two others
// (CR 118.3, 602.2b); the search elects its zones (the library, the graveyard,
// the hand) before the pick. The game is recorded by TestExileGraveSelfRecord
// (EXILE_GRAVE_SELF_RECORD=1) and replayed by TestStdSetReprosLockstep.
func exileGraveSelfCase() effectLongCase {
	return effectLongCase{
		name: "exile_this_and_two_others_from_graveyard",
		decks: [2][]string{
			append(append(deckOf(26, "Say Its Name"), deckOf(12, "Forest")...), deckOf(2, "Altanak, the Thrice-Called")...),
			deckOf(40, "Plains"),
		},
		surface: "autopay",
		reached: sayItsNameFoundAltanak,
	}
}

// sayItsNameFoundAltanak: three Say Its Name cards were exiled from the
// graveyard and an Altanak, the Thrice-Called reached the battlefield.
func sayItsNameFoundAltanak(g *GorgeSide) bool {
	name := func(e *events.Event) string {
		if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	exiled, found := 0, false
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if e.Kind != events.MoveZone {
			continue
		}
		switch {
		case name(e) == "Say Its Name" && e.From == state.ZGraveyard && e.To == state.ZExile:
			exiled++
		case name(e) == "Altanak, the Thrice-Called" && e.To == state.ZBattlefield:
			found = true
		}
	}
	return exiled >= 3 && found
}

// exileGraveSelfPolicy plays lands, casts Say Its Name (milling the others
// into the graveyard), and takes the graveyard ability once it is offered; the
// random tail answers every pick.
func exileGraveSelfPolicy(m *Matchup, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DPriority {
			for _, o := range p.D.Options {
				if o.Kind() == ActActivate && m.InstanceCard(int(o.Src())) == "Say Its Name" {
					return o
				}
			}
			for _, o := range p.D.Options {
				if o.Kind() == ActPlayLand {
					return o
				}
			}
			for _, o := range p.D.Options {
				if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == "Say Its Name" {
					return o
				}
			}
		}
		return rnd.Pick(p)
	})
}

// TestExileGraveSelfReprosReach replays the recorded game: no divergence,
// every scripted action offered, the three cards exiled and Altanak found.
func TestExileGraveSelfReprosReach(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	c := exileGraveSelfCase()
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
		t.Fatal("the recorded game never exiles three Say Its Names and finds Altanak")
	}
}

// TestExileGraveSelfRecord plays the policy over seeds until one reaches the
// ability, and writes the repro. It runs only with EXILE_GRAVE_SELF_RECORD=1.
func TestExileGraveSelfRecord(t *testing.T) {
	if os.Getenv("EXILE_GRAVE_SELF_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	c := exileGraveSelfCase()
	for seed := uint64(1); seed <= 400; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks,
			Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
		set := reproSet(t, "custom", r)
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		res, reached := effectLongPlay(t, reg, c, r, set, exileGraveSelfPolicy(m, seed))
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
	t.Fatal("no seed reached the ability")
}
