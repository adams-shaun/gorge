package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
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

// twofoldRite chains TWO ChooseColor SAs in one resolution -- the review's
// sequential shape: the first ask's answer lands on the object
// (o.ChosenColor), and the SECOND ask must still be posed over the stale
// answer. The downstream GainLife reads Count$Devotion.Chosen AFTER the
// second answer, so the second pick provably governs.
const twofoldRite = "Name:Twofold Rite\nManaCost:G\nTypes:Sorcery\n" +
	"A:SP$ ChooseColor | Defined$ You | SubAbility$ DBSecond\n" +
	"SVar:DBSecond:DB$ ChooseColor | Defined$ You | SubAbility$ DBGain\n" +
	"SVar:DBGain:DB$ GainLife | LifeAmount$ X\n" +
	"SVar:X:Count$Devotion.Chosen\nOracle:x\n"

// castTwofold funds {G} and casts the fixture rite from seat 0's hand,
// leaving the resolution suspended on the FIRST colour ask. Same board and
// preconditions as castRite: one White pip on the battlefield (White
// devotion 1, Black 0 -- the two answers under comparison actually differ).
func castTwofold(t *testing.T) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	e := handEngine(t, card(t, twofoldRite))
	pip := battlefieldCreature(t, e, "Name:White Pip\nManaCost:W\nTypes:Creature Bear\nPT:1/1\nOracle:x\n")
	rite := e.G.Zone(state.ZHand, 0)[0]
	if o := e.G.Obj(pip); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: the devotion source is not on the battlefield: %+v", o)
	}
	e.G.Players[0].Pool[state.MG] = 1
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

// TestChosenColorSecondSequentialAskIsPosedAndGoverns pins the review's
// sequential half end to end: the first ask is answered White (recorded on
// the object), and the SECOND ChooseColor in the same resolution still poses
// its own ask -- the stale ChosenColor must not suppress it. Answering Black
// proves the second answer governs the downstream: Devotion.Chosen at the
// GainLife is the BLACK devotion (0, the pip is White), so no life. Without
// the fix the second SA early-returned on the stale "W", never asked, and
// the GainLife counted the WHITE devotion (1 life).
func TestChosenColorSecondSequentialAskIsPosedAndGoverns(t *testing.T) {
	t.Parallel()
	e, rite, _ := castTwofold(t)
	d1 := passUntilAsk(t, e)
	if d1 == nil || d1.ResumeKind != "choosecolor" {
		t.Fatalf("first ask = %+v, want the choosecolour ask", d1)
	}
	white := optionByLabel(d1.Options, "White")
	if white < 0 {
		t.Fatalf("no White option in %+v", d1.Options)
	}
	submitChoices(t, e, white)
	if o := e.G.Obj(rite); o.ChosenColor != "W" {
		t.Fatalf("precondition for the regression: the first answer did not record: %q", o.ChosenColor)
	}
	d2 := e.Pending()
	if d2 == nil || d2.Kind != decision.KChoose || d2.ResumeKind != "choosecolor" {
		t.Fatalf("the SECOND sequential ChooseColor ask was suppressed by the first answer, pending = %+v", d2)
	}
	black := optionByLabel(d2.Options, "Black")
	if black < 0 {
		t.Fatalf("no Black option in the second ask %+v", d2.Options)
	}
	submitChoices(t, e, black)
	finishCast(t, e, rite)
	if got := e.G.Players[0].Life; got != 20 {
		t.Fatalf("the SECOND (Black) answer did not govern: life = %d, want 20 (the stale White devotion would gain 1)", got)
	}
	if o := e.G.Obj(rite); o.ChosenColor != "B" {
		t.Fatalf("recorded choice = %q, want the second answer B", o.ChosenColor)
	}
}

