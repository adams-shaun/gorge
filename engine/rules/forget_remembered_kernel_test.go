package rules

// ForgetOtherRemembered$ / ForgetChosen$. A resolution re-executes from its
// checkpoint, so what is pinned is the BEHAVIOUR: each ask offers the pre-clear remembered
// candidates, every answer is honoured, the source's persistent memory ends
// as Forge leaves it, and the clear is recorded exactly once.
//
// The source is relicSrc (a tap-ability artifact on seat 0's battlefield),
// seeded with its remembered set by a logged Choose "remembered".

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// creatureFixtureSrc is a vanilla creature fixture named name.
func creatureFixtureSrc(name string) string {
	return "Name:" + name + "\nManaCost:G\nTypes:Creature\nPT:2/2\nOracle:x\n"
}

func creatureFixtureSrcs(names ...string) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		if strings.HasPrefix(n, "Name:") {
			out = append(out, n) // a raw fixture source
			continue
		}
		out = append(out, creatureFixtureSrc(n))
	}
	return out
}

// forgetBoard builds the relic game (seat 0 holds names0, seat 1
// names1 as vanilla creatures), puts the relic on the battlefield and
// empties both hands, ready for the test to place its cards.
func forgetBoard(t *testing.T, seed uint64, body string, names0, names1 []string) (*Engine, Config, state.ObjID) {
	t.Helper()
	e, cfg, id := newFixtureGame(t, seed, relicSrc(body), creatureFixtureSrcs(names0...), creatureFixtureSrcs(names1...))
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	clearHandExcept(t, e, 0, 0)
	clearHandExcept(t, e, 1, 0)
	return e, cfg, id
}

// clearNoteCount counts src's clear-remembered Choose events.
func clearNoteCount(e *Engine, src state.ObjID) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.Choose && ev.Counter == "clear-remembered" && ev.Obj == src {
			n++
		}
	}
	return n
}

func requireSameIDs(t *testing.T, what string, got []state.ObjID, want ...state.ObjID) {
	t.Helper()
	if !sameObjIDs(got, want) {
		t.Fatalf("%s = %v, want %v", what, got, want)
	}
}

// requireInZone asserts each id is in zone z.
func requireInZone(t *testing.T, e *Engine, z state.Zone, ids ...state.ObjID) {
	t.Helper()
	for _, id := range ids {
		if got := zoneOfID(e, id); got != z {
			t.Fatalf("object %d (%s) zone = %s, want %s", id, e.G.Obj(id).Face().Name, got, z)
		}
	}
}

// optionIdxOfKind returns the option index for kind k ("yes"/"no") or fatal.
func optionIdxOfKind(t *testing.T, d *decision.Decision, k string) int {
	t.Helper()
	for _, o := range d.Options {
		if o.Kind == k {
			return o.Index
		}
	}
	t.Fatalf("no %q option on %+v", k, d)
	return -1
}

