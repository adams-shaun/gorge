package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// the trigger-line qualifiers kgen lowers beyond the single
// shapes (ActivatorThisTurnCast$ and ValidAttackersAmount$ <op><n>,
// AttackedTarget$ You, a Self alternative beside ValidPlayer$ You, a player
// alternative in TargetsValid$), on synthetic scripts so the shapes of
// Jiang Yanggu, Sabotage Strategist and Esoteric Duplicator (whose bodies
// stay refused) are covered too.

// ktqBuild lowers and builds one synthetic card with the trigger line; it
// returns the card's refusals joined and its first triggered ability.
func ktqBuild(t *testing.T, trigger string) (string, kop.Trigger, *poolTables) {
	t.Helper()
	why, ab, pt := ktqBuildSV(t, trigger, "SVar:TrigGain:DB$ GainLife | LifeAmount$ 1")
	return why, ab.Trig, pt
}

// ktqBuildSV is ktqBuild with the card's SVar lines given; it returns the
// first triggered ability whole (its Prog is the lowered body).
func ktqBuildSV(t *testing.T, trigger string, svars ...string) (string, kop.Ability, *poolTables) {
	t.Helper()
	src := strings.Join(append([]string{"Name:Probe", "ManaCost:1 U", "Types:Creature Bear", "PT:2/2", trigger}, svars...), "\n")
	card, diags := cards.ParseBytes("Probe.txt", []byte(src))
	if card == nil || len(diags) != 0 {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Probe"}, byName: map[string]*cards.Card{"Probe": card},
		nDecks: map[string]int{}, reg: &cards.Registry{}}
	lp := lowerPool(p)
	var out []string
	for _, r := range lp.cards[0].Refusals {
		out = append(out, r.Reason)
	}
	if len(out) > 0 {
		return strings.Join(out, "; "), kop.Ability{}, nil
	}
	all, bs, pt, err := buildPool(lp)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range all {
		if c.Name != "Probe" {
			continue
		}
		var rs []string
		for _, r := range bs[i].refusals {
			rs = append(rs, r.Reason)
		}
		if len(rs) > 0 {
			return strings.Join(rs, "; "), kop.Ability{}, nil
		}
		for _, a := range bs[i].def.Abilities {
			if a.Kind == kop.AbTriggered {
				return "", a, pt
			}
		}
	}
	t.Fatal("no triggered ability built")
	return "", kop.Ability{}, nil
}

