package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The Commander count forms behind Battle Angels of Tyr, Iron Mastiff, Jace's
// Archivist and Windfall. Each test evaluates the REAL corpus SVar text (never
// a hand-copied body) against a fixture built so the compared values differ.

// putTestCard adds a fresh object owned by owner and places it in p's zone
// (p differs from owner only for the "someone else's card in this zone"
// control). It returns the object ID so the caller can stick it on a
// Remembered list.
func putTestCard(t *testing.T, g *state.Game, owner, p state.PlayerID, zone state.Zone, src string) state.ObjID {
	t.Helper()
	o := g.AddObject(mkCard(t, src), owner)
	o.Zone = zone
	g.SetZone(zone, p, append(g.Zone(zone, p), o.ID))
	return o.ID
}

// moveTestCard moves an object already listed in from to to, keeping the
// object's Zone field in step with the zone lists.
func moveTestCard(t *testing.T, g *state.Game, id state.ObjID, from, to state.Zone, p state.PlayerID) {
	t.Helper()
	o := g.Obj(id)
	if o == nil || o.Zone != from {
		t.Fatalf("precondition: object %d is not in zone %v (got %+v)", id, from, o)
	}
	var kept []state.ObjID
	for _, x := range g.Zone(from, p) {
		if x != id {
			kept = append(kept, x)
		}
	}
	g.SetZone(from, p, kept)
	g.SetZone(to, p, append(g.Zone(to, p), id))
	o.Zone = to
}

// TestPlayerCountMultiZoneExtremeWindfall pins the multi-zone HighestValid
// head (real Windfall SVar X): the extreme is the SUM over Graveyard, Library
// and Exile of the remembered cards a player owns. Seat 0 has 8 remembered
// cards across the three zones; seat 1's seven remembered cards split six in
// its own graveyard and one sitting in seat 0's graveyard, so the single-zone
// control is 6 and the multi-zone sum is 8. A non-remembered graveyard card
// and the seat-1-owned card in seat 0's graveyard are the negative controls.
func TestPlayerCountMultiZoneExtremeWindfall(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	wf, ok := reg.Lookup("Windfall")
	if !ok || wf == nil {
		t.Fatal("corpus fixture: Windfall missing")
	}
	face := wf.Faces[0]
	body := face.SVars["X"]
	if !strings.Contains(body, "HighestValidGraveyard,Library,Exile") ||
		!strings.Contains(body, "Card.IsRemembered+YouOwn") {
		t.Fatalf("precondition: Windfall X = %q, want the multi-zone remembered count", body)
	}

	h := newHost(t, 3)
	g := h.g
	var remembered []state.Target
	remember := func(id state.ObjID) state.ObjID {
		remembered = append(remembered, state.Target{Obj: id})
		return id
	}
	card := "Name:Memory Card\nTypes:Instant\nOracle:x\n"

	// Seat 0: 6 remembered in the graveyard, 1 in the library, 1 in exile.
	for i := 0; i < 6; i++ {
		remember(putTestCard(t, g, 0, 0, state.ZGraveyard, card))
	}
	remember(putTestCard(t, g, 0, 0, state.ZLibrary, card))
	remember(putTestCard(t, g, 0, 0, state.ZExile, card))
	// Seat 1: 6 remembered in its own graveyard plus 1 remembered card owned
	// by seat 1 sitting in seat 0's graveyard (the seventh remembered card).
	for i := 0; i < 6; i++ {
		remember(putTestCard(t, g, 1, 1, state.ZGraveyard, card))
	}
	remember(putTestCard(t, g, 1, 0, state.ZGraveyard, card))
	// Negative control: a NON-remembered card in seat 1's graveyard.
	putTestCard(t, g, 1, 1, state.ZGraveyard, card)

	// Precondition: both players' raw graveyards hold 7 objects, so only the
	// remembered+owned read can tell them apart.
	if got0, got1 := len(g.Zone(state.ZGraveyard, 0)), len(g.Zone(state.ZGraveyard, 1)); got0 != 7 || got1 != 7 {
		t.Fatalf("precondition: graveyard sizes = %d/%d, want 7/7", got0, got1)
	}

	c := &Ctx{Source: 0, Controller: 0, SVars: face.SVars, Remembered: remembered}
	got, ok := EvalCountOK(h, c, body)
	if !ok || got != 8 {
		t.Fatalf("Windfall X %q = (%d,%v), want (8,true)", body, got, ok)
	}
	// Single-zone control on the SAME board: the graveyard alone is 6.
	single := strings.Replace(body, "ValidGraveyard,Library,Exile", "ValidGraveyard", 1)
	if single == body {
		t.Fatalf("precondition: single-zone body did not replace the multi-zone head: %q", body)
	}
	if got, ok := EvalCountOK(h, c, single); !ok || got != 6 {
		t.Fatalf("single-zone %q = (%d,%v), want (6,true)", single, got, ok)
	}
	// An unknown zone word stays unresolvable.
	unknown := strings.Replace(body, "ValidGraveyard,Library,Exile", "ValidGraveyard,Nowhere", 1)
	if got, ok := EvalCountOK(h, c, unknown); ok || got != 0 {
		t.Fatalf("unknown-zone %q = (%d,%v), want (0,false)", unknown, got, ok)
	}
}

