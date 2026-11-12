package rules

// The may-reveal and reveal-pick behaviour pins. Each drives a real engine: the spell is cast, every posed
// decision is answered with Submit, and the reveal Notes on the log are the
// observable outcome.

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const boltFixtureSrc = "Name:Bolt\nManaCost:R\nTypes:Instant\nOracle:3 damage\n"

// rememberGainBody is a rider that gains 5 life only when the chain's
// Remembered holds a card: the observable trace of RememberRevealed$.
const rememberGainBody = "SVar:DBGain:DB$ GainLife | LifeAmount$ 5 | ConditionDefined$ Remembered | ConditionPresent$ Card | ConditionCompare$ GE1\n"

// peekSpellSrc is a sorcery resolving line, chained to the Remembered rider.
func peekSpellSrc(name, line string) string {
	return "Name:" + name + "\nManaCost:U\nTypes:Sorcery\nA:" + line + " | SubAbility$ DBGain\n" + rememberGainBody + "Oracle:x\n"
}

// peekBoard is a 2-seat game whose seat 0 holds spell, with Bolt on top
// of seat 0's library. Returns the engine, its config and Bolt's id.
func peekBoard(t *testing.T, spell string) (*Engine, Config, state.ObjID) {
	t.Helper()
	e, cfg := fixtureGame(t, 31, cardsOfSrcs(t, spell, boltFixtureSrc), nil)
	moveNamedTo(t, e, 0, card(t, spell).Faces[0].Name, state.ZHand)
	bolt := findNamedInZone(e, 0, state.ZLibrary, "Bolt")
	if bolt == 0 {
		bolt = moveNamedTo(t, e, 0, "Bolt", state.ZLibrary)
	}
	libraryTopOrder(t, e, 0, bolt)
	return e, cfg, bolt
}

// TestRevealOptionalPeekPosesTheYesNoAsk: a RevealOptional$ peek poses a
// KChoose yes/no (ResumeKind reveal_optional) to the peeking player that
// names the top card in its prompt and its yes option (whose Obj carries the
// card), with nothing revealed and the chained rider not yet run.
func TestRevealOptionalPeekPosesTheYesNoAsk(t *testing.T) {
	t.Parallel()
	spell := peekSpellSrc("Peek Optional", "SP$ PeekAndReveal | Defined$ You | NumCards$ 1 | RevealOptional$ True | RememberRevealed$ True")
	e, _, bolt := peekBoard(t, spell)
	life := e.G.Players[0].Life
	mark := len(e.L.Events)
	d := castNamedSpell(t, e, "Peek Optional", "U", -1)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "reveal_optional" || d.Player != 0 {
		t.Fatalf("decision = %+v, want seat 0's reveal_optional KChoose", d)
	}
	if len(d.Options) != 2 || d.Options[0].Kind != "yes" || d.Options[1].Kind != "no" {
		t.Fatalf("options = %+v, want yes then no", d.Options)
	}
	if d.Options[0].Label != "Yes — reveal Bolt" || d.Options[0].Obj != bolt {
		t.Fatalf("yes option = %+v, want the label naming the top card and its Obj", d.Options[0])
	}
	if !strings.Contains(d.Prompt, "Bolt") {
		t.Fatalf("prompt = %q, want it to name the top card", d.Prompt)
	}
	if n := publicRevealsFrom(e, mark); len(n) != 0 {
		t.Fatalf("a reveal Note landed before the answer: %+v", n)
	}
	if e.G.Players[0].Life != life {
		t.Fatal("the chained rider ran before the answer")
	}
}

