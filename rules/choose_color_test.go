package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// The mid-resolution ChooseColor ask (task cli-20260923T060000Z-choose-color):
// a resolution-time "choose a color" poses a real KChoose over the fixed
// WUBRG colour list, and the ANSWERED colour is what the downstream
// Count$Devotion.Chosen reader counts -- the reader Hotel of Fears' "Praise
// Him" and Nyx Lotus ride. Pinned end to end on a fixture spell with that
// exact downstream shape (PutCounter X = Count$Devotion.Chosen): the corpus
// SP$ ChooseColor carriers' own downstreams (Wash Out's ChangeType$
// Permanent.ChosenColor, Akroma's Blessing's Gains$ ChosenColor) are
// unregistered filter vocabulary -- a separate, pre-existing gap reported in
// the ticket, not this ask's.

// devotionRite is the fixture spell: choose a color, then put X +1/+1
// counters on itself, where X is the caster's devotion to the chosen colour
// -- the Hotel of Fears shape minus the planar trigger.
const devotionRite = "Name:Rite of Praise\nManaCost:1 B\nTypes:Sorcery\n" +
	"A:SP$ ChooseColor | Defined$ You | SubAbility$ DBGain\n" +
	"SVar:DBGain:DB$ GainLife | LifeAmount$ X\n" +
	"SVar:X:Count$Devotion.Chosen\nOracle:x\n"

// castRite funds {1}{B} and casts the fixture rite from seat 0's hand,
// leaving the resolution suspended on the mid-resolution colour ask.
func castRite(t *testing.T) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	e := handEngine(t, card(t, devotionRite))
	pip := battlefieldCreature(t, e, "Name:White Pip\nManaCost:W\nTypes:Creature Bear\nPT:1/1\nOracle:x\n")
	rite := e.G.Zone(state.ZHand, 0)[0]
	// Precondition: the devotion source is a real battlefield permanent and
	// the devotions under comparison actually differ -- White 1, Black 0 --
	// so the answered colour provably governs the downstream count.
	if o := e.G.Obj(pip); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: the devotion source is not on the battlefield: %+v", o)
	}
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MB] = 1, 1
	e.priorityRound()
	d := e.Pending()
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == rite {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for the rite in %+v", d.Options)
	}
	submitChoices(t, e, idx)
	return e, rite, pip
}

// TestChosenColorAskIsPosedOverTheWUBRGList pins the ask itself: the
// resolution suspends on a KChoose whose ResumeKind is "choosecolor", whose
// options are the five WUBRG colours in fixed order with full names as
// Labels. Without the fix the effect never asked -- it recorded the
// first-WUBRG "W" outright and kept resolving.
func TestChosenColorAskIsPosedOverTheWUBRGList(t *testing.T) {
	t.Parallel()
	e, rite, _ := castRite(t)
	d := passUntilAsk(t, e)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "choosecolor" ||
		d.Player != 0 || d.Min != 1 || d.Max != 1 || d.Prompt != "Choose a color" {
		t.Fatalf("expected the mid-resolution colour ask, got %+v", d)
	}
	if len(d.Options) != 5 {
		t.Fatalf("option list = %+v, want the five WUBRG colours", d.Options)
	}
	want := []string{"White", "Blue", "Black", "Red", "Green"}
	for i, w := range want {
		if d.Options[i].Kind != "color" || d.Options[i].Label != w {
			t.Fatalf("option %d = %+v, want the %q colour option", i, d.Options[i], w)
		}
	}
	if o := e.G.Obj(rite); o.ChosenColor != "" {
		t.Fatalf("a choice was recorded before the ask was answered: %q", o.ChosenColor)
	}
}

// TestChosenColorAnswerGovernsTheDownstreamDevotionCount answers "Black" --
// the OPPOSITE of the pre-fix first-WUBRG fallback ("White") -- so the
// downstream Devotion.Chosen must count ZERO (no black pips on the board),
// gain no life, and record the answer "B" on the object. Without the fix
// the resolution silently chose White, counted 1, and gained the life.
func TestChosenColorAnswerGovernsTheDownstreamDevotionCount(t *testing.T) {
	t.Parallel()
	e, rite, _ := castRite(t)
	d := passUntilAsk(t, e)
	black := optionByLabel(d.Options, "Black")
	if black < 0 {
		t.Fatalf("no Black option in %+v", d.Options)
	}
	submitChoices(t, e, black)
	finishCast(t, e, rite)
	if got := e.G.Players[0].Life; got != 20 {
		t.Fatalf("the BLACK answer gained life to %d (the White fallback's devotion), want 20", got)
	}
	if o := e.G.Obj(rite); o.ChosenColor != "B" {
		t.Fatalf("recorded choice = %q, want the answered B", o.ChosenColor)
	}
}

// TestChosenColorWhiteAnswerCountsTheBoard answers "White" -- the positive
// arm: the downstream Devotion.Chosen counts the white pip and the counter
// lands, proving the ask's answer is the colour the count reads (and that
// the resolution completed its whole SubAbility chain).
func TestChosenColorWhiteAnswerCountsTheBoard(t *testing.T) {
	t.Parallel()
	e, rite, _ := castRite(t)
	d := passUntilAsk(t, e)
	white := optionByLabel(d.Options, "White")
	if white < 0 {
		t.Fatalf("no White option in %+v", d.Options)
	}
	submitChoices(t, e, white)
	finishCast(t, e, rite)
	if got := e.G.Players[0].Life; got != 21 {
		t.Fatalf("the WHITE answer counted %d devotion, want the 1 life gained", got-20)
	}
	if o := e.G.Obj(rite); o.ChosenColor != "W" {
		t.Fatalf("recorded choice = %q, want the answered W", o.ChosenColor)
	}
	if o := e.G.Obj(rite); o.Zone != state.ZGraveyard {
		t.Fatalf("resolved rite in %s, want graveyard", o.Zone)
	}
}