// TestKernelHandMoveForgetOtherRememberedOwnersAsk: the owner-selected hand walk
// (DefinedPlayer$ RememberedOwner). Owner A's answer settles and clears the
// set, yet owner B's ask still offers B's two pre-clear candidates and B's
// answer moves; the source ends remembering exactly the two moved cards and
// the clear is recorded once.
func TestKernelHandMoveForgetOtherRememberedOwnersAsk(t *testing.T) {
	t.Parallel()
	e, cfg, src := forgetBoard(t, 200,
		"ChangeZone | Origin$ Hand | Destination$ Exile | DefinedPlayer$ RememberedOwner | ChangeType$ Creature.IsRemembered | ChangeNum$ 1 | Mandatory$ True | RememberChanged$ True | ForgetOtherRemembered$ True",
		[]string{"First Creature", "Extra A", "Stale Creature"}, []string{"Second Creature", "Extra B"})
	first := putNamed(t, e, 0, "First Creature", state.ZHand)
	extraA := putNamed(t, e, 0, "Extra A", state.ZHand)
	second := putNamed(t, e, 1, "Second Creature", state.ZHand)
	extraB := putNamed(t, e, 1, "Extra B", state.ZHand)
	stale := putNamed(t, e, 0, "Stale Creature", state.ZExile)
	d := activateRelic(t, e, src, first, extraA, second, extraB, stale)
	if d == nil || d.ResumeKind != "hand_move" || len(d.Options) != 2 {
		t.Fatalf("owner A not offered its two remembered candidates: %+v", d)
	}
	d = pickChoices(t, e, optionIdxOfObj(t, d, first))
	requireInZone(t, e, state.ZExile, first)
	if d == nil || d.ResumeKind != "hand_move" || len(d.Options) != 2 || d.Options[0].Obj != second || d.Options[1].Obj != extraB {
		t.Fatalf("owner B's remembered candidates lost after A's settle: %+v", d)
	}
	if d = pickChoices(t, e, 0); d != nil {
		t.Fatalf("unexpected further ask %+v", d)
	}
	requireInZone(t, e, state.ZExile, first, second)
	requireInZone(t, e, state.ZHand, extraA, extraB)
	requireSameIDs(t, "persistent remembered", rememberedOf(e, src), first, second)
	if n := clearNoteCount(e, src); n != 1 {
		t.Fatalf("clear-remembered recorded %d times, want 1", n)
	}
	replayCheck(t, e, cfg)
}

// TestKernelHandMoveForgetOtherRememberedHandAsk: the whole-hand
// shape offers both remembered hand cards; the answered one is exiled, the
// other stays, the source remembers only the moved card, the clear is once.
func TestKernelHandMoveForgetOtherRememberedHandAsk(t *testing.T) {
	t.Parallel()
	e, cfg, src := forgetBoard(t, 201,
		"ChangeZone | Origin$ Hand | Destination$ Exile | ChangeType$ Card.IsRemembered | ChangeNum$ 1 | Mandatory$ True | RememberChanged$ True | ForgetOtherRemembered$ True",
		[]string{"Hand Pick", "Other Hand Pick", "Stale"}, nil)
	picked := putNamed(t, e, 0, "Hand Pick", state.ZHand)
	other := putNamed(t, e, 0, "Other Hand Pick", state.ZHand)
	stale := putNamed(t, e, 0, "Stale", state.ZExile)
	d := activateRelic(t, e, src, picked, other, stale)
	if d == nil || d.ResumeKind != "hand_move" || len(d.Options) != 2 {
		t.Fatalf("hand ask missing the remembered options: %+v", d)
	}
	if d = pickChoices(t, e, optionIdxOfObj(t, d, picked)); d != nil {
		t.Fatalf("unexpected further ask %+v", d)
	}
	requireInZone(t, e, state.ZExile, picked)
	requireInZone(t, e, state.ZHand, other)
	requireSameIDs(t, "persistent remembered", rememberedOf(e, src), picked)
	if n := clearNoteCount(e, src); n != 1 {
		t.Fatalf("clear-remembered recorded %d times, want 1", n)
	}
	replayCheck(t, e, cfg)
}

// TestKernelDigForgetOtherRememberedAsk:
// a two-player Dig over Creature.IsRemembered: player 0's answered take
// clears the set, and player 1's window ask still offers its pre-clear
// remembered cards; both answered takes are exiled, the untaken stay in the
// libraries, the clear is once.
func TestKernelDigForgetOtherRememberedAsk(t *testing.T) {
	t.Parallel()
	e, cfg, src := forgetBoard(t, 202,
		"Dig | DigNum$ 2 | ChangeNum$ 1 | ChangeValid$ Creature.IsRemembered | DestinationZone$ Exile | Defined$ You & Opponent | RememberChanged$ True | ForgetOtherRemembered$ True",
		[]string{"First Dug", "Second Dug", "Stale"}, []string{"Third Dug", "Fourth Dug"})
	stale := putNamed(t, e, 0, "Stale", state.ZExile)
	w0 := stackLibraryTop(t, e, 0, "First Dug", "Second Dug")
	w1 := stackLibraryTop(t, e, 1, "Third Dug", "Fourth Dug")
	d := activateRelic(t, e, src, w0[0], w0[1], w1[0], w1[1], stale)
	if d == nil || d.ResumeKind != "dig" || len(d.Options) != 2 {
		t.Fatalf("player 0 not offered its two remembered window cards: %+v", d)
	}
	d = pickChoices(t, e, optionIdxOfObj(t, d, w0[0]))
	if d == nil || d.ResumeKind != "dig" || len(d.Options) != 2 || d.Options[0].Obj != w1[0] {
		t.Fatalf("player 1's remembered window lost after player 0's answer: %+v", d)
	}
	if d = pickChoices(t, e, 0); d != nil {
		t.Fatalf("unexpected further ask %+v", d)
	}
	requireInZone(t, e, state.ZExile, w0[0], w1[0])
	requireInZone(t, e, state.ZLibrary, w0[1], w1[1])
	if n := clearNoteCount(e, src); n != 1 {
		t.Fatalf("clear-remembered recorded %d times, want 1", n)
	}
	replayCheck(t, e, cfg)
}

