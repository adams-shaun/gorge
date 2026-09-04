package effects

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// mkCard parses and links a standalone card for a test's own board, applying
// intrinsics the same way board(t) does in filter_test.go.
func mkCard(t *testing.T, src string) *cards.Card {
	t.Helper()
	c, d := cards.ParseBytes("t.txt", []byte(src))
	if len(d) != 0 {
		t.Fatalf("diags: %v", d)
	}
	c.Link()
	for _, f := range c.Faces {
		f.ApplyIntrinsics()
	}
	return c
}

// fillLibrary adds n fresh copies of c to p's library, in the order they end
// up on top (index 0 first), and returns their IDs.
func fillLibrary(g *state.Game, p state.PlayerID, c *cards.Card, n int) []state.ObjID {
	var ids []state.ObjID
	for i := 0; i < n; i++ {
		ids = append(ids, g.AddObject(c, p).ID)
	}
	g.SetZone(state.ZLibrary, p, ids)
	return ids
}

// twoFacedCard is a minimal double-faced fixture: parse.go starts a new Face
// on an "ALTERNATE" line.
func twoFacedCard(t *testing.T) *cards.Card {
	t.Helper()
	return mkCard(t, "Name:Front\nTypes:Creature\nPT:1/1\nOracle:x\n\nALTERNATE\n\nName:Back\nTypes:Creature\nPT:3/3\nOracle:x\n")
}

func TestDealDamageHitsATargetedPlayer(t *testing.T) {
	h := newHost(t, 2)
	h.g.Players[1].Life = 20
	c := &Ctx{Controller: 0, Targets: []state.Target{{Player: 1, IsPlayer: true}}}
	Resolve(h, c, sa(t, "SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3"))
	if h.g.Players[1].Life != 17 {
		t.Fatalf("life = %d, want 17", h.g.Players[1].Life)
	}
}

func TestDealDamageMarksAPermanent(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0, Targets: []state.Target{{Obj: ids["myBear"]}}}
	Resolve(h, c, sa(t, "SP$ DealDamage | ValidTgts$ Creature | NumDmg$ 2"))
	if got := g.Obj(ids["myBear"]).Damage; got != 2 {
		t.Fatalf("marked damage = %d, want 2", got)
	}
}

func TestDealDamageDefaultsMissingNumDmgToZero(t *testing.T) {
	h := newHost(t, 2)
	h.g.Players[1].Life = 20
	c := &Ctx{Controller: 0, Targets: []state.Target{{Player: 1, IsPlayer: true}}}
	Resolve(h, c, sa(t, "SP$ DealDamage | ValidTgts$ Any"))
	if h.g.Players[1].Life != 20 {
		t.Fatalf("life = %d, want unchanged at 20", h.g.Players[1].Life)
	}
}

func TestAddManaFillsTheControllersPool(t *testing.T) {
	h := newHost(t, 2)
	c := &Ctx{Controller: 0}
	Resolve(h, c, sa(t, "AB$ Mana | Cost$ T | Produced$ R | Amount$ 1"))
	if h.g.Players[0].Pool[state.MR] != 1 {
		t.Fatalf("pool = %v, want 1 red", h.g.Players[0].Pool)
	}
	// The other seat's pool must be untouched.
	if h.g.Players[1].Pool.Total() != 0 {
		t.Fatalf("player 1 pool = %v, want empty", h.g.Players[1].Pool)
	}
}

func TestAddManaDefaultsToColorlessAndAmountOne(t *testing.T) {
	h := newHost(t, 2)
	c := &Ctx{Controller: 0}
	Resolve(h, c, sa(t, "AB$ Mana | Cost$ T"))
	if h.g.Players[0].Pool[state.MC] != 1 {
		t.Fatalf("pool = %v, want 1 colourless", h.g.Players[0].Pool)
	}
}

// TestDealDamageClampsNegativeNumDmgToZero is Ruling T14-f's regression test:
// NumDmg$ is Num()'s unclamped output, and events.Apply's Damage case is a
// plain subtraction from Life, so a negative NumDmg would otherwise heal
// instead of doing nothing. No real card text ever specifies this, but the
// inversion (a damage spell that heals) is stark enough to close directly.
func TestDealDamageClampsNegativeNumDmgToZero(t *testing.T) {
	h := newHost(t, 2)
	h.g.Players[1].Life = 20
	c := &Ctx{Controller: 0, Targets: []state.Target{{Player: 1, IsPlayer: true}}}
	Resolve(h, c, sa(t, "SP$ DealDamage | ValidTgts$ Any | NumDmg$ -5"))
	if h.g.Players[1].Life != 20 {
		t.Fatalf("life = %d, want unchanged at 20 (negative damage must not heal)", h.g.Players[1].Life)
	}
}

// TestAddManaClampsNegativeAmountToZero is Mana's half of Ruling T14-f: a
// negative Amount$ would otherwise drop the pool below zero.
func TestAddManaClampsNegativeAmountToZero(t *testing.T) {
	h := newHost(t, 2)
	c := &Ctx{Controller: 0}
	Resolve(h, c, sa(t, "AB$ Mana | Cost$ T | Produced$ R | Amount$ -3"))
	if h.g.Players[0].Pool[state.MR] != 0 {
		t.Fatalf("pool = %v, want unchanged at 0 (negative amount must not go below zero)", h.g.Players[0].Pool)
	}
}

