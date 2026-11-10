package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// An Effect's trigger bound to the set RememberObjects$ names, played decision
// by decision against gorge: The Eagles Are Coming! (a Bird at the next upkeep
// for each creature the spell returned), T'Pring, Never Parted (a card drawn
// when either fighter dies this turn) and Mysterio, Master of Illusion (its
// tokens stay when it leaves: gorge's Effect registration ends with Mysterio).
// TestStdSetReprosLockstep replays the three repros; TestEffectTrigRememberReprosReach
// requires each recorded game to reach the trigger's observable effect, since a
// lockstep entry alone would pass a script that never cast the card.
//
// The games are recorded by TestEffectTrigRememberRecord
// (EFFECT_TRIG_REMEMBER_RECORD=1) on the kernel table built from the decks.
func effectTrigRememberCases() []effectLongCase {
	return []effectLongCase{
		{
			name:   "effect_trig_remember_eagles_bird_per_returned_creature",
			prefer: []string{"The Eagles Are Coming!", "Savannah Lions"},
			decks: [2][]string{
				append(append(deckOf(6, "The Eagles Are Coming!"), deckOf(6, "Savannah Lions")...), deckOf(12, "Plains")...),
				append(deckOf(8, "Grizzly Bears"), deckOf(16, "Forest")...),
			},
			surface: "autopay",
			reached: eaglesBirdAtUpkeep,
		},
		{
			name:   "effect_trig_remember_tpring_draws_when_a_fighter_dies",
			prefer: []string{"T'Pring, Never Parted", "Grizzly Bears"},
			decks: [2][]string{
				append(append(deckOf(6, "T'Pring, Never Parted"), deckOf(6, "Grizzly Bears")...), append(deckOf(6, "Forest"), deckOf(6, "Island")...)...),
				append(deckOf(8, "Grizzly Bears"), deckOf(16, "Forest")...),
			},
			surface: "autopay",
			reached: tpringDrawsOnDeath,
		},
		{
			name:   "effect_trig_remember_mysterio_tokens_stay_when_it_dies",
			prefer: []string{"Mysterio, Master of Illusion", "Mysterio's Phantasm", "Lightning Bolt"},
			decks: [2][]string{
				append(append(deckOf(6, "Mysterio, Master of Illusion"), deckOf(6, "Mysterio's Phantasm")...), deckOf(12, "Island")...),
				append(deckOf(8, "Lightning Bolt"), deckOf(16, "Mountain")...),
			},
			surface: "autopay",
			reached: mysterioTokensStay,
		},
		{
			name:   "effect_trig_remember_mysterio_exiles_tokens_of_a_registration_made_after_it_left",
			prefer: []string{"Mysterio's Phantasm", "Mysterio, Master of Illusion", "Raise Dead", "Lightning Bolt"},
			decks: [2][]string{
				append(append(append(deckOf(5, "Mysterio, Master of Illusion"), deckOf(4, "Mysterio's Phantasm")...), deckOf(4, "Raise Dead")...), append(deckOf(6, "Island"), deckOf(5, "Swamp")...)...),
				append(deckOf(8, "Lightning Bolt"), deckOf(16, "Mountain")...),
			},
			surface: "autopay",
			reached: mysterioRegistrationAfterLeaving,
		},
	}
}

// evName is the card name of event object id, "" if it is gone.
func evName(g *GorgeSide, id state.ObjID) string {
	if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
		return o.Face().Name
	}
	return ""
}

// eaglesBirdAtUpkeep: a creature of seat 0's returned from the battlefield to
// its hand (the spell), and a Bird Soldier token was minted at a later step.
func eaglesBirdAtUpkeep(g *GorgeSide) bool {
	returned := false
	for _, e := range g.E.L.Events {
		switch {
		case e.Kind == events.MoveZone && e.From == state.ZBattlefield && e.To == state.ZHand && evName(g, e.Obj) == "Savannah Lions":
			returned = true
		case returned && e.Kind == events.TokenCreate && strings.Contains(e.Text, "bird"):
			return true
		}
	}
	return false
}

// tpringDrawsOnDeath: after T'Pring entered, a Grizzly Bears died and seat 0
// drew a card before the turn ended (outside the draw step, the trigger's).
func tpringDrawsOnDeath(g *GorgeSide) bool {
	entered, died := false, false
	for _, e := range g.E.L.Events {
		switch {
		case e.Kind == events.TurnChange:
			entered, died = false, false
		case e.Kind == events.MoveZone && e.To == state.ZBattlefield && evName(g, e.Obj) == "T'Pring, Never Parted":
			entered = true
		case entered && e.Kind == events.MoveZone && e.From == state.ZBattlefield && e.To == state.ZGraveyard && evName(g, e.Obj) == "Grizzly Bears":
			died = true
		case died && e.Kind == events.Draw && e.Player == 0:
			return true
		}
	}
	return false
}