// TestRevealOptionalAnsweredYesRevealsAndRemembers: "yes" emits the public
// reveal Note (no text of its own) carrying the top card and remembers it,
// so the chained Remembered rider fires.
func TestRevealOptionalAnsweredYesRevealsAndRemembers(t *testing.T) {
	t.Parallel()
	spell := peekSpellSrc("Peek Optional", "SP$ PeekAndReveal | Defined$ You | NumCards$ 1 | RevealOptional$ True | RememberRevealed$ True")
	e, cfg, bolt := peekBoard(t, spell)
	life := e.G.Players[0].Life
	mark := len(e.L.Events)
	d := castNamedSpell(t, e, "Peek Optional", "U", -1)
	if d == nil || d.ResumeKind != "reveal_optional" {
		t.Fatalf("decision = %+v, want reveal_optional", d)
	}
	if d := answerChoices(t, e, optionKindOrFail(t, d, "yes")); d != nil {
		t.Fatalf("the accepted reveal posed another decision: %+v", d)
	}
	notes := publicRevealsFrom(e, mark)
	if len(notes) != 1 || len(notes[0].IDs) != 1 || notes[0].IDs[0] != bolt || notes[0].Text != "" {
		t.Fatalf("reveal notes = %+v, want one text-less public Note carrying Bolt %d", notes, bolt)
	}
	if got := e.G.Players[0].Life; got != life+5 {
		t.Fatalf("life = %d, want %d (the revealed card must be remembered for the rider)", got, life+5)
	}
	replayCheck(t, e, cfg)
}

// TestRevealOptionalAnsweredNoRevealsNothing: the decline emits no reveal
// Note and remembers nothing, so the chained rider does not fire.
func TestRevealOptionalAnsweredNoRevealsNothing(t *testing.T) {
	t.Parallel()
	spell := peekSpellSrc("Peek Optional", "SP$ PeekAndReveal | Defined$ You | NumCards$ 1 | RevealOptional$ True | RememberRevealed$ True")
	e, cfg, _ := peekBoard(t, spell)
	life := e.G.Players[0].Life
	mark := len(e.L.Events)
	d := castNamedSpell(t, e, "Peek Optional", "U", -1)
	if d == nil || d.ResumeKind != "reveal_optional" {
		t.Fatalf("decision = %+v, want reveal_optional", d)
	}
	if d := answerChoices(t, e, optionKindOrFail(t, d, "no")); d != nil {
		t.Fatalf("the declined reveal posed another decision: %+v", d)
	}
	if n := publicRevealsFrom(e, mark); len(n) != 0 {
		t.Fatalf("a declined reveal emitted a Note: %+v", n)
	}
	if got := e.G.Players[0].Life; got != life {
		t.Fatalf("life = %d, want %d (a declined reveal remembers nothing)", got, life)
	}
	replayCheck(t, e, cfg)
}

// TestMayRevealAskKeysOnTheFlagNotTheAPI: the may-reveal ask keys on the
// flag -- RevealOptional$ on the peek shape, Optional$ on the hand shapes --
// and either flag asks on any of the three APIs; a peek with neither flag
// never asks.
func TestMayRevealAskKeysOnTheFlagNotTheAPI(t *testing.T) {
	t.Parallel()
	for i, line := range []string{
		"SP$ Reveal | Defined$ You | NumCards$ 1 | Optional$ True",
		"SP$ RevealHand | Defined$ You | NumCards$ 1 | Optional$ True",
		"SP$ Reveal | Defined$ You | NumCards$ 1 | RevealOptional$ True",
		"SP$ PeekAndReveal | Defined$ You | NumCards$ 1 | RevealOptional$ True",
	} {
		t.Run(line, func(t *testing.T) {
			spell := peekSpellSrc("May Reveal", line)
			e, _ := fixtureGame(t, uint64(40+i), cardsOfSrcs(t, spell, creatureLineSrc("Bear")), nil)
			emptyHand(t, e, 0)
			moveNamedTo(t, e, 0, "May Reveal", state.ZHand)
			moveNamedTo(t, e, 0, "Bear", state.ZHand)
			mark := len(e.L.Events)
			d := castNamedSpell(t, e, "May Reveal", "U", -1)
			if d == nil || d.Player != 0 || d.ResumeKind != "reveal_optional" || len(d.Options) != 2 || d.Options[0].Kind != "yes" {
				t.Fatalf("decision = %+v, want a reveal_optional yes/no for seat 0", d)
			}
			if n := publicRevealsFrom(e, mark); len(n) != 0 {
				t.Fatalf("revealed before the answer: %+v", n)
			}
		})
	}
	t.Run("no flag", func(t *testing.T) {
		spell := peekSpellSrc("Plain Peek", "SP$ PeekAndReveal | Defined$ You | NumCards$ 1")
		e, _, _ := peekBoard(t, spell)
		if d := castNamedSpell(t, e, "Plain Peek", "U", -1); d != nil {
			t.Fatalf("a PeekAndReveal without a may-reveal flag posed %+v", d)
		}
	})
}

