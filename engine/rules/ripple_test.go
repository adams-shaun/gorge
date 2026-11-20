package rules

// kw:Ripple (CR 702.60) end to end, on the real corpus carriers: the printed
// K:Ripple:4 line (the five Surging cards) and the layer-6 AddKeyword$
// Ripple:4 grant (Thrumming Stone's "Spells you cast have ripple 4").
//
// The deck is built from compiled corpus cards only; the helpers come from
// search_library_test.go (searchCorpusCard, searchMoveByName), cast_test.go
// (addMana, submitChoices, toMain1), resolution_test.go (castFixture),
// cascade_test.go (movedTo), replacement_updated_test.go
// (passUntilStackEmpty) and conspire_test.go (chooseTargetPlayer) — all the
// same package.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// rippleRevealText is the Note text effRipple emits for the public reveal;
// the tests find it in the log.
const rippleRevealText = "ripple reveals the top of the library"

// rippleTestEngine deals seat 0 a corpus deck led by hero, moves hero to
// hand, puts the battlefield names on the battlefield, and arranges the
// window names on top of seat 0's library in the given order (the
// cascadeArrangeWindow shape). The hero is never taken for a window slot:
// a window entry with the hero's own name (the Ripple reveal windows all
// have one) is always a DIFFERENT copy, so the cast spell stays in hand.
func rippleTestEngine(t *testing.T, seed uint64, hero string, window, battlefield []string) (*Engine, Config, state.ObjID, *cards.Registry) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	forest := searchCorpusCard(t, reg, "Forest")
	mountain := searchCorpusCard(t, reg, "Mountain")
	bear := searchCorpusCard(t, reg, "Grizzly Bears")
	deck := []*cards.Card{searchCorpusCard(t, reg, hero)}
	for _, name := range window {
		deck = append(deck, searchCorpusCard(t, reg, name))
	}
	for _, name := range battlefield {
		deck = append(deck, searchCorpusCard(t, reg, name))
	}
	for i := 0; i < 8; i++ {
		deck = append(deck, forest, mountain)
	}
	for len(deck) < 40 {
		deck = append(deck, bear)
	}
	opp := make([]*cards.Card, 40)
	for i := range opp {
		opp[i] = mountain
	}
	cfg := seatZeroStart(Config{Seed: seed, Names: []string{"rippler", "opponent"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	heroID := searchMoveByName(t, e, hero, state.ZHand)
	for _, name := range battlefield {
		searchMoveByName(t, e, name, state.ZBattlefield)
	}
	win := make([]state.ObjID, 0, len(window))
	taken := make(map[state.ObjID]bool, len(window))
	for _, name := range window {
		picked := state.ObjID(0)
		for _, z := range []state.Zone{state.ZLibrary, state.ZHand} {
			for _, id := range e.G.Zone(z, 0) {
				if id == heroID || taken[id] {
					continue // the cast spell never becomes a revealed card
				}
				if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
					if z == state.ZHand {
						e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary})
					}
					picked = id
					break
				}
			}
			if picked != 0 {
				break
			}
		}
		if picked == 0 {
			t.Fatalf("window card %q not in seat 0's hand or library", name)
		}
		taken[picked] = true
		win = append(win, picked)
	}
	inWin := make(map[state.ObjID]bool, len(win))
	for _, id := range win {
		inWin[id] = true
	}
	newLib := append([]state.ObjID(nil), win...)
	for _, id := range e.G.Zone(state.ZLibrary, 0) {
		if !inWin[id] {
			newLib = append(newLib, id)
		}
	}
	e.emit(events.Event{Kind: events.LibraryOrder, Player: 0, IDs: newLib})
	e.pending = nil
	e.priorityRound()
	return e, cfg, heroID, reg
}

// rippleDrainLibraryTo exiles every library card past the first keep (logged
// MoveZone events), so a "fewer than N" library is real state.
func rippleDrainLibraryTo(t *testing.T, e *Engine, keep int) {
	t.Helper()
	for _, id := range e.G.Zone(state.ZLibrary, 0)[keep:] {
		if o := e.G.Obj(id); o != nil && o.Zone == state.ZLibrary {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZExile})
		}
	}
	e.pending = nil
	e.priorityRound()
}

