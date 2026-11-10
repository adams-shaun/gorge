package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// An Effect's "that creature doesn't untap (can't become untapped) for as long
// as you control CARDNAME" (Duration$ AsLongAsControl over an Event$ Untap |
// Layer$ CantHappen replacement on the remembered target), played decision by
// decision against gorge: Ty Lee, Chi Blocker (untap step only) and
// Spider-Woman, Secret Agent (every untap), each holding an opposing Wall of
// Wood (a defender, so the game runs long) tapped through its controller's untap step, and Ty Lee's lock ending
// when an opponent's Act of Treason takes her (control change) or a Lightning
// Bolt kills her (source gone), after which the Wall untaps.
// TestStdSetReprosLockstep replays the repros; TestReplDurationReprosReach
// requires each recorded game to reach the observable the case names.
//
// The games are recorded by TestReplDurationRecord (REPL_DURATION_RECORD=1) on
// the kernel table built from the decks.
type replDurationCase struct {
	effectLongCase
}

// durationLock walks the log: src entered, then an opposing victim was tapped
// (its enters trigger), then what happened to the lock. It reports how many of
// the victim's controller's untap steps passed while it stayed tapped before
// released (the source's control change or departure, when the case wants
// one) and whether the victim untapped after that.
type durationLockRun struct {
	heldSteps   int  // the victim's controller's untap steps that left it tapped
	released    bool // the source changed controller (or, with leaves, left the battlefield)
	untappedOff bool // the victim untapped after the release
	untappedOn  bool // the victim untapped before any release
	spells      int  // spell resolutions of the named card after the victim was tapped
}

func walkDurationLock(g *GorgeSide, src, victim, spell string, leaves bool) durationLockRun {
	var r durationLockRun
	var srcObj, locked state.ObjID
	active := state.PlayerID(0)
	for _, e := range g.E.L.Events {
		switch {
		case e.Kind == events.MoveZone && e.To == state.ZBattlefield && srcObj == 0:
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == src {
				srcObj = e.Obj
			}
		case e.Kind == events.Tap && srcObj != 0 && locked == 0 && e.Obj != srcObj:
			t, s := g.E.G.Obj(e.Obj), g.E.G.Obj(srcObj)
			if t != nil && s != nil && t.Face() != nil && t.Face().Name == victim && t.Controller != s.Controller {
				locked = e.Obj
			}
		case locked == 0:
		case e.Kind == events.Resolve && spell != "":
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == spell {
				r.spells++
			}
		case e.Kind == events.TurnChange:
			active = e.Player
		case e.Kind == events.StepChange && e.Step == state.StepUntap:
			if t := g.E.G.Obj(locked); t != nil && t.Controller == active && !r.released && !r.untappedOn {
				r.heldSteps++
			}
		case e.Kind == events.Untap && e.Obj == locked:
			if r.released {
				r.untappedOff = true
			} else {
				r.untappedOn = true
			}
		case !leaves && e.Kind == events.ControlChange && e.Obj == srcObj:
			r.released = true
		case leaves && e.Kind == events.MoveZone && e.Obj == srcObj && e.From == state.ZBattlefield:
			r.released = true
		}
	}
	return r
}

func durationHeld(src, victim string) func(g *GorgeSide) bool {
	return func(g *GorgeSide) bool {
		r := walkDurationLock(g, src, victim, "", false)
		return r.heldSteps >= 1 && !r.untappedOn
	}
}

// durationEffectUntap: spell resolved after the victim was tapped while the
// source held it; held says the victim stayed tapped (a lock on every untap),
// not held that it untapped (a lock on the untap step alone).
func durationEffectUntap(src, victim, spell string, held bool) func(g *GorgeSide) bool {
	return func(g *GorgeSide) bool {
		r := walkDurationLock(g, src, victim, spell, false)
		if r.spells == 0 || r.released {
			return false
		}
		if held {
			return r.heldSteps >= 1 && !r.untappedOn
		}
		return r.untappedOn
	}
}