// TestPrimitivesAreRegistered guards against a silent unregistration: if any
// of these were ever dropped, Resolve would fall back to emitting a Note
// event ("unimplemented API ...") instead of the real effect, and the tests
// covering it would need to fail loudly rather than quietly pass on a no-op.
// Token and CopySpellAbility are deliberately absent -- see
// TestTokenAndCopySpellAbilityAreNotYetRegistered.
func TestPrimitivesAreRegistered(t *testing.T) {
	sup := Supported()
	for _, api := range []string{
		"DealDamage", "DamageAll", "Mana",
		"Draw", "Discard", "Mill", "Dig", "Reveal", "RevealHand", "PeekAndReveal",
		"RearrangeTopOfLibrary", "NameCard",
		"ChangeZone", "ChangeZoneAll", "Destroy", "DestroyAll", "Sacrifice",
		"GainLife", "LoseLife",
		"PutCounter", "RemoveCounterAll", "Regenerate",
		"Tap", "Pump", "PumpAll", "Animate", "Protection",
		"Effect", "Cleanup", "SetState", "Counter", "DelayedTrigger", "Repeat",
		"Charm", "Vote", "BecomeMonarch", "RestartGame",
	} {
		if !sup["api:"+api] {
			t.Fatalf("api:%s not registered", api)
		}
	}
}

// ---------------------------------------------------------------------------
// damage.go

// TestDealDamageHitsPlayersAndPermanents is the brief's own worked-example
// regression test.
func TestDealDamageHitsPlayersAndPermanents(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0, Targets: []state.Target{{Player: 1, IsPlayer: true}}}
	Resolve(h, c, sa(t, "SP$ DealDamage | NumDmg$ 3"))
	if g.Players[1].Life != 17 {
		t.Fatalf("life = %d, want 17", g.Players[1].Life)
	}
	c.Targets = []state.Target{{Obj: ids["theirBig"]}}
	Resolve(h, c, sa(t, "SP$ DealDamage | NumDmg$ 2"))
	if g.Obj(ids["theirBig"]).Damage != 2 {
		t.Fatalf("damage = %d", g.Obj(ids["theirBig"]).Damage)
	}
}

// TestDealDamageIgnoresObjectsOffTheBattlefield is the enhancement Task 18
// folds into Task 14's stopgap: a permanent that left the battlefield (in
// response, say) is no longer a legal recipient.
func TestDealDamageIgnoresObjectsOffTheBattlefield(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	moveTo(g, ids["myBear"], state.ZGraveyard)
	Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: ids["myBear"]}}},
		sa(t, "SP$ DealDamage | NumDmg$ 3"))
	if g.Obj(ids["myBear"]).Damage != 0 {
		t.Fatal("damage applied to a card in the graveyard")
	}
}

func TestDamageAllHitsMatchingCreaturesOnly(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ DamageAll | NumDmg$ 1"))
	for _, name := range []string{"myBear", "myFlier", "theirBig"} {
		if got := g.Obj(ids[name]).Damage; got != 1 {
			t.Errorf("%s damage = %d, want 1", name, got)
		}
	}
	if g.Obj(ids["myLand"]).Damage != 0 {
		t.Error("DamageAll's default ValidCards$ Creature must not hit a land")
	}
}

// ---------------------------------------------------------------------------
// cardflow.go