// spellHandBoard is a 2-seat game whose seat 0 holds exactly spell plus the
// named creatures (returned in hand order, spell excluded).
func spellHandBoard(t *testing.T, seed uint64, spell string, names ...string) (*Engine, Config, []state.ObjID) {
	t.Helper()
	srcs := []string{spell}
	for _, n := range names {
		srcs = append(srcs, creatureLineSrc(n))
	}
	e, cfg := fixtureGame(t, seed, cardsOfSrcs(t, srcs...), nil)
	emptyHand(t, e, 0)
	moveNamedTo(t, e, 0, card(t, spell).Faces[0].Name, state.ZHand)
	var hand []state.ObjID
	for _, n := range names {
		hand = append(hand, moveNamedTo(t, e, 0, n, state.ZHand))
	}
	return e, cfg, hand
}

// TestOptionalHandRevealDeclinePosesNoPick: a declined PICKABLE optional hand
// reveal (two cards, NumCards default 1) must not fall into a mandatory
// reveal pick: nothing is revealed and the resolution finishes.
func TestOptionalHandRevealDeclinePosesNoPick(t *testing.T) {
	t.Parallel()
	spell := "Name:Hand Reveal\nManaCost:U\nTypes:Sorcery\nA:SP$ Reveal | Defined$ You | Optional$ True\nOracle:x\n"
	e, cfg, _ := spellHandBoard(t, 51, spell, "Alpha", "Beta")
	mark := len(e.L.Events)
	d := castNamedSpell(t, e, "Hand Reveal", "U", -1)
	if d == nil || d.ResumeKind != "reveal_optional" {
		t.Fatalf("decision = %+v, want the reveal_optional ask", d)
	}
	if d := answerChoices(t, e, optionKindOrFail(t, d, "no")); d != nil {
		t.Fatalf("a declined optional hand reveal posed another ask: %+v", d)
	}
	if n := publicRevealsFrom(e, mark); len(n) != 0 {
		t.Fatalf("a declined optional hand reveal revealed: %+v", n)
	}
	replayCheck(t, e, cfg)
}

// TestOptionalHandRevealDeclineWithRevealValidPosesNoPick: Vault 21: House
// Gambit's parameters (NumCards$ 5 over a six-card nonland hand): the decline
// poses no 5-of-6 pick, reveals nothing and remembers nothing.
func TestOptionalHandRevealDeclineWithRevealValidPosesNoPick(t *testing.T) {
	t.Parallel()
	spell := "Name:Gambit Reveal\nManaCost:U\nTypes:Sorcery\n" +
		"A:SP$ Reveal | NumCards$ 5 | Optional$ True | RevealValid$ Card.nonLand+YouOwn | RememberRevealed$ True | SubAbility$ DBGain\n" +
		rememberGainBody + "Oracle:x\n"
	e, cfg, _ := spellHandBoard(t, 52, spell, "Alpha", "Beta", "Gamma", "Delta", "Epsilon", "Zeta")
	life := e.G.Players[0].Life
	mark := len(e.L.Events)
	d := castNamedSpell(t, e, "Gambit Reveal", "U", -1)
	if d == nil || d.ResumeKind != "reveal_optional" {
		t.Fatalf("decision = %+v, want the reveal_optional ask", d)
	}
	if d := answerChoices(t, e, optionKindOrFail(t, d, "no")); d != nil {
		t.Fatalf("a declined NumCards$ 5 optional reveal posed a pick: %+v", d)
	}
	if n := publicRevealsFrom(e, mark); len(n) != 0 {
		t.Fatalf("a declined NumCards$ 5 optional reveal revealed: %+v", n)
	}
	if e.G.Players[0].Life != life {
		t.Fatal("a declined reveal remembered a card (the Remembered rider fired)")
	}
	replayCheck(t, e, cfg)
}