// chromaticBear is the review's ability-after-entry shape: the permanent
// chooses a colour AS IT ENTERS (the real K:ETBReplacement:Other:ChooseColor
// carrier shape) and carries an ACTIVATED ChooseColor ability. The entry ask
// is the machinery's; the entry body must stay the no-op (no second ask at
// the re-emitted move); the later ability activation must still re-ask even
// though the object already carries the entry choice.
const chromaticBear = "Name:Chromatic Bear\nManaCost:G\nTypes:Creature Bear\nPT:2/2\n" +
	"K:ETBReplacement:Other:ChooseColor\n" +
	"SVar:ChooseColor:DB$ ChooseColor\n" +
	"A:AB$ ChooseColor | Defined$ You | SubAbility$ DBGain | SpellDescription$ Choose a color.\n" +
	"SVar:DBGain:DB$ GainLife | LifeAmount$ X\n" +
	"SVar:X:Count$Devotion.Chosen\nOracle:x\n"

// TestChosenColorAbilityReasksAfterTheEntryChoice pins both halves on the
// entry/activation integration: (1) entering the bear poses the machinery's
// etb ask, the answer records, and the entry body at the re-emitted move
// poses NO second colour ask; (2) activating the bear's own ChooseColor
// ability afterwards DOES pose its ask over the stale entry answer, and the
// ability's own answer governs the downstream (White pip on board: the White
// answer gains 1 life, the entry's Black answer would gain 0). Without the
// fix the AB ask is suppressed by the stale ChosenColor and the ability
// resolves silently on the entry's answer.
func TestChosenColorAbilityReasksAfterTheEntryChoice(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, chromaticBear))
	bear := e.G.Zone(state.ZHand, 0)[0]
	if o := e.G.Obj(bear); o.Zone != state.ZHand {
		t.Fatalf("precondition: bear zone = %s, want hand", o.Zone)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZHand, To: state.ZBattlefield})
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "etb" ||
		len(d.Options) == 0 || d.Options[0].Kind != "color" {
		t.Fatalf("entry ask = %+v, want the machinery's etb colour ask", d)
	}
	black := optionByLabel(d.Options, "Black")
	if black < 0 {
		t.Fatalf("no Black option in the entry ask %+v", d.Options)
	}
	submitChoices(t, e, black)
	if o := e.G.Obj(bear); o.Zone != state.ZBattlefield || o.ChosenColor != "B" {
		t.Fatalf("after the entry answer: zone=%s choice=%q, want battlefield and B", o.Zone, o.ChosenColor)
	}
	if d2 := e.Pending(); d2 != nil && d2.Kind == decision.KChoose && d2.ResumeKind == "choosecolor" {
		t.Fatalf("the entry body posed its own second colour ask after the machinery's: %+v", d2)
	}
	// The ability half: a White pip makes the two devotions differ (White 1,
	// Black 0), so the ability's answer provably governs the GainLife.
	pip := battlefieldCreature(t, e, "Name:White Pip\nManaCost:W\nTypes:Creature Bear\nPT:1/1\nOracle:x\n")
	if o := e.G.Obj(pip); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: the devotion source is not on the battlefield: %+v", o)
	}
	e.priorityRound()
	opt := abilityOption(t, e, bear, 0)
	submitChoices(t, e, opt.Index)
	d3 := passUntilAsk(t, e)
	if d3 == nil || d3.Kind != decision.KChoose || d3.ResumeKind != "choosecolor" {
		t.Fatalf("the already-chosen permanent's ability did not re-ask: %+v", d3)
	}
	white := optionByLabel(d3.Options, "White")
	if white < 0 {
		t.Fatalf("no White option in the ability ask %+v", d3.Options)
	}
	submitChoices(t, e, white)
	passUntilStackEmpty(t, e, 60)
	if got := e.G.Players[0].Life; got != 21 {
		t.Fatalf("the ability's WHITE answer gained %d life (want 1 -- the stale Black entry answer would gain 0)", got-20)
	}
	if o := e.G.Obj(bear); o.ChosenColor != "W" {
		t.Fatalf("recorded choice = %q, want the ability's own answer W", o.ChosenColor)
	}
}