func TestDrawPutsTopLibraryCardsInHand(t *testing.T) {
	h := newHost(t, 2)
	bear := mkCard(t, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n")
	ids := fillLibrary(h.g, 0, bear, 3)
	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ Draw | Defined$ You | NumCards$ 2"))
	hand := h.g.Zone(state.ZHand, 0)
	if len(hand) != 2 || hand[0] != ids[0] || hand[1] != ids[1] {
		t.Fatalf("hand = %v, want [%d %d]", hand, ids[0], ids[1])
	}
	if lib := h.g.Zone(state.ZLibrary, 0); len(lib) != 1 || lib[0] != ids[2] {
		t.Fatalf("library = %v, want [%d]", lib, ids[2])
	}
}

func TestDrawFromEmptyLibraryLosesTheGame(t *testing.T) {
	h := newHost(t, 2)
	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ Draw | Defined$ You"))
	if !h.g.Players[0].Lost {
		t.Fatal("drawing from an empty library must lose the game")
	}
}

func TestDiscardMovesFromHandToGraveyard(t *testing.T) {
	h := newHost(t, 2)
	bear := mkCard(t, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n")
	o := h.g.AddObject(bear, 0)
	o.Zone = state.ZHand
	h.g.SetZone(state.ZHand, 0, []state.ObjID{o.ID})
	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ Discard | Defined$ You | NumCards$ 1"))
	if len(h.g.Zone(state.ZHand, 0)) != 0 {
		t.Fatal("card was not discarded from hand")
	}
	if gy := h.g.Zone(state.ZGraveyard, 0); len(gy) != 1 || gy[0] != o.ID {
		t.Fatalf("graveyard = %v, want [%d]", gy, o.ID)
	}
}

func TestMillMovesLibraryTopToGraveyard(t *testing.T) {
	h := newHost(t, 2)
	bear := mkCard(t, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n")
	ids := fillLibrary(h.g, 0, bear, 3)
	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ Mill | Defined$ You | NumCards$ 2"))
	if gy := h.g.Zone(state.ZGraveyard, 0); len(gy) != 2 || gy[0] != ids[0] || gy[1] != ids[1] {
		t.Fatalf("graveyard = %v, want [%d %d]", gy, ids[0], ids[1])
	}
	if lib := h.g.Zone(state.ZLibrary, 0); len(lib) != 1 || lib[0] != ids[2] {
		t.Fatalf("library = %v, want [%d]", lib, ids[2])
	}
}

func TestDigMovesMatchingCardsToDestinationLeavingTheRestOnTop(t *testing.T) {
	h := newHost(t, 2)
	land := mkCard(t, "Name:Isle\nTypes:Basic Land Island\nOracle:x\n")
	bear := mkCard(t, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n")
	g := h.g
	c0 := g.AddObject(bear, 0).ID
	c1 := g.AddObject(land, 0).ID
	c2 := g.AddObject(bear, 0).ID
	g.SetZone(state.ZLibrary, 0, []state.ObjID{c0, c1, c2})

	Resolve(h, &Ctx{Controller: 0}, sa(t,
		"SP$ Dig | Defined$ You | DigNum$ 3 | ChangeNum$ 1 | ChangeValid$ Land | DestinationZone$ Hand"))

	if hand := g.Zone(state.ZHand, 0); len(hand) != 1 || hand[0] != c1 {
		t.Fatalf("hand = %v, want [%d] (the land)", hand, c1)
	}
	if lib := g.Zone(state.ZLibrary, 0); len(lib) != 2 || lib[0] != c0 || lib[1] != c2 {
		t.Fatalf("library = %v, want [%d %d], unchanged relative order", lib, c0, c2)
	}
}

// TestDigsSecretMoveNamesTheLibraryOwner is Task 23 §11's regression test: a
// pre-existing leak. effDig's MoveZone event for a card taken from a hidden
// library was emitted Secret with no Player at all, so under the redaction
// contract (Player names the seat whose secret this is) PlayerID(0) -- a
// real seat, not "nobody" -- ended up seeing every other seat's dug card
// while the actual owner lost their own. Player must be the library's
// owner, the same seat DrawFor (line 48) and rules/engine.go's Shuffle
// already carry.
func TestDigsSecretMoveNamesTheLibraryOwner(t *testing.T) {
	h := newHost(t, 2)
	bear := mkCard(t, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n")
	fillLibrary(h.g, 1, bear, 3)
	Resolve(h, &Ctx{Controller: 1},
		sa(t, "SP$ Dig | Defined$ You | DigNum$ 3 | ChangeNum$ 1 | ChangeValid$ Creature | DestinationZone$ Hand"))

	var found bool
	for _, e := range h.log {
		if e.Kind != events.MoveZone || !e.Secret {
			continue
		}
		found = true
		if e.Player != 1 {
			t.Fatalf("Dig's Secret MoveZone has Player %d, want the library's owner (1)", e.Player)
		}
	}
	if !found {
		t.Fatal("no Secret MoveZone event was emitted")
	}
}

func TestRevealRecordsIdentitiesWithoutMovingCards(t *testing.T) {
	h := newHost(t, 2)
	bear := mkCard(t, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n")
	o := h.g.AddObject(bear, 0)
	o.Zone = state.ZHand
	h.g.SetZone(state.ZHand, 0, []state.ObjID{o.ID})
	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ Reveal | Defined$ You | NumCards$ 1"))
	if hand := h.g.Zone(state.ZHand, 0); len(hand) != 1 || hand[0] != o.ID {
		t.Fatal("Reveal must not move the revealed card")
	}
	var found bool
	for _, e := range h.log {
		if e.Kind == events.Note && !e.Secret && len(e.IDs) == 1 && e.IDs[0] == o.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("no non-secret Note recorded the revealed card's identity")
	}
}

func TestPeekAndRevealLooksAtTheLibrary(t *testing.T) {
	h := newHost(t, 2)
	bear := mkCard(t, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n")
	ids := fillLibrary(h.g, 0, bear, 2)
	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ PeekAndReveal | Defined$ You | NumCards$ 1"))
	if lib := h.g.Zone(state.ZLibrary, 0); len(lib) != 2 {
		t.Fatal("PeekAndReveal must not move library cards")
	}
	var found bool
	for _, e := range h.log {
		if e.Kind == events.Note && len(e.IDs) == 1 && e.IDs[0] == ids[0] {
			found = true
		}
	}
	if !found {
		t.Fatal("no Note recorded the peeked card")
	}
}

func TestRearrangeTopOfLibraryKeepsExistingOrder(t *testing.T) {
	h := newHost(t, 2)
	bear := mkCard(t, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n")
	ids := fillLibrary(h.g, 0, bear, 3)
	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ RearrangeTopOfLibrary | Defined$ You | NumCards$ 3"))
	if lib := h.g.Zone(state.ZLibrary, 0); len(lib) != 3 || lib[0] != ids[0] || lib[1] != ids[1] || lib[2] != ids[2] {
		t.Fatalf("library = %v, want unchanged %v", lib, ids)
	}
}

func TestNameCardNamesTheFirstLibraryCard(t *testing.T) {
	h := newHost(t, 2)
	bear := mkCard(t, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n")
	fillLibrary(h.g, 0, bear, 1)
	Resolve(h, &Ctx{Controller: 0, Source: 1}, sa(t, "SP$ NameCard"))
	var got string
	for _, e := range h.log {
		if e.Kind == events.Note {
			got = e.Text
		}
	}
	if got != "names Bear" {
		t.Fatalf("Note text = %q, want %q", got, "names Bear")
	}
}

// ---------------------------------------------------------------------------
// zone.go

func TestChangeZoneMovesTheTarget(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: ids["myBear"]}}},
		sa(t, "SP$ ChangeZone | Destination$ Hand"))
	if g.Obj(ids["myBear"]).Zone != state.ZHand {
		t.Fatalf("zone = %v, want Hand", g.Obj(ids["myBear"]).Zone)
	}
}

// TestChangeZoneSkipsObjectNoLongerAtOrigin is a CR 608.2b-flavoured guard:
// Origin$ is a precondition, so a target that already left where the effect
// expected it (destroyed in response, say) is skipped rather than moved from
// the wrong place.
func TestChangeZoneSkipsObjectNoLongerAtOrigin(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	moveTo(g, ids["myBear"], state.ZGraveyard)
	Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: ids["myBear"]}}},
		sa(t, "SP$ ChangeZone | Origin$ Battlefield | Destination$ Exile"))
	if g.Obj(ids["myBear"]).Zone != state.ZGraveyard {
		t.Fatalf("zone = %v, want unchanged Graveyard", g.Obj(ids["myBear"]).Zone)
	}
	for _, e := range h.log {
		if e.Kind == events.MoveZone {
			t.Fatal("a move event should not have been emitted at all")
		}
	}
}

func TestChangeZoneAllSweepsMatchingLibraryCards(t *testing.T) {
	h := newHost(t, 2)
	land := mkCard(t, "Name:Isle\nTypes:Basic Land Island\nOracle:x\n")
	bear := mkCard(t, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n")
	g := h.g
	landID := g.AddObject(land, 0).ID
	bearID := g.AddObject(bear, 0).ID
	g.SetZone(state.ZLibrary, 0, []state.ObjID{landID, bearID})

	Resolve(h, &Ctx{Controller: 0}, sa(t,
		"SP$ ChangeZoneAll | Origin$ Library | Destination$ Hand | ChangeType$ Land"))

	if hand := g.Zone(state.ZHand, 0); len(hand) != 1 || hand[0] != landID {
		t.Fatalf("hand = %v, want [%d]", hand, landID)
	}
	if lib := g.Zone(state.ZLibrary, 0); len(lib) != 1 || lib[0] != bearID {
		t.Fatalf("library = %v, want [%d] (bear untouched)", lib, bearID)
	}
}

func TestDestroyMovesToGraveyard(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: ids["myBear"]}}}, sa(t, "SP$ Destroy"))
	if g.Obj(ids["myBear"]).Zone != state.ZGraveyard {
		t.Fatal("destroyed permanent did not move to the graveyard")
	}
}