// TestPlayerCountNonTriggeredTargetBattleAngels pins the three "more than
// each other player" conditions and the two read heads from Battle Angels of
// Tyr's real SVars. The group is every living player except the triggered
// target, so a tie with the target is not a win, and the target's own cards
// never inflate the comparison.
func TestPlayerCountNonTriggeredTargetBattleAngels(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	ba, ok := reg.Lookup("Battle Angels of Tyr")
	if !ok || ba == nil {
		t.Fatal("corpus fixture: Battle Angels of Tyr missing")
	}
	face := ba.Faces[0]
	for _, name := range []string{"MostInHand", "MostLands", "MostLife", "NumInHand", "NumLands", "NumLife"} {
		if face.SVars[name] == "" {
			t.Fatalf("precondition: Battle Angels of Tyr has no %s SVar", name)
		}
	}

	h := newHost(t, 3)
	g := h.g
	src := putTestCard(t, g, 0, 0, state.ZBattlefield, "Name:Battle Angels of Tyr\nTypes:Creature Angel\nPT:4/4\nOracle:x\n")
	c := &Ctx{Source: src, Controller: 0, SVars: face.SVars}
	c.TriggerTarget = state.Target{Player: 1, IsPlayer: true}

	card := "Name:Filler\nTypes:Sorcery\nOracle:x\n"
	land := "Name:Plains\nTypes:Land\nOracle:x\n"
	hand := func(p state.PlayerID, n int) []state.ObjID {
		var ids []state.ObjID
		for i := 0; i < n; i++ {
			ids = append(ids, putTestCard(t, g, p, p, state.ZHand, card))
		}
		return ids
	}
	battle := func(p state.PlayerID, src string, n int) []state.ObjID {
		var ids []state.ObjID
		for i := 0; i < n; i++ {
			ids = append(ids, putTestCard(t, g, p, p, state.ZBattlefield, src))
		}
		return ids
	}

	// Hand: seat 1 has 5, seats 0 and 2 have 3 and 5 -- a tie, so not "more
	// than each other player".
	hand(0, 3)
	hand(1, 5)
	seat2Hand := hand(2, 5)
	if got, ok := EvalCountOK(h, c, face.SVars["NumInHand"]); !ok || got != 5 {
		t.Fatalf("NumInHand = (%d,%v), want (5,true)", got, ok)
	}
	if got, ok := EvalCountOK(h, c, face.SVars["MostInHand"]); !ok || got != 5 {
		t.Fatalf("MostInHand with a tie = (%d,%v), want (5,true)", got, ok)
	}
	moveTestCard(t, g, seat2Hand[len(seat2Hand)-1], state.ZHand, state.ZGraveyard, 2)
	if got, ok := EvalCountOK(h, c, face.SVars["MostInHand"]); !ok || got != 4 {
		t.Fatalf("MostInHand after seat 2 drops to 4 = (%d,%v), want (4,true)", got, ok)
	}

	// Lands: seat 1 has 3, seats 0 and 2 have 2 and 3 -- tie, then seat 2
	// drops to 2 and the target is strictly ahead.
	battle(0, land, 2)
	battle(1, land, 3)
	seat2Lands := battle(2, land, 3)
	if got, ok := EvalCountOK(h, c, face.SVars["NumLands"]); !ok || got != 3 {
		t.Fatalf("NumLands = (%d,%v), want (3,true)", got, ok)
	}
	if got, ok := EvalCountOK(h, c, face.SVars["MostLands"]); !ok || got != 3 {
		t.Fatalf("MostLands with a tie = (%d,%v), want (3,true)", got, ok)
	}
	moveTestCard(t, g, seat2Lands[len(seat2Lands)-1], state.ZBattlefield, state.ZGraveyard, 2)
	if got, ok := EvalCountOK(h, c, face.SVars["MostLands"]); !ok || got != 2 {
		t.Fatalf("MostLands after seat 2 drops to 2 = (%d,%v), want (2,true)", got, ok)
	}

	// Life: seat 1 at 25 against 20/20 is strictly ahead; then 20 against
	// 20/22 is not.
	g.Players[0].Life, g.Players[1].Life, g.Players[2].Life = 20, 25, 20
	if got, ok := EvalCountOK(h, c, face.SVars["NumLife"]); !ok || got != 25 {
		t.Fatalf("NumLife = (%d,%v), want (25,true)", got, ok)
	}
	if got, ok := EvalCountOK(h, c, face.SVars["MostLife"]); !ok || got != 20 {
		t.Fatalf("MostLife at 25 vs 20/20 = (%d,%v), want (20,true)", got, ok)
	}
	g.Players[1].Life, g.Players[2].Life = 20, 22
	if got, ok := EvalCountOK(h, c, face.SVars["MostLife"]); !ok || got != 22 {
		t.Fatalf("MostLife at 20 vs 20/22 = (%d,%v), want (22,true)", got, ok)
	}

	// The group excludes the triggered player: give seat 1 nine cards, but
	// the other seats hold 3 and 1, so the Highest is 3.
	hand(1, 4)
	for _, id := range seat2Hand[:3] {
		moveTestCard(t, g, id, state.ZHand, state.ZGraveyard, 2)
	}
	if got, ok := EvalCountOK(h, c, face.SVars["MostInHand"]); !ok || got != 3 {
		t.Fatalf("MostInHand with the triggered player holding 9 = (%d,%v), want (3,true): the group must exclude seat 1", got, ok)
	}

	// With no triggered player in the Ctx there is nothing to exclude: every
	// living player is in the group, and the head still resolves.
	bare := &Ctx{Source: src, Controller: 0, SVars: face.SVars}
	g.Players[0].Life, g.Players[1].Life, g.Players[2].Life = 17, 19, 23
	if got, ok := EvalCountOK(h, bare, face.SVars["MostLife"]); !ok || got != 23 {
		t.Fatalf("bare-Ctx MostLife = (%d,%v), want (23,true) over every living player", got, ok)
	}
}

