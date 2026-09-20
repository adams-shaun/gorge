package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// twopilesBoard builds a 2-seat game with five cards on seat 0's library
// (ids[0] on top), a source object, and the FoF-shaped SVar table the pile
// bodies resolve through: DBHand/DBGrave are exactly Fact or Fiction's real
// sub-abilities, DBBottom Jace, Architect of Thought's ChangeZoneAll tail.
// Returns the host, the five library ids, and the source id (ids[5]).
func twopilesBoard(t *testing.T) (*fakeHost, []state.ObjID) {
	t.Helper()
	h := newHost(t, 2)
	var ids []state.ObjID
	cards := []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon", "Source"}
	for i, n := range cards {
		_ = i
		c := mkCard(t, "Name:"+n+"\nTypes:Creature\nPT:1/1\nOracle:x\n")
		ids = append(ids, h.g.AddObject(c, 0).ID)
	}
	lib := append([]state.ObjID(nil), ids[:5]...)
	h.g.SetZone(state.ZLibrary, 0, lib)
	for _, id := range ids[:5] {
		h.g.Obj(id).Zone = state.ZLibrary
	}
	return h, ids
}

// twopilesSVars is the SVar table the FoF-shaped TwoPiles SA resolves its
// pile bodies through.
func twopilesSVars() map[string]string {
	return map[string]string{
		"DBHand":  "DB$ ChangeZone | Defined$ Remembered | Origin$ Library | Destination$ Hand",
		"DBGrave": "DB$ ChangeZone | Defined$ Remembered | Origin$ Library | Destination$ Graveyard",
	}
}

// twopilesSA parses the exact FoF TwoPiles sub-ability.
func twopilesSA(t *testing.T) *cards.SA {
	t.Helper()
	src := "Name:FoF\nTypes:Instant\nOracle:x\n" +
		"SVar:DBTwoPiles:DB$ TwoPiles | Defined$ You | DefinedCards$ Remembered | Separator$ Opponent | ChosenPile$ DBHand | UnchosenPile$ DBGrave\n" +
		"SVar:DBHand:DB$ ChangeZone | Defined$ Remembered | Origin$ Library | Destination$ Hand\n" +
		"SVar:DBGrave:DB$ ChangeZone | Defined$ Remembered | Origin$ Library | Destination$ Graveyard\n"
	c, d := cards.ParseBytes("twopiles.txt", []byte(src))
	if len(d) != 0 {
		t.Fatalf("diags: %v", d)
	}
	c.Link()
	f := c.Faces[0]
	sa := cards.ResolveSVar(f.SVars, "DBTwoPiles")
	if sa == nil {
		t.Fatalf("DBTwoPiles did not resolve")
	}
	return sa
}

// twopilesRemembered is the ctx-level Remembered the upstream
// PeekAndReveal's RememberRevealed$ leaves (a fresh backing slice — the
// aliasing hazard the pile sub-Ctx must not write into).
func twopilesRemembered(ids []state.ObjID) []state.Target {
	out := make([]state.Target, 0, len(ids))
	for _, id := range ids {
		out = append(out, state.Target{Obj: id})
	}
	return out
}