// TestDestroySkipsIndestructible is the brief's own worked-example test.
func TestDestroySkipsIndestructible(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	o := g.Obj(ids["myBear"])
	o.Card.Faces[0].Keywords = append(o.Card.Faces[0].Keywords, "Indestructible")
	Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: o.ID}}}, sa(t, "SP$ Destroy"))
	if o.Zone != state.ZBattlefield {
		t.Fatal("indestructible permanent was destroyed")
	}
	for _, e := range h.log {
		if e.Kind == events.MoveZone {
			t.Fatal("a move event should not have been emitted at all")
		}
	}
}

func TestDestroyAllSweepsSkippingIndestructible(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	g.Obj(ids["theirBig"]).Card.Faces[0].Keywords = append(g.Obj(ids["theirBig"]).Card.Faces[0].Keywords, "Indestructible")
	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ DestroyAll | ValidCards$ Creature"))
	if g.Obj(ids["myBear"]).Zone != state.ZGraveyard {
		t.Error("myBear should have been destroyed")
	}
	if g.Obj(ids["myFlier"]).Zone != state.ZGraveyard {
		t.Error("myFlier should have been destroyed")
	}
	if g.Obj(ids["theirBig"]).Zone != state.ZBattlefield {
		t.Error("indestructible theirBig should have survived")
	}
	if g.Obj(ids["myLand"]).Zone != state.ZBattlefield {
		t.Error("DestroyAll's Creature filter must not touch the land")
	}
}

func TestSacrificeIgnoresIndestructible(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	o := g.Obj(ids["myBear"])
	o.Card.Faces[0].Keywords = append(o.Card.Faces[0].Keywords, "Indestructible")
	Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: o.ID}}}, sa(t, "SP$ Sacrifice"))
	if o.Zone != state.ZGraveyard {
		t.Fatal("sacrifice must ignore Indestructible")
	}
}

// ---------------------------------------------------------------------------
// life.go

func TestGainLifeIncreasesLife(t *testing.T) {
	h := newHost(t, 2)
	h.g.Players[0].Life = 20
	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ GainLife | Defined$ You | LifeAmount$ 3"))
	if h.g.Players[0].Life != 23 {
		t.Fatalf("life = %d, want 23", h.g.Players[0].Life)
	}
}

func TestLoseLifeDecreasesLife(t *testing.T) {
	h := newHost(t, 2)
	h.g.Players[0].Life = 20
	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ LoseLife | Defined$ You | LifeAmount$ 5"))
	if h.g.Players[0].Life != 15 {
		t.Fatalf("life = %d, want 15", h.g.Players[0].Life)
	}
}

func TestGainLifeClampsNegativeAmountToZero(t *testing.T) {
	h := newHost(t, 2)
	h.g.Players[0].Life = 20
	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ GainLife | Defined$ You | LifeAmount$ -4"))
	if h.g.Players[0].Life != 20 {
		t.Fatalf("life = %d, want unchanged at 20 (negative gain must not lose life)", h.g.Players[0].Life)
	}
}