// rippleOptional answers the pending ripple may (the CR 603.5
// askOptionalAtResolution ask) with yes or no; option 0 is the yes arm.
func rippleOptional(t *testing.T, e *Engine, yes bool) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTriggerOptional {
		t.Fatalf("expected the ripple trigger_optional ask, got %+v", d)
	}
	idx := 0
	if !yes {
		idx = 1
	}
	if idx >= len(d.Options) {
		t.Fatalf("option %d out of range: %+v", idx, d.Options)
	}
	submitChoices(t, e, d.Options[idx].Index)
}

// rippleElection asserts the pending decision is the ripple free-cast
// election (the play KModes shape) and submits the options whose cards are
// named, in the order given; no names declines (Min 0).
func rippleElection(t *testing.T, e *Engine, names ...string) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KModes || d.ResumeKind != "play" {
		t.Fatalf("expected the ripple play election, got kind %v resume %q: %+v", d.Kind, d.ResumeKind, d)
	}
	if len(names) == 0 {
		submitChoices(t, e)
		return
	}
	used := make(map[int]bool, len(names))
	choices := make([]int, 0, len(names))
	for _, name := range names {
		found := false
		for _, opt := range d.Options {
			if used[opt.Index] || opt.Obj == 0 {
				continue
			}
			if card := e.G.Obj(opt.Obj); card != nil && card.Face() != nil && card.Face().Name == name {
				choices = append(choices, opt.Index)
				used[opt.Index] = true
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("election does not offer %q: %+v", name, d.Options)
		}
	}
	submitChoices(t, e, choices...)
}

// rippleArrangement answers the pending KArrange bottom-order ask; the ids,
// when given, pin the picked order (all options must be named), else the
// offered order is taken.
func rippleArrangement(t *testing.T, e *Engine, ids ...state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KArrange {
		t.Fatalf("expected the ripple bottom-order arrange ask, got %+v", d)
	}
	choices := make([]int, 0, len(ids))
	if len(ids) > 0 {
		for _, want := range ids {
			found := false
			for _, o := range d.Options {
				if o.Obj == want {
					choices = append(choices, o.Index)
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("arrange does not offer %d: %+v", want, d.Options)
			}
		}
		if len(choices) != len(d.Options) {
			t.Fatalf("arrange options %+v do not match the named ids %v", d.Options, ids)
		}
	} else {
		for _, o := range d.Options {
			choices = append(choices, o.Index)
		}
	}
	submitChoices(t, e, choices...)
}

// castFromLib counts the log's PutOnStack events that pushed id from the
// library -- the shape a Play-cast of a library card emits (there is no
// MoveZone on that route).
func castFromLib(t *testing.T, e *Engine, id state.ObjID) int {
	t.Helper()
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.PutOnStack && ev.Obj == id && ev.From == state.ZLibrary {
			n++
		}
	}
	return n
}

// rippleDrive answers the pending asks until the stack and the trigger
// queue settle: targets go to the opponent, optional asks are accepted for
// the first acceptOpt occurrences and declined after, bottom-order
// arrangements are answered in offer order, priorities pass.
func rippleDrive(t *testing.T, e *Engine, limit int, acceptOpt int) {
	t.Helper()
	accepted := 0
	for i := 0; i < limit && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil {
			if len(e.G.Stack) == 0 && len(e.pendingTriggers) == 0 {
				return
			}
			continue
		}
		switch d.Kind {
		case decision.KPriority:
			if len(e.G.Stack) == 0 && len(e.pendingTriggers) == 0 {
				return // nothing of the ripple flow is left; stop before the turn ends
			}
			idx := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("priority with no pass option: %+v", d)
			}
			submitChoices(t, e, idx)
		case decision.KTriggerOptional:
			rippleOptional(t, e, accepted < acceptOpt)
			accepted++
		case decision.KArrange:
			rippleArrangement(t, e)
		case decision.KTarget:
			chooseTargetPlayer(t, e, 1)
		case decision.KTriggerOrder:
			answerTriggerOrders(t, e)
		default:
			t.Fatalf("unexpected %v ask: %+v", d.Kind, d)
		}
	}
	t.Fatalf("rippleDrive did not settle within %d steps", limit)
}

