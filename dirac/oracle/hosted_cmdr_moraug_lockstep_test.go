package oracle

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Moraug, Fury of Akoum's two mechanisms, played decision by decision against
// gorge (gorge leads):
//
//   - the AffectedX static "each creature you control gets +1/+0 for each time
//     it has attacked this turn" (Count$CardNumAttacksThisTurn, the kernel's
//     per-object attack tally, dirac/value_attacktally.go);
//   - the landfall's DB$ AddPhase with ExtraPhaseDelayedTrigger$: an extra
//     combat after this main phase, at whose beginning every creature its
//     controller controls untaps (dirac/act_addphaserider.go).
//
// The real Moraug costs six, which the mirror reaches late if at all, so its
// case proves kgen lowers the printed card and the engines agree; the shapes
// are forced by "Moraug Probe", Moraug's own script on a {1}{R} 2/2 body. The
// probe is derived from the corpus script at run time (moraugProbeScript), so
// no line of the GPL script is committed (TestNoVerbatimForgeLines). The
// policy casts the probe, then plays its land only in the second main phase:
// the combat already fought tapped the attackers, the landfall's extra combat
// untaps them at its beginning and they attack again, so a creature attacks
// twice in one turn (reading +2/+0) and the untap acts on tapped creatures.

// moraugProbeScript is the corpus's Moraug, Fury of Akoum with its name, cost
// and power/toughness replaced; every ability line is the printed card's.
func moraugProbeScript(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(corpusDirOrSkip(t), "cardsfolder", "m", "moraug_fury_of_akoum.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	n := 0
	for _, line := range strings.Split(strings.TrimRight(string(body), "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "Name:"):
			line, n = "Name:Moraug Probe", n+1
		case strings.HasPrefix(line, "ManaCost:"):
			line, n = "ManaCost:1 R", n+1
		case strings.HasPrefix(line, "Types:"):
			line, n = "Types:Creature Minotaur Warrior", n+1
		case strings.HasPrefix(line, "PT:"):
			line, n = "PT:2/2", n+1
		}
		b.WriteString(line + "\n")
	}
	if n != 4 {
		t.Fatalf("the Moraug script has %d of its Name/ManaCost/Types/PT lines, want 4", n)
	}
	return b.String()
}

func moraugCases(t *testing.T, reached func(g *GorgeSide) bool) []hostedCmdrCase {
	probeDeck := append(deckOf(12, "Moraug Probe"), deckOf(28, "Mountain")...)
	moraugDeck := append(deckOf(8, "Moraug, Fury of Akoum"), deckOf(32, "Mountain")...)
	bearsDeck := append(deckOf(8, "Grizzly Bears"), deckOf(32, "Forest")...)
	return []hostedCmdrCase{
		{
			name:     "moraug_probe",
			carrier:  "Moraug Probe",
			seats:    [2]string{"hcm-probe-a", "hcm-probe-b"},
			decks:    [2][]string{probeDeck, bearsDeck},
			fixtures: map[string]string{"Moraug Probe": moraugProbeScript(t)},
			reached:  reached,
		},
		{
			name:    "moraug_printed",
			carrier: "Moraug, Fury of Akoum",
			seats:   [2]string{"hcm-moraug-a", "hcm-moraug-b"},
			decks:   [2][]string{moraugDeck, bearsDeck},
			surface: true,
		},
	}
}

// moraugAttackedTwice: with a seat-0 probe on the battlefield, one object was
// declared as an attacker twice in a turn, so the static read +2/+0 for it.
func moraugAttackedTwice(g *GorgeSide) bool {
	probe := false
	seen := map[state.ObjID]bool{}
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		switch e.Kind {
		case events.TurnChange:
			seen = map[state.ObjID]bool{}
		case events.MoveZone:
			if o := g.E.G.Obj(e.Obj); e.To == state.ZBattlefield && o != nil && o.Face() != nil && o.Face().Name == "Moraug Probe" && o.Controller == 0 {
				probe = true
			}
		case events.DeclareAttackers:
			for _, id := range e.IDs {
				if seen[id] && probe {
					return true
				}
				seen[id] = true
			}
		}
	}
	return false
}

// moraugExtraCombatUntap: after a seat-0 extra-phase grant, a creature that
// had attacked earlier that turn untapped during a beginning of combat step
// (the rider's UntapAll at the extra combat's beginning).
func moraugExtraCombatUntap(g *GorgeSide) bool {
	granted := false
	step := state.Step(0)
	attacked := map[state.ObjID]bool{}
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		switch e.Kind {
		case events.TurnChange:
			granted, attacked = false, map[state.ObjID]bool{}
		case events.StepChange:
			step = e.Step
		case events.ExtraPhase:
			if e.Amount > 0 && e.Player == 0 {
				granted = true
			}
		case events.DeclareAttackers:
			for _, id := range e.IDs {
				attacked[id] = true
			}
		case events.Untap:
			if granted && step == state.StepBeginCombat && attacked[e.Obj] {
				return true
			}
		}
	}
	return false
}

