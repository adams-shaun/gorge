package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// "Copy that card X times, where X is the number of creature types among
// non-Borg creatures you control. You may cast the copies without paying
// their mana costs" (a Play with CopyCard$, AllowRepeats$ and Amount$ X, the
// remembered card exiled from the graveyard by the spell's first node; the
// spell exiles itself last). Each copy is cast (CR 707.12, 601.2): its own
// targets and modes, no mana, declinable one at a time, and it ceases to
// exist after resolving, while the original stays exiled (it is not cast).
// The fixtures are authored here with the carrier's parameter shape.

const infDivSpell = "Name:Federation fixture\nManaCost:7 R R\nTypes:Sorcery\n" +
	"A:SP$ ChangeZone | Origin$ Graveyard | Destination$ Exile | ValidTgts$ Instant.YouOwn,Sorcery.YouOwn | RememberChanged$ True | SubAbility$ DBPlay\n" +
	"SVar:DBPlay:DB$ Play | Valid$ Card.IsRemembered | ValidSA$ Spell | ValidZone$ Exile | Amount$ X | AllowRepeats$ True | WithoutManaCost$ True | Optional$ True | CopyCard$ True | SubAbility$ DBExile\n" +
	"SVar:DBExile:DB$ ChangeZone | Defined$ Self | Origin$ Stack | Destination$ Exile | SubAbility$ DBCleanup\n" +
	"SVar:DBCleanup:DB$ Cleanup | ClearRemembered$ True\n" +
	"SVar:X:Count$Valid Creature.nonBorg+YouCtrl$CreatureType\n" +
	"Oracle:Exile target instant or sorcery card from your graveyard. Copy it X times.\n"

const (
	infDivBolt = "Name:Federation bolt\nManaCost:R\nTypes:Instant\n" +
		"A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 2\nOracle:Deal 2 damage to any target.\n"
	infDivCharm = "Name:Federation charm\nManaCost:R\nTypes:Instant\n" +
		"A:SP$ Charm | CharmNum$ 1 | Choices$ Gain,Lose\n" +
		"SVar:Gain:DB$ GainLife | Defined$ You | LifeAmount$ 3 | SpellDescription$ You gain 3 life.\n" +
		"SVar:Lose:DB$ LoseLife | Defined$ Player.Opponent | LifeAmount$ 5 | SpellDescription$ Each opponent loses 5 life.\n" +
		"Oracle:Choose one.\n"
	infDivElf    = "Name:Federation elf\nManaCost:G\nTypes:Creature Elf\nPT:1/1\nOracle:\n"
	infDivGoblin = "Name:Federation goblin\nManaCost:R\nTypes:Creature Goblin\nPT:1/1\nOracle:\n"
	infDivGob2   = "Name:Federation goblin two\nManaCost:R\nTypes:Creature Goblin\nPT:1/1\nOracle:\n"
	infDivWizard = "Name:Federation wizard\nManaCost:U\nTypes:Creature Human Wizard\nPT:1/1\nOracle:\n"
	infDivBorg   = "Name:Federation drone\nManaCost:1\nTypes:Artifact Creature Borg Drone\nPT:1/1\nOracle:\n"
	infDivChange = "Name:Federation changeling\nManaCost:1\nTypes:Creature Shapeshifter\nPT:1/1\nK:Changeling\nOracle:\n"
)

// infDivStart seats the fixture: the creatures on seat 0's battlefield, the
// instant or sorcery in its graveyard, nine mana, the spell cast at that card.
// It returns the engine, the graveyard card, the spell and the Play ask (nil
// when X is 0 and the resolution never asks).
func infDivStart(t *testing.T, seed uint64, spell string, creatures ...string) (e *Engine, original, source state.ObjID, ask *decision.Decision) {
	t.Helper()
	extras := append([]string{spell}, creatures...)
	e, _, source = newFixtureDeck(t, seed, infDivSpell, extras...)
	for _, c := range creatures {
		putCreature(t, e, 0, c)
	}
	original = addToGraveyard(t, e, 0, spell)
	addMana(t, e, 0, "RRRRRRRRR")
	d := e.Pending()
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == source {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for the spell: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("after the cast: %+v, want the graveyard target ask", d)
	}
	tIdx := -1
	for _, o := range d.Options {
		if o.Obj == original {
			tIdx = o.Index
		}
	}
	if tIdx < 0 {
		t.Fatalf("the graveyard card %d is not offered: %+v", original, d.Options)
	}
	submitChoices(t, e, tIdx)
	for i := 0; i < 20 && !e.G.Over; i++ {
		d = e.Pending()
		if d == nil || d.Kind == decision.KModes && d.ResumeKind == "play" {
			break
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("unexpected decision %+v while resolving the spell", d)
		}
		if len(e.G.Stack) == 0 {
			break
		}
		for _, o := range d.Options {
			if o.Kind == "pass" {
				submitChoices(t, e, o.Index)
				break
			}
		}
	}
	if d := e.Pending(); d != nil && d.Kind == decision.KModes && d.ResumeKind == "play" {
		ask = d
	}
	return e, original, source, ask
}