// rippleRevealNotes counts the log's ripple reveal Notes.
func rippleRevealNotes(t *testing.T, e *Engine) int {
	t.Helper()
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && ev.Text == rippleRevealText {
			n++
		}
	}
	return n
}

// rippleNoteIDs returns the ids of the nth reveal Note (0-based).
func rippleNoteIDs(t *testing.T, e *Engine, n int) []state.ObjID {
	t.Helper()
	seen := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && ev.Text == rippleRevealText {
			if seen == n {
				return ev.IDs
			}
			seen++
		}
	}
	t.Fatalf("reveal note %d not found", n)
	return nil
}

// TestSurgingFlameRippleCastsSameNameFree is CR 702.60a on the real printed
// carrier: Surging Flame ({1}{R}, K:Ripple:4) is cast from hand with two
// other Surging Flames among the top four; both may be cast free (targeting
// the opponent), and the two non-matching cards go to the bottom.
func TestSurgingFlameRippleCastsSameNameFree(t *testing.T) {
	t.Parallel()
	e, cfg, heroID, _ := rippleTestEngine(t, 77101, "Surging Flame",
		[]string{"Mountain", "Surging Flame", "Grizzly Bears", "Surging Flame"}, nil)
	lib := e.G.Zone(state.ZLibrary, 0)
	mtnID, flameA, bearID, flameB := lib[0], lib[1], lib[2], lib[3]
	for _, id := range []state.ObjID{mtnID, flameA, bearID, flameB} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZLibrary {
			t.Fatalf("precondition: window card %d not in the library", id)
		}
	}
	if name := e.G.Obj(flameA).Face().Name; name != "Surging Flame" {
		t.Fatalf("precondition: window[1] is %q, want Surging Flame", name)
	}

	addMana(t, e, 0, "RR")
	castFixture(t, e, heroID, 1) // Surging Flame targets the opponent

	// The ripple ability resolves above the spell: the may is offered.
	rippleOptional(t, e, true)
	// The reveal is public and covers exactly the top four.
	if n := rippleRevealNotes(t, e); n != 1 {
		t.Fatalf("reveal notes = %d, want 1", n)
	}
	if ids := rippleNoteIDs(t, e, 0); len(ids) != 4 {
		t.Fatalf("reveal covered %d cards, want the top 4", len(ids))
	}
	// Both same-name cards are offered for a free cast; both are taken.
	rippleElection(t, e, "Surging Flame", "Surging Flame")
	// The free casts' own ripple abilities are declined, targets go to the
	// opponent, and the two non-matching cards bottom.
	rippleDrive(t, e, 120, 0)

	for _, id := range []state.ObjID{flameA, flameB} {
		if n := castFromLib(t, e, id); n == 0 {
			t.Fatalf("revealed Surging Flame %d was never cast from the library", id)
		}
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZGraveyard {
			t.Fatalf("the cast copy of %d is in %v, want the graveyard after resolving", id, o.Zone)
		}
	}
	// Three Surging Flames hit the opponent: the cast spell and two free casts.
	if life := e.G.Players[1].Life; life != 14 {
		t.Fatalf("opponent life = %d, want 14 (three 2-damage Surging Flames)", life)
	}
	// The two non-matching cards are at the bottom of the library.
	lib = e.G.Zone(state.ZLibrary, 0)
	bottom := map[state.ObjID]bool{lib[len(lib)-1]: true, lib[len(lib)-2]: true}
	if !bottom[mtnID] || !bottom[bearID] {
		t.Fatalf("the uncast revealed cards are not at the bottom: last two = %v/%v, want %d/%d",
			lib[len(lib)-2], lib[len(lib)-1], bearID, mtnID)
	}
	replayCheck(t, e, cfg)
}