// moraugPolicy is hostedCmdrKwCostsPolicy with seat 0's land held for the
// second main phase once a probe is out: the landfall then grants the extra
// combat after the combat already fought.
func moraugPolicy(m *Matchup, c hostedCmdrCase, seed uint64) Policy {
	base := hostedCmdrKwCostsPolicy(m, c, seed)
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind != DPriority || p.D.Player != 0 || p.G == nil {
			return base.Pick(p)
		}
		for _, a := range p.D.Options {
			if (a.Kind() == ActCast || a.Kind() == ActActivate) && m.InstanceCard(int(a.Src())) == c.carrier {
				return a
			}
		}
		probeOut := false
		for i := range p.G.E.G.Objs {
			o := &p.G.E.G.Objs[i]
			if o.Zone == state.ZBattlefield && o.Controller == 0 && o.Face() != nil && o.Face().Name == c.carrier {
				probeOut = true
				break
			}
		}
		if !probeOut || p.G.E.G.Step == state.StepMain2 {
			for _, a := range p.D.Options {
				if a.Kind() == ActPlayLand {
					return a
				}
			}
		}
		for _, a := range p.D.Options {
			if a.Kind() != ActPlayLand && a.Kind() != ActCast && a.Kind() != ActActivate {
				return a // pass rather than play the land early or cast at random
			}
		}
		return rnd.Pick(p)
	})
}

func runMoraugCases(t *testing.T, reached func(g *GorgeSide) bool) {
	for _, c := range moraugCases(t, reached) {
		t.Run(c.name, func(t *testing.T) {
			if div := moraugRun(t, c, false); div != "" {
				t.Fatal(div)
			}
		})
	}
}

// TestHostedCmdrMoraugAttackTallyLockstep: the probe's static reads each
// creature's attacks this turn; the case fails unless some seed has a
// creature attack twice in a turn under the probe (the +2/+0 read).
func TestHostedCmdrMoraugAttackTallyLockstep(t *testing.T) {
	runMoraugCases(t, moraugAttackedTwice)
}

// TestHostedCmdrMoraugExtraCombatUntapLockstep: the landfall's extra combat
// untaps the creatures that attacked in the combat before it; the case fails
// unless some seed reaches that untap.
func TestHostedCmdrMoraugExtraCombatUntapLockstep(t *testing.T) {
	runMoraugCases(t, moraugExtraCombatUntap)
}

// TestHostedCmdrMoraugMutation: each kernel half's mutation switch makes its
// probe case diverge, so the lockstep cases can fail.
func TestHostedCmdrMoraugMutation(t *testing.T) {
	probe := moraugCases(t, nil)[0]
	for _, m := range []struct {
		name string
		sw   *bool
	}{
		{"attack_tally_once", &dirac.DebugAtkTallyOnce},
		{"extra_combat_rider_off", &dirac.DebugExtraCombatRiderOff},
	} {
		t.Run(m.name, func(t *testing.T) {
			*m.sw = true
			defer func() { *m.sw = false }()
			if moraugRun(t, probe, true) == "" {
				t.Fatalf("%s: no seed diverged with the mutation on", m.name)
			}
		})
	}
}

// moraugRun plays the case's seeds in lockstep (the
// TestHostedCmdrKeywordsCostsLockstep loop) and returns the first divergence,
// or, with stopAtDiv unset, a failure for too few real games or an unreached
// predicate; "" when the case holds. With stopAtDiv set it plays until a seed
// diverges and returns that divergence ("" if none does).
func moraugRun(t *testing.T, c hostedCmdrCase, stopAtDiv bool) string {
	t.Helper()
	dir := filters11Corpus(t, c.fixtures, "Mountain", "Forest", "Grizzly Bears", "Moraug, Fury of Akoum")
	reg, err := cards.SharedCorpus(dir)
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
	var decks [2][]*cards.Card
	for s, names := range c.decks {
		for _, n := range names {
			card, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("%s not in the corpus", n)
			}
			decks[s] = append(decks[s], card)
		}
	}
	set := printedDelveSetIn(t, dir, c.decks) // kgen over the decks: a refused card fails here
	tbl, _, _, err := set.Matchup(entries(c.decks[0]), entries(c.decks[1]))
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{GorgeAutopaySurface: c.surface}
	seen, played := false, 0
	for seed := uint64(1); seed <= 60 && (stopAtDiv || !(seen && played >= 3)); seed++ {
		m, err := NewMatchup(reg, c.seats, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 40}
		rc.Finish = func(g *GorgeSide) {
			if !seen && c.reached != nil && c.reached(g) {
				seen = true
			}
		}
		res := Lockstep(m, seed, k, moraugPolicy(m, c, seed), rc)
		CloseSide(k)
		if d := res.Div; d != nil {
			return fmt.Sprintf("seed %d: divergence at %d: %s %s %v\ngorge %v\ndirac %v", seed, d.At, d.What, d.Field, d.Diff, d.RefOptions, d.KerOptions)
		}
		if res.Decisions > 100 {
			played++
		}
	}
	if stopAtDiv {
		return ""
	}
	if played < 3 {
		return fmt.Sprintf("only %d seeds played a real game for %s", played, c.name)
	}
	if c.reached != nil && !seen {
		return c.name + ": the mechanism was not reached on any seed played, so the lockstep is vacuous"
	}
	return ""
}