// oppHandsBoard is a 3-seat game whose seat 0 holds spell and whose seats 1 and
// 2 each hold exactly two named cards. Returns the engine, config and the
// four opponent card ids (seat 1's two, then seat 2's two).
func oppHandsBoard(t *testing.T, seed uint64, spell string) (*Engine, Config, [4]state.ObjID) {
	t.Helper()
	e, cfg := fixtureGame(t, seed,
		cardsOfSrcs(t, spell),
		cardsOfSrcs(t, creatureLineSrc("Seat1Alpha"), creatureLineSrc("Seat1Beta")),
		cardsOfSrcs(t, creatureLineSrc("Seat2Alpha"), creatureLineSrc("Seat2Beta")))
	moveNamedTo(t, e, 0, card(t, spell).Faces[0].Name, state.ZHand)
	var ids [4]state.ObjID
	for i, n := range []string{"Seat1Alpha", "Seat1Beta", "Seat2Alpha", "Seat2Beta"} {
		p := state.PlayerID(1 + i/2)
		if i%2 == 0 {
			emptyHand(t, e, p)
		}
		ids[i] = moveNamedTo(t, e, p, n, state.ZHand)
	}
	for p := state.PlayerID(1); p <= 2; p++ {
		if got := e.G.Zone(state.ZHand, p); len(got) != 2 {
			t.Fatalf("precondition: seat %d hand = %v, want two cards", p, got)
		}
	}
	return e, cfg, ids
}

// TestOptionalRevealPosesOncePerDefinedTarget: an optional reveal over two
// Defined$ opponents asks each its OWN yes/no and applies each answer to its
// own hand: seat 1 declines (nothing revealed), seat 2 is still asked and
// accepts, revealing only seat 2's hand.
func TestOptionalRevealPosesOncePerDefinedTarget(t *testing.T) {
	t.Parallel()
	spell := "Name:Opp Reveal\nManaCost:U\nTypes:Sorcery\nA:SP$ RevealHand | Defined$ Player.Opponent | Optional$ True\nOracle:x\n"
	e, cfg, ids := oppHandsBoard(t, 61, spell)
	mark := len(e.L.Events)
	d := castNamedSpell(t, e, "Opp Reveal", "U", -1)
	if d == nil || d.ResumeKind != "reveal_optional" || d.Player != 1 {
		t.Fatalf("first ask = %+v, want seat 1's reveal_optional", d)
	}
	d = answerChoices(t, e, optionKindOrFail(t, d, "no"))
	if n := publicRevealsFrom(e, mark); len(n) != 0 {
		t.Fatalf("a declined reveal emitted a note: %+v", n)
	}
	if d == nil || d.ResumeKind != "reveal_optional" || d.Player != 2 {
		t.Fatalf("second ask = %+v, want seat 2's own reveal_optional (a no must not leak)", d)
	}
	if d := answerChoices(t, e, optionKindOrFail(t, d, "yes")); d != nil {
		t.Fatalf("the walk re-posed after every target answered: %+v", d)
	}
	notes := publicRevealsFrom(e, mark)
	if len(notes) != 1 || notes[0].Player != 2 || !slices.Equal(notes[0].IDs, []state.ObjID{ids[2], ids[3]}) {
		t.Fatalf("notes = %+v, want exactly seat 2's hand %v", notes, ids[2:])
	}
	replayCheck(t, e, cfg)
}

// TestOptionalRevealAcceptDoesNotAcceptLaterTargets: a yes for seat 1 reveals
// only seat 1's hand, and seat 2 is then asked its own question rather than
// revealed unasked.
func TestOptionalRevealAcceptDoesNotAcceptLaterTargets(t *testing.T) {
	t.Parallel()
	spell := "Name:Opp Reveal\nManaCost:U\nTypes:Sorcery\nA:SP$ RevealHand | Defined$ Player.Opponent | Optional$ True\nOracle:x\n"
	e, _, ids := oppHandsBoard(t, 62, spell)
	mark := len(e.L.Events)
	d := castNamedSpell(t, e, "Opp Reveal", "U", -1)
	if d == nil || d.ResumeKind != "reveal_optional" || d.Player != 1 {
		t.Fatalf("first ask = %+v, want seat 1's reveal_optional", d)
	}
	d = answerChoices(t, e, optionKindOrFail(t, d, "yes"))
	notes := publicRevealsFrom(e, mark)
	if len(notes) != 1 || notes[0].Player != 1 || !slices.Equal(notes[0].IDs, []state.ObjID{ids[0], ids[1]}) {
		t.Fatalf("after seat 1 accepted notes = %+v, want exactly seat 1's hand", notes)
	}
	if d == nil || d.ResumeKind != "reveal_optional" || d.Player != 2 {
		t.Fatalf("second ask = %+v, want seat 2's OWN reveal_optional (accept did not leak)", d)
	}
}