// infDivCopies lists the copies cast from the log (a Play with CopyCard$
// mints one object per cast, a copy, and puts it on the stack), in order.
func infDivCopies(e *Engine, from int) []state.ObjID {
	var out []state.ObjID
	for _, ev := range e.L.Events[from:] {
		if ev.Kind == events.PutOnStack {
			if o := e.G.Obj(ev.Obj); o != nil && o.IsCopy {
				out = append(out, ev.Obj)
			}
		}
	}
	return out
}

// infDivPick answers the pending target ask with the player seat.
func infDivPick(t *testing.T, e *Engine, seat state.PlayerID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending %+v, want a copy's target ask", d)
	}
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == seat {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("player %d is not a target: %+v", seat, d.Options)
}

func TestInfiniteDiversityGorge(t *testing.T) {
	t.Parallel()
	t.Run("X copies offered, each cast with its own target, original stays exiled", func(t *testing.T) {
		t.Parallel()
		// Elf, Goblin (twice), Wizard and Human are four types; the Borg
		// drone's Borg and Drone types are excluded with it. Human + Wizard
		// + Elf + Goblin = 4.
		e, original, source, ask := infDivStart(t, 51001, infDivBolt, infDivElf, infDivGoblin, infDivGob2, infDivWizard, infDivBorg)
		if ask == nil {
			t.Fatal("no Play ask after the spell's first node")
		}
		if ask.Max != 4 || ask.Min != 0 || len(ask.Options) != 4 {
			t.Fatalf("Play ask Min/Max/options = %d/%d/%d, want 0/4/4 (four creature types, Borg excluded)", ask.Min, ask.Max, len(ask.Options))
		}
		for _, o := range ask.Options {
			if o.Obj != original {
				t.Fatalf("an entry names %d, want the exiled card %d", o.Obj, original)
			}
		}
		if z := e.G.Obj(original).Zone; z != state.ZExile {
			t.Fatalf("the original is in %s, want exile before any copy is cast", z)
		}
		before := len(e.L.Events)
		life0, life1 := e.G.Players[0].Life, e.G.Players[1].Life
		// take two of the four (a decline of the others is legal: Optional$)
		submitChoices(t, e, ask.Options[0].Index, ask.Options[1].Index)
		infDivPick(t, e, 1) // the first copy's own target
		infDivPick(t, e, 0) // the second copy's own, a different one
		copies := infDivCopies(e, before)
		if len(copies) != 2 || copies[0] == copies[1] || copies[0] == original {
			t.Fatalf("copies cast = %v, want two distinct new objects", copies)
		}
		passUntilStackEmpty(t, e, 40)
		if got := e.G.Players[1].Life; got != life1-2 {
			t.Fatalf("opponent life = %d, want %d (one copy aimed at them)", got, life1-2)
		}
		if got := e.G.Players[0].Life; got != life0-2 {
			t.Fatalf("own life = %d, want %d (one copy aimed at us)", got, life0-2)
		}
		if z := e.G.Obj(original).Zone; z != state.ZExile {
			t.Fatalf("the original is in %s after the copies, want exile (it was not cast)", z)
		}
		for _, c := range copies {
			if o := e.G.Obj(c); o != nil && o.Zone != state.ZCeased && o.Zone != state.ZStack {
				if o.Zone == state.ZGraveyard || o.Zone == state.ZExile {
					t.Fatalf("copy %d rests in %s, want it ceased to exist (CR 704.5d)", c, o.Zone)
				}
			}
		}
		if z := e.G.Obj(source).Zone; z != state.ZExile {
			t.Fatalf("the spell is in %s, want exile", z)
		}
	})

	t.Run("a declined ask casts nothing", func(t *testing.T) {
		t.Parallel()
		e, original, source, ask := infDivStart(t, 51002, infDivBolt, infDivElf, infDivGoblin)
		if ask == nil || ask.Max != 2 {
			t.Fatalf("Play ask = %+v, want Max 2", ask)
		}
		before := len(e.L.Events)
		submitChoices(t, e)
		passUntilStackEmpty(t, e, 40)
		if n := len(infDivCopies(e, before)); n != 0 {
			t.Fatalf("%d copies cast after declining", n)
		}
		if z := e.G.Obj(original).Zone; z != state.ZExile {
			t.Fatalf("the original is in %s, want exile", z)
		}
		if z := e.G.Obj(source).Zone; z != state.ZExile {
			t.Fatalf("the spell is in %s, want exile", z)
		}
	})

	t.Run("X of zero offers no copy", func(t *testing.T) {
		t.Parallel()
		// no creature at all, and a Borg alone: X = 0 in both
		for _, creatures := range [][]string{nil, {infDivBorg}} {
			e, original, source, ask := infDivStart(t, 51003, infDivBolt, creatures...)
			if ask != nil {
				t.Fatalf("X = 0 asked %+v, want no Play ask (no copy exists to cast)", ask)
			}
			before := len(e.L.Events)
			passUntilStackEmpty(t, e, 40)
			if n := len(infDivCopies(e, before)); n != 0 {
				t.Fatalf("%d copies cast for X = 0", n)
			}
			if z := e.G.Obj(original).Zone; z != state.ZExile {
				t.Fatalf("the original is in %s, want exile (the spell exiled it before counting)", z)
			}
			if z := e.G.Obj(source).Zone; z != state.ZExile {
				t.Fatalf("the spell is in %s, want exile", z)
			}
		}
	})

	t.Run("each copy chooses its own mode", func(t *testing.T) {
		t.Parallel()
		e, _, _, ask := infDivStart(t, 51004, infDivCharm, infDivElf, infDivGoblin)
		if ask == nil || ask.Max != 2 {
			t.Fatalf("Play ask = %+v, want Max 2", ask)
		}
		life0, life1 := e.G.Players[0].Life, e.G.Players[1].Life
		submitChoices(t, e, ask.Options[0].Index, ask.Options[1].Index)
		var picked []string
		for i := 0; i < 2; i++ {
			d := e.Pending()
			if d == nil || d.Kind != decision.KModes || d.ResumeKind == "play" || len(d.Options) != 2 {
				t.Fatalf("copy %d: pending %+v, want its own mode ask over two modes", i, d)
			}
			picked = append(picked, d.Options[i].Label)
			submitChoices(t, e, d.Options[i].Index) // the first copy takes mode 0, the second mode 1
		}
		passUntilStackEmpty(t, e, 40)
		if e.G.Players[0].Life != life0+3 || e.G.Players[1].Life != life1-5 {
			t.Fatalf("life %d/%d after modes %v, want %d/%d (one gain, one drain)", e.G.Players[0].Life, e.G.Players[1].Life, picked, life0+3, life1-5)
		}
	})

	t.Run("a changeling is every type, Borg included, so nonBorg leaves it out", func(t *testing.T) {
		t.Parallel()
		// CR 702.73a: a changeling has every creature type, Borg among them, so
		// the non-Borg filter excludes it (Forge reads the same) and it adds
		// no type: alone X is 0, beside an Elf X is 1 (one copy, no repeat).
		_, _, _, ask := infDivStart(t, 51005, infDivBolt, infDivChange)
		if ask != nil {
			t.Fatalf("a lone changeling asked %+v, want X = 0", ask)
		}
		_, _, _, ask = infDivStart(t, 51006, infDivBolt, infDivChange, infDivElf)
		if ask == nil || ask.Max != 1 || len(ask.Options) != 1 {
			t.Fatalf("changeling + elf: Play ask = %+v, want one entry (X = 1)", ask)
		}
	})
}
