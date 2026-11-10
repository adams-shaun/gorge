package kgen

import (
	"bytes"
	"strings"
	"testing"
)

// lirOf is a lowered card's IR dump (the text kgen -lir prints).
func lirOf(c *lCard) string {
	var b bytes.Buffer
	dumpLIR(&b, c)
	return b.String()
}

// TestTargetedPlayerAmount: TargetedPlayer$Amount over an opponents-only
// player slot lowers to "the slot holds a player" (two-player the count is
// 0 or 1), including Hollow Marauder's draw count with the remembered
// discards of mana value 4 or more taken off; a slot that may hold the
// controller refuses by name, because its count reaches 2.
func TestTargetedPlayerAmount(t *testing.T) {
	cs := lowerNamed(t, []string{"Hollow Marauder"})
	if len(cs) != 1 || cs[0].Name != "Hollow Marauder" {
		t.Fatalf("lowered %d cards; want Hollow Marauder", len(cs))
	}
	for _, r := range cs[0].Refusals {
		t.Errorf("Hollow Marauder refused: %s", r.Reason)
	}
	if got := lirOf(cs[0]); !strings.Contains(got, "TargetPlayers(Target0)") || !strings.Contains(got, "RefCount(") {
		t.Errorf("Hollow Marauder's draw is not the slot's player count less the remembered big discards:\n%s", got)
	}
	probe := func(name, tgts string) []refusal {
		src := "Name:" + name + "\nManaCost:2 B\nTypes:Creature Specter\nPT:2/2\n" +
			"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigDiscard | TriggerDescription$ x\n" +
			"SVar:TrigDiscard:DB$ Discard | " + tgts + " | NumCards$ 1 | RememberDiscarded$ True | SubAbility$ DBDraw\n" +
			"SVar:DBDraw:DB$ Draw | NumCards$ Y | Defined$ You | SubAbility$ DBCleanup\n" +
			"SVar:DBCleanup:DB$ Cleanup | ClearRemembered$ True\n" +
			"SVar:Y:TargetedPlayer$Amount\n"
		_, rs := kwardpowBuild(t, name, src)
		return rs
	}
	if rs := probe("Opponent Slot Probe", "ValidTgts$ Opponent | TargetMin$ 0 | TargetMax$ 1 | TgtPrompt$ x"); len(rs) != 0 {
		t.Errorf("an opponents-only slot refused: %v", rs)
	}
	rs := probe("Player Slot Probe", "ValidTgts$ Player | TargetMin$ 0 | TargetMax$ 2 | TgtPrompt$ x")
	if all := kpaylifexReasons(rs); !strings.Contains(all, "TargetedPlayer$Amount of a slot that is not opponents-only") {
		t.Errorf("a slot that may hold the controller: refusals %q, want the opponents-only refusal", all)
	}
}

// TestDiscardedAmountTriggerCost: a trigger's own Discard<1/Hand> cost does
// not lower Discarded$Amount. gorge's triggered-cost window settles the
// hand's discards without recording them as the body's paid-cost list, so the
// body reads an empty list and U.S.S. Enterprise-C draws nothing; the card
// stays refused by that head until gorge counts the cards (a separate ticket)
// rather than dirac copying the zero.
func TestDiscardedAmountTriggerCost(t *testing.T) {
	cs := lowerNamed(t, []string{"U.S.S. Enterprise-C, Time-Tossed"})
	if len(cs) != 1 {
		t.Fatalf("lowered %d cards; want one", len(cs))
	}
	var all []string
	for _, r := range cs[0].Refusals {
		all = append(all, r.Reason)
	}
	if got := strings.Join(all, " | "); !strings.Contains(got, "Discarded$Amount") {
		t.Errorf("Enterprise-C refusals %q; want the Discarded$Amount head named", got)
	}
}

// TestPayLifeTriggeredPower: a printed trigger's own "pay life equal to its
// power" (PayLife<X>, X the triggering card's power) lowers to a life payment
// whose amount is that power read as the cost is offered, and the counters
// the body puts read the same value; any other X body refuses by name, and
// the triggering card's power does not lower on an activated ability's cost
// or a dies trigger's.
func TestPayLifeTriggeredPower(t *testing.T) {
	cs := lowerNamed(t, []string{"Madame Null, Power Broker"})
	if len(cs) != 1 || cs[0].Name != "Madame Null, Power Broker" {
		t.Fatalf("lowered %d cards; want Madame Null", len(cs))
	}
	for _, r := range cs[0].Refusals {
		t.Errorf("Madame Null refused: %s", r.Reason)
	}
	if got := lirOf(cs[0]); !strings.Contains(got, "IfPaid obj=You A=PowerOf(Triggered) CS=PayLife") {
		t.Errorf("Madame Null's cost is not a life payment of the triggering card's power:\n%s", got)
	}
	trig := func(name, ability, x string) []refusal {
		src := "Name:" + name + "\nManaCost:2 B\nTypes:Creature Demon\nPT:1/3\n" +
			"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Creature.Other+YouCtrl | Execute$ TrigPut | TriggerZones$ Battlefield\n" +
			"SVar:TrigPut:AB$ PutCounter | Cost$ PayLife<X> | Defined$ TriggeredCardLKICopy | CounterType$ P1P1 | CounterNum$ X\nSVar:X:" + x + "\n" + ability
		_, rs := kwardpowBuild(t, name, src)
		return rs
	}
	if rs := trig("Power Probe", "", "TriggeredCard$CardPower"); len(rs) != 0 {
		t.Errorf("the triggering card's power refused: %v", rs)
	}
	for _, x := range []string{"Count$CardPower", "TriggeredCard$CardToughness", "TriggeredCard$CardManaCost"} {
		if all := kpaylifexReasons(trig("Body Probe", "", x)); !strings.Contains(all, "life cost PayLife<X> with SVar X") {
			t.Errorf("SVar X %s: refusals %q, want the PayLife<X> with SVar X refusal", x, all)
		}
	}
	// the same cost on a dies trigger (Mask of Griselbrand) is a different
	// moment for the power to be read and is not played against gorge: it
	// refuses by name
	mask := lowerNamed(t, []string{"Mask of Griselbrand"})
	var all []string
	for _, r := range mask[0].Refusals {
		all = append(all, r.Reason)
	}
	if got := strings.Join(all, " | "); !strings.Contains(got, "life cost PayLife<X> with SVar X") {
		t.Errorf("Mask of Griselbrand refusals %q, want the PayLife<X> with SVar X refusal", got)
	}
	act := "A:AB$ Draw | Cost$ T PayLife<X> | NumCards$ 1\n"
	if all := kpaylifexReasons(trig("Activated Probe", act, "TriggeredCard$CardPower")); !strings.Contains(all, "life cost PayLife<X> with SVar X") {
		t.Errorf("an activated ability's PayLife<X> by the triggering card's power: refusals %q, want the PayLife<X> with SVar X refusal", all)
	}
}