// TestKernelDigUntilForgetOtherRememberedAsk: the found-move election over a
// Creature.IsRemembered walk; accepting moves the found card to hand
// and RememberFound$ makes it the resolution's Remembered (a Defined$
// Remembered sub moves exactly it on); the filler below it is untouched and
// the clear is once.
func TestKernelDigUntilForgetOtherRememberedAsk(t *testing.T) {
	t.Parallel()
	e, cfg, src := forgetBoard(t, 203,
		"DigUntil | Valid$ Creature.IsRemembered | OptionalFoundMove$ True | FoundDestination$ Hand | RememberFound$ True | ForgetOtherRemembered$ True | SubAbility$ DBRead\n"+
			"SVar:DBRead:DB$ ChangeZone | Defined$ Remembered | Origin$ Hand | Destination$ Graveyard",
		[]string{"Found Creature", "Stale"}, nil)
	stale := putNamed(t, e, 0, "Stale", state.ZExile)
	ids := stackLibraryTop(t, e, 0, "Found Creature", "Mountain")
	d := activateRelic(t, e, src, ids[0], stale)
	if d == nil || d.ResumeKind != "diguntil_move" {
		t.Fatalf("found-move election not posed: %+v", d)
	}
	if d = pickChoices(t, e, optionIdxOfKind(t, d, "yes")); d != nil {
		t.Fatalf("unexpected further ask %+v", d)
	}
	// RememberFound$ replaces the resolution's Remembered with the found
	// card, so the DBRead sub (Defined$ Remembered, hand -> graveyard)
	// moves exactly it on.
	requireInZone(t, e, state.ZGraveyard, ids[0])
	requireInZone(t, e, state.ZExile, stale)
	if lib := e.G.Zone(state.ZLibrary, 0); len(lib) == 0 || lib[0] != ids[1] {
		t.Fatalf("the filler below the found card was disturbed: library top %v", lib[:1])
	}
	if n := clearNoteCount(e, src); n != 1 {
		t.Fatalf("clear-remembered recorded %d times, want 1", n)
	}
	replayCheck(t, e, cfg)
}

// TestKernelChooseCardForgetOtherRememberedAsk: chooser 0's answer clears the
// set, chooser 1's pool still holds its pre-clear remembered creature, and
// the source ends remembering both picks with one clear.
func TestKernelChooseCardForgetOtherRememberedAsk(t *testing.T) {
	t.Parallel()
	e, cfg, src := forgetBoard(t, 204,
		"ChooseCard | Defined$ You & Opponent | Amount$ 1 | Choices$ Creature.IsRemembered | ChoiceZone$ Graveyard | Mandatory$ True | RememberChosen$ True | ForgetOtherRemembered$ True",
		[]string{"First Kept", "Stale"}, []string{"Second Kept"})
	first := putNamed(t, e, 0, "First Kept", state.ZGraveyard)
	putNamed(t, e, 0, "Mountain", state.ZGraveyard)
	second := putNamed(t, e, 1, "Second Kept", state.ZGraveyard)
	stale := putNamed(t, e, 0, "Stale", state.ZExile)
	d := activateRelic(t, e, src, first, second, stale)
	if d == nil || d.ResumeKind != "choice" || len(d.Options) == 0 {
		t.Fatalf("chooser 0 not asked: %+v", d)
	}
	d = pickChoices(t, e, optionIdxOfObj(t, d, first))
	if d == nil || len(d.Options) == 0 || d.Options[len(d.Options)-1].Obj != second {
		t.Fatalf("chooser 1's remembered pool lost after chooser 0's answer: %+v", d)
	}
	if d = pickChoices(t, e, optionIdxOfObj(t, d, second)); d != nil {
		t.Fatalf("unexpected further ask %+v", d)
	}
	requireSameIDs(t, "persistent remembered", rememberedOf(e, src), first, second)
	if n := clearNoteCount(e, src); n != 1 {
		t.Fatalf("clear-remembered recorded %d times, want 1", n)
	}
	replayCheck(t, e, cfg)
}

