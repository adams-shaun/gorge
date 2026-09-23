package effects

// The NoteCards$/NoteCardsFor$ player-notation family (task
// agent-20260918T230450Z-c3e84a8e): a DB$ Pump body records "seat X chose
// branch <label>" through events.PlayerNoted (Forge's NoteCardsEffect), and a
// later resolution reads it back through the shared player filter's
// `Player.NotedFor<label>` qualifier -- so RepeatEach's RepeatPlayers$, a
// Defined$ on Draw/Discard/ChangeZone, a Continuous static's Affected$ and a
// Count$ head all resolve the same seats from one read.
//
// The real corpus carriers pin the exact parameter spellings. Seize the
// Spotlight is the brief's pinned card:
//
//	SVar:Fame:DB$ Pump | Defined$ Remembered | NoteCards$ Self | NoteCardsFor$ Fame
//	SVar:DBFame:DB$ RepeatEach | RepeatPlayers$ Player.NotedForFame | ... | ClearRememberedBeforeLoop$ True
//
// so the two fame/fortune branches split a two-opponent table (one fame, one
// fortune) exactly as the card's oracle text does. The per-chooser ASK that
// fills `Defined$ Remembered` with each chooser is the separate
// api:GenericChoice `Defined$` family, out of this ticket's scope: the tests
// below bind Ctx.Remembered to the chooser directly, which is exactly the
// binding that ask will provide on landing.

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// noteCardAt puts a corpus card's object on seat 0's battlefield and returns
// it, so a body SA (effPump) has a real Source.
func noteCardAt(t *testing.T, h *fakeHost, card *cards.Card) state.ObjID {
	t.Helper()
	src := h.g.AddObject(card, 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: src.ID, From: state.ZLibrary, To: state.ZBattlefield})
	return src.ID
}

// TestPlayerNotedEventIsIdempotentAndOrdered pins the event fold the whole
// family rests on: one PlayerNoted appends its label to that seat's note set,
// a re-note of the SAME label does not duplicate it, order is first-note
// order, and an empty label or an out-of-range seat writes nothing (rather
// than a ghost note). It also proves the note is event-backed state, so a
// replay rebuilds it.
func TestPlayerNotedEventIsIdempotentAndOrdered(t *testing.T) {
	h := newHost(t, 2)
	h.Emit(events.Event{Kind: events.PlayerNoted, Player: 1, Text: "Fame"})
	h.Emit(events.Event{Kind: events.PlayerNoted, Player: 1, Text: "Fortune"})
	h.Emit(events.Event{Kind: events.PlayerNoted, Player: 1, Text: "Fame"})
	h.Emit(events.Event{Kind: events.PlayerNoted, Player: 0, Text: ""})
	h.Emit(events.Event{Kind: events.PlayerNoted, Player: 7, Text: "Fame"})

	if got := h.g.Players[1].Notes; len(got) != 2 || got[0] != "Fame" || got[1] != "Fortune" {
		t.Fatalf("player 1 notes = %v, want [Fame Fortune] (deduped, first-note order)", got)
	}
	if got := h.g.Players[0].Notes; len(got) != 0 {
		t.Fatalf("an empty label wrote %v, want no note", got)
	}
	if len(h.g.Players) <= 7 {
		// The out-of-range emit must not have grown the player list or
		// panicked; assert the seats that DO exist are untouched.
		for i := range h.g.Players {
			if h.g.Players[i].ID == 1 {
				continue
			}
			if len(h.g.Players[i].Notes) != 0 {
				t.Fatalf("out-of-range PlayerNoted leaked onto seat %d: %v", i, h.g.Players[i].Notes)
			}
		}
	}
}

// TestPlayerNotedForQualifierIsASharedFilterRead pins the read side at its one
// home, MatchesPlayerSpecFrom: the qualifier matches exactly the notated
// seats, is case-sensitive, requires the Player/Any base, and an unnotated
// seat never matches.
func TestPlayerNotedForQualifierIsASharedFilterRead(t *testing.T) {
	h := newHost(t, 3)
	h.Emit(events.Event{Kind: events.PlayerNoted, Player: 1, Text: "Fame"})
	h.Emit(events.Event{Kind: events.PlayerNoted, Player: 2, Text: "Fortune"})

	// PRECONDITION: the notes under test really exist, so a matching read
	// below cannot pass on an empty set.
	if len(h.g.Players[1].Notes) != 1 || h.g.Players[1].Notes[0] != "Fame" {
		t.Fatalf("setup: player 1 notes = %v, want [Fame]", h.g.Players[1].Notes)
	}

	cases := []struct {
		spec string
		p    state.PlayerID
		want bool
	}{
		{"Player.NotedForFame", 1, true},
		{"Player.NotedForFame", 2, false},
		{"Player.NotedForFame", 0, false},
		{"Player.NotedForFortune", 2, true},
		{"Player.NotedForFortune", 1, false},
		{"Player.NotedForfame", 1, false}, // case-sensitive, like Forge's string set
		{"Any.NotedForFame", 1, true},
		{"You.NotedForFame", 1, false}, // a qualified base fails closed
	}
	for _, tc := range cases {
		if got := MatchesPlayerSpecFrom(h.g, tc.spec, tc.p, 0, 0); got != tc.want {
			t.Errorf("MatchesPlayerSpecFrom(%q, seat %d) = %v, want %v", tc.spec, tc.p, got, tc.want)
		}
	}
}