func TestTrigqualsRefusals(t *testing.T) {
	const tail = " | TriggerZones$ Battlefield | Execute$ TrigGain"
	cast := func(v string) string {
		return "T:Mode$ SpellCast | ValidCard$ Card.YouCtrl | ValidActivatingPlayer$ You | ActivatorThisTurnCast$ " + v + tail
	}
	atk := func(v string) string {
		return "T:Mode$ AttackersDeclared | ValidAttackers$ Creature.YouCtrl | ValidAttackersAmount$ " + v + tail
	}
	// ActivatorThisTurnCast$: EQ stays the plain literal, the other
	// operators ride the high byte, an out-of-range literal refuses
	for _, c := range []struct {
		v    string
		want int16
	}{{"EQ2", 2}, {"GT1", kop.DmgGate(kop.DmgGT, 1)}, {"LE3", kop.DmgGate(kop.DmgLE, 3)}, {"NE1", kop.DmgGate(kop.DmgNE, 1)}} {
		why, tr, _ := ktqBuild(t, cast(c.v))
		if why != "" || tr.N != c.want || tr.Flags2&kop.Tf2NthSpell == 0 {
			t.Errorf("ActivatorThisTurnCast %s: refusal %q, N %#x (want %#x)", c.v, why, tr.N, c.want)
		}
	}
	for _, v := range []string{"GT200", "GT0", "XX1", "GT", "EQ-1"} {
		if why, _, _ := ktqBuild(t, cast(v)); !strings.Contains(why, `ActivatorThisTurnCast "`+v+`"`) {
			t.Errorf("ActivatorThisTurnCast %q: refusal %q", v, why)
		}
	}
	// ValidAttackersAmount$: GE stays the plain literal
	for _, c := range []struct {
		v    string
		want int16
	}{{"GE2", 2}, {"EQ1", kop.DmgGate(kop.DmgEQ, 1)}, {"LT3", kop.DmgGate(kop.DmgLT, 3)}} {
		why, tr, _ := ktqBuild(t, atk(c.v))
		if why != "" || tr.N != c.want {
			t.Errorf("ValidAttackersAmount %s: refusal %q, N %#x (want %#x)", c.v, why, tr.N, c.want)
		}
	}
	for _, v := range []string{"XX1", "EQ200", "GE0"} {
		if why, _, _ := ktqBuild(t, atk(v)); !strings.Contains(why, `ValidAttackersAmount "`+v+`"`) {
			t.Errorf("ValidAttackersAmount %q: refusal %q", v, why)
		}
	}
	// AttackedTarget$: Opponent keeps Tf2AtkOpp, You is Tf2AtkYou
	for _, c := range []struct {
		v    string
		want kop.TrigFlags2
	}{{"Player", kop.Tf2DefPlayer}, {"Opponent", kop.Tf2DefPlayer | kop.Tf2AtkOpp}, {"You", kop.Tf2DefPlayer | kop.Tf2AtkYou}} {
		why, tr, _ := ktqBuild(t, "T:Mode$ AttackersDeclared | AttackedTarget$ "+c.v+tail)
		if why != "" || tr.Flags2&(kop.Tf2DefPlayer|kop.Tf2AtkOpp|kop.Tf2AtkYou) != c.want {
			t.Errorf("AttackedTarget %s: refusal %q, flags %#x (want %#x)", c.v, why, tr.Flags2&(kop.Tf2DefPlayer|kop.Tf2AtkOpp|kop.Tf2AtkYou), c.want)
		}
	}
	if why, _, _ := ktqBuild(t, "T:Mode$ AttackersDeclared | AttackedTarget$ Planeswalker"+tail); !strings.Contains(why, `AttackedTarget "Planeswalker"`) {
		t.Errorf("AttackedTarget Planeswalker: refusal %q", why)
	}
	// Sacrificed: a bare Self alternative beside ValidPlayer$ You only
	sac := func(player, extra string) string {
		return "T:Mode$ Sacrificed | ValidPlayer$ " + player + " | ValidCard$ Card.Self,Artifact.Other" + extra + tail
	}
	why, tr, pt := ktqBuild(t, sac("You", ""))
	if why != "" || tr.Flags2&kop.Tf2SelfOrFilter == 0 || tr.Filter == 0 || pt.filters[tr.Filter].Ctrl != kop.CtrlYou || pt.filters[tr.Filter].Types != typeMask(tArtifact) {
		t.Errorf("Sacrificed You with Self: refusal %q, trigger %+v", why, tr)
	}
	for _, c := range []struct{ trig, want string }{
		{sac("Opponent", ""), `Sacrificed ValidPlayer "Opponent" with ValidCard`},
		{sac("You", " | ActivationLimit$ 1"), "Sacrificed Self alternative with ActivationLimit"},
	} {
		if why, _, _ := ktqBuild(t, c.trig); !strings.Contains(why, c.want) {
			t.Errorf("%s: refusal %q, want %q", c.trig, why, c.want)
		}
	}
	// TargetsValid$: a You or Opponent alternative beside a battlefield
	// object one lowers with the player alternative on the Filter2 chain; a
	// zone-free creature alternative (Danitha, Spear of Agony) lowers too
	// (TestTargetsValidAnyZone pins its kop.ZoneAny marker)
	tv := func(v string) string {
		return "T:Mode$ SpellCast | ValidActivatingPlayer$ Opponent | TargetsValid$ " + v + tail
	}
	why, tr, pt = ktqBuild(t, tv("You,Creature.YouCtrl+inZoneBattlefield"))
	var player *kop.Filter
	for f := tr.Filter2; f != 0; f = pt.filters[f].Or {
		if pt.filters[f].Flags&kop.FPlayer != 0 {
			player = &pt.filters[f]
		}
	}
	if why != "" || tr.Flags2&kop.Tf2TargetsFilter2 == 0 || player == nil || player.Ctrl != kop.CtrlYou {
		t.Errorf("TargetsValid You,Creature: refusal %q, trigger %+v", why, tr)
	}
	for _, v := range []string{"Player,Creature.YouCtrl+inZoneBattlefield", "You"} {
		if why, _, _ := ktqBuild(t, tv(v)); !strings.Contains(why, `TargetsValid "`+v+`"`) {
			t.Errorf("TargetsValid %q: refusal %q", v, why)
		}
	}
}

