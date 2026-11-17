package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// sweepCapProbe is the Long River Lurker pair shape (an ETB Effect that
// remembers its target beside a CantBlockBy static and a combat
// DamageDealtOnce trigger) whose trigger body is the Arcbond emblem sweep:
// DamageAll | ValidCards$ Creature.!IsRemembered | ValidPlayers$ Player |
// NumDmg$ X | DamageSource$ Remembered. The captured object is the damage
// source, every other creature is hit, then each player, in one damage batch.
const sweepCapProbe = `Name:Sweep Cap Probe
ManaCost:2 U
Types:Creature Frog Scout
PT:2/3
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigCap | TriggerDescription$ probe
SVar:TrigCap:DB$ Effect | ValidTgts$ Creature.YouCtrl | RememberObjects$ Targeted | ExileOnMoved$ Battlefield | StaticAbilities$ Unblockable | Triggers$ TrigDmg
SVar:Unblockable:Mode$ CantBlockBy | Description$ probe | ValidAttacker$ Card.IsRemembered
SVar:TrigDmg:Mode$ DamageDealtOnce | ValidSource$ Creature.IsRemembered | CombatDamage$ True | Execute$ DmgOther | TriggerDescription$ probe
SVar:DmgOther:DB$ DamageAll | DamageSource$ Remembered | NumDmg$ X | ValidPlayers$ Player | ValidCards$ Creature.!IsRemembered
SVar:X:TriggerCount$Amount
`

// sweepCapSynth parses and lowers one inline probe script.
func sweepCapSynth(t *testing.T, src string) *lCard {
	t.Helper()
	card, diags := cards.ParseBytes("sweepcap_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	name := strings.TrimSpace(strings.SplitN(src, "\n", 2)[0][len("Name:"):])
	p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card},
		nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, name, card, false)
}

// sweepCapSweep returns the card's one SweepFrom instruction (nil: none).
func sweepCapSweep(lc *lCard) *lInstr {
	var sw *lInstr
	for _, ab := range lc.Abilities {
		for i := range ab.Prog {
			if ab.Prog[i].Op == "SweepFrom" {
				if sw != nil {
					return nil // more than one: not the single-sweep shape
				}
				sw = &ab.Prog[i]
			}
		}
	}
	return sw
}