// TestSeizeTheSpotlightBranchesNoteTheChooser drives the REAL corpus branch
// bodies (SVar Fame / Fortune) and pins the exact parameter spellings the
// engine reads. Each branch notes the seat named by Defined$ (the chooser the
// GenericChoice ask will bind).
func TestSeizeTheSpotlightBranchesNoteTheChooser(t *testing.T) {
	card, fame := corpusSA(t, "Seize the Spotlight", "Fame")
	if got := fame.Params["NoteCardsFor"]; got != "Fame" {
		t.Fatalf("Seize the Spotlight SVar Fame NoteCardsFor$ = %q, want %q (corpus spelling changed)", got, "Fame")
	}
	if got := fame.Params["NoteCards"]; got != "Self" {
		t.Fatalf("Seize the Spotlight SVar Fame NoteCards$ = %q, want %q", got, "Self")
	}
	_, fortune := corpusSA(t, "Seize the Spotlight", "Fortune")

	h := newHost(t, 3)
	src := noteCardAt(t, h, card)
	svars := h.g.Obj(src).Face().SVars

	// Chooser seat 1 picks Fame; chooser seat 2 picks Fortune.
	effPump(h, &Ctx{Source: src, Controller: 0, SVars: svars,
		Remembered: []state.Target{{Player: 1, IsPlayer: true}}}, fame)
	effPump(h, &Ctx{Source: src, Controller: 0, SVars: svars,
		Remembered: []state.Target{{Player: 2, IsPlayer: true}}}, fortune)

	if got := h.g.Players[1].Notes; len(got) != 1 || got[0] != "Fame" {
		t.Fatalf("seat 1 notes = %v, want [Fame]", got)
	}
	if got := h.g.Players[2].Notes; len(got) != 1 || got[0] != "Fortune" {
		t.Fatalf("seat 2 notes = %v, want [Fortune]", got)
	}
	if got := h.g.Players[0].Notes; len(got) != 0 {
		t.Fatalf("the resolving controller was noted %v, want nothing", got)
	}
}