func durationReleased(src, victim string, leaves bool) func(g *GorgeSide) bool {
	return func(g *GorgeSide) bool {
		r := walkDurationLock(g, src, victim, "", leaves)
		return r.released && r.untappedOff && !r.untappedOn
	}
}

func replDurationCases() []replDurationCase {
	const tyLee, spider, bears = "Ty Lee, Chi Blocker", "Spider-Woman, Secret Agent", "Wall of Wood"
	bearDeck := func() []string { return append(deckOf(8, bears), deckOf(16, "Forest")...) }
	return []replDurationCase{
		{effectLongCase{
			name:    "ty_lee_target_stays_tapped_through_its_untap_step",
			prefer:  []string{tyLee, bears},
			decks:   [2][]string{append(deckOf(8, tyLee), deckOf(16, "Island")...), bearDeck()},
			surface: "autopay",
			reached: durationHeld(tyLee, bears),
		}},
		{effectLongCase{
			name:    "spider_woman_target_stays_tapped_through_its_untap_step",
			prefer:  []string{spider, bears},
			decks:   [2][]string{append(deckOf(8, spider), deckOf(16, "Island")...), bearDeck()},
			surface: "full", // the hybrid {W/U} is priced only on the full surface
			reached: durationHeld(spider, bears),
		}},
		{effectLongCase{
			name:   "ty_lee_stolen_by_act_of_treason_frees_the_target",
			prefer: []string{tyLee, bears, "Act of Treason"},
			decks: [2][]string{append(deckOf(8, tyLee), deckOf(16, "Island")...),
				append(append(deckOf(5, bears), deckOf(4, "Act of Treason")...), append(deckOf(7, "Forest"), deckOf(8, "Mountain")...)...)},
			surface: "autopay",
			reached: durationReleased(tyLee, bears, false),
		}},
		{effectLongCase{
			name:   "ty_lee_dies_to_lightning_bolt_frees_the_target",
			prefer: []string{tyLee, bears, "Lightning Bolt"},
			decks: [2][]string{append(deckOf(8, tyLee), deckOf(16, "Island")...),
				append(append(deckOf(5, bears), deckOf(5, "Lightning Bolt")...), append(deckOf(7, "Forest"), deckOf(7, "Mountain")...)...)},
			surface: "autopay",
			reached: durationReleased(tyLee, bears, true),
		}},
		{effectLongCase{
			name:   "ty_lee_target_untaps_by_an_effect",
			prefer: []string{tyLee, bears, "Aim High"},
			decks: [2][]string{append(deckOf(8, tyLee), deckOf(16, "Island")...),
				append(append(deckOf(5, bears), deckOf(5, "Aim High")...), deckOf(14, "Forest")...)},
			surface: "autopay",
			reached: durationEffectUntap(tyLee, bears, "Aim High", false),
		}},
		{effectLongCase{
			name:   "spider_woman_target_cannot_be_untapped_by_an_effect",
			prefer: []string{spider, bears, "Aim High"},
			decks: [2][]string{append(deckOf(8, spider), deckOf(16, "Island")...),
				append(append(deckOf(5, bears), deckOf(5, "Aim High")...), deckOf(14, "Forest")...)},
			surface: "full",
			reached: durationEffectUntap(spider, bears, "Aim High", true),
		}},
	}
}

func TestReplDurationReprosReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range replDurationCases() {
		t.Run(c.name, func(t *testing.T) {
			replDurationReplay(t, reg, c)
		})
	}
}

func replDurationReplay(t *testing.T, reg *cards.Registry, c replDurationCase) {
	t.Helper()
	r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	script := &ScriptPolicy{Actions: r.Actions}
	res, reached := effectLongPlay(t, reg, c.effectLongCase, r, printedDelveSet(t, r.Cards), script)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if !reached {
		t.Fatalf("the recorded game never reaches %s", c.name)
	}
}

func TestReplDurationRecord(t *testing.T) {
	if os.Getenv("REPL_DURATION_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips, since a skip here must name a missing pool (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range replDurationCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			for seed := uint64(1); seed <= 600; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c.effectLongCase, r, set, effectLongPolicy(m, c.effectLongCase, seed))
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
