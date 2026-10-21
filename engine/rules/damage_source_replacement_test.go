package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// cr-g18c: the damage replacement and prevention pipeline reads the source
// of the Damage event in flight -- the source the emitter published with
// SetDamageSource (a fighting creature, CR 701.14a; a DamageSource$ biter,
// CR 120.2b) -- not the resolving spell or ability. Protection and DamageDone
// triggers already read that published source (inFlightDamageSource).

// cr18cDamageTo returns the Damage events logged after mark whose recipient
// is obj.
func cr18cDamageTo(e *Engine, mark int, obj state.ObjID) []events.Event {
	var out []events.Event
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.Damage && ev.Obj == obj {
			out = append(out, ev)
		}
	}
	return out
}

// cr18cCastBushwhackFight casts seat 0's Bushwhack in its fight mode ("Target
// creature you control fights target creature you don't control") with
// fighter and foe as the two targets, leaving the spell on the stack.
func cr18cCastBushwhackFight(t *testing.T, e *Engine, spell, fighter, foe state.ObjID) {
	t.Helper()
	addMana(t, e, 0, "G")
	cr601Cast(t, e, spell, "")
	modes := e.Pending()
	if modes == nil || modes.Kind != decision.KModes {
		t.Fatalf("precondition: Bushwhack mode ask, got %+v", modes)
	}
	fight := -1
	for _, o := range modes.Options {
		if strings.Contains(o.Label, "fights") {
			fight = o.Index
		}
	}
	if fight < 0 {
		t.Fatalf("precondition: no fight mode offered: %+v", modes.Options)
	}
	submitChoices(t, e, fight)
	root := e.Pending()
	if root == nil || root.Kind != decision.KTarget {
		t.Fatalf("precondition: fight-mode root target ask, got %+v", root)
	}
	for _, o := range root.Options {
		if o.Kind != "player" && o.Obj == fighter {
			submitChoices(t, e, o.Index)
			answerCastSubObj(t, e, foe)
			return
		}
	}
	t.Fatalf("precondition: fighter %d not offered: %+v", fighter, root.Options)
}

// TestCR701_14aFightDamageSourceIsTheCreature: seat 1 controls Twinflame
// Tyrant ("If a source you control would deal damage to ... a permanent an
// opponent controls, it deals double that damage instead"). Seat 0's
// Bushwhack has its Craw Wurm (6/4) fight the Tyrant (3/5). The Tyrant's
// hit's source is the Tyrant, which seat 1 controls, so it doubles to 6 and
// the Wurm dies; the Wurm's hit is a seat-0 source and is not doubled.
func TestCR701_14aFightDamageSourceIsTheCreature(t *testing.T) {
	t.Parallel()
	e, cfg, mine, theirs := cr601Board(t, 70114,
		map[string]state.Zone{"Bushwhack": state.ZHand, "Craw Wurm": state.ZBattlefield},
		map[string]state.Zone{"Twinflame Tyrant": state.ZBattlefield})
	wurm, tyrant := mine["Craw Wurm"], theirs["Twinflame Tyrant"]
	cr18cCastBushwhackFight(t, e, mine["Bushwhack"], wurm, tyrant)
	mark := len(e.L.Events)
	cr601ResolveQuietly(t, e)
	if got := cr18cDamageTo(e, mark, tyrant); len(got) != 1 || got[0].Amount != 6 {
		t.Errorf("Wurm -> Tyrant damage = %+v, want one 6 (a seat-0 source is not doubled by seat 1's Tyrant)", got)
	}
	if got := cr18cDamageTo(e, mark, wurm); len(got) != 1 || got[0].Amount != 6 {
		t.Fatalf("Tyrant -> Wurm damage = %+v, want one 6: CR 701.14a, the fighting Tyrant is the source, so its own replacement doubles 3 to 6", got)
	}
	if o := e.G.Obj(wurm); o == nil || o.Zone != state.ZGraveyard {
		t.Errorf("Craw Wurm (4 toughness) zone = %+v after 6 damage, want graveyard", o)
	}
	replayCheck(t, e, cfg)
}