const optionalHandFetchBody = "ChangeZone | Origin$ Hand | Destination$ Exile | DefinedPlayer$ You | ChangeType$ Creature.IsRemembered | ChangeNum$ 1 | Optional$ True | ForgetOtherRemembered$ True"

// TestKernelChangeZoneHandOwnersForgetOtherRememberedOptionalConfirm: Optional$
// asks a yes/no BEFORE the pick and before any clear. Declining keeps the
// memory and picks nothing; accepting clears before the pick, whether the
// pick then takes nothing (the card stays) or takes the card (exiled).
func TestKernelChangeZoneHandOwnersForgetOtherRememberedOptionalConfirm(t *testing.T) {
	t.Parallel()
	for i, branch := range []string{"decline", "accept pick-none", "accept pick"} {
		branch := branch
		seed := uint64(205 + i)
		t.Run(branch, func(t *testing.T) {
			t.Parallel()
			e, cfg, src := forgetBoard(t, seed, optionalHandFetchBody, []string{"Remembered"}, nil)
			card := putNamed(t, e, 0, "Remembered", state.ZHand)
			d := activateRelic(t, e, src, card)
			if d == nil || d.ResumeKind != "hand_move_confirm" || len(d.Options) != 2 || d.Options[0].Kind != "yes" || d.Options[1].Kind != "no" {
				t.Fatalf("the optional hand move did not ask yes/no first: %+v", d)
			}
			requireSameIDs(t, "memory at the confirmation", rememberedOf(e, src), card)
			if branch == "decline" {
				if d = pickChoices(t, e, 1); d != nil {
					t.Fatalf("a declined confirmation posted a pick: %+v", d)
				}
				requireInZone(t, e, state.ZHand, card)
				requireSameIDs(t, "memory after decline", rememberedOf(e, src), card)
				replayCheck(t, e, cfg)
				return
			}
			d = pickChoices(t, e, 0)
			if d == nil || d.ResumeKind != "hand_move" || len(d.Options) != 1 || d.Options[0].Obj != card {
				t.Fatalf("accepted confirmation did not post the card pick: %+v", d)
			}
			requireSameIDs(t, "memory at the pick (cleared on accept)", rememberedOf(e, src))
			if branch == "accept pick-none" {
				pickChoices(t, e)
				requireInZone(t, e, state.ZHand, card)
			} else {
				pickChoices(t, e, 0)
				requireInZone(t, e, state.ZExile, card)
			}
			requireSameIDs(t, "memory after the pick", rememberedOf(e, src))
			replayCheck(t, e, cfg)
		})
	}
}

