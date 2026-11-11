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
// Wood (a defender, so the game runs long) tapped through its controller's
// untap steps; Dungeon Geists and Time of Ice (a Saga) beside them; a Vitalize
// that untaps the Wall under each of the two locks; and Ty Lee's lock ending
// when an opponent's Act of Treason takes her (control change) or a Lightning
// Bolt kills her (source gone), after which the Wall untaps.
// TestStdSetReprosLockstep replays the repros; TestReplDurationReprosReach
// requires each recorded game to reach the observable the case names.
//
// The games are recorded by TestReplDurationRecord (REPL_DURATION_RECORD=1) on
// the kernel table built from the decks.
type replDurationCase struct {
	effectLongCase
	gated, needOut string // cast the gated spell only once needOut is on the battlefield
}

// durationLockRun is what walkDurationLock reads off a finished game: src
// entered, its enters trigger tapped an opposing victim (in the same step),
// and then what happened to that lock.
type durationLockRun struct {
	locked       bool // the victim was tapped as the source entered
	heldSteps    int  // the victim's controller's untap steps that left it tapped before any release
	released     bool // the source changed controller, or (with leaves) left the battlefield
	stepsAfter   int  // the victim's controller's untap steps after the release
	untapsOn     int  // untaps of the victim before any release, in the untap step
	effectUntaps int  // untaps of the victim outside the untap step, before any release
	untapsOff    int  // untaps of the victim after the release
	srcGone      bool // the source left the battlefield before the victim first untapped after the release
	spells       int  // resolutions of the named spell after the victim was locked, before any release
}

func walkDurationLock(g *GorgeSide, src, victim, spell string, leaves bool) durationLockRun {
	var r durationLockRun
	var srcObj, locked state.ObjID
	active := state.PlayerID(0)
	inUntap, fresh := false, false // fresh: no step has begun since src entered
	for _, e := range g.E.L.Events {
		switch {
		case e.Kind == events.MoveZone && e.To == state.ZBattlefield && srcObj == 0:
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == src {
				srcObj, fresh = e.Obj, true
			}
		case e.Kind == events.StepChange:
			fresh = false
			inUntap = e.Step == state.StepUntap
			if !inUntap || locked == 0 {
				continue
			}
			if t := g.E.G.Obj(locked); t != nil && t.Controller == active {
				if r.released {
					r.stepsAfter++
				} else if r.untapsOn == 0 {
					r.heldSteps++
				}
			}
		case e.Kind == events.TurnChange:
			fresh = false
			active = e.Player
		case e.Kind == events.Tap && fresh && locked == 0 && e.Obj != srcObj:
			t, s := g.E.G.Obj(e.Obj), g.E.G.Obj(srcObj)
			if t != nil && s != nil && t.Face() != nil && t.Face().Name == victim && t.Controller != s.Controller {
				locked, r.locked = e.Obj, true
			}
		case locked == 0:
		case e.Kind == events.Resolve && spell != "":
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == spell && !r.released {
				r.spells++
			}
		case e.Kind == events.MoveZone && e.Obj == srcObj && e.From == state.ZBattlefield && r.untapsOff == 0:
			r.srcGone = true
			if leaves {
				r.released = true
			}
		case e.Kind == events.Untap && e.Obj == locked:
			switch {
			case r.released:
				r.untapsOff++
			case inUntap:
				r.untapsOn++
			default:
				r.effectUntaps++
			}
		case !leaves && e.Kind == events.ControlChange && e.Obj == srcObj:
			r.released = true
		}
	}
	return r
}

// durationHeld: the lock held the victim tapped through at least one of its
// controller's untap steps, and nothing untapped it while its source stayed.
// A source that leaves (a second copy's legend rule) ends the lock (CR
// 611.2b), so the walk stops reading untaps there.
func durationHeld(src, victim string) func(g *GorgeSide) bool {
	return func(g *GorgeSide) bool {
		r := walkDurationLock(g, src, victim, "", true)
		return r.heldSteps >= 1 && r.untapsOn == 0 && r.effectUntaps == 0
	}
}

// durationEffectUntap: after a step the lock held, spell (an untap-all of the
// victim's controller's creatures) resolved while the source held it. held
// says the victim stayed tapped (a lock on every untap); not held that the
// spell untapped it (a lock on the untap step alone).
func durationEffectUntap(src, victim, spell string, held bool) func(g *GorgeSide) bool {
	return func(g *GorgeSide) bool {
		r := walkDurationLock(g, src, victim, spell, false)
		if r.spells == 0 || r.heldSteps == 0 || r.untapsOn != 0 {
			return false
		}
		if held {
			return r.effectUntaps == 0
		}
		return r.effectUntaps >= 1
	}
}

// durationReleased: the lock held through an untap step, the source then
// changed controller (or left), and the victim untapped in a later untap
// step. A control change counts only with the source still on the battlefield
// then, so it is the control change that freed the victim.
func durationReleased(src, victim string, leaves bool) func(g *GorgeSide) bool {
	return func(g *GorgeSide) bool {
		r := walkDurationLock(g, src, victim, "", leaves)
		return r.heldSteps >= 1 && r.released && r.stepsAfter >= 1 && r.untapsOff >= 1 && (leaves || !r.srcGone)
	}
}