// TestFightParkedReplacementChoiceKeepsTheFighter: seat 0 controls Torbran
// (2/4, red: "If a red source you control would deal damage to ... a
// permanent an opponent controls, it deals that much damage plus 2 instead")
// and Twinflame Tyrant; Bushwhack has Torbran fight seat 1's Craw Wurm. Both
// replacements apply to Torbran's 2, so the Wurm's controller is asked the
// CR 616.1 order. Answering Twinflame first doubles 2 to 4, and the
// recomputation must still read Torbran as the source, so Torbran's +2
// applies too: 6. A parked choice that recorded the resolving spell as the
// source would drop Torbran's replacement after the answer (4).
func TestFightParkedReplacementChoiceKeepsTheFighter(t *testing.T) {
	t.Parallel()
	e, cfg, mine, theirs := cr601Board(t, 70118,
		map[string]state.Zone{"Bushwhack": state.ZHand, "Torbran, Thane of Red Fell": state.ZBattlefield,
			"Twinflame Tyrant": state.ZBattlefield},
		map[string]state.Zone{"Craw Wurm": state.ZBattlefield})
	tyrant, wurm := mine["Twinflame Tyrant"], theirs["Craw Wurm"]
	cr18cCastBushwhackFight(t, e, mine["Bushwhack"], mine["Torbran, Thane of Red Fell"], wurm)
	mark := len(e.L.Events)
	asked := false
	for i := 0; i < 40 && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while resolving Bushwhack")
		}
		if d.Kind != decision.KReplacement {
			submitChoices(t, e, passIndex(t, d))
			continue
		}
		pick := -1
		for _, o := range d.Options {
			if o.Obj == tyrant {
				pick = o.Index
			}
		}
		if pick < 0 {
			t.Fatalf("replacement order ask does not offer Twinflame Tyrant: %+v", d.Options)
		}
		asked = true
		submitChoices(t, e, pick)
	}
	if !asked {
		t.Fatalf("no CR 616.1 order ask: Torbran and Twinflame Tyrant both apply to the red fighter's hit (damage %+v)", cr18cDamageTo(e, mark, wurm))
	}
	if got := cr18cDamageTo(e, mark, wurm); len(got) != 1 || got[0].Amount != 6 {
		t.Fatalf("Torbran -> Craw Wurm damage = %+v, want one 6: Twinflame doubles 2 to 4, then Torbran (still the source) adds 2", got)
	}
	replayCheck(t, e, cfg)
}

// TestFightReplacedSourceIsTheFighter: seat 0's Bushwhack has its Swans of
// Bryn Argoll fight seat 1's Craw Wurm. The Wurm's 6 damage to the Swans is
// prevented and "the source's controller draws cards equal to the damage
// prevented" (ReplacedSourceController): the source is the Wurm, so seat 1
// draws 6 and seat 0 draws nothing.
func TestFightReplacedSourceIsTheFighter(t *testing.T) {
	t.Parallel()
	e, cfg, mine, theirs := cr601Board(t, 70119,
		map[string]state.Zone{"Bushwhack": state.ZHand, "Swans of Bryn Argoll": state.ZBattlefield},
		map[string]state.Zone{"Craw Wurm": state.ZBattlefield})
	cr18cCastBushwhackFight(t, e, mine["Bushwhack"], mine["Swans of Bryn Argoll"], theirs["Craw Wurm"])
	hand0, hand1 := handSize(e, 0), handSize(e, 1)
	cr601ResolveQuietly(t, e)
	if got := handSize(e, 1) - hand1; got != 6 {
		t.Errorf("seat 1 (the Wurm's controller) drew %d, want 6: the fighting Wurm is the prevented damage's source", got)
	}
	if got := handSize(e, 0) - hand0; got != 0 {
		t.Errorf("seat 0 (Bushwhack's controller) drew %d, want 0", got)
	}
	replayCheck(t, e, cfg)
}

// TestFightGrantedPreventByReadsTheFighter: seat 1's Craw Wurm wears
// Temporal Isolation ("Prevent all damage that would be dealt by enchanted
// creature", a granted keyword). Seat 0's Bushwhack has its Hill Giant fight
// the Wurm: the Wurm's hit is dealt by the Wurm, so it is prevented, while
// the Giant's 3 lands.
func TestFightGrantedPreventByReadsTheFighter(t *testing.T) {
	t.Parallel()
	e, cfg, mine, theirs := cr601Board(t, 70120,
		map[string]state.Zone{"Bushwhack": state.ZHand, "Hill Giant": state.ZBattlefield},
		map[string]state.Zone{"Craw Wurm": state.ZBattlefield, "Temporal Isolation": state.ZHand})
	giant, wurm := mine["Hill Giant"], theirs["Craw Wurm"]
	lossAttach(t, e, "Temporal Isolation", 1, wurm)
	cr18cCastBushwhackFight(t, e, mine["Bushwhack"], giant, wurm)
	mark := len(e.L.Events)
	cr601ResolveQuietly(t, e)
	if got := cr18cDamageTo(e, mark, giant); len(got) != 0 {
		t.Errorf("Craw Wurm -> Hill Giant damage = %+v, want none: the enchanted Wurm deals it, so Temporal Isolation prevents it", got)
	}
	if got := cr18cDamageTo(e, mark, wurm); len(got) != 1 || got[0].Amount != 3 {
		t.Errorf("Hill Giant -> Craw Wurm damage = %+v, want one 3", got)
	}
	replayCheck(t, e, cfg)
}

