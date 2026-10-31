package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// Board readers that make every cast emit a count event for the value they
// read, whatever the cast spell itself reads.
const (
	spendReaderSrc = "Name:Spend Reader\nManaCost:1\nTypes:Creature Bird\nPT:1/1\n" +
		"SVar:Spent:TriggeredCard$CastTotalManaSpent\nOracle:x\n"
	colourReaderSrc = "Name:Colour Reader\nManaCost:1\nTypes:Creature Bird\nPT:1/1\n" +
		"SVar:Colours:TriggeredCard$Converge\nOracle:x\n"
)

const teamworkHelperSrc = "Name:Team Helper\nManaCost:1\nTypes:Creature Bird\nPT:2/2\nOracle:x\n"

// CR 107.3: X is the announced value, so the paid Teamwork marker cannot give a
// spell with no {X} a value.
func TestPaidTeamworkLeavesNonXSpellXZero(t *testing.T) {
	t.Parallel()
	src := "Name:Plain Team Spell\nManaCost:W\nTypes:Sorcery\nK:Teamwork:2\nA:SP$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"
	e, cfg, id := newFixtureDeck(t, 311, src, teamworkHelperSrc, teamworkHelperSrc)
	helper := putCreature(t, e, 0, teamworkHelperSrc)
	putCreature(t, e, 0, teamworkHelperSrc)
	addMana(t, e, 0, "W")
	submitChoices(t, e, castOptMode(t, e.Pending().Options, id, "teamworked").Index)
	submitChoices(t, e, teamworkOption(t, teamworkAskOptions(t, e), helper))

	o := e.G.Obj(id)
	if o.Zone != state.ZStack || !o.TeamworkPaid {
		t.Fatalf("zone %s, TeamworkPaid %v: the paid cast did not finish", o.Zone, o.TeamworkPaid)
	}
	if o.X != 0 {
		t.Fatalf("X = %d on a spell with no {X}, want 0", o.X)
	}
	replayCheck(t, e, cfg)
}

func TestPaidTeamworkKeepsAnnouncedX(t *testing.T) {
	t.Parallel()
	src := "Name:Scaling Team Spell\nManaCost:X W\nTypes:Sorcery\nK:Teamwork:2\nA:SP$ GainLife | Defined$ You | LifeAmount$ X\nSVar:X:Count$xPaid\nOracle:x\n"
	e, cfg, id := newFixtureDeck(t, 312, src, teamworkHelperSrc, teamworkHelperSrc)
	helper := putCreature(t, e, 0, teamworkHelperSrc)
	putCreature(t, e, 0, teamworkHelperSrc)
	addMana(t, e, 0, "WWW")
	submitChoices(t, e, castOptMode(t, e.Pending().Options, id, "teamworked").Index)
	submitChoices(t, e, teamworkOption(t, teamworkAskOptions(t, e), helper))
	chooseX(t, e, 2)

	o := e.G.Obj(id)
	if o.Zone != state.ZStack || !o.TeamworkPaid {
		t.Fatalf("zone %s, TeamworkPaid %v: the paid cast did not finish", o.Zone, o.TeamworkPaid)
	}
	if o.X != 2 {
		t.Fatalf("X = %d, want the announced 2", o.X)
	}
	replayCheck(t, e, cfg)
}

// A replicated spell's copies come from its count (CR 702.56a), so a spend
// reader on the board must not cost the caster the copies.
func TestReplicateCountSurvivesSpendReader(t *testing.T) {
	t.Parallel()
	src := "Name:Echoing Gift\nManaCost:W\nTypes:Sorcery\nK:Replicate:W\nA:SP$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"
	e, cfg, id := newFixtureDeck(t, 313, src, spendReaderSrc)
	putCreature(t, e, 0, spendReaderSrc)
	addMana(t, e, 0, "WWW")
	submitChoices(t, e, replicateOption(t, e, id, "replicated").Index)
	chooseReplicate(t, e, 2)

	o := e.G.Obj(id)
	if o.ReplicateTimes != 2 || o.ManaSpent != 3 {
		t.Fatalf("ReplicateTimes = %d, ManaSpent = %d, want 2 and 3", o.ReplicateTimes, o.ManaSpent)
	}
	life := e.G.Players[0].Life
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Players[0].Life - life; got != 3 {
		t.Fatalf("gained %d life, want 3: the spell and its two copies", got)
	}
	replayCheck(t, e, cfg)
}