// TestTwoPilesSplitAskPosesToTheSeparatorAndSuspends pins stage 1: the
// separator's KChoose over the card set (Min 0 — piles can be empty — Max 5,
// options in Remembered order) suspends the resolution BEFORE any movement.
func TestTwoPilesSplitAskPosesToTheSeparatorAndSuspends(t *testing.T) {
	h, ids := twopilesBoard(t)
	sh := &suspendHost{fakeHost: *h}
	ctx := &Ctx{Controller: 0, Source: ids[5], Remembered: twopilesRemembered(ids[:5]), SVars: twopilesSVars()}
	sa := twopilesSA(t)
	Resolve(sh, ctx, sa)
	if !sh.suspended || sh.asked == nil {
		t.Fatal("the split ask posed no decision")
	}
	d := sh.asked
	if d.Player != 1 {
		t.Fatalf("ask player = %d, want the separator (opponent) 1", d.Player)
	}
	if d.Kind != decision.KChoose || d.ResumeKind != "twopiles_split" || d.ResumeSA != sa {
		t.Fatalf("ask = kind %s resume %q sa %v, want KChoose/twopiles_split/the SA", d.Kind, d.ResumeKind, d.ResumeSA)
	}
	if d.Min != 0 || d.Max != 5 {
		t.Fatalf("bounds = %d..%d, want 0..5 (piles can be empty)", d.Min, d.Max)
	}
	if len(d.Options) != 5 {
		t.Fatalf("options = %d, want one per remembered card", len(d.Options))
	}
	for i, o := range d.Options {
		if o.Obj != ids[i] {
			t.Fatalf("option %d = obj %d, want %d (Remembered order)", i, o.Obj, ids[i])
		}
	}
	if len(d.ResumeRemembered) != 5 {
		t.Fatalf("ResumeRemembered = %d entries, want the full card set the re-entry re-derives pile B from", len(d.ResumeRemembered))
	}
	for _, e := range sh.log {
		if e.Kind == events.MoveZone || e.Kind == events.Note {
			t.Fatalf("the chain moved or noted before the answer: %+v", e)
		}
	}
}

// TestTwoPilesAnsweredSplitAsksThePick pins stage 2: the split answer
// (Ctx.TwoPiles/TwoPilesDone, what rules' resume arm writes) re-enters and
// poses the CHOOSER's two-option pick, not a second split.
func TestTwoPilesAnsweredSplitAsksThePick(t *testing.T) {
	h, ids := twopilesBoard(t)
	sh := &suspendHost{fakeHost: *h}
	ctx := &Ctx{Controller: 0, Source: ids[5], Remembered: twopilesRemembered(ids[:5]), SVars: twopilesSVars()}
	sa := twopilesSA(t)
	Resolve(sh, ctx, sa)
	sh.suspended, sh.asked = false, nil
	ctx.TwoPiles = []state.ObjID{ids[0], ids[2]}
	ctx.TwoPilesDone = true
	Resolve(sh, ctx, sa)
	if !sh.suspended || sh.asked == nil {
		t.Fatal("the pick ask posed no decision")
	}
	d := sh.asked
	if d.Player != 0 {
		t.Fatalf("ask player = %d, want the chooser (Defined$ You) 0", d.Player)
	}
	if d.Kind != decision.KChoose || d.ResumeKind != "twopiles_pick" {
		t.Fatalf("ask = kind %s resume %q, want KChoose/twopiles_pick", d.Kind, d.ResumeKind)
	}
	if d.Min != 1 || d.Max != 1 || len(d.Options) != 2 ||
		d.Options[0].Kind != "pile-a" || d.Options[1].Kind != "pile-b" {
		t.Fatalf("pick options = %+v, want the two synthetic pile options at Min=Max=1", d.Options)
	}
	if len(d.ResumeChoices) != 2 || d.ResumeChoices[0].Obj != ids[0] || d.ResumeChoices[1].Obj != ids[2] {
		t.Fatalf("ResumeChoices = %+v, want pile A riding to the pick re-entry", d.ResumeChoices)
	}
	for _, e := range sh.log {
		if e.Kind == events.MoveZone {
			t.Fatalf("the piles moved before the pick was answered: %+v", e)
		}
	}
}