func TestLoseLifeClampsNegativeAmountToZero(t *testing.T) {
	h := newHost(t, 2)
	h.g.Players[0].Life = 20
	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ LoseLife | Defined$ You | LifeAmount$ -4"))
	if h.g.Players[0].Life != 20 {
		t.Fatalf("life = %d, want unchanged at 20 (negative loss must not gain life)", h.g.Players[0].Life)
	}
}

// ---------------------------------------------------------------------------
// counters.go

func TestPutCounterAddsToTarget(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: ids["myBear"]}}},
		sa(t, "SP$ PutCounter | CounterType$ P1P1 | CounterNum$ 2"))
	if got := g.Obj(ids["myBear"]).Counter("P1P1"); got != 2 {
		t.Fatalf("P1P1 counters = %d, want 2", got)
	}
}

func TestPutCounterDefaultsToOneP1P1(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: ids["myBear"]}}}, sa(t, "SP$ PutCounter"))
	if got := g.Obj(ids["myBear"]).Counter("P1P1"); got != 1 {
		t.Fatalf("P1P1 counters = %d, want 1", got)
	}
}

func TestRemoveCounterAllRemovesFromMatchingPermanentsOnly(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	g.Obj(ids["myBear"]).AddCounter("P1P1", 3)
	g.Obj(ids["myFlier"]).AddCounter("P1P1", 3)
	g.Obj(ids["theirBig"]).AddCounter("P1P1", 3)
	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ RemoveCounterAll | ValidCards$ Creature.YouCtrl | CounterType$ P1P1 | CounterNum$ 1"))
	if got := g.Obj(ids["myBear"]).Counter("P1P1"); got != 2 {
		t.Errorf("myBear P1P1 = %d, want 2", got)
	}
	if got := g.Obj(ids["myFlier"]).Counter("P1P1"); got != 2 {
		t.Errorf("myFlier P1P1 = %d, want 2", got)
	}
	if got := g.Obj(ids["theirBig"]).Counter("P1P1"); got != 3 {
		t.Errorf("theirBig P1P1 = %d, want unchanged at 3", got)
	}
}

func TestRemoveCounterAllHonoursAllCounters(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	g.Obj(ids["myBear"]).AddCounter("P1P1", 5)
	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ RemoveCounterAll | ValidCards$ Creature.YouCtrl | CounterType$ P1P1 | AllCounters$ True"))
	if got := g.Obj(ids["myBear"]).Counter("P1P1"); got != 0 {
		t.Fatalf("P1P1 counters = %d, want 0 (AllCounters$ True removes everything)", got)
	}
}

func TestRegenerateGrantsAShieldCounter(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: ids["myBear"]}}}, sa(t, "AB$ Regenerate"))
	if got := g.Obj(ids["myBear"]).Counter("Shield"); got != 1 {
		t.Fatalf("Shield counters = %d, want 1", got)
	}
}

// ---------------------------------------------------------------------------
// combatfx.go

func TestTapTapsUntappedTargetOnly(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: ids["myBear"]}}}, sa(t, "AB$ Tap"))
	if !g.Obj(ids["myBear"]).Tapped {
		t.Fatal("target was not tapped")
	}
	before := len(h.log)
	Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: ids["myBear"]}}}, sa(t, "AB$ Tap"))
	if len(h.log) != before {
		t.Fatal("tapping an already-tapped permanent should not emit a second Tap event")
	}
}

// TestPumpRegistersLayerContinuousEffectsOnTheTarget is the Task 19c
// regression test: Pump must build real state.ContinuousEffect values and
// hand them to the Host, not just log a Note, so the layer system Task 19
// built actually has a caller. Power/Toughness and a granted keyword go on
// separate layers (7c and 6), so they are two effects, not one.
func TestPumpRegistersLayerContinuousEffectsOnTheTarget(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: ids["myBear"]}}},
		sa(t, "AB$ Pump | NumAtt$ +2 | NumDef$ +1 | KW$ Flying"))
	if len(h.continuous) != 2 {
		t.Fatalf("continuous = %+v, want 2 effects", h.continuous)
	}
	pt := h.continuous[0]
	if pt.Source != ids["myBear"] || pt.Affects != "Card.Self" || pt.Layer != state.LPT ||
		pt.Sub != state.SubModify || pt.AddPower != 2 || pt.AddToughness != 1 || !pt.UntilEOT {
		t.Fatalf("P/T continuous effect = %+v", pt)
	}
	kw := h.continuous[1]
	if kw.Source != ids["myBear"] || kw.Affects != "Card.Self" || kw.Layer != state.LAbilities ||
		len(kw.AddKeywords) != 1 || kw.AddKeywords[0] != "Flying" || !kw.UntilEOT {
		t.Fatalf("keyword continuous effect = %+v", kw)
	}
}

// TestPumpRequiresTheBattlefield mirrors the old Note-based zone guard: a
// target that has left the battlefield gets no continuous effect registered.
func TestPumpRequiresTheBattlefield(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	moveTo(g, ids["myBear"], state.ZGraveyard)
	Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: ids["myBear"]}}},
		sa(t, "AB$ Pump | NumAtt$ +2 | NumDef$ +1"))
	if len(h.continuous) != 0 {
		t.Fatalf("continuous = %+v, want none off the battlefield", h.continuous)
	}
}