// TestRippleDeclinedRevealsNothing is the may declined (CR 702.60a): the
// library is untouched and nothing is revealed.
func TestRippleDeclinedRevealsNothing(t *testing.T) {
	t.Parallel()
	e, cfg, heroID, _ := rippleTestEngine(t, 77102, "Surging Flame",
		[]string{"Mountain", "Surging Flame", "Grizzly Bears", "Mountain"}, nil)
	lib := e.G.Zone(state.ZLibrary, 0)
	flameID := lib[1]
	before := append([]state.ObjID(nil), lib...)

	addMana(t, e, 0, "RR")
	castFixture(t, e, heroID, 1)

	// The may is declined.
	rippleOptional(t, e, false)
	rippleDrive(t, e, 60, 0)

	if n := rippleRevealNotes(t, e); n != 0 {
		t.Fatalf("a declined ripple revealed %d times", n)
	}
	if n := castFromLib(t, e, flameID); n != 0 {
		t.Fatalf("a declined ripple still cast the same-name card")
	}
	lib = e.G.Zone(state.ZLibrary, 0)
	if len(lib) != len(before) {
		t.Fatalf("library size %d, want the untouched %d", len(lib), len(before))
	}
	for i, id := range before {
		if lib[i] != id {
			t.Fatalf("library order changed at %d: %d != %d", i, lib[i], id)
		}
	}
	if life := e.G.Players[1].Life; life != 18 {
		t.Fatalf("opponent life = %d, want 18 (only the cast spell resolved)", life)
	}
	replayCheck(t, e, cfg)
}

// TestRippleFewerThanNRevealsAll is CR 702.60a's "(all of them if fewer)"
// clause: a two-card library with ripple 4 reveals both and bottoms both.
func TestRippleFewerThanNRevealsAll(t *testing.T) {
	t.Parallel()
	e, cfg, heroID, _ := rippleTestEngine(t, 77103, "Surging Flame",
		[]string{"Grizzly Bears", "Mountain"}, nil)
	rippleDrainLibraryTo(t, e, 2)
	lib := e.G.Zone(state.ZLibrary, 0)
	if len(lib) != 2 {
		t.Fatalf("precondition: library has %d cards, want 2", len(lib))
	}
	bearID, mtnID := lib[0], lib[1]

	addMana(t, e, 0, "RR")
	castFixture(t, e, heroID, 1)

	rippleOptional(t, e, true)
	if n := rippleRevealNotes(t, e); n != 1 {
		t.Fatalf("reveal notes = %d, want 1", n)
	}
	if ids := rippleNoteIDs(t, e, 0); len(ids) != 2 {
		t.Fatalf("reveal covered %d cards, want both of the two-card library", len(ids))
	}
	// No same-name card was revealed, so no cast election: straight to the
	// bottom arrangement.
	rippleDrive(t, e, 60, 0)

	lib = e.G.Zone(state.ZLibrary, 0)
	bottom := map[state.ObjID]bool{lib[len(lib)-1]: true, lib[len(lib)-2]: true}
	if !bottom[bearID] || !bottom[mtnID] {
		t.Fatalf("the revealed cards are not at the bottom: %v/%v, want %d/%d",
			lib[len(lib)-2], lib[len(lib)-1], bearID, mtnID)
	}
	replayCheck(t, e, cfg)
}