// TestTrigqualsTriggeredAttackers: Defined$ TriggeredAttackers (the event's
// attacker group) lowers as the subject of a
// plain Pump or PutCounter in an AttackersDeclared trigger and refuses
// everywhere the kernel does not run it (an op it does not loop, a rider, a
// Double read, an Attacks trigger, a ConditionDefined$).
func TestTrigqualsTriggeredAttackers(t *testing.T) {
	const line = "T:Mode$ AttackersDeclared | ValidAttackers$ Creature.YouCtrl | ValidAttackersAmount$ EQ1 | TriggerZones$ Battlefield | Execute$ TrigX"
	body := func(s string) string { return "SVar:TrigX:" + s }
	for _, c := range []struct {
		name, body string
		op         kop.Op
	}{
		{"pump", "DB$ Pump | Defined$ TriggeredAttackers | NumAtt$ -1", kop.OpPumpUntilEOT},
		{"pump keyword", "DB$ Pump | Defined$ TriggeredAttackers | KW$ Double Strike", kop.OpGrantKeyword},
		{"counter", "DB$ PutCounter | Defined$ TriggeredAttackers | CounterType$ P1P1 | CounterNum$ 2", kop.OpPutCounter},
	} {
		why, ab, _ := ktqBuildSV(t, line, body(c.body))
		if why != "" || len(ab.Prog) == 0 {
			t.Errorf("%s: refusal %q", c.name, why)
			continue
		}
		for _, in := range ab.Prog {
			if in.Obj != kop.RefTriggerAttackers || in.Op != c.op {
				t.Errorf("%s: instruction %+v, want %v on RefTriggerAttackers", c.name, in, c.op)
			}
		}
	}
	for _, c := range []struct{ name, trig, body, want string }{
		{"damage", line, "DB$ DealDamage | Defined$ TriggeredAttackers | NumDmg$ 1", "TriggeredAttackers"},
		{"destroy", line, "DB$ Destroy | Defined$ TriggeredAttackers", "TriggeredAttackers"},
		{"pump double", line, "DB$ Pump | Defined$ TriggeredAttackers | NumAtt$ Double", "Double"},
		{"pump duration", line, "DB$ Pump | Defined$ TriggeredAttackers | KW$ Flying | Duration$ Permanent", "Duration"},
		{"pump atEOT", line, "DB$ Pump | Defined$ TriggeredAttackers | NumAtt$ 1 | AtEOT$ Sacrifice", "AtEOT"},
		{"pump must be blocked", line, "DB$ Pump | Defined$ TriggeredAttackers | KW$ HIDDEN CARDNAME must be blocked if able.", "KW$"},
		{"counter divided", line, "DB$ PutCounter | Defined$ TriggeredAttackers | CounterType$ P1P1 | CounterNum$ 3 | DividedAsYouChoose$ 3", "DividedAsYouChoose"},
		{"counter energy", line, "DB$ PutCounter | Defined$ TriggeredAttackers | CounterType$ ENERGY | CounterNum$ 1", "ENERGY"},
		{"counter remember", line, "DB$ PutCounter | Defined$ TriggeredAttackers | CounterType$ P1P1 | RememberAmount$ True", "RememberAmount"},
		{"attacks trigger", "T:Mode$ Attacks | ValidCard$ Creature.YouCtrl | TriggerZones$ Battlefield | Execute$ TrigX",
			"DB$ Pump | Defined$ TriggeredAttackers | NumAtt$ -1", "outside an AttackersDeclared trigger"},
		{"condition only", line, "DB$ Pump | Defined$ Self | NumAtt$ 1 | ConditionDefined$ TriggeredAttackers | ConditionPresent$ Card | ConditionCompare$ GE1", "read by something other than the node's subject"},
	} {
		why, _, _ := ktqBuildSV(t, c.trig, body(c.body))
		if why == "" {
			t.Errorf("%s: lowered, want a refusal", c.name)
		} else if !strings.Contains(why, c.want) {
			t.Errorf("%s: refusal %q, want %q", c.name, why, c.want)
		}
	}
}

// A ValidCard$ that turns some spells away makes ActivatorThisTurnCast$ count the
// spells it matches; one that turns none away keeps the turn's tally, and a
// provenance qualifier refuses.
func TestNthSpellFilterLowering(t *testing.T) {
	line := func(vc string) string {
		return "T:Mode$ SpellCast | ValidCard$ " + vc + " | ValidActivatingPlayer$ You | ActivatorThisTurnCast$ EQ1 | TriggerZones$ Battlefield | Execute$ TrigGain"
	}
	for vc, want := range map[string]bool{"Instant,Sorcery": true, "Card.nonCreature": true, "Card": false, "Card.YouCtrl": false} {
		why, tr, _ := ktqBuild(t, line(vc))
		if why != "" || tr.Flags2&kop.Tf2NthSpell == 0 || (tr.Flags4&kop.Tf4NthFiltered != 0) != want {
			t.Errorf("ValidCard %s: refusal %q, flags4 %#x, want filtered %v", vc, why, tr.Flags4, want)
		}
	}
	if why, _, _ := ktqBuild(t, line("Instant.YouCtrl+wasCastFromYourGraveyard")); why == "" {
		t.Errorf("a provenance ValidCard: refusal %q", why)
	}
	// The filter also tests pushes that have left the stack, so it names no zone, and
	// a clause the ring cannot read off every zone refuses.
	_, tr, pt := ktqBuild(t, line("Instant,Sorcery"))
	if z := pt.filters[tr.Filter].Zone; z != kop.ZoneNone {
		t.Errorf("ValidCard Instant,Sorcery: filter zone %v, want none", z)
	}
	if why, _, _ := ktqBuild(t, line("Card.MultiColor")); !strings.Contains(why, "ActivatorThisTurnCast") {
		t.Errorf("ValidCard Card.MultiColor: refusal %q", why)
	}
}