// TestOptionalPickableRevealAsksEachTarget: a pickable optional reveal over
// two opponents: seat 1 accepts and picks one of its two cards, then seat 2
// still gets its own may-reveal ask before its own pick.
func TestOptionalPickableRevealAsksEachTarget(t *testing.T) {
	t.Parallel()
	spell := "Name:Opp Pick\nManaCost:U\nTypes:Sorcery\nA:SP$ Reveal | Defined$ Player.Opponent | Optional$ True\nOracle:x\n"
	e, cfg, ids := oppHandsBoard(t, 63, spell)
	mark := len(e.L.Events)
	d := castNamedSpell(t, e, "Opp Pick", "U", -1)
	if d == nil || d.ResumeKind != "reveal_optional" || d.Player != 1 {
		t.Fatalf("ask 1 = %+v, want seat 1's optional gate", d)
	}
	d = answerChoices(t, e, optionKindOrFail(t, d, "yes"))
	if d == nil || d.ResumeKind != "reveal_pick" || d.Player != 1 {
		t.Fatalf("ask 2 = %+v, want seat 1's reveal pick", d)
	}
	d = answerChoices(t, e, objOptionIndex(t, d, ids[1]))
	if n := publicRevealsFrom(e, mark); len(n) != 1 || n[0].Player != 1 || !slices.Equal(n[0].IDs, []state.ObjID{ids[1]}) {
		t.Fatalf("after seat 1's pick notes = %+v, want its chosen card", n)
	}
	if d == nil || d.ResumeKind != "reveal_optional" || d.Player != 2 {
		t.Fatalf("ask 3 = %+v, want seat 2's own optional gate", d)
	}
	d = answerChoices(t, e, optionKindOrFail(t, d, "yes"))
	if d == nil || d.ResumeKind != "reveal_pick" || d.Player != 2 {
		t.Fatalf("ask 4 = %+v, want seat 2's reveal pick", d)
	}
	if d := answerChoices(t, e, objOptionIndex(t, d, ids[2])); d != nil {
		t.Fatalf("walk re-posed after both picks: %+v", d)
	}
	notes := publicRevealsFrom(e, mark)
	if len(notes) != 2 || notes[1].Player != 2 || !slices.Equal(notes[1].IDs, []state.ObjID{ids[2]}) {
		t.Fatalf("final notes = %+v, want seat 2's chosen card", notes)
	}
	replayCheck(t, e, cfg)
}

// TestHandRevealPickPosesOncePerDefinedTarget: a mandatory pickable reveal
// over two opponents poses one pick per target, over that target's OWN
// hand, and reveals each answer from its own hand.
func TestHandRevealPickPosesOncePerDefinedTarget(t *testing.T) {
	t.Parallel()
	spell := "Name:Opp Must Pick\nManaCost:U\nTypes:Sorcery\nA:SP$ Reveal | Defined$ Player.Opponent | RememberRevealed$ True\nOracle:x\n"
	e, cfg, ids := oppHandsBoard(t, 64, spell)
	mark := len(e.L.Events)
	d := castNamedSpell(t, e, "Opp Must Pick", "U", -1)
	if d == nil || d.ResumeKind != "reveal_pick" || d.Player != 1 {
		t.Fatalf("pick 1 = %+v, want seat 1's reveal_pick", d)
	}
	if got := optionObjIDs(d); !slices.Equal(got, []state.ObjID{ids[0], ids[1]}) {
		t.Fatalf("pick 1 options = %v, want seat 1's hand %v", got, ids[:2])
	}
	if n := publicRevealsFrom(e, mark); len(n) != 0 {
		t.Fatalf("a reveal note landed before any answer: %+v", n)
	}
	d = answerChoices(t, e, objOptionIndex(t, d, ids[0]))
	if n := publicRevealsFrom(e, mark); len(n) != 1 || n[0].Player != 1 || !slices.Equal(n[0].IDs, []state.ObjID{ids[0]}) {
		t.Fatalf("after seat 1's answer notes = %+v, want exactly seat 1's chosen card", n)
	}
	if d == nil || d.ResumeKind != "reveal_pick" || d.Player != 2 {
		t.Fatalf("pick 2 = %+v, want seat 2's reveal_pick", d)
	}
	if got := optionObjIDs(d); !slices.Equal(got, []state.ObjID{ids[2], ids[3]}) {
		t.Fatalf("pick 2 options = %v, want seat 2's hand %v", got, ids[2:])
	}
	if d := answerChoices(t, e, objOptionIndex(t, d, ids[3])); d != nil {
		t.Fatalf("the walk re-posed after every target answered: %+v", d)
	}
	notes := publicRevealsFrom(e, mark)
	if len(notes) != 2 || notes[0].IDs[0] != ids[0] || notes[1].Player != 2 || notes[1].IDs[0] != ids[3] {
		t.Fatalf("notes = %+v, want seat 1 revealing %d then seat 2 revealing %d", notes, ids[0], ids[3])
	}
	replayCheck(t, e, cfg)
}