// TestPumpAllRegistersOneEffectPerMatchingCreatureDeterministically covers
// both the CR 611.2c requirement that a PumpAll bakes in its affected set at
// resolution time (one continuous effect per matching object, scoped with
// Affects$ "Card.Self", rather than one shared filter effect that would also
// catch creatures entering later) and the no-nondeterminism constraint: the
// order must be the same every run, since it comes from AliveFrom/Zone
// (fixed APNAP + slice order), never a map.
func TestPumpAllRegistersOneEffectPerMatchingCreatureDeterministically(t *testing.T) {
	run := func() []state.ObjID {
		g, _ := board(t)
		h := &fakeHost{g: g}
		Resolve(h, &Ctx{Controller: 0}, sa(t, "DB$ PumpAll | ValidCards$ Creature.YouCtrl | NumAtt$ +1 | NumDef$ +1"))
		var order []state.ObjID
		for _, ce := range h.continuous {
			order = append(order, ce.Source)
		}
		return order
	}
	_, ids := board(t)
	first := run()
	if len(first) != 2 || first[0] != ids["myBear"] || first[1] != ids["myFlier"] {
		t.Fatalf("order = %v, want [myBear myFlier] (registration order)", first)
	}
	for i := 0; i < 10; i++ {
		if got := run(); len(got) != len(first) || got[0] != first[0] || got[1] != first[1] {
			t.Fatalf("run %d: order changed: %v vs %v", i, got, first)
		}
	}
}

// TestPumpAllSkipsNonMatchingCreatures is the filter-scoping half of the old
// test: YouCtrl must exclude the opponent's creature.
func TestPumpAllSkipsNonMatchingCreatures(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	Resolve(h, &Ctx{Controller: 0}, sa(t, "DB$ PumpAll | ValidCards$ Creature.YouCtrl | NumAtt$ +1 | NumDef$ +1"))
	for _, ce := range h.continuous {
		if ce.Source == ids["theirBig"] {
			t.Fatal("PumpAll with YouCtrl must not touch the opponent's creature")
		}
	}
}

// TestAnimateRegistersASetContinuousEffectEvenOffTheBattlefield keeps the
// original off-battlefield coverage (Forge's own Animate targets a graveyard
// card as often as a permanent) while asserting the real layer-7b "setting"
// registration Task 19c adds.
func TestAnimateRegistersASetContinuousEffectEvenOffTheBattlefield(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	moveTo(g, ids["myBear"], state.ZGraveyard)
	Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: ids["myBear"]}}},
		sa(t, "DB$ Animate | Power$ 4 | Toughness$ 4"))
	if len(h.continuous) != 1 {
		t.Fatalf("continuous = %+v, want 1", h.continuous)
	}
	ce := h.continuous[0]
	if ce.Source != ids["myBear"] || ce.Layer != state.LPT || ce.Sub != state.SubSet ||
		!ce.HasSet || ce.SetPower != 4 || ce.SetToughness != 4 || !ce.UntilEOT {
		t.Fatalf("continuous effect = %+v", ce)
	}
}

// TestAnimateWithNoPowerOrToughnessOnlyGrantsTypes covers the real corpus
// shape (e.g. Kitesail Larcenist, Kami of Industry) where DB$ Animate omits
// Power$/Toughness$ entirely to grant only a type or keyword change. Without
// this guard, Num's zero default would register a SubSet effect setting the
// object to 0/0 -- turning a type-granting Animate into an accidental kill
// spell.
func TestAnimateWithNoPowerOrToughnessOnlyGrantsTypes(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: ids["myBear"]}}},
		sa(t, "DB$ Animate | Types$ Artifact Treasure"))
	if len(h.continuous) != 1 {
		t.Fatalf("continuous = %+v, want 1 (types only, no P/T set)", h.continuous)
	}
	ce := h.continuous[0]
	if ce.Layer != state.LType || len(ce.AddTypes) != 2 || ce.AddTypes[0] != "Artifact" || ce.AddTypes[1] != "Treasure" {
		t.Fatalf("continuous effect = %+v", ce)
	}
}

func TestProtectionRequiresTheBattlefield(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: ids["myBear"]}}}, sa(t, "AB$ Protection | Gains$ red"))
	if len(h.continuous) != 1 {
		t.Fatalf("continuous = %+v, want one registration", h.continuous)
	}
	ce := h.continuous[0]
	if ce.Source != ids["myBear"] || ce.Layer != state.LAbilities || len(ce.AddKeywords) != 1 ||
		ce.AddKeywords[0] != "Protection from red" || !ce.UntilEOT {
		t.Fatalf("continuous effect = %+v", ce)
	}
	moveTo(g, ids["myBear"], state.ZGraveyard)
	h.continuous = nil
	Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: ids["myBear"]}}}, sa(t, "AB$ Protection | Gains$ red"))
	if len(h.continuous) != 0 {
		t.Fatalf("continuous = %+v, want none off the battlefield", h.continuous)
	}
}

// TestProtectionChoiceDefaultsDeterministically covers Mother of Runes'
// shape (Gains$ Choice | Choices$ AnyColor): with no chooser mechanism yet
// (a real choice is Task 20's job, the same simplification effCharm and
// effVote already document), it must still resolve to a concrete, fixed
// quality rather than granting a nonsense "Protection from Choice" keyword.
func TestProtectionChoiceDefaultsDeterministically(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: ids["myBear"]}}},
		sa(t, "AB$ Protection | Gains$ Choice | Choices$ AnyColor"))
	if len(h.continuous) != 1 {
		t.Fatalf("continuous = %+v, want one registration", h.continuous)
	}
	if got := h.continuous[0].AddKeywords[0]; got != "Protection from white" {
		t.Fatalf("AddKeywords[0] = %q, want a concrete default colour", got)
	}
}