// TestTwoPilesAnsweredPickMovesThePiles pins stage 3: the chosen pile's
// cards land in the caster's hand and the other pile in the graveyard,
// entirely through the existing ChangeZone/MoveZone path.
func TestTwoPilesAnsweredPickMovesThePiles(t *testing.T) {
	h, ids := twopilesBoard(t)
	sh := &suspendHost{fakeHost: *h}
	ctx := &Ctx{Controller: 0, Source: ids[5], Remembered: twopilesRemembered(ids[:5]), SVars: twopilesSVars()}
	sa := twopilesSA(t)
	ctx.TwoPiles = []state.ObjID{ids[0], ids[2]}
	ctx.TwoPilesDone = true
	ctx.TwoPilesPick = "a"
	ctx.TwoPilesPickDone = true
	Resolve(sh, ctx, sa)
	if sh.suspended {
		t.Fatal("a fully answered TwoPiles suspended again")
	}
	for _, id := range []state.ObjID{ids[0], ids[2]} {
		if z := h.g.Obj(id).Zone; z != state.ZHand {
			t.Fatalf("pile-A card %d zone = %d, want hand %d", id, z, state.ZHand)
		}
	}
	for _, id := range []state.ObjID{ids[1], ids[3], ids[4]} {
		if z := h.g.Obj(id).Zone; z != state.ZGraveyard {
			t.Fatalf("pile-B card %d zone = %d, want graveyard %d", id, z, state.ZGraveyard)
		}
	}
	// The outer resolution's Remembered must NOT be rewritten by the split.
	if len(ctx.Remembered) != 5 {
		t.Fatalf("outer Remembered = %d entries, want the untouched five", len(ctx.Remembered))
	}
}

// TestTwoPilesPickBChoosesTheSecondPile pins the reversed pick: pile B is
// the chosen one, so B goes to hand and A to the graveyard.
func TestTwoPilesPickBChoosesTheSecondPile(t *testing.T) {
	h, ids := twopilesBoard(t)
	sh := &suspendHost{fakeHost: *h}
	ctx := &Ctx{Controller: 0, Source: ids[5], Remembered: twopilesRemembered(ids[:5]), SVars: twopilesSVars()}
	sa := twopilesSA(t)
	ctx.TwoPiles = []state.ObjID{ids[0], ids[2]}
	ctx.TwoPilesDone = true
	ctx.TwoPilesPick = "b"
	ctx.TwoPilesPickDone = true
	Resolve(sh, ctx, sa)
	for _, id := range []state.ObjID{ids[1], ids[3], ids[4]} {
		if z := h.g.Obj(id).Zone; z != state.ZHand {
			t.Fatalf("pile-B card %d zone = %d, want hand %d", id, z, state.ZHand)
		}
	}
	for _, id := range []state.ObjID{ids[0], ids[2]} {
		if z := h.g.Obj(id).Zone; z != state.ZGraveyard {
			t.Fatalf("pile-A card %d zone = %d, want graveyard %d", id, z, state.ZGraveyard)
		}
	}
}

// TestTwoPilesEmptyPileAnswerMovesEverythingToUnchosen pins the "piles can
// be empty" bound: a split answered with NOTHING (Min 0 makes it legal)
// moves the whole set through the UnchosenPile body when the chooser takes
// pile A.
func TestTwoPilesEmptyPileAnswerMovesEverythingToUnchosen(t *testing.T) {
	h, ids := twopilesBoard(t)
	sh := &suspendHost{fakeHost: *h}
	ctx := &Ctx{Controller: 0, Source: ids[5], Remembered: twopilesRemembered(ids[:5]), SVars: twopilesSVars()}
	sa := twopilesSA(t)
	ctx.TwoPiles = nil
	ctx.TwoPilesDone = true
	ctx.TwoPilesPick = "a"
	ctx.TwoPilesPickDone = true
	Resolve(sh, ctx, sa)
	for _, id := range ids[:5] {
		if z := h.g.Obj(id).Zone; z != state.ZGraveyard {
			t.Fatalf("card %d zone = %d, want graveyard (empty chosen pile)", id, z)
		}
	}
}

// TestTwoPilesEmptyCardSetIsSilent pins the fail-to-find shape: an empty
// card set asks nothing, emits nothing, and the chain continues.
func TestTwoPilesEmptyCardSetIsSilent(t *testing.T) {
	h, ids := twopilesBoard(t)
	sh := &suspendHost{fakeHost: *h}
	ctx := &Ctx{Controller: 0, Source: ids[5], SVars: twopilesSVars()}
	sa := twopilesSA(t)
	Resolve(sh, ctx, sa)
	if sh.suspended || sh.asked != nil {
		t.Fatal("an empty card set posed a decision")
	}
	if len(sh.log) != 0 {
		t.Fatalf("an empty card set emitted %+v, want nothing", sh.log)
	}
}