// TestSeizeTheSpotlightTwoOpponentSplit pins the brief's headline proof: with
// one opponent noted Fame and the other Fortune, the card's own DBFame loop
// walks ONLY the fame chooser and DBFortune walks ONLY the fortune chooser.
// The inner bodies are swapped for an observable LoseLife so the split is
// measured at the engine, not inferred.
func TestSeizeTheSpotlightTwoOpponentSplit(t *testing.T) {
	card, fame := corpusSA(t, "Seize the Spotlight", "Fame")
	_, fortune := corpusSA(t, "Seize the Spotlight", "Fortune")
	_, dbFame := corpusSA(t, "Seize the Spotlight", "DBFame")
	_, dbFortune := corpusSA(t, "Seize the Spotlight", "DBFortune")

	// Pin the selectors and the loop-hygiene param the split rests on.
	if got := dbFame.Params["RepeatPlayers"]; got != "Player.NotedForFame" {
		t.Fatalf("SVar DBFame RepeatPlayers$ = %q, want Player.NotedForFame", got)
	}
	if got := dbFortune.Params["RepeatPlayers"]; got != "Player.NotedForFortune" {
		t.Fatalf("SVar DBFortune RepeatPlayers$ = %q, want Player.NotedForFortune", got)
	}
	if got := dbFame.Params["ClearRememberedBeforeLoop"]; got != "True" {
		t.Fatalf("SVar DBFame ClearRememberedBeforeLoop$ = %q, want True", got)
	}

	h := newHost(t, 3)
	src := noteCardAt(t, h, card)
	svars := h.g.Obj(src).Face().SVars

	// Seat 1 chooses fame, seat 2 chooses fortune.
	effPump(h, &Ctx{Source: src, Controller: 0, SVars: svars,
		Remembered: []state.Target{{Player: 1, IsPlayer: true}}}, fame)
	effPump(h, &Ctx{Source: src, Controller: 0, SVars: svars,
		Remembered: []state.Target{{Player: 2, IsPlayer: true}}}, fortune)

	// PRECONDITION: the split exists before the loops run.
	if !MatchesPlayerSpecFrom(h.g, "Player.NotedForFame", 1, 0, 0) ||
		MatchesPlayerSpecFrom(h.g, "Player.NotedForFame", 2, 0, 0) {
		t.Fatalf("setup: fame notes not split as expected: seat1=%v seat2=%v",
			h.g.Players[1].Notes, h.g.Players[2].Notes)
	}

	// Observe each walked subject by making the loop body life-loss on the
	// bound Remembered (the iteration subject).
	body := map[string]string{
		"GainControl": "DB$ LoseLife | Defined$ Remembered | LifeAmount$ 1",
		"DBDraw":      "DB$ LoseLife | Defined$ Remembered | LifeAmount$ 1",
	}
	for k, v := range svars {
		if _, isBody := body[k]; !isBody {
			body[k] = v
		}
	}

	life0 := h.g.Players[0].Life
	effRepeatEach(h, &Ctx{Source: src, Controller: 0, SVars: body}, dbFame)
	if h.g.Players[1].Life != 19 {
		t.Fatalf("after DBFame: fame chooser (seat 1) life = %d, want 19 (the loop must walk it once)", h.g.Players[1].Life)
	}
	if h.g.Players[2].Life != 20 {
		t.Fatalf("after DBFame: fortune chooser (seat 2) life = %d, want 20 (it must NOT be walked)", h.g.Players[2].Life)
	}
	if h.g.Players[0].Life != life0 {
		t.Fatalf("after DBFame: controller life = %d, want %d (never notated)", h.g.Players[0].Life, life0)
	}

	effRepeatEach(h, &Ctx{Source: src, Controller: 0, SVars: body}, dbFortune)
	if h.g.Players[2].Life != 19 {
		t.Fatalf("after DBFortune: fortune chooser (seat 2) life = %d, want 19 (the loop must walk it once)", h.g.Players[2].Life)
	}
	if h.g.Players[1].Life != 19 {
		t.Fatalf("after DBFortune: fame chooser (seat 1) life = %d, want 19 (unchanged by the fortune loop)", h.g.Players[1].Life)
	}
}

// TestRepeatEachClearRememberedBeforeLoop pins the loop-hygiene param on the
// real carrier: a pre-loop remembered CARD is dropped before the first
// iteration, so it cannot leak into the bodies; without the param the card
// survives the loop. The observable is Ctx.Remembered after the loop, which
// re-folds every iteration's additions.
func TestRepeatEachClearRememberedBeforeLoop(t *testing.T) {
	card, dbFame := corpusSA(t, "Seize the Spotlight", "DBFame")
	if got := dbFame.Params["ClearRememberedBeforeLoop"]; got != "True" {
		t.Fatalf("SVar DBFame ClearRememberedBeforeLoop$ = %q, want True", got)
	}
	// A sibling control loop with the exact same subject but no clear, so the
	// test cannot pass merely because Remembered is not re-folded.
	noClear := *dbFame
	noClear.Params = map[string]string{}
	for k, v := range dbFame.Params {
		if k == "ClearRememberedBeforeLoop" {
			continue
		}
		noClear.Params[k] = v
	}

	run := func(sa *cards.SA) (remembered []state.Target, controllerLife int32) {
		t.Helper()
		h := newHost(t, 3)
		src := noteCardAt(t, h, card)
		h.Emit(events.Event{Kind: events.PlayerNoted, Player: 1, Text: "Fame"})
		chaff := h.g.AddObject(mkCard(t, "Name:Chaff\nTypes:Sorcery\nOracle:x\n"), 0)
		h.Emit(events.Event{Kind: events.MoveZone, Obj: chaff.ID, From: state.ZLibrary, To: state.ZGraveyard})
		// The body observes that the loop RAN (controller loses 1) without
		// reading Remembered, so the remembered-set observation below is about
		// the clear alone.
		c := &Ctx{Source: src, Controller: 0,
			SVars:      map[string]string{"GainControl": "DB$ LoseLife | Defined$ You | LifeAmount$ 1"},
			Remembered: []state.Target{{Obj: chaff.ID}}}
		effRepeatEach(h, c, sa)
		if h.g.Players[0].Life != 19 {
			t.Fatalf("setup: the loop walked no subject (controller life = %d), so the Remembered observation is vacuous", h.g.Players[0].Life)
		}
		return c.Remembered, h.g.Players[0].Life
	}

	cleared, _ := run(dbFame)
	if len(cleared) != 0 {
		t.Fatalf("Ctx.Remembered after the clearing loop = %+v, want empty (the one pre-loop remembered card was cleared and no body added any)", cleared)
	}
	kept, _ := run(&noClear)
	if len(kept) != 1 {
		t.Fatalf("control loop WITHOUT the param: Ctx.Remembered = %+v, want the one pre-loop remembered card (proving the clear, not the fold, is what removed it)", kept)
	}
}