// ---------------------------------------------------------------------------
// misc.go

func TestEffectRecordsTheIntendedRegistration(t *testing.T) {
	h := newHost(t, 2)
	Resolve(h, &Ctx{Controller: 0, Source: 1}, sa(t, "DB$ Effect | StaticAbilities$ SNoCombatDamage | Duration$ UntilHostLeavesPlayOrEOT"))
	if len(h.log) != 1 || h.log[0].Kind != events.Note {
		t.Fatalf("log = %+v", h.log)
	}
}

func TestCleanupRecordsANote(t *testing.T) {
	h := newHost(t, 2)
	Resolve(h, &Ctx{Controller: 0, Source: 1}, sa(t, "DB$ Cleanup | ClearRemembered$ True"))
	if len(h.log) != 1 || h.log[0].Kind != events.Note {
		t.Fatalf("log = %+v", h.log)
	}
}

func TestSetStateFlipsToTheOtherFace(t *testing.T) {
	h := newHost(t, 2)
	two := twoFacedCard(t)
	o := h.g.AddObject(two, 0)
	o.Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{o.ID})

	Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: o.ID}}}, sa(t, "DB$ SetState | Mode$ Transform"))
	if o.FaceIdx != 1 || o.Face().Name != "Back" {
		t.Fatalf("FaceIdx = %d, name = %q", o.FaceIdx, o.Face().Name)
	}
}

func TestSetStateNoOpsOnASingleFaceCard(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: ids["myBear"]}}}, sa(t, "DB$ SetState | Mode$ Flip"))
	if len(h.log) != 0 {
		t.Fatalf("log = %+v, want no events for a single-face card", h.log)
	}
}

func TestCounterMovesTheTargetedSpellToItsOwnersGraveyard(t *testing.T) {
	h := newHost(t, 2)
	bolt := mkCard(t, "Name:Bolt\nTypes:Instant\nOracle:x\n")
	o := h.g.AddObject(bolt, 1)
	o.Zone = state.ZStack
	o.Controller = 1
	h.g.Stack = []state.ObjID{o.ID}
	Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: o.ID}}}, sa(t, "SP$ Counter"))
	if o.Zone != state.ZGraveyard {
		t.Fatalf("zone = %v, want Graveyard", o.Zone)
	}
	if gy := h.g.Zone(state.ZGraveyard, 1); len(gy) != 1 || gy[0] != o.ID {
		t.Fatalf("owner's graveyard = %v, want [%d]", gy, o.ID)
	}
}

// TestCounterIgnoresATargetNoLongerOnTheStack is CR 608.2b's canonical case
// for this primitive: the targeted spell already resolved (or was itself
// countered) before this Counter got to resolve.
func TestCounterIgnoresATargetNoLongerOnTheStack(t *testing.T) {
	h := newHost(t, 2)
	bolt := mkCard(t, "Name:Bolt\nTypes:Instant\nOracle:x\n")
	o := h.g.AddObject(bolt, 1)
	o.Zone = state.ZGraveyard
	o.Owner, o.Controller = 1, 1
	h.g.SetZone(state.ZGraveyard, 1, []state.ObjID{o.ID})
	Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: o.ID}}}, sa(t, "SP$ Counter"))
	if len(h.log) != 0 {
		t.Fatalf("log = %+v, want no events for a target already off the stack", h.log)
	}
}

func TestDelayedTriggerRecordsANote(t *testing.T) {
	h := newHost(t, 2)
	Resolve(h, &Ctx{Controller: 0, Source: 1}, sa(t, "DB$ DelayedTrigger | Mode$ Phase | Phase$ End of Turn | Execute$ X"))
	if len(h.log) != 1 || h.log[0].Kind != events.Note {
		t.Fatalf("log = %+v", h.log)
	}
}

func TestRepeatRunsTheSubAbilityRepeatNumTimes(t *testing.T) {
	h := newHost(t, 2)
	h.g.Players[0].Life = 10
	c := &Ctx{Controller: 0, SVars: map[string]string{
		"Boost": "DB$ GainLife | Defined$ You | LifeAmount$ 1",
	}}
	Resolve(h, c, sa(t, "SP$ Repeat | RepeatSubAbility$ Boost | RepeatNum$ 3"))
	if h.g.Players[0].Life != 13 {
		t.Fatalf("life = %d, want 13 (three runs of +1)", h.g.Players[0].Life)
	}
}

func TestRepeatDoesNothingWithoutSVars(t *testing.T) {
	h := newHost(t, 2)
	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ Repeat | RepeatSubAbility$ Boost | RepeatNum$ 3"))
	if len(h.log) != 0 {
		t.Fatalf("log = %+v, want no events when no SVars are bound", h.log)
	}
}

func TestCharmRunsOnlyTheFirstChoice(t *testing.T) {
	h := newHost(t, 2)
	h.g.Players[0].Life = 10
	c := &Ctx{Controller: 0, SVars: map[string]string{
		"DoGain": "DB$ GainLife | Defined$ You | LifeAmount$ 5",
		"DoLose": "DB$ LoseLife | Defined$ You | LifeAmount$ 5",
	}}
	Resolve(h, c, sa(t, "SP$ Charm | Choices$ DoGain,DoLose"))
	if h.g.Players[0].Life != 15 {
		t.Fatalf("life = %d, want 15 (only the first choice runs)", h.g.Players[0].Life)
	}
}

