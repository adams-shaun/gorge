package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// kdmgRefusals lowers a synthetic instant whose DamageSource$ node deals
// `amount` damage to a `valid` target and returns its refusal reasons.
func kdmgRefusals(t *testing.T, valid, amount string) []string {
	t.Helper()
	src := strings.Join([]string{
		"Name:Probe",
		"ManaCost:R",
		"Types:Instant",
		"A:SP$ Pump | ValidTgts$ Creature.YouCtrl | SubAbility$ DBDmg | SpellDescription$ Probe.",
		"SVar:DBDmg:DB$ DealDamage | ValidTgts$ " + valid + " | NumDmg$ " + amount + " | DamageSource$ ParentTarget",
		"SVar:SrcPower:ParentTargeted$CardPower",
	}, "\n")
	card, diags := cards.ParseBytes("Probe.txt", []byte(src))
	if card == nil || len(diags) != 0 {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Probe"}, byName: map[string]*cards.Card{"Probe": card},
		nDecks: map[string]int{}, reg: &cards.Registry{}}
	lw := lowerPool(p)
	var out []string
	for _, r := range lw.cards[0].Refusals {
		out = append(out, r.Reason)
	}
	return out
}

// TestGenDamageSourcePlayerTargetRefusal: Bite / DealDamageFrom recipients
// are permanent slots only, so a DamageSource$ target slot that may name a
// player is refused (the kernel would offer the player and drop the op).
func TestGenDamageSourcePlayerTargetRefusal(t *testing.T) {
	const reason = "DamageSource$ to a target slot that may name a player"
	for _, c := range []struct {
		name, valid, amount string
		refused             bool
	}{
		{"mixed, source power", "Any", "SrcPower", true},
		{"player-only, source power", "Player", "SrcPower", true},
		{"mixed, other amount", "Any", "2", true},
		{"creature, source power", "Creature", "SrcPower", false},
		{"creature/planeswalker", "Creature,Planeswalker", "SrcPower", false},
		{"creature, other amount", "Creature", "2", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := kdmgRefusals(t, c.valid, c.amount)
			if c.refused {
				kdefWant(t, got, reason)
			} else if len(got) != 0 {
				t.Errorf("unexpected refusals %q", got)
			}
		})
	}
}
