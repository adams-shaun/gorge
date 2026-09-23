package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// pcsZoneObjs adds n copies of the card text to seat p's zone, returning the
// ids. The objects' Zone field is set alongside the zone membership, the same
// two writes the engine's own MoveZone fold performs.
func pcsZoneObjs(t *testing.T, h *fakeHost, p state.PlayerID, n int, src string, zone state.Zone) []state.ObjID {
	t.Helper()
	out := make([]state.ObjID, 0, n)
	for i := 0; i < n; i++ {
		o := h.g.AddObject(mkCard(t, src), p)
		h.g.SetZone(zone, p, append(h.g.Zone(zone, p), o.ID))
		o.Zone = zone
		out = append(out, o.ID)
	}
	return out
}

// pcsMoveZone moves every object of seat p's `from` zone into `to` — both
// the zone membership and the objects' Zone field, the two writes the
// engine's own MoveZone fold performs.
func pcsMoveZone(h *fakeHost, p state.PlayerID, from, to state.Zone) {
	ids := h.g.Zone(from, p)
	for _, id := range ids {
		if o := h.g.Obj(id); o != nil {
			o.Zone = to
		}
	}
	h.g.SetZone(from, p, nil)
	h.g.SetZone(to, p, append(h.g.Zone(to, p), ids...))
}

// pcsTmpl expands a "...HasProperty%s" template with the property name.
func pcsTmpl(tmpl, prop string) string { return strings.ReplaceAll(tmpl, "%s", prop) }

const pcsMountain = "Name:Mountain\nTypes:Land\nOracle:x\n"

