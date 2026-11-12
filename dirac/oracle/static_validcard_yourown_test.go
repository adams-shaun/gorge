package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// svcyoOn reports whether seat p controls a battlefield permanent named name
// (the ids slice is g.Zone's).
func svcyoOn(g *state.Game, p state.PlayerID, name string) bool {
	for _, id := range g.Zone(state.ZBattlefield, p) {
		if chName(g, id) == name {
			return true
		}
	}
	return false
}

// svcyoReach is one recorded game's reach predicate: it records into st what
// the game did since event index from, as the lockstep rows alone would
// accept a script that never got there.
type svcyoReach func(g *state.Game, evs []events.Event, from int, st map[string]bool)

var svcyoCases = []struct {
	file  string
	reach svcyoReach
	want  []string
}{
	// Unwinding Clock: during the other player's untap step an Alpha Myr its
	// controller attacked with untaps (a non-Self ValidCard$ of
	// UntapOtherPlayer).
	{"static_untap_other_own_artifacts_unwinding_clock", func(g *state.Game, evs []events.Event, from int, st map[string]bool) {
		for _, ev := range evs[from:] {
			if ev.Kind != events.Untap || g.Active != 1 {
				continue
			}
			if o := g.Obj(ev.Obj); o != nil && o.Controller == 0 && chName(g, ev.Obj) == "Alpha Myr" && svcyoOn(g, 0, "Unwinding Clock") {
				st["artifact untapped in the other player's untap step"] = true
			}
		}
	}, []string{"artifact untapped in the other player's untap step"}},
	// Ghalta the Immovable: an Reito Sentinel (Defender) attacks.
	{"static_can_attack_defender_own_creatures_ghalta", func(g *state.Game, evs []events.Event, from int, st map[string]bool) {
		for _, ev := range evs[from:] {
			if ev.Kind != events.DeclareAttackers || !svcyoOn(g, 0, "Ghalta the Immovable") {
				continue
			}
			for _, id := range ev.IDs {
				if chName(g, id) == "Reito Sentinel" {
					st["a defender attacked beside Ghalta"] = true
				}
			}
		}
	}, []string{"a defender attacked beside Ghalta"}},
	// Eriette of the Charmed Apple: the other player declares attackers
	// holding a creature Eriette's controller's Aura enchants.
	{"static_cant_attack_enchanted_by_own_aura_eriette", func(g *state.Game, evs []events.Event, from int, st map[string]bool) {
		if g.Step != state.StepDeclareAttackers || g.Active != 1 || !svcyoOn(g, 0, "Eriette of the Charmed Apple") {
			return
		}
		for _, id := range g.Zone(state.ZBattlefield, 0) {
			o := g.Obj(id)
			if o == nil || o.AttachedTo == 0 {
				continue
			}
			if host := g.Obj(o.AttachedTo); host != nil && host.Controller == 1 {
				st["an enchanted creature faced a declaration"] = true
			}
		}
	}, []string{"an enchanted creature faced a declaration"}},
	// Guardians of Oboro: a modified creature (here Guardians itself under
	// an Aura) attacks as though it lacked defender.
	{"static_can_attack_defender_modified_guardians_of_oboro", func(g *state.Game, evs []events.Event, from int, st map[string]bool) {
		for _, ev := range evs[from:] {
			if ev.Kind != events.DeclareAttackers {
				continue
			}
			for _, id := range ev.IDs {
				if o := g.Obj(id); o != nil && chName(g, id) == "Guardians of Oboro" && o.Controller == 0 {
					st["a modified defender attacked"] = true
				}
			}
		}
	}, []string{"a modified defender attacked"}},
	// Prop Room: a creature its controller attacked with untaps during the
	// other player's untap step.
	{"static_untap_other_own_creatures_prop_room", func(g *state.Game, evs []events.Event, from int, st map[string]bool) {
		for _, ev := range evs[from:] {
			if ev.Kind != events.Untap || g.Active != 1 {
				continue
			}
			if o := g.Obj(ev.Obj); o != nil && o.Controller == 0 && chName(g, ev.Obj) == "Savannah Lions" && svcyoOn(g, 0, "Prop Room") {
				st["a creature untapped in the other player's untap step"] = true
			}
		}
	}, []string{"a creature untapped in the other player's untap step"}},
}

// svcyoRun replays repro file in lockstep, calling reach after every
// decision: the result, the reach map and the script's skipped count.
func svcyoRun(t *testing.T, file string, reach svcyoReach) (Result, map[string]bool, int) {
	t.Helper()
	reg := synthReg(t)
	r, err := ReadRepro(filepath.Join("testdata", "repro", file+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	set := printedDelveSet(t, r.Cards)
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 200
	st, seen := map[string]bool{}, 0
	rc.Check = func(ref *GorgeSide, _ Side) *Divergence {
		reach(ref.E.G, ref.E.L.Events, seen, st)
		seen = len(ref.E.L.Events)
		return nil
	}
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	script := &ScriptPolicy{Actions: r.Actions}
	res := Lockstep(m, r.Seed, k, script, rc)
	return res, st, script.Skipped
}

// TestStaticValidCardYourOwnReprosReach: each recorded game replays clean in
// lockstep with every scripted action offered and reaches what it pins.
func TestStaticValidCardYourOwnReprosReach(t *testing.T) {
	t.Parallel()
	for _, c := range svcyoCases {
		t.Run(c.file, func(t *testing.T) {
			res, st, skipped := svcyoRun(t, c.file, c.reach)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if skipped != 0 {
				t.Errorf("%d scripted actions were not offered", skipped)
			}
			for _, w := range c.want {
				if !st[w] {
					t.Errorf("the recorded game never reaches %q", w)
				}
			}
		})
	}
}