// TestMasterOfCeremoniesThreeWayNotation covers the CLASS, not the instance:
// the real three-way carrier (money/friends/secrets) notes each chooser
// distinctly and each of its three loops walks only its own branch, so a fix
// that special-cased "Fame" would fail here.
func TestMasterOfCeremoniesThreeWayNotation(t *testing.T) {
	card, money := corpusSA(t, "Master of Ceremonies", "Money")
	_, friends := corpusSA(t, "Master of Ceremonies", "Friends")
	_, secrets := corpusSA(t, "Master of Ceremonies", "Secrets")
	_, dbMoney := corpusSA(t, "Master of Ceremonies", "DBMoney")
	_, dbFriends := corpusSA(t, "Master of Ceremonies", "DBFriends")
	_, dbSecrets := corpusSA(t, "Master of Ceremonies", "DBSecrets")

	if got := money.Params["NoteCardsFor"]; got != "Money" {
		t.Fatalf("SVar Money NoteCardsFor$ = %q, want Money", got)
	}
	if got := friends.Params["NoteCardsFor"]; got != "Friends" {
		t.Fatalf("SVar Friends NoteCardsFor$ = %q, want Friends", got)
	}
	if got := secrets.Params["NoteCardsFor"]; got != "Secrets" {
		t.Fatalf("SVar Secrets NoteCardsFor$ = %q, want Secrets", got)
	}

	h := newHost(t, 4)
	src := noteCardAt(t, h, card)
	svars := h.g.Obj(src).Face().SVars

	// Seats 1/2/3 choose money/friends/secrets.
	effPump(h, &Ctx{Source: src, Controller: 0, SVars: svars, Remembered: []state.Target{{Player: 1, IsPlayer: true}}}, money)
	effPump(h, &Ctx{Source: src, Controller: 0, SVars: svars, Remembered: []state.Target{{Player: 2, IsPlayer: true}}}, friends)
	effPump(h, &Ctx{Source: src, Controller: 0, SVars: svars, Remembered: []state.Target{{Player: 3, IsPlayer: true}}}, secrets)

	for seat, want := range map[state.PlayerID]string{1: "Money", 2: "Friends", 3: "Secrets"} {
		if got := h.g.Players[seat].Notes; len(got) != 1 || got[0] != want {
			t.Fatalf("seat %d notes = %v, want [%s]", seat, got, want)
		}
	}

	body := map[string]string{"DBMoneyPump": "DB$ LoseLife | Defined$ Remembered | LifeAmount$ 1"}
	for k, v := range svars {
		if _, isBody := body[k]; !isBody {
			body[k] = v
		}
	}
	// Re-point every loop body at the observable arm.
	for _, k := range []string{"DBMoneyPump", "DBFriendsPump", "DBSecretsPump"} {
		body[k] = "DB$ LoseLife | Defined$ Remembered | LifeAmount$ 1"
	}

	type loop struct {
		sa    *cards.SA
		seat  state.PlayerID
		label string
	}
	for _, l := range []loop{{dbMoney, 1, "Money"}, {dbFriends, 2, "Friends"}, {dbSecrets, 3, "Secrets"}} {
		before := [4]int32{}
		for p := range before {
			before[p] = h.g.Players[p].Life
		}
		effRepeatEach(h, &Ctx{Source: src, Controller: 0, SVars: body}, l.sa)
		if h.g.Players[l.seat].Life != before[l.seat]-1 {
			t.Fatalf("%s loop: its chooser seat %d life = %d, want %d", l.label, l.seat, h.g.Players[l.seat].Life, before[l.seat]-1)
		}
		for p := range before {
			if p == int(l.seat) {
				continue
			}
			if h.g.Players[p].Life != before[p] {
				t.Fatalf("%s loop walked the wrong seat %d (life %d, want %d)", l.label, p, h.g.Players[p].Life, before[p])
			}
		}
	}
}