// TestKernelChangeZoneForgetOtherRememberedFetchEmpty: the remembered card is not
// in the hand, so the pool is empty. Optional$ still asks whether to
// proceed: decline keeps the memory, accept clears it with no pick; a
// Mandatory$ empty fetch asks nothing and clears.
func TestKernelChangeZoneForgetOtherRememberedFetchEmpty(t *testing.T) {
	t.Parallel()
	mandatory := "ChangeZone | Origin$ Hand | Destination$ Exile | DefinedPlayer$ You | ChangeType$ Creature.IsRemembered | ChangeNum$ 1 | Mandatory$ True | ForgetOtherRemembered$ True"
	for i, branch := range []string{"optional decline", "optional accept", "mandatory"} {
		branch := branch
		seed := uint64(208 + i)
		t.Run(branch, func(t *testing.T) {
			t.Parallel()
			body := optionalHandFetchBody
			if branch == "mandatory" {
				body = mandatory
			}
			e, cfg, src := forgetBoard(t, seed, body, []string{"Remembered", "Not Remembered"}, nil)
			card := putNamed(t, e, 0, "Remembered", state.ZExile)
			other := putNamed(t, e, 0, "Not Remembered", state.ZHand)
			d := activateRelic(t, e, src, card)
			switch branch {
			case "mandatory":
				if d != nil {
					t.Fatalf("a mandatory empty pool posted a decision: %+v", d)
				}
				requireSameIDs(t, "memory", rememberedOf(e, src))
				if hasNote(e, "unimplemented API ChangeZone") {
					t.Fatal("the ChangeZone handler did not run")
				}
			default:
				if d == nil || d.ResumeKind != "hand_move_confirm" {
					t.Fatalf("an optional empty pool must ask whether to proceed: %+v", d)
				}
				if branch == "optional decline" {
					if d = pickChoices(t, e, optionIdxOfKind(t, d, "no")); d != nil {
						t.Fatalf("unexpected ask %+v", d)
					}
					requireSameIDs(t, "memory after decline", rememberedOf(e, src), card)
				} else {
					if d = pickChoices(t, e, optionIdxOfKind(t, d, "yes")); d != nil {
						t.Fatalf("an accepted empty fetch posted a pick: %+v", d)
					}
					requireSameIDs(t, "memory after accept", rememberedOf(e, src))
				}
			}
			requireInZone(t, e, state.ZExile, card)
			requireInZone(t, e, state.ZHand, other)
			replayCheck(t, e, cfg)
		})
	}
}

// TestKernelChangeZoneDefinedForgetOtherRememberedFetchEmpty: an Optional$
// Defined$ Remembered library fetch whose card is in exile asks yes/no;
// declining keeps the memory, accepting clears it and moves nothing.
func TestKernelChangeZoneDefinedForgetOtherRememberedFetchEmpty(t *testing.T) {
	t.Parallel()
	for i, accept := range []bool{false, true} {
		accept := accept
		seed := uint64(211 + i)
		name := "decline keeps"
		if accept {
			name = "accept clears with no move"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e, cfg, src := forgetBoard(t, seed,
				"ChangeZone | Origin$ Library | Destination$ Battlefield | Defined$ Remembered | Optional$ True | ForgetOtherRemembered$ True",
				[]string{"Remembered"}, nil)
			card := putNamed(t, e, 0, "Remembered", state.ZExile)
			d := activateRelic(t, e, src, card)
			if d == nil || d.ResumeKind != "defined_library_optional" {
				t.Fatalf("the optional defined fetch did not ask yes/no: %+v", d)
			}
			if accept {
				pickChoices(t, e, optionIdxOfKind(t, d, "yes"))
				requireSameIDs(t, "memory after accept", rememberedOf(e, src))
			} else {
				pickChoices(t, e, optionIdxOfKind(t, d, "no"))
				requireSameIDs(t, "memory after decline", rememberedOf(e, src), card)
			}
			requireInZone(t, e, state.ZExile, card)
			replayCheck(t, e, cfg)
		})
	}
}