// TestThrummingStoneGrantsRipple is CR 702.60b through the granted path:
// Thrumming Stone's "Spells you cast have ripple 4" gives a cast Relentless
// Rats the ripple trigger, which reveals the top four and offers the Rats
// among them.
func TestThrummingStoneGrantsRipple(t *testing.T) {
	t.Parallel()
	e, cfg, heroID, _ := rippleTestEngine(t, 77104, "Relentless Rats",
		[]string{"Relentless Rats", "Mountain", "Grizzly Bears", "Relentless Rats"},
		[]string{"Thrumming Stone"})
	if o := e.G.Obj(heroID); o == nil || o.Zone != state.ZHand {
		t.Fatalf("precondition: Rats in %v, want hand", o.Zone)
	}
	lib := e.G.Zone(state.ZLibrary, 0)
	ratA, mtnID, bearID, ratB := lib[0], lib[1], lib[2], lib[3]

	addMana(t, e, 0, "BBB") // {1}{B}{B}
	castFixture(t, e, heroID, -1)

	// The GRANTED ripple ability is offered (the stone's grant, not a
	// printed line) and accepted.
	rippleOptional(t, e, true)
	if ids := rippleNoteIDs(t, e, 0); len(ids) != 4 {
		t.Fatalf("granted reveal covered %d cards, want 4", len(ids))
	}
	rippleElection(t, e, "Relentless Rats", "Relentless Rats")
	// The free-cast Rats' granted ripples are declined; the two non-matching
	// cards bottom.
	rippleDrive(t, e, 120, 0)

	for _, id := range []state.ObjID{ratA, ratB} {
		if n := castFromLib(t, e, id); n == 0 {
			t.Fatalf("revealed Relentless Rats %d was never cast", id)
		}
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("the free-cast Rats %d is in %v, want the battlefield", id, o.Zone)
		}
	}
	if life := e.G.Players[1].Life; life != 20 {
		t.Fatalf("opponent life = %d, want untouched 20", life)
	}
	// The two non-matching revealed cards were bottomed.
	lib = e.G.Zone(state.ZLibrary, 0)
	bottom := map[state.ObjID]bool{lib[len(lib)-1]: true, lib[len(lib)-2]: true}
	if !bottom[mtnID] || !bottom[bearID] {
		t.Fatalf("the uncast revealed cards are not at the bottom: %v/%v, want %d/%d",
			lib[len(lib)-2], lib[len(lib)-1], bearID, mtnID)
	}
	replayCheck(t, e, cfg)
}

// TestRippleCastCopyRipplesAgain is CR 702.60b: a card cast by ripple is
// itself a spell with ripple, so its own instance triggers again.
func TestRippleCastCopyRipplesAgain(t *testing.T) {
	t.Parallel()
	e, cfg, heroID, _ := rippleTestEngine(t, 77105, "Surging Flame",
		[]string{"Surging Flame", "Mountain", "Grizzly Bears", "Mountain"}, nil)
	lib := e.G.Zone(state.ZLibrary, 0)
	flameID := lib[0]
	if name := e.G.Obj(flameID).Face().Name; name != "Surging Flame" {
		t.Fatalf("precondition: window[0] is %q, want Surging Flame", name)
	}

	addMana(t, e, 0, "RR")
	castFixture(t, e, heroID, 1)

	// The first ripple reveals four, offers the one Flame, and the cast is
	// taken (it targets the opponent).
	rippleOptional(t, e, true)
	rippleElection(t, e, "Surging Flame")
	// The ripple-cast Flame's own ripple is the next optional ask; it is
	// accepted (acceptOpt 1) and reveals a second time. Later asks decline.
	rippleDrive(t, e, 120, 1)

	if n := rippleRevealNotes(t, e); n != 2 {
		t.Fatalf("reveal notes = %d, want 2 (the ripple-cast spell ripples again)", n)
	}
	if n := castFromLib(t, e, flameID); n == 0 {
		t.Fatal("the revealed Flame was never cast")
	}
	// Two Flames resolved on the opponent: the original and the one free
	// cast (the second window held no Flame to offer).
	if life := e.G.Players[1].Life; life != 16 {
		t.Fatalf("opponent life = %d, want 16 (original + one free cast)", life)
	}
	replayCheck(t, e, cfg)
}

// TestEveryRippleCardIsPlayable is the class census: every printed
// K:Ripple carrier in the corpus leaves cards.Registry.Unsupported.
func TestEveryRippleCardIsPlayable(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	carriers := 0
	for _, c := range reg.AllCards() {
		printed := false
		for _, f := range c.Faces {
			if _, ok := f.KeywordParam("Ripple"); ok {
				printed = true
				break
			}
		}
		if !printed {
			continue
		}
		carriers++
		if miss := reg.Unsupported(c, sup); len(miss) != 0 {
			t.Errorf("%s unsupported: %v", c.Faces[0].Name, miss)
		}
	}
	if carriers < 5 {
		t.Fatalf("corpus has %d printed Ripple carriers, want at least the 5 Surging cards", carriers)
	}
	for _, name := range []string{"Surging Aether", "Surging Dementia", "Surging Flame", "Surging Might", "Surging Sentinels"} {
		if _, ok := reg.Lookup(name); !ok {
			t.Errorf("%s not in the corpus", name)
		}
	}
}