// TestTwoPilesOneCardSkipsTheSplitButAsksThePick pins the strict-supersets
// gate: a one-card set poses no split (a decision nobody could answer
// differently), but the chooser's pick is still asked.
func TestTwoPilesOneCardSkipsTheSplitButAsksThePick(t *testing.T) {
	h, ids := twopilesBoard(t)
	sh := &suspendHost{fakeHost: *h}
	ctx := &Ctx{Controller: 0, Source: ids[5], Remembered: twopilesRemembered(ids[:1]), SVars: twopilesSVars()}
	sa := twopilesSA(t)
	Resolve(sh, ctx, sa)
	if !sh.suspended || sh.asked == nil {
		t.Fatal("the one-card set posed no pick ask")
	}
	if sh.asked.ResumeKind != "twopiles_pick" {
		t.Fatalf("resume = %q, want the pick ask straight away (no split)", sh.asked.ResumeKind)
	}
	if len(sh.asked.ResumeChoices) != 1 || sh.asked.ResumeChoices[0].Obj != ids[0] {
		t.Fatalf("ResumeChoices = %+v, want the single card as pile A", sh.asked.ResumeChoices)
	}
}

// TestTwoPilesNoHostStandIn pins the R-9 stand-in: with a host that cannot
// ask, pile A is the FIRST card of the set, the chooser takes pile A, no
// Note records it, and nothing wedges.
func TestTwoPilesNoHostStandIn(t *testing.T) {
	h, ids := twopilesBoard(t)
	ctx := &Ctx{Controller: 0, Source: ids[5], Remembered: twopilesRemembered(ids[:5]), SVars: twopilesSVars()}
	sa := twopilesSA(t)
	Resolve(h, ctx, sa) // fakeHost.Ask returns false: no host
	if z := h.g.Obj(ids[0]).Zone; z != state.ZHand {
		t.Fatalf("pile-A card zone = %d, want hand (the stand-in takes pile A)", z)
	}
	for _, id := range ids[1:5] {
		if z := h.g.Obj(id).Zone; z != state.ZGraveyard {
			t.Fatalf("pile-B card %d zone = %d, want graveyard", id, z)
		}
	}
	for _, e := range h.log {
		if e.Kind == events.Note {
			t.Fatalf("the no-host stand-in recorded a Note: %+v", e)
		}
	}
}