// TestSweepFromCapturedPlayers: in a capturing emblem's DamageDealtOnce body
// the DamageAll with DamageSource$ Remembered lowers to ExtSweepFrom over the
// capture (rTriggeredSource): the captured object is the damage source,
// Creature.!IsRemembered folds into the C&1 own-slot exclusion (the kernel's
// emblem does not seed the remembered set the filter flag would read), and
// ValidPlayers$ Player adds the player arm of the same batch (C&2). Each
// out-of-shape variant still refuses by name.
func TestSweepFromCapturedPlayers(t *testing.T) {
	lc := sweepCapSynth(t, sweepCapProbe)
	if got := keclReason(lc.Refusals); got != "" {
		t.Fatalf("the probe refuses: %q", got)
	}
	sw := sweepCapSweep(lc)
	if sw == nil {
		t.Fatal("no SweepFrom lowered")
	}
	if sw.Obj.Kind != rTriggeredSource {
		t.Errorf("SweepFrom Obj kind %v, want the captured damage source (rTriggeredSource)", sw.Obj.Kind)
	}
	if sw.C&1 == 0 {
		t.Errorf("SweepFrom C %#x: want bit 1, the captured creature's own slot excluded (!IsRemembered)", sw.C)
	}
	if sw.C&2 == 0 {
		t.Errorf("SweepFrom C %#x: want bit 2, each player after the creatures", sw.C)
	}
	if sw.F == nil || len(sw.F.Clauses) != 1 || sw.F.Clauses[0].Types != tCreature {
		t.Fatalf("SweepFrom F %+v, want one creature clause", sw.F)
	}
	if sw.F.Players != 0 {
		t.Errorf("SweepFrom F.Players %#x: the player arm rides C, not the filter", sw.F.Players)
	}
	if fl := sw.F.Clauses[0].Flags; fl&(fRemembered|fNotRemembered) != 0 {
		t.Errorf("SweepFrom F flags %#x: the Remembered flag must fold into the C&1 exclusion", fl)
	}
	if sw.B.Kind != vTriggerAmount {
		t.Errorf("SweepFrom B kind %v, want the trigger's damage total (TriggerCount$DamageAmount)", sw.B.Kind)
	}

	// The same body without ValidPlayers$ is the creature-only sweep of the
	// same source: bit 2 unset.
	lc = sweepCapSynth(t, strings.Replace(sweepCapProbe, " | ValidPlayers$ Player", "", 1))
	if got := keclReason(lc.Refusals); got != "" {
		t.Fatalf("the creature-only probe refuses: %q", got)
	}
	if sw = sweepCapSweep(lc); sw == nil || sw.Obj.Kind != rTriggeredSource || sw.C&1 == 0 || sw.C&2 != 0 {
		t.Fatalf("creature-only sweep %+v, want the capture source, C&1 only", sw)
	}

	// ValidCards$ without !IsRemembered: every matching creature, the
	// captured one included, so no exclusion bit.
	lc = sweepCapSynth(t, strings.Replace(sweepCapProbe, "Creature.!IsRemembered", "Creature", 1))
	if got := keclReason(lc.Refusals); got != "" {
		t.Fatalf("the unexcluded probe refuses: %q", got)
	}
	if sw = sweepCapSweep(lc); sw == nil || sw.C&1 != 0 || sw.C&2 == 0 {
		t.Fatalf("unexcluded sweep C %#x, want bit 2 only", sw.C)
	}

	// Out of shape, each refuses by name.
	for _, c := range []struct {
		name, from, to, want string
	}{
		{"player value", "ValidPlayers$ Player", "ValidPlayers$ Opponent", "DamageAll DamageSource$ beside ValidPlayers"},
		{"source ParentTarget", "DamageSource$ Remembered", "DamageSource$ ParentTarget", "DamageAll DamageSource$ ParentTarget over 0 target slots"},
		{"RememberDamaged", "ValidPlayers$ Player", "RememberDamaged$ True", "DamageAll DamageSource$ beside RememberDamaged"},
		{"ReplaceDyingDefined", "ValidPlayers$ Player", "ReplaceDyingDefined$ Remembered", "DamageAll DamageSource$ beside ReplaceDyingDefined"},
		{"DamageMap", "ValidPlayers$ Player", "DamageMap$ True", "DamageAll DamageSource$ beside DamageMap"},
		{"ValidTgts", "ValidPlayers$ Player", "ValidTgts$ Player", "DamageAll DamageSource$ beside ValidTgts"},
		{"player scope", "Creature.!IsRemembered", "Creature,Player", "DamageAll DamageSource$ with a player scope"},
		{"plain IsRemembered", "Creature.!IsRemembered", "Creature.IsRemembered", "DamageAll DamageSource$ over an IsRemembered filter"},
		// An OR of clauses where only some exclude the capture: C&1 would
		// skip the capture's slot for every clause, but gorge's first
		// unexcluding clause still matches it. Refuse by name, both orders.
		{"mixed clauses, plain first", "Creature.!IsRemembered", "Creature.Red,Creature.!IsRemembered", "DamageAll DamageSource$ over a mixed IsRemembered filter"},
		{"mixed clauses, excluded first", "Creature.!IsRemembered", "Creature.!IsRemembered,Creature.Red", "DamageAll DamageSource$ over a mixed IsRemembered filter"},
	} {
		lc := sweepCapSynth(t, strings.Replace(sweepCapProbe, c.from, c.to, 1))
		if got := keclReason(lc.Refusals); !strings.Contains(got, c.want) {
			t.Errorf("%s: refusals %q, want %q", c.name, got, c.want)
		}
	}

	// DamageSource$ Remembered outside a capturing emblem's body stays
	// refused: no capture, no source.
	const noCap = `Name:Sweep NoCap Probe
ManaCost:1 R
Types:Instant
A:SP$ DamageAll | ValidCards$ Creature | ValidPlayers$ Player | NumDmg$ 2 | DamageSource$ Remembered | SpellDescription$ probe
`
	lc = sweepCapSynth(t, noCap)
	if got := keclReason(lc.Refusals); !strings.Contains(got, `DamageAll DamageSource$ "Remembered"`) {
		t.Errorf("no-capture probe: refusals %q, want the Remembered source refusal", got)
	}
}