// TestKernelChangeZoneHandOwnersForgetOtherRememberedOptionalConfirmPerOwner:
// each remembered owner gets its own confirmation; owner 0 declining keeps
// the memory and does not consume owner 1's; owner 1 accepting its empty
// fetch clears the memory and moves nothing.
func TestKernelChangeZoneHandOwnersForgetOtherRememberedOptionalConfirmPerOwner(t *testing.T) {
	t.Parallel()
	e, cfg, src := forgetBoard(t, 213,
		"ChangeZone | Origin$ Hand | Destination$ Exile | DefinedPlayer$ RememberedOwner | ChangeType$ Creature.IsRemembered | ChangeNum$ 1 | Optional$ True | ForgetOtherRemembered$ True",
		[]string{"Owner zero"}, []string{"Owner one"})
	first := putNamed(t, e, 0, "Owner zero", state.ZExile)
	second := putNamed(t, e, 1, "Owner one", state.ZExile)
	d := activateRelic(t, e, src, first, second)
	if d == nil || d.ResumeKind != "hand_move_confirm" || d.Player != 0 {
		t.Fatalf("owner 0 did not get the first confirmation: %+v", d)
	}
	d = pickChoices(t, e, optionIdxOfKind(t, d, "no"))
	if d == nil || d.ResumeKind != "hand_move_confirm" || d.Player != 1 {
		t.Fatalf("owner 1's confirmation was skipped after owner 0 declined: %+v", d)
	}
	requireSameIDs(t, "memory after owner 0's decline", rememberedOf(e, src), first, second)
	if d = pickChoices(t, e, optionIdxOfKind(t, d, "yes")); d != nil {
		t.Fatalf("owner 1's accepted empty fetch posted a pick: %+v", d)
	}
	requireSameIDs(t, "memory after owner 1's accept", rememberedOf(e, src))
	requireInZone(t, e, state.ZExile, first, second)
	replayCheck(t, e, cfg)
}

// TestKernelChangeZoneHandOwnersForgetOtherRememberedResume: Optional$, owner
// selector RememberedOwner, NO RememberChanged$. Owner 0 confirms and
// exiles its card (clearing the memory to nothing); owner 1 must still be
// confirmed and offered its pre-clear remembered card, which then moves;
// the unremembered creature in owner 1's hand stays.
func TestKernelChangeZoneHandOwnersForgetOtherRememberedResume(t *testing.T) {
	t.Parallel()
	e, cfg, src := forgetBoard(t, 214,
		"ChangeZone | Origin$ Hand | Destination$ Exile | DefinedPlayer$ RememberedOwner | ChangeType$ Creature.IsRemembered | ChangeNum$ 1 | Optional$ True | ForgetOtherRemembered$ True",
		[]string{"First", "Stale"}, []string{"Second", "Unremembered"})
	first := putNamed(t, e, 0, "First", state.ZHand)
	second := putNamed(t, e, 1, "Second", state.ZHand)
	unrem := putNamed(t, e, 1, "Unremembered", state.ZHand)
	stale := putNamed(t, e, 0, "Stale", state.ZExile)
	d := activateRelic(t, e, src, first, second, stale)
	if d == nil || d.ResumeKind != "hand_move_confirm" || len(d.Options) != 2 {
		t.Fatalf("first ask is not the Optional$ confirmation: %+v", d)
	}
	d = pickChoices(t, e, optionIdxOfKind(t, d, "yes"))
	if d == nil || d.ResumeKind != "hand_move" || len(d.Options) != 1 || d.Options[0].Obj != first {
		t.Fatalf("owner 0 not offered its remembered card after confirm: %+v", d)
	}
	d = pickChoices(t, e, 0)
	requireInZone(t, e, state.ZExile, first)
	if d == nil || d.ResumeKind != "hand_move_confirm" {
		t.Fatalf("owner 1's confirmation lost after owner 0's move: %+v", d)
	}
	d = pickChoices(t, e, optionIdxOfKind(t, d, "yes"))
	if d == nil || d.ResumeKind != "hand_move" || len(d.Options) != 1 || d.Options[0].Obj != second {
		t.Fatalf("owner 1's remembered card not offered after its confirm: %+v", d)
	}
	if d = pickChoices(t, e, 0); d != nil {
		t.Fatalf("unexpected further ask %+v", d)
	}
	requireInZone(t, e, state.ZExile, first, second)
	requireInZone(t, e, state.ZHand, unrem)
	replayCheck(t, e, cfg)
}