// TestTwoPilesSplitTheSpoilsTargetsAndChooserOpponent pins the second core
// carrier's shape: DefinedCards$ Targeted reads the resolution's targets
// (Split the Spoils' exiled cards) and Chooser$ overrides the Defined$
// default — the split asks the Separator$ (You), the pick asks the
// opponent, and the chosen pile moves from the exile.
func TestTwoPilesSplitTheSpoilsTargetedChooserOpponent(t *testing.T) {
	h, ids := twopilesBoard(t)
	// Move three of the five cards to the exile: Split the Spoils exiles
	// its targets first and the pile bodies move them Origin$ Exile.
	h.g.SetZone(state.ZExile, 0, []state.ObjID{ids[0], ids[1], ids[2]})
	for _, id := range ids[:3] {
		h.g.Obj(id).Zone = state.ZExile
	}
	sh := &suspendHost{fakeHost: *h}
	spoils := map[string]string{
		"DBHand":  "DB$ ChangeZone | Defined$ Remembered | Origin$ Exile | Destination$ Hand",
		"DBGrave": "DB$ ChangeZone | Defined$ Remembered | Origin$ Exile | Destination$ Graveyard",
	}
	src := "Name:StS\nTypes:Sorcery\nOracle:x\n" +
		"SVar:DBTwoPiles:DB$ TwoPiles | Chooser$ Opponent | DefinedCards$ Targeted | Separator$ You | ChosenPile$ DBHand | UnchosenPile$ DBGrave\n"
	c, d := cards.ParseBytes("sts.txt", []byte(src))
	if len(d) != 0 {
		t.Fatalf("diags: %v", d)
	}
	c.Link()
	sa := cards.ResolveSVar(c.Faces[0].SVars, "DBTwoPiles")
	if sa == nil {
		t.Fatalf("DBTwoPiles did not resolve")
	}
	ctx := &Ctx{Controller: 0, Source: ids[5],
		Targets: twopilesRemembered(ids[:3]), SVars: spoils}
	Resolve(sh, ctx, sa)
	if !sh.suspended || sh.asked == nil {
		t.Fatal("the split ask posed no decision")
	}
	if sh.asked.Player != 0 {
		t.Fatalf("split ask player = %d, want the Separator$ You caster 0", sh.asked.Player)
	}
	if len(sh.asked.Options) != 3 {
		t.Fatalf("split options = %d, want the three exiled targets", len(sh.asked.Options))
	}
	// Stage 2 (the split answer), then the pick ask goes to the OPONENT.
	sh.suspended, sh.asked = false, nil
	ctx.TwoPiles = []state.ObjID{ids[1]}
	ctx.TwoPilesDone = true
	Resolve(sh, ctx, sa)
	if !sh.suspended || sh.asked == nil {
		t.Fatal("the pick ask posed no decision")
	}
	if sh.asked.Player != 1 {
		t.Fatalf("pick ask player = %d, want Chooser$ Opponent 1", sh.asked.Player)
	}
	// Stage 3: the opponent takes pile A (Beta) → the caster's hand. The
	// pick resume arm re-rides pile A from the decision's ResumeChoices;
	// the manual resume mirrors it by setting Ctx.TwoPiles again.
	sh.suspended, sh.asked = false, nil
	ctx.TwoPiles = []state.ObjID{ids[1]}
	ctx.TwoPilesPick, ctx.TwoPilesPickDone = "a", true
	Resolve(sh, ctx, sa)
	if z := h.g.Obj(ids[1]).Zone; z != state.ZHand {
		t.Fatalf("chosen pile card zone = %d, want hand", z)
	}
	for _, id := range []state.ObjID{ids[0], ids[2]} {
		if z := h.g.Obj(id).Zone; z != state.ZGraveyard {
			t.Fatalf("unchosen pile card %d zone = %d, want graveyard", id, z)
		}
	}
}

// TestTwoPilesPlayBodyHandsThePileToPlay pins Brilliant Ultimatum's shape:
// a ChosenPile$ naming a DB$ Play body hands the pile to the registered Play
// primitive through the fresh sub-Ctx — the play ask is posed over exactly
// the pile's cards, proving the handoff (the optional play's own answer flow
// is effPlay's business, pinned there).
func TestTwoPilesPlayBodyHandsThePileToPlay(t *testing.T) {
	h, ids := twopilesBoard(t)
	sh := &suspendHost{fakeHost: *h}
	svars := map[string]string{
		"DBPlay": "DB$ Play | Defined$ Remembered | WithoutManaCost$ True | Optional$ True | Amount$ All",
	}
	ctx := &Ctx{Controller: 0, Source: ids[5], Remembered: twopilesRemembered(ids[:5]), SVars: svars}
	sa := sa(t, "DB$ TwoPiles | Defined$ You | DefinedCards$ Remembered | Separator$ Opponent | ChosenPile$ DBPlay")
	ctx.TwoPiles = []state.ObjID{ids[0], ids[1]}
	ctx.TwoPilesDone = true
	ctx.TwoPilesPick, ctx.TwoPilesPickDone = "a", true
	Resolve(sh, ctx, sa)
	if !sh.suspended || sh.asked == nil {
		t.Fatal("the DB$ Play pile body posed no ask — the handoff did not reach effPlay")
	}
	if sh.asked.Kind != decision.KModes || sh.asked.ResumeKind != "play" {
		t.Fatalf("ask = kind %s resume %q, want the play KModes", sh.asked.Kind, sh.asked.ResumeKind)
	}
	played := map[state.ObjID]bool{}
	for _, o := range sh.asked.Options {
		if o.Obj != 0 {
			played[o.Obj] = true
		}
	}
	if len(played) != 2 || !played[ids[0]] || !played[ids[1]] {
		t.Fatalf("play options = %+v, want exactly the chosen pile's two cards", sh.asked.Options)
	}
	for _, id := range ids[2:5] {
		if played[id] {
			t.Fatalf("unchosen pile card %d leaked into the play options", id)
		}
	}
}