// mysterioTokensStay: Mysterio made Illusion Villain tokens and then died
// with them out, and nothing was exiled from the battlefield afterwards. The
// Effect's "exile those tokens when it leaves" never fires in gorge: a
// permanent Effect registered by a battlefield permanent ends with that
// incarnation (delayedRegistrationLive, SourceBattlefield), so its leave
// trigger is gone when Mysterio leaves, and the kernel agrees.
func mysterioTokensStay(g *GorgeSide) bool {
	tokens, left := false, false
	for _, e := range g.E.L.Events {
		switch {
		case e.Kind == events.TokenCreate && strings.Contains(e.Text, "illusion"):
			tokens = true
		case tokens && e.Kind == events.MoveZone && e.From == state.ZBattlefield && evName(g, e.Obj) == "Mysterio, Master of Illusion":
			left = true
		case left && e.Kind == events.MoveZone && e.From == state.ZBattlefield && e.To == state.ZExile:
			return false
		}
	}
	return left
}

// mysterioRegistrationAfterLeaving: Mysterio died while his enters trigger was
// on the stack, so the Effect was registered with him off the battlefield (it
// does not end with him); Raise Dead and a second cast brought him back, and
// when he died again the first Effect's trigger exiled the token it remembered.
func mysterioRegistrationAfterLeaving(g *GorgeSide) bool {
	deaths, entered := 0, 0
	for _, e := range g.E.L.Events {
		switch {
		case e.Kind == events.MoveZone && e.To == state.ZBattlefield && evName(g, e.Obj) == "Mysterio, Master of Illusion":
			entered++
		case e.Kind == events.MoveZone && e.From == state.ZBattlefield && evName(g, e.Obj) == "Mysterio, Master of Illusion":
			deaths++
		case deaths >= 2 && entered >= 2 && e.Kind == events.MoveZone && e.From == state.ZBattlefield && e.To == state.ZExile:
			return true
		}
	}
	return false
}

// mysterioDeaths counts how often Mysterio has left the battlefield.
func mysterioDeaths(g *GorgeSide) int {
	n := 0
	for _, e := range g.E.L.Events {
		if e.Kind == events.MoveZone && e.From == state.ZBattlefield && evName(g, e.Obj) == "Mysterio, Master of Illusion" {
			n++
		}
	}
	return n
}

// mysterioOnBattlefield: Mysterio is on seat 0's battlefield.
func mysterioOnBattlefield(g *GorgeSide) bool {
	for _, id := range g.E.G.Zone(state.ZBattlefield, 0) {
		if evName(g, id) == "Mysterio, Master of Illusion" {
			return true
		}
	}
	return false
}

// mysterioOut: Mysterio and one of his Illusion tokens are on seat 0's
// battlefield (his enters trigger has resolved).
func mysterioOut(g *GorgeSide) bool {
	if g == nil {
		return false
	}
	mys, tok := false, false
	for _, id := range g.E.G.Zone(state.ZBattlefield, 0) {
		n := evName(g, id)
		mys = mys || n == "Mysterio, Master of Illusion"
		tok = tok || strings.Contains(n, "Illusion Villain")
	}
	return mys && tok
}

// effectTrigRememberPolicy is effectLongPolicy, except that in Mysterio's games
// a Lightning Bolt is held back and always aims at Mysterio: the first game
// (and the second's last death) casts it once Mysterio and a token are out; the
// second game's first death casts it while his enters trigger is still on the
// stack, so the Effect is registered with him gone.
func effectTrigRememberPolicy(m *Matchup, c effectLongCase, seed uint64) Policy {
	inner := effectLongPolicy(m, c, seed)
	if !strings.Contains(c.name, "mysterio") {
		return inner
	}
	again := strings.Contains(c.name, "after_it_left")
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DTarget && p.G != nil {
			for _, a := range p.D.Options {
				if o, ok := p.G.GorgeOption(a); ok && a.Kind() == ActTarget && o.Obj != 0 && evName(p.G, o.Obj) == "Mysterio, Master of Illusion" {
					return a
				}
			}
		}
		if p.D.Kind != DPriority {
			return inner.Pick(p)
		}
		bolt := mysterioOut(p.G)
		if again && mysterioDeaths(p.G) == 0 {
			bolt = mysterioOnBattlefield(p.G) && len(p.G.E.G.Stack) > 0
		}
		if bolt {
			return inner.Pick(p)
		}
		d := *p.D
		d.Options = nil
		for _, o := range p.D.Options {
			if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == "Lightning Bolt" {
				continue
			}
			d.Options = append(d.Options, o)
		}
		return inner.Pick(&PickCtx{D: &d, G: p.G, Seq: p.Seq})
	})
}

func TestEffectTrigRememberReprosReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range effectTrigRememberCases() {
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
				t.Fatalf("the recorded game never reaches %s", c.name)
			}
		})
	}
}

// TestEffectTrigRememberRecord plays each case's policy over seeds until one
// reaches the effect and writes the repro. It runs only with
// EFFECT_TRIG_REMEMBER_RECORD=1.
func TestEffectTrigRememberRecord(t *testing.T) {
	if os.Getenv("EFFECT_TRIG_REMEMBER_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips, since a skip here must name a missing pool (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range effectTrigRememberCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c, r, set, effectTrigRememberPolicy(m, c, seed))
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
		})
	}
}