// TestGitaxianProbeLookIsAPrivateLookScopedToTheActivator: the real Gitaxian
// Probe (SP$ RevealHand | ValidTgts$ Player | Look$ True, no NumCards$) first
// poses a one-option Continue look_ack to the caster naming every hand card,
// with no look Note yet; the answer emits exactly one Secret Note scoped to
// the caster (From hand) carrying the target's WHOLE hand, then draws.
func TestGitaxianProbeLookIsAPrivateLookScopedToTheActivator(t *testing.T) {
	t.Parallel()
	probe := corpusAlternativeCard(t, "Gitaxian Probe")
	sa := probe.Faces[0].SpellAbility()
	if sa == nil || sa.API != "RevealHand" || sa.Params["Look"] != "True" {
		t.Fatalf("corpus pin moved: Gitaxian Probe SA = %+v", sa)
	}
	if _, has := sa.Params["NumCards"]; has {
		t.Fatal("corpus pin moved: Gitaxian Probe now carries NumCards$")
	}
	names := []string{"Bolt", "Bear", "Wrenn", "Snares"}
	e, cfg := fixtureGame(t, 71, []*cards.Card{probe},
		cardsOfSrcs(t, creatureLineSrc("Bolt"), creatureLineSrc("Bear"), creatureLineSrc("Wrenn"), creatureLineSrc("Snares")))
	moveNamedTo(t, e, 0, "Gitaxian Probe", state.ZHand)
	emptyHand(t, e, 1)
	var hand []state.ObjID
	for _, n := range names {
		hand = append(hand, moveNamedTo(t, e, 1, n, state.ZHand))
	}
	mark := len(e.L.Events)
	d := castNamedSpell(t, e, "Gitaxian Probe", "U", 1)
	if d == nil || d.Player != 0 || d.Kind != decision.KChoose || d.ResumeKind != "look_ack" {
		t.Fatalf("ack = %+v, want a look_ack KChoose for the caster", d)
	}
	if len(d.Options) != 1 || d.Options[0].Kind != "yes" || d.Options[0].Label != "Continue" {
		t.Fatalf("ack options = %+v, want a single Continue", d.Options)
	}
	for _, n := range names {
		if !strings.Contains(d.Prompt, n) {
			t.Fatalf("ack prompt = %q, want it to name every hand card", d.Prompt)
		}
	}
	if countKindFrom(e, mark, events.Note) != 0 {
		t.Fatal("a look Note landed before the ack")
	}
	if d := answerChoices(t, e, 0); d != nil {
		t.Fatalf("the acknowledged look posed a second ask: %+v", d)
	}
	var looks []events.Event
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.Note && len(ev.IDs) > 0 {
			looks = append(looks, ev)
		}
	}
	if len(looks) != 1 {
		t.Fatalf("look notes = %+v, want exactly one", looks)
	}
	if n := looks[0]; !n.Secret || n.Player != 0 || n.From != state.ZHand || n.Text != "" || !slices.Equal(n.IDs, hand) {
		t.Fatalf("look Note = %+v, want a Secret Note scoped to seat 0 From=hand carrying the whole hand %v", n, hand)
	}
	replayCheck(t, e, cfg)
}