func replDurationCases() []replDurationCase {
	const tyLee, spider, wall = "Ty Lee, Chi Blocker", "Spider-Woman, Secret Agent", "Wall of Wood"
	wallDeck := func() []string { return append(deckOf(8, wall), deckOf(16, "Forest")...) }
	vitalizeDeck := func() []string {
		return append(append(deckOf(5, wall), deckOf(5, "Vitalize")...), deckOf(14, "Forest")...)
	}
	return []replDurationCase{
		{effectLongCase: effectLongCase{
			name:    "ty_lee_target_stays_tapped_through_its_untap_step",
			prefer:  []string{tyLee, wall},
			decks:   [2][]string{append(deckOf(8, tyLee), deckOf(16, "Island")...), wallDeck()},
			surface: "autopay",
			reached: durationHeld(tyLee, wall),
		}},
		{effectLongCase: effectLongCase{
			name:    "spider_woman_target_stays_tapped_through_its_untap_step",
			prefer:  []string{spider, wall},
			decks:   [2][]string{append(deckOf(8, spider), deckOf(16, "Island")...), wallDeck()},
			surface: "full", // the hybrid {W/U} is priced only on the full surface
			reached: durationHeld(spider, wall),
		}},
		{effectLongCase: effectLongCase{
			name:    "dungeon_geists_target_stays_tapped_through_its_untap_step",
			prefer:  []string{"Dungeon Geists", wall},
			decks:   [2][]string{append(deckOf(8, "Dungeon Geists"), deckOf(16, "Island")...), wallDeck()},
			surface: "autopay",
			reached: durationHeld("Dungeon Geists", wall),
		}},
		{effectLongCase: effectLongCase{
			name:    "icefall_regent_target_stays_tapped_through_its_untap_step",
			prefer:  []string{"Icefall Regent", wall},
			decks:   [2][]string{append(deckOf(8, "Icefall Regent"), deckOf(16, "Island")...), wallDeck()},
			surface: "full",
			reached: durationHeld("Icefall Regent", wall),
		}},
		{effectLongCase: effectLongCase{
			name:    "merchant_raiders_target_stays_tapped_through_its_untap_step",
			prefer:  []string{"Merchant Raiders", wall},
			decks:   [2][]string{append(deckOf(8, "Merchant Raiders"), deckOf(16, "Island")...), wallDeck()},
			surface: "autopay",
			reached: durationHeld("Merchant Raiders", wall),
		}},
		{effectLongCase: effectLongCase{
			name:    "time_of_ice_target_stays_tapped_through_its_untap_step",
			prefer:  []string{"Time of Ice", wall},
			decks:   [2][]string{append(deckOf(8, "Time of Ice"), deckOf(16, "Island")...), wallDeck()},
			surface: "autopay",
			reached: durationHeld("Time of Ice", wall),
		}},
		{effectLongCase: effectLongCase{
			name:    "ty_lee_target_untaps_by_an_effect",
			prefer:  []string{tyLee, wall, "Vitalize"},
			decks:   [2][]string{append(deckOf(8, tyLee), deckOf(16, "Island")...), vitalizeDeck()},
			surface: "autopay",
			reached: durationEffectUntap(tyLee, wall, "Vitalize", false),
		}, gated: "Vitalize", needOut: tyLee},
		{effectLongCase: effectLongCase{
			name:    "spider_woman_target_cannot_be_untapped_by_an_effect",
			prefer:  []string{spider, wall, "Vitalize"},
			decks:   [2][]string{append(deckOf(8, spider), deckOf(16, "Island")...), vitalizeDeck()},
			surface: "full",
			reached: durationEffectUntap(spider, wall, "Vitalize", true),
		}, gated: "Vitalize", needOut: spider},
		{effectLongCase: effectLongCase{
			name:   "ty_lee_stolen_by_act_of_treason_frees_the_target",
			prefer: []string{tyLee, wall, "Act of Treason"},
			decks: [2][]string{append(deckOf(2, tyLee), deckOf(22, "Island")...),
				append(append(deckOf(5, wall), deckOf(4, "Act of Treason")...), append(deckOf(7, "Forest"), deckOf(8, "Mountain")...)...)},
			surface: "autopay",
			reached: durationReleased(tyLee, wall, false),
		}, gated: "Act of Treason", needOut: tyLee},
		{effectLongCase: effectLongCase{
			name:   "ty_lee_dies_to_lightning_bolt_frees_the_target",
			prefer: []string{tyLee, wall, "Lightning Bolt"},
			decks: [2][]string{append(deckOf(8, tyLee), deckOf(16, "Island")...),
				append(append(deckOf(5, wall), deckOf(5, "Lightning Bolt")...), append(deckOf(7, "Forest"), deckOf(7, "Mountain")...)...)},
			surface: "autopay",
			reached: durationReleased(tyLee, wall, true),
		}, gated: "Lightning Bolt", needOut: tyLee},
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
				res, reached := effectLongPlay(t, reg, c.effectLongCase, r, set, exileMovedPolicy(m, exileMovedCase{effectLongCase: c.effectLongCase, gated: c.gated, needOut: c.needOut}, seed))
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