// TestKernelChangeZoneForgetOtherRememberedMultiOwner: the hidden-pick
// (graveyard) and library-search walks over two remembered owners offer
// each owner its pre-clear remembered card even after the first move
// clears the memory; both move and the source remembers both.
func TestKernelChangeZoneForgetOtherRememberedMultiOwner(t *testing.T) {
	t.Parallel()
	for i, tc := range []struct {
		name, body string
		from, to   state.Zone
	}{
		{"hidden graveyard", "ChangeZone | Hidden$ True | Origin$ Graveyard | Destination$ Hand | DefinedPlayer$ RememberedOwner | ChangeType$ Creature.IsRemembered | ChangeNum$ 1 | Mandatory$ True | RememberChanged$ True | ForgetOtherRemembered$ True", state.ZGraveyard, state.ZHand},
		{"library search", "ChangeZone | Origin$ Library | Destination$ Exile | DefinedPlayer$ RememberedOwner | ChangeType$ Creature.IsRemembered | ChangeNum$ 1 | Mandatory$ True | RememberChanged$ True | ForgetOtherRemembered$ True", state.ZLibrary, state.ZExile},
	} {
		tc := tc
		seed := uint64(215 + i)
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, cfg, src := forgetBoard(t, seed, tc.body, []string{"First", "Stale"}, []string{"Second"})
			var first, second state.ObjID
			if tc.from == state.ZLibrary {
				first, second = stackLibraryTop(t, e, 0, "First")[0], stackLibraryTop(t, e, 1, "Second")[0]
			} else {
				first, second = putNamed(t, e, 0, "First", tc.from), putNamed(t, e, 1, "Second", tc.from)
			}
			stale := putNamed(t, e, 0, "Stale", state.ZExile)
			d := activateRelic(t, e, src, first, second, stale)
			if d == nil || len(d.Options) != 1 || d.Options[0].Obj != first {
				t.Fatalf("owner 0 not offered its remembered card: %+v", d)
			}
			d = pickChoices(t, e, 0)
			requireInZone(t, e, tc.to, first)
			if d == nil || len(d.Options) != 1 || d.Options[0].Obj != second {
				t.Fatalf("owner 1's remembered card lost after the first move: %+v", d)
			}
			if d = pickChoices(t, e, 0); d != nil {
				t.Fatalf("unexpected further ask %+v", d)
			}
			requireInZone(t, e, tc.to, second)
			requireSameIDs(t, "persistent remembered", rememberedOf(e, src), first, second)
			replayCheck(t, e, cfg)
		})
	}
}

// TestKernelChangeZoneHiddenForgetOtherRememberedSelector: a hidden pick over
// Graveyard,Exile offers both remembered candidates; the answer returns the
// graveyard one and the source remembers only it.
func TestKernelChangeZoneHiddenForgetOtherRememberedSelector(t *testing.T) {
	t.Parallel()
	e, cfg, src := forgetBoard(t, 217,
		"ChangeZone | Hidden$ True | Origin$ Graveyard,Exile | Destination$ Hand | ChangeType$ Creature.IsRemembered | ChangeNum$ 1 | RememberChanged$ True | ForgetOtherRemembered$ True",
		[]string{"Hidden Pick", "Stale"}, nil)
	picked := putNamed(t, e, 0, "Hidden Pick", state.ZGraveyard)
	stale := putNamed(t, e, 0, "Stale", state.ZExile)
	d := activateRelic(t, e, src, picked, stale)
	if d == nil || len(d.Options) != 2 {
		t.Fatalf("hidden ask missing the remembered options: %+v", d)
	}
	pickChoices(t, e, optionIdxOfObj(t, d, picked))
	requireInZone(t, e, state.ZHand, picked)
	requireSameIDs(t, "persistent remembered", rememberedOf(e, src), picked)
	replayCheck(t, e, cfg)
}

// TestKernelChangeZoneSearchForgetOtherRememberedReplacesSet (Myr Incubator): the
// library search replaces the source's memory with the found card.
func TestKernelChangeZoneSearchForgetOtherRememberedReplacesSet(t *testing.T) {
	t.Parallel()
	e, cfg, src := forgetBoard(t, 218,
		"ChangeZone | Origin$ Library | Destination$ Exile | ChangeType$ Artifact | ChangeNum$ 1 | RememberChanged$ True | ForgetOtherRemembered$ True",
		[]string{"Stale", "Name:Found Artifact\nManaCost:1\nTypes:Artifact\nOracle:x\n"}, nil)
	stale := putNamed(t, e, 0, "Stale", state.ZExile)
	found := stackLibraryTop(t, e, 0, "Found Artifact")[0]
	d := activateRelic(t, e, src, stale)
	if d == nil || len(d.Options) != 1 || d.Options[0].Obj != found {
		t.Fatalf("search did not offer exactly the artifact: %+v", d)
	}
	pickChoices(t, e, 0)
	requireInZone(t, e, state.ZExile, found)
	requireSameIDs(t, "persistent remembered", rememberedOf(e, src), found)
	replayCheck(t, e, cfg)
}