// TestTriggeredTargetRefReadsZoneHeads pins the widened default-ref arm:
// TriggeredTarget reads CardsInHand and the Valid-family zone counts, but the
// widening is confined to that one ref (TriggeredDefendingPlayer still fails
// closed for CardsInHand).
func TestTriggeredTargetRefReadsZoneHeads(t *testing.T) {
	h := newHost(t, 3)
	g := h.g
	handCard := "Name:Hand Card\nTypes:Sorcery\nOracle:x\n"
	land := "Name:Island\nTypes:Land\nOracle:x\n"
	filler := "Name:Relic\nTypes:Artifact\nOracle:x\n"
	for i := 0; i < 4; i++ {
		putTestCard(t, g, 1, 1, state.ZHand, handCard)
	}
	// Seat 1 controls 3 lands and 1 non-Demon artifact; seat 0 controls 5
	// lands, so the read must be seat 1's side.
	for i := 0; i < 3; i++ {
		putTestCard(t, g, 1, 1, state.ZBattlefield, land)
	}
	putTestCard(t, g, 1, 1, state.ZBattlefield, filler)
	for i := 0; i < 5; i++ {
		putTestCard(t, g, 0, 0, state.ZBattlefield, land)
	}

	c := &Ctx{Controller: 0}
	c.TriggerTarget = state.Target{Player: 1, IsPlayer: true}
	if got, ok := EvalCountOK(h, c, "TriggeredTarget$CardsInHand"); !ok || got != 4 {
		t.Fatalf("TriggeredTarget$CardsInHand = (%d,%v), want (4,true)", got, ok)
	}
	if got, ok := EvalCountOK(h, c, "TriggeredTarget$Valid Land.YouCtrl"); !ok || got != 3 {
		t.Fatalf("TriggeredTarget$Valid Land.YouCtrl = (%d,%v), want (3,true)", got, ok)
	}
	// Permanent.nonDemon+YouCtrl counts seat 1's four permanents, then
	// HalfUp halves that to 2.
	if got, ok := EvalCountOK(h, c, "TriggeredTarget$Valid Permanent.nonDemon+YouCtrl/HalfUp"); !ok || got != 2 {
		t.Fatalf("TriggeredTarget$Valid Permanent.nonDemon+YouCtrl/HalfUp = (%d,%v), want (2,true)", got, ok)
	}

	// Confinement: a different default ref keeps CardsInHand unresolvable.
	c.DefendingPlayer = state.Target{Player: 1, IsPlayer: true}
	if got, ok := EvalCountOK(h, c, "TriggeredDefendingPlayer$CardsInHand"); ok || got != 0 {
		t.Fatalf("TriggeredDefendingPlayer$CardsInHand = (%d,%v), want (0,false): the widening is TriggeredTarget-only", got, ok)
	}
}