// TestTwoPilesExoticShapeIsLoudAndInert pins the scope boundary: a line
// carrying an unread parameter (Raging River's LeftRightPile$) emits exactly
// ONE loud Note naming it and moves nothing.
func TestTwoPilesExoticShapeIsLoudAndInert(t *testing.T) {
	h, ids := twopilesBoard(t)
	sh := &suspendHost{fakeHost: *h}
	svars := twopilesSVars()
	svars["DBDefLeftPile"] = "DB$ ChangeZone | Defined$ Remembered | Destination$ Graveyard"
	svars["DBDefRightPile"] = "DB$ ChangeZone | Defined$ Remembered | Destination$ Hand"
	ctx := &Ctx{Controller: 0, Source: ids[5], Remembered: twopilesRemembered(ids[:5]), SVars: svars}
	sa := sa(t, "DB$ TwoPiles | Defined$ Remembered | Separator$ Remembered | Zone$ Battlefield | LeftRightPile$ True | ChosenPile$ DBDefLeftPile | UnchosenPile$ DBDefRightPile")
	Resolve(sh, ctx, sa)
	if sh.suspended {
		t.Fatal("an exotic shape posed a decision")
	}
	notes := 0
	for _, e := range sh.log {
		if e.Kind == events.Note {
			notes++
			if e.Text == "" || !(strings.Contains(e.Text, "Zone$") || strings.Contains(e.Text, "LeftRightPile")) {
				t.Fatalf("the Note names the wrong param: %q", e.Text)
			}
		}
		if e.Kind == events.MoveZone {
			t.Fatalf("the exotic shape moved a card: %+v", e)
		}
	}
	if notes != 1 {
		t.Fatalf("notes = %d, want exactly one naming LeftRightPile$", notes)
	}
	for _, id := range ids[:5] {
		if z := h.g.Obj(id).Zone; z != state.ZLibrary {
			t.Fatalf("card %d moved (zone %d), want the library", id, z)
		}
	}
}

// TestTwoPilesNonSVarPileValueIsLoud pins the other exotic one-off: a
// ChosenPile$ value that is not an SVar name (TurnFaceUp/ToHand/ToGrave)
// emits one Note and moves nothing.
func TestTwoPilesNonSVarPileValueIsLoud(t *testing.T) {
	h, ids := twopilesBoard(t)
	sh := &suspendHost{fakeHost: *h}
	ctx := &Ctx{Controller: 0, Source: ids[5], Remembered: twopilesRemembered(ids[:5]), SVars: twopilesSVars()}
	sa := sa(t, "DB$ TwoPiles | Defined$ You | DefinedCards$ Remembered | ChosenPile$ ToHand | UnchosenPile$ ToGrave")
	Resolve(sh, ctx, sa)
	if sh.suspended {
		t.Fatal("a non-SVar pile value posed a decision")
	}
	notes := 0
	for _, e := range sh.log {
		if e.Kind == events.Note {
			notes++
			if !strings.Contains(e.Text, "ChosenPile$ ToHand") {
				t.Fatalf("the Note names the wrong param: %q", e.Text)
			}
		}
	}
	if notes != 1 {
		t.Fatalf("notes = %d, want exactly one", notes)
	}
}