// TestFightCantPreventReadsTheFighter: Excruciator ("Damage that would be
// dealt by Excruciator can't be prevented", 7/7) fights seat 1's Cho-Manno,
// Revolutionary ("Prevent all damage that would be dealt to Cho-Manno"). The
// fighting Excruciator is the source, so its 7 lands.
func TestFightCantPreventReadsTheFighter(t *testing.T) {
	t.Parallel()
	e, cfg, mine, theirs := cr601Board(t, 70121,
		map[string]state.Zone{"Bushwhack": state.ZHand, "Excruciator": state.ZBattlefield},
		map[string]state.Zone{"Cho-Manno, Revolutionary": state.ZBattlefield})
	cho := theirs["Cho-Manno, Revolutionary"]
	cr18cCastBushwhackFight(t, e, mine["Bushwhack"], mine["Excruciator"], cho)
	mark := len(e.L.Events)
	cr601ResolveQuietly(t, e)
	if got := cr18cDamageTo(e, mark, cho); len(got) != 1 || got[0].Amount != 7 {
		t.Fatalf("Excruciator -> Cho-Manno damage = %+v, want one 7: CR 701.14a, Excruciator is the source and its damage can't be prevented", got)
	}
	replayCheck(t, e, cfg)
}

// TestDamageReplacementReadsTheBitingCreature: Torbran and a red Hill Giant
// (3/3) on seat 0. The green Bite Down has the Giant deal damage equal to
// its power to seat 1's Craw Wurm: the source is the red Giant
// (DamageSource$ ParentTarget, CR 120.2b), so Torbran adds 2 -- 5, not 3.
func TestDamageReplacementReadsTheBitingCreature(t *testing.T) {
	t.Parallel()
	e, cfg, mine, theirs := cr601Board(t, 70116,
		map[string]state.Zone{"Bite Down": state.ZHand, "Torbran, Thane of Red Fell": state.ZBattlefield, "Hill Giant": state.ZBattlefield},
		map[string]state.Zone{"Craw Wurm": state.ZBattlefield})
	spell, giant, wurm := mine["Bite Down"], mine["Hill Giant"], theirs["Craw Wurm"]
	addMana(t, e, 0, "G1")
	cr601Cast(t, e, spell, "")
	answerTargetAsk(t, e, []state.ObjID{giant})
	answerCastSubObj(t, e, wurm)
	mark := len(e.L.Events)
	cr601ResolveQuietly(t, e)
	if got := cr18cDamageTo(e, mark, wurm); len(got) != 1 || got[0].Amount != 5 {
		t.Fatalf("Hill Giant -> Craw Wurm damage = %+v, want one 5: the red biting Giant is the source, so Torbran adds 2", got)
	}
	replayCheck(t, e, cfg)
}

// TestDamageReplacementReadsTheSpellGuard is a guard (passes today): a red
// spell that deals its own damage (Shock, 2 to seat 1's Craw Wurm) is still
// the source Torbran reads -- 4 damage.
func TestDamageReplacementReadsTheSpellGuard(t *testing.T) {
	t.Parallel()
	e, cfg, mine, theirs := cr601Board(t, 70117,
		map[string]state.Zone{"Shock": state.ZHand, "Torbran, Thane of Red Fell": state.ZBattlefield},
		map[string]state.Zone{"Craw Wurm": state.ZBattlefield})
	wurm := theirs["Craw Wurm"]
	addMana(t, e, 0, "R")
	cr601Cast(t, e, mine["Shock"], "")
	answerTargetAsk(t, e, []state.ObjID{wurm})
	mark := len(e.L.Events)
	cr601ResolveQuietly(t, e)
	if got := cr18cDamageTo(e, mark, wurm); len(got) != 1 || got[0].Amount != 4 {
		t.Fatalf("Shock -> Craw Wurm damage = %+v, want one 4 (Torbran adds 2 to the red spell's 2)", got)
	}
	replayCheck(t, e, cfg)
}