// Converge counts colours (CR 207.2c), and the spend count rides a later event
// carrying the converge flag: each value must reach its own field, whichever
// side's reader asked for the event.
func TestConvergeAndSpendCountsKeepTheirFields(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, src, reader string
	}{
		{"converge spell, spend reader",
			"Name:Prism Gift\nManaCost:W U\nTypes:Sorcery\nA:SP$ GainLife | Defined$ You | LifeAmount$ Y\nSVar:Y:Count$Converge\nOracle:x\n",
			spendReaderSrc},
		{"spend spell, colour reader",
			"Name:Tally Gift\nManaCost:W U\nTypes:Sorcery\nA:SP$ GainLife | Defined$ You | LifeAmount$ Tally\nSVar:Tally:Count$CastTotalManaSpent\nOracle:x\n",
			colourReaderSrc},
	} {
		t.Run(c.name, func(t *testing.T) {
			e, cfg, id := newFixtureDeck(t, 314, c.src, c.reader)
			putCreature(t, e, 0, c.reader)
			addMana(t, e, 0, "WU")
			submitChoices(t, e, castOptionFor(t, e, id).Index)

			o := e.G.Obj(id)
			if o.ConvergeColours != 2 || o.ManaSpent != 2 {
				t.Fatalf("ConvergeColours = %d, ManaSpent = %d, want 2 and 2", o.ConvergeColours, o.ManaSpent)
			}
			life := e.G.Players[0].Life
			passUntilStackEmpty(t, e, 20)
			if got := e.G.Players[0].Life - life; got != 2 {
				t.Fatalf("gained %d life, want 2", got)
			}
			replayCheck(t, e, cfg)
		})
	}
}

// CR 702.33c: the times kicked is its own count, and the colour count's flag
// rides the kick event.
func TestTimesKickedSurvivesColourReader(t *testing.T) {
	t.Parallel()
	body := "Types:Sorcery\nA:SP$ GainLife | Defined$ You | LifeAmount$ K\nSVar:K:Count$TimesKicked\nOracle:x\n"
	for _, c := range []struct {
		name, src, mode string
		mana            string
		want            int32
	}{
		{"multikicker", "Name:Many Kicks\nManaCost:W\nK:Multikicker:W\n" + body, "multikicked", "WWW", 2},
		{"kicker", "Name:One Kick\nManaCost:W\nK:Kicker:W\n" + body, "kicked", "WW", 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			e, cfg, id := newFixtureDeck(t, 315, c.src, colourReaderSrc)
			putCreature(t, e, 0, colourReaderSrc)
			addMana(t, e, 0, c.mana)
			submitChoices(t, e, castOptMode(t, e.Pending().Options, id, c.mode).Index)
			if c.mode == "multikicked" {
				d := e.Pending()
				submitChoices(t, e, d.Options[len(d.Options)-1].Index)
			}

			o := e.G.Obj(id)
			if o.TimesKicked != c.want || o.ConvergeColours != 1 {
				t.Fatalf("TimesKicked = %d, ConvergeColours = %d, want %d and 1", o.TimesKicked, o.ConvergeColours, c.want)
			}
			life := e.G.Players[0].Life
			passUntilStackEmpty(t, e, 20)
			if got := e.G.Players[0].Life - life; got != c.want {
				t.Fatalf("gained %d life, want %d", got, c.want)
			}
			replayCheck(t, e, cfg)
		})
	}
}

// CR 702.33d: a multikicked spell is kicked, and the spend event that follows the
// kick event assigns the object's flags whole.
func TestMultikickedStaysKickedThroughASpendReader(t *testing.T) {
	t.Parallel()
	const src = "Name:Counted Kicks\nManaCost:W\nK:Multikicker:W\nTypes:Sorcery\n" +
		"A:SP$ GainLife | Defined$ You | LifeAmount$ S | Condition$ Kicked\n" +
		"SVar:S:Count$CastTotalManaSpent\nOracle:x\n"
	e, cfg, id := newFixtureDeck(t, 316, src)
	addMana(t, e, 0, "WW")
	submitChoices(t, e, castOptMode(t, e.Pending().Options, id, "multikicked").Index)
	d := e.Pending()
	submitChoices(t, e, d.Options[len(d.Options)-1].Index)
	o := e.G.Obj(id)
	if o.CastFlags&state.FlagKicked == 0 || o.TimesKicked != 1 || o.ManaSpent != 2 {
		t.Fatalf("flags %b, TimesKicked = %d, ManaSpent = %d, want kicked, 1 and 2", o.CastFlags, o.TimesKicked, o.ManaSpent)
	}
	life := e.G.Players[0].Life
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Players[0].Life - life; got != 2 {
		t.Fatalf("gained %d life, want 2", got)
	}
	replayCheck(t, e, cfg)
}