// TestKernelChangeZoneHandForgetOtherRememberedClearsBeforeMoving: a hand move
// with RememberChanged$ replaces the stale memory with the moved card. (The
// old fixture carried no Optional$/Mandatory$ marker; a markerless hand move
// now fails loud with no move by design, so this one is Mandatory$.)
func TestKernelChangeZoneHandForgetOtherRememberedClearsBeforeMoving(t *testing.T) {
	t.Parallel()
	e, cfg, src := forgetBoard(t, 219,
		"ChangeZone | Origin$ Hand | Destination$ Exile | ChangeNum$ 1 | Mandatory$ True | ForgetOtherRemembered$ True | RememberChanged$ True",
		[]string{"Hand Card", "Stale Card"}, nil)
	moving := putNamed(t, e, 0, "Hand Card", state.ZHand)
	stale := putNamed(t, e, 0, "Stale Card", state.ZExile)
	d := activateRelic(t, e, src, stale)
	if d != nil {
		pickChoices(t, e, optionIdxOfObj(t, d, moving))
	}
	requireInZone(t, e, state.ZExile, moving)
	requireSameIDs(t, "persistent remembered", rememberedOf(e, src), moving)
	replayCheck(t, e, cfg)
}

// TestKernelChooseCardForgetChosen: ForgetChosen$ True removes
// exactly the chosen card from the source's memory (one forget-remembered
// event naming it) while the choice itself is still recorded as Chosen;
// without the parameter both stay remembered.
func TestKernelChooseCardForgetChosen(t *testing.T) {
	t.Parallel()
	for i, forget := range []bool{true, false} {
		forget := forget
		seed := uint64(220 + i)
		name := "ForgetChosen"
		body := "ChooseCard | Defined$ You | Choices$ Card.IsRemembered | ChoiceZone$ Exile | Amount$ 1"
		if forget {
			body += " | ForgetChosen$ True"
		} else {
			name = "control"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e, cfg, src := forgetBoard(t, seed, body, []string{"Chosen Creature", "Kept Creature"}, nil)
			chosen := putNamed(t, e, 0, "Chosen Creature", state.ZExile)
			kept := putNamed(t, e, 0, "Kept Creature", state.ZExile)
			d := activateRelic(t, e, src, chosen, kept)
			if d == nil || len(d.Options) != 2 {
				t.Fatalf("choose ask = %+v, want both remembered cards offered", d)
			}
			pickChoices(t, e, optionIdxOfObj(t, d, chosen))
			if !forget {
				requireSameIDs(t, "persistent remembered (no ForgetChosen$)", rememberedOf(e, src), chosen, kept)
				return
			}
			requireSameIDs(t, "persistent remembered", rememberedOf(e, src), kept)
			found := false
			for _, tg := range e.G.Obj(src).Chosen {
				if !tg.IsPlayer && tg.Obj == chosen {
					found = true
				}
			}
			if !found {
				t.Fatalf("the chosen card left the source's Chosen list: %+v", e.G.Obj(src).Chosen)
			}
			forgets := 0
			for _, ev := range e.L.Events {
				if ev.Kind == events.Choose && ev.Counter == "forget-remembered" {
					forgets++
					if len(ev.IDs) != 1 || ev.IDs[0] != chosen {
						t.Fatalf("forget event named %v, want [%d]", ev.IDs, chosen)
					}
				}
			}
			if forgets != 1 {
				t.Fatalf("forget-remembered events = %d, want 1", forgets)
			}
			replayCheck(t, e, cfg)
		})
	}
}