// TestPlayerCountHasPropertyStateBacked pins the four state-backed
// HasProperty heads on the three plain living groups (Players$, Opponents$,
// RegisteredOpponents$): each family must read a nonzero count only when the
// member-level state differs from its baseline, an Opponents/Registered-
// Opponents head must never count a qualifying CONTROLLER, and an unread or
// malformed property or card spec must stay (0, false) — fail closed, never
// a fabricated zero.
func TestPlayerCountHasPropertyStateBacked(t *testing.T) {
	h, c := fixtureHost(t)
	opponents := []string{
		"Count$PlayerCountOpponents$HasProperty%s",
		"Count$PlayerCountRegisteredOpponents$HasProperty%s",
	}

	// ---- isMonarch ------------------------------------------------------
	// Precondition: no monarch anywhere — every group reads an evaluated 0.
	for _, expr := range []string{
		"Count$PlayerCountPlayers$HasPropertyisMonarch",
		"Count$PlayerCountOpponents$HasPropertyisMonarch",
		"Count$PlayerCountRegisteredOpponents$HasPropertyisMonarch",
	} {
		if got, ok := EvalCountOK(h, c, expr); !ok || got != 0 {
			t.Fatalf("precondition %s = (%d, %v), want evaluated 0 with no monarch", expr, got, ok)
		}
	}
	// The controller takes the crown: the monarch IS a member of Players$
	// but must not be counted by the opponent heads — player scoping.
	h.g.HasMonarch, h.g.Monarch = true, 0
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountPlayers$HasPropertyisMonarch"); !ok || got != 1 {
		t.Fatalf("controller-monarch Players head = (%d, %v), want 1", got, ok)
	}
	for _, tmpl := range opponents {
		if got, ok := EvalCountOK(h, c, pcsTmpl(tmpl, "isMonarch")); !ok || got != 0 {
			t.Fatalf("controller-monarch %s = (%d, %v), want 0: a qualifying controller is not an opponent", pcsTmpl(tmpl, "isMonarch"), got, ok)
		}
	}
	// The crown moves to the opponent: the opponent heads flip 0 -> 1.
	h.g.Monarch = 1
	for _, tmpl := range opponents {
		if got, ok := EvalCountOK(h, c, pcsTmpl(tmpl, "isMonarch")); !ok || got != 1 {
			t.Fatalf("opponent-monarch %s = (%d, %v), want 1", pcsTmpl(tmpl, "isMonarch"), got, ok)
		}
	}

	// ---- HasCardsInHand_<spec>_<cmp> ------------------------------------
	// 3 cards in the opponent's hand: below the GE4 boundary, evaluated 0.
	pcsZoneObjs(t, h, 1, 3, pcsMountain, state.ZHand)
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountOpponents$HasPropertyHasCardsInHand_Card_GE4"); !ok || got != 0 {
		t.Fatalf("3-card hand = (%d, %v), want evaluated 0 below the GE4 boundary", got, ok)
	}
	// The fourth card crosses the boundary: 0 -> 1.
	pcsZoneObjs(t, h, 1, 1, pcsMountain, state.ZHand)
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountOpponents$HasPropertyHasCardsInHand_Card_GE4"); !ok || got != 1 {
		t.Fatalf("4-card hand = (%d, %v), want 1", got, ok)
	}
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountRegisteredOpponents$HasPropertyHasCardsInHand_Card_GE4"); !ok || got != 1 {
		t.Fatalf("registered 4-card hand = (%d, %v), want 1", got, ok)
	}
	// The controller's own qualifying hand must not add a member on the
	// opponent heads (scoping), only on Players$.
	pcsZoneObjs(t, h, 0, 4, pcsMountain, state.ZHand)
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountPlayers$HasPropertyHasCardsInHand_Card_GE4"); !ok || got != 2 {
		t.Fatalf("both hands qualifying: Players head = (%d, %v), want 2", got, ok)
	}
	for _, tmpl := range opponents {
		if got, ok := EvalCountOK(h, c, pcsTmpl(tmpl, "HasCardsInHand_Card_GE4")); !ok || got != 1 {
			t.Fatalf("both hands qualifying: %s = (%d, %v), want 1 (controller not counted)", pcsTmpl(tmpl, "HasCardsInHand_Card_GE4"), got, ok)
		}
	}
	// Wrong TYPE: the qualifying cards are Mountains; a Creature spec must
	// not count them.
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountOpponents$HasPropertyHasCardsInHand_Creature_GE1"); !ok || got != 0 {
		t.Fatalf("Creature spec over a Mountain hand = (%d, %v), want evaluated 0", got, ok)
	}
	// Wrong ZONE: the opponent's four cards move to the graveyard — the hand
	// head drops back to 0.
	pcsMoveZone(h, 1, state.ZHand, state.ZGraveyard)
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountOpponents$HasPropertyHasCardsInHand_Card_GE4"); !ok || got != 0 {
		t.Fatalf("hand moved to graveyard: hand head = (%d, %v), want 0", got, ok)
	}

	// ---- HasCardsInGraveyard_<spec>_<cmp> --------------------------------
	// The four mountains ARE now in the opponent's graveyard: the graveyard
	// head reads them (zero -> nonzero as the state changed).
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountOpponents$HasPropertyHasCardsInGraveyard_Card_GE4"); !ok || got != 1 {
		t.Fatalf("4-card graveyard = (%d, %v), want 1", got, ok)
	}
	// A card spec the zone does not satisfy stays 0: no Instant or Sorcery
	// is in the graveyard yet (the comma-alternative Mysterious Stranger
	// spelling).
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountPlayers$HasPropertyHasCardsInGraveyard_Instant,Sorcery_GE1"); !ok || got != 0 {
		t.Fatalf("land-only graveyard, Instant,Sorcery spec = (%d, %v), want evaluated 0", got, ok)
	}
	pcsZoneObjs(t, h, 0, 1, "Name:Shock\nTypes:Instant\nOracle:x\n", state.ZGraveyard)
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountPlayers$HasPropertyHasCardsInGraveyard_Instant,Sorcery_GE1"); !ok || got != 1 {
		t.Fatalf("graveyard with an Instant = (%d, %v), want 1", got, ok)
	}
	// The /Times.2 count suffix (Master's Councillors' GE7/Times.2) applies
	// AFTER counting qualifying players: six more cards bring the
	// controller's graveyard to exactly 7 — the boundary — and the head
	// doubles.
	pcsZoneObjs(t, h, 0, 6, pcsMountain, state.ZGraveyard)
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountPlayers$HasPropertyHasCardsInGraveyard_Card_GE7"); !ok || got != 1 {
		t.Fatalf("7-card graveyard GE7 = (%d, %v), want 1 (exactly at the boundary)", got, ok)
	}
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountPlayers$HasPropertyHasCardsInGraveyard_Card_GE7/Times.2"); !ok || got != 2 {
		t.Fatalf("7-card graveyard GE7/Times.2 = (%d, %v), want 2", got, ok)
	}
	// Below the threshold the same head reads 0: the seat 1 graveyard holds
	// 4.
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountOpponents$HasPropertyHasCardsInGraveyard_Card_GE7"); !ok || got != 0 {
		t.Fatalf("4-card graveyard GE7 = (%d, %v), want evaluated 0", got, ok)
	}

	// ---- wasDealtDamageThisTurn ------------------------------------------
	// Precondition: nobody was dealt damage — evaluated 0 everywhere.
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountPlayers$HasPropertywasDealtDamageThisTurn"); !ok || got != 0 {
		t.Fatalf("no-damage Players head = (%d, %v), want evaluated 0", got, ok)
	}
	// The controller alone took damage: counted by Players$, never by the
	// opponent heads (scoping).
	h.dmgTaken = map[state.PlayerID]int32{0: 4}
	for _, tmpl := range opponents {
		if got, ok := EvalCountOK(h, c, pcsTmpl(tmpl, "wasDealtDamageThisTurn")); !ok || got != 0 {
			t.Fatalf("controller-only damage %s = (%d, %v), want 0", pcsTmpl(tmpl, "wasDealtDamageThisTurn"), got, ok)
		}
	}
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountPlayers$HasPropertywasDealtDamageThisTurn"); !ok || got != 1 {
		t.Fatalf("controller-only damage Players head = (%d, %v), want 1", got, ok)
	}
	// The opponent takes damage too: the opponent heads flip 0 -> 1.
	h.dmgTaken = map[state.PlayerID]int32{0: 4, 1: 2}
	for _, tmpl := range opponents {
		if got, ok := EvalCountOK(h, c, pcsTmpl(tmpl, "wasDealtDamageThisTurn")); !ok || got != 1 {
			t.Fatalf("both damaged %s = (%d, %v), want 1", pcsTmpl(tmpl, "wasDealtDamageThisTurn"), got, ok)
		}
	}

	// ---- wasDealtCombatDamageThisTurn (the no-source ledger) -------------
	// Precondition: an empty ledger — evaluated 0 everywhere.
	h.dmgTaken = nil
	for _, expr := range []string{
		"Count$PlayerCountPlayers$HasPropertywasDealtCombatDamageThisTurn",
		"Count$PlayerCountOpponents$HasPropertywasDealtCombatDamageThisTurn",
		"Count$PlayerCountRegisteredOpponents$HasPropertywasDealtCombatDamageThisTurn",
	} {
		if got, ok := EvalCountOK(h, c, expr); !ok || got != 0 {
			t.Fatalf("empty ledger %s = (%d, %v), want evaluated 0", expr, got, ok)
		}
	}
	// A combat hit for the CONTROLLER only: scoping again.
	h.combatHits = []CombatDamageHit{{Player: 0, Amount: 1}}
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountPlayers$HasPropertywasDealtCombatDamageThisTurn"); !ok || got != 1 {
		t.Fatalf("controller combat hit Players head = (%d, %v), want 1", got, ok)
	}
	for _, tmpl := range opponents {
		if got, ok := EvalCountOK(h, c, pcsTmpl(tmpl, "wasDealtCombatDamageThisTurn")); !ok || got != 0 {
			t.Fatalf("controller combat hit %s = (%d, %v), want 0", pcsTmpl(tmpl, "wasDealtCombatDamageThisTurn"), got, ok)
		}
	}
	// A hit for the opponent: 0 -> 1.
	h.combatHits = append(h.combatHits, CombatDamageHit{Player: 1, Amount: 3})
	for _, tmpl := range opponents {
		if got, ok := EvalCountOK(h, c, pcsTmpl(tmpl, "wasDealtCombatDamageThisTurn")); !ok || got != 1 {
			t.Fatalf("opponent combat hit %s = (%d, %v), want 1", pcsTmpl(tmpl, "wasDealtCombatDamageThisTurn"), got, ok)
		}
	}

	// ---- fail-closed controls -------------------------------------------
	// A made-up HasProperty family, an unknown base word, an unknown predicate
	// and a malformed comparison each stay unresolvable — the caller's
	// condition then follows its own documented fail direction instead of
	// enforcing a fabricated zero.
	for _, expr := range []string{
		"Count$PlayerCountOpponents$HasPropertyNoSuchProperty",
		"Count$PlayerCountPlayers$HasPropertyHasCardsInHand_Card_BAD1",
		"Count$PlayerCountPlayers$HasPropertyHasCardsInHand_Card_GE",
		"Count$PlayerCountPlayers$HasPropertyHasCardsInHand_Card.NoSuchPredicate_GE1",
		"Count$PlayerCountPlayers$HasPropertyHasCardsInGraveyard_NoSuchBase_GE1",
	} {
		if got, ok := EvalCountOK(h, c, expr); ok {
			t.Errorf("%s = (%d, true), want unresolvable (0, false)", expr, got)
		}
	}
}