func TestVoteRecordsANotePerVotingPlayer(t *testing.T) {
	h := newHost(t, 2)
	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ Vote | Defined$ Player | Choices$ Sickness,Psychosis"))
	if len(h.log) != 2 {
		t.Fatalf("log = %+v, want one Note per player", h.log)
	}
	for _, e := range h.log {
		if e.Kind != events.Note || e.Text != "votes for Sickness" {
			t.Fatalf("event = %+v", e)
		}
	}
}

func TestBecomeMonarchRecordsTheTargetPlayer(t *testing.T) {
	h := newHost(t, 2)
	Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Player: 1, IsPlayer: true}}}, sa(t, "AB$ BecomeMonarch"))
	if len(h.log) != 1 || h.log[0].Kind != events.Note || h.log[0].Player != 1 {
		t.Fatalf("log = %+v", h.log)
	}
}

// TestRestartGameEndsTheGame is the fix-round-2 regression test for the
// re-review's N3: effRestartGame's own comment and Text both say the game
// ends as a draw, but the GameOver event it emitted left Amount at its zero
// value, which events.Apply's GameOver case (Task 22 fix round 1) reads as
// "Amount 0: a win" -- and Player, also left at zero, validates as seat 0.
// So despite every piece of this primitive's own documentation, it was
// actually encoding a seat-0 win. Asserting Draw (not just Over) is what
// catches that; asserting Over alone, as this test used to, cannot tell a
// win from a draw at all.
func TestRestartGameEndsTheGame(t *testing.T) {
	h := newHost(t, 2)
	Resolve(h, &Ctx{Controller: 0}, sa(t, "AB$ RestartGame"))
	if !h.g.Over {
		t.Fatal("RestartGame must end the game")
	}
	if !h.g.Draw {
		t.Fatal("RestartGame ends the game as a draw (its own Text says so), not a win for seat 0")
	}
}

func TestMana(t *testing.T) {
	h := newHost(t, 2)
	Resolve(h, &Ctx{Controller: 0}, sa(t, "AB$ Mana | Cost$ T | Produced$ R | Amount$ 2"))
	if h.g.Players[0].Pool[state.MR] != 2 {
		t.Fatalf("pool = %v, want 2 red", h.g.Players[0].Pool)
	}
}

// TestTokenAndCopySpellAbilityAreNotYetRegistered documents a deliberate gap:
// both need to mint a brand-new game object mid-match, which nothing in this
// engine can do without writing to state.Game outside events.Apply. See the
// Task 18 report for why, and for the new-event-kind design that would close
// it. Resolve's existing "unimplemented API" fallback is exercised here, not
// bypassed.
func TestTokenAndCopySpellAbilityAreNotYetRegistered(t *testing.T) {
	sup := Supported()
	for _, api := range []string{"Token", "CopySpellAbility"} {
		if sup["api:"+api] {
			t.Fatalf("api:%s unexpectedly registered", api)
		}
	}
	h := newHost(t, 2)
	Resolve(h, &Ctx{Controller: 0, Source: 1}, sa(t, "DB$ Token | TokenAmount$ 1 | TokenScript$ r_1_1_goblin"))
	if len(h.log) != 1 || h.log[0].Kind != events.Note || h.log[0].Text != "unimplemented API Token" {
		t.Fatalf("log = %+v", h.log)
	}
}

// TestCardflowAPIsGuardOutOfRangePlayerID is Ruling T18-a's regression test.
// Every one of cardflow.go's nine registered APIs eventually reads a zone via
// PlayerOf's result (a target's raw Player field) or, for NameCard,
// Ctx.Controller directly. Neither is validated before this task's fix, and
// Game.Zone computes int(p)*numZones+int(z) with no bounds check of its own
// before indexing a fixed-size slice -- an out-of-range PlayerID reached that
// arithmetic and panicked. Each API is exercised two ways: an out-of-range
// target Player (PlayerOf returns t.Player unchecked), and an out-of-range
// Ctx.Controller reaching the same code path via "Defined$ You" (NameCard
// reads Ctx.Controller directly regardless of Defined$, so it skips that
// suffix). Both must be a total no-op: no panic, and the game state
// afterwards is reflect.DeepEqual to a clone taken beforehand.
func TestCardflowAPIsGuardOutOfRangePlayerID(t *testing.T) {
	apis := []string{"Draw", "Discard", "Mill", "Dig",
		"Reveal", "RevealHand", "PeekAndReveal", "RearrangeTopOfLibrary", "NameCard"}

	run := func(t *testing.T, line string, c *Ctx) {
		t.Helper()
		g, _ := board(t)
		h := &fakeHost{g: g}
		before := g.Clone()
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panicked resolving %q: %v", line, r)
				}
			}()
			Resolve(h, c, sa(t, line))
		}()
		if !reflect.DeepEqual(before, g) {
			t.Fatalf("state changed resolving %q with an out-of-range PlayerID", line)
		}
	}

	for _, api := range apis {
		api := api
		t.Run(api+"/target_player_out_of_range", func(t *testing.T) {
			run(t, "SP$ "+api, &Ctx{Controller: 0,
				Targets: []state.Target{{Player: 250, IsPlayer: true}}})
		})
		t.Run(api+"/controller_out_of_range", func(t *testing.T) {
			line := "SP$ " + api
			if api != "NameCard" {
				line += " | Defined$ You"
			}
			run(t, line, &Ctx{Controller: 250})
		})
	}
}