// TestHasPropertyDefendingCountsAttackedPlayers pins HasPropertyDefending:
// the count is players being attacked (one per member, however many creatures
// attack them), read from live combat state, and a readable 0 outside combat.
func TestHasPropertyDefendingCountsAttackedPlayers(t *testing.T) {
	h := newHost(t, 4)
	g := h.g
	attacker := "Name:Attacker\nTypes:Creature\nPT:1/1\nOracle:x\n"
	attack := func(defending state.PlayerID) state.ObjID {
		id := putTestCard(t, g, 0, 0, state.ZBattlefield, attacker)
		o := g.Obj(id)
		o.IsAttacking = true
		o.Attacking = defending
		return id
	}
	twoOnSeat1 := []state.ObjID{attack(1), attack(1)}
	oneOnSeat2 := []state.ObjID{attack(2)}

	if got, ok := EvalCountOK(h, &Ctx{Controller: 0}, "PlayerCountOpponents$HasPropertyDefending"); !ok || got != 2 {
		t.Fatalf("PlayerCountOpponents$HasPropertyDefending = (%d,%v), want (2,true): seats 1 and 2 are attacked, seat 3 is not", got, ok)
	}
	if got, ok := EvalCountOK(h, &Ctx{Controller: 0}, "PlayerCountPlayers$HasPropertyDefending"); !ok || got != 2 {
		t.Fatalf("PlayerCountPlayers$HasPropertyDefending = (%d,%v), want (2,true)", got, ok)
	}
	// Precondition: two creatures attack seat 1, so a per-creature count
	// would be 3. The per-member read is 2.
	if len(twoOnSeat1)+len(oneOnSeat2) != 3 {
		t.Fatalf("precondition: attacking creatures = %d, want 3", len(twoOnSeat1)+len(oneOnSeat2))
	}

	for _, id := range append(append([]state.ObjID{}, twoOnSeat1...), oneOnSeat2...) {
		g.Obj(id).IsAttacking = false
	}
	if got, ok := EvalCountOK(h, &Ctx{Controller: 0}, "PlayerCountOpponents$HasPropertyDefending"); !ok || got != 0 {
		t.Fatalf("outside combat PlayerCountOpponents$HasPropertyDefending = (%d,%v), want (0,true)", got, ok)
	}
}

// TestIronMastiffRollsOneDicePerAttackedPlayer is the sibling of
// TestIronMastiffUsesOnlyItsHighestRoll: it does NOT overwrite
// NbAttackedPlayers, so the real corpus SVar
// (PlayerCountOpponents$HasPropertyDefending) sizes the roll. Two players
// being attacked roll two d20s; one rolls one.
func TestIronMastiffRollsOneDicePerAttackedPlayer(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	im, ok := reg.Lookup("Iron Mastiff")
	if !ok || im == nil {
		t.Fatal("corpus fixture: Iron Mastiff missing")
	}
	face := im.Faces[0]
	if got := face.SVars["NbAttackedPlayers"]; got != "PlayerCountOpponents$HasPropertyDefending" {
		t.Fatalf("precondition: NbAttackedPlayers = %q, want the real defending-players SVar", got)
	}
	saRoll := cards.ResolveSVar(face.SVars, "DBTrigRollDice")
	if saRoll == nil || saRoll.ParamStr(cards.PKAmount) != "NbAttackedPlayers" {
		t.Fatalf("precondition: DBTrigRollDice = %+v, want Amount$ NbAttackedPlayers", saRoll)
	}

	roll := func(attacked []state.PlayerID, seq []int) ([]int32, *scriptedRollHost) {
		g := state.NewGame(names(3))
		h := &scriptedRollHost{seq: seq}
		h.g = g
		src := g.AddObject(im, 0)
		src.Zone = state.ZBattlefield
		for _, defending := range attacked {
			o := g.AddObject(mkCard(t, "Name:Attacker\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
			o.Zone = state.ZBattlefield
			o.IsAttacking = true
			o.Attacking = defending
			g.SetZone(state.ZBattlefield, 0, append(g.Zone(state.ZBattlefield, 0), o.ID))
		}
		Resolve(h, &Ctx{Controller: 0, Source: src.ID, SVars: face.SVars}, saRoll)
		return rollResults(t, h), h
	}

	both, _ := roll([]state.PlayerID{1, 2}, []int{19, 0})
	one, _ := roll([]state.PlayerID{1}, []int{19})
	if len(both) != 2 || both[0] != 20 || both[1] != 1 {
		t.Fatalf("two attacked players rolled %v, want [20 1]", both)
	}
	if len(one) != 1 || one[0] != 20 {
		t.Fatalf("one attacked player rolled %v, want [20]", one)
	}
	if len(both) == len(one) {
		t.Fatalf("precondition: the two scenarios must roll different dice counts, both = %d", len(both))
	}
}
