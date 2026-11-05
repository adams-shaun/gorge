package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRoomsRefusals: a two-door Room lowers as kop.FaceRoom with its two
// doors and six state keys (lower_rooms.go); a static's own door line naming
// an SVar body still refuses with its named reason (Greenhouse), while a
// trigger's resolution chain may name one (Painter's Studio lowers fully),
// and every state entry carries the unlocked doors' summed cost.
func TestRoomsRefusals(t *testing.T) {
	cs := lowerNamed(t, []string{"Bottomless Pool", "Painter's Studio", "Greenhouse"})
	by := map[string]*lCard{}
	for _, c := range cs {
		by[c.Name] = c
	}
	bp := by["Bottomless Pool"]
	if bp == nil || len(bp.Refusals) > 0 || bp.FaceMode != kop.FaceRoom || len(bp.Faces) != 8 {
		t.Fatalf("Bottomless Pool: %+v", bp)
	}
	if bp.Faces[1] != faceKey("Bottomless Pool", 1) || bp.Faces[2] != "room:Bottomless Pool#00" || bp.Faces[7] != "room:Bottomless Pool#13" {
		t.Errorf("Bottomless Pool faces %v", bp.Faces)
	}
	if ps := by["Painter's Studio"]; ps == nil || len(ps.Refusals) > 0 {
		t.Errorf("Painter's Studio: trigger SVar bodies must lower, refusals: %v", ps.Refusals)
	}
	if gh := by["Greenhouse"]; gh == nil || len(gh.Refusals) == 0 || !strings.HasPrefix(gh.Refusals[0].Reason, "Room door line naming an SVar body") {
		t.Errorf("Greenhouse: a static's own named SVar body must stay refused: %+v", gh)
	}
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte("Bottomless Pool\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, ratchetCards(t))
	if err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]int{"room:Bottomless Pool#00": 0, "room:Bottomless Pool#03": 6, "room:Bottomless Pool#11": 1} {
		e := lowerRoomEntry(p, k)
		if len(e.Refusals) > 0 {
			t.Fatalf("%s refused: %v", k, e.Refusals)
		}
		if mv := e.Cost.Generic + e.Cost.Pips[0] + e.Cost.Pips[1] + e.Cost.Pips[2] + e.Cost.Pips[3] + e.Cost.Pips[4]; mv != want {
			t.Errorf("%s: MV %d, want %d", k, mv, want)
		}
	}
	// (1,{0}): face 1's record, door 0's UnlockDoor line only, RoomSplit 0
	e := lowerRoomEntry(p, "room:Bottomless Pool#11")
	if e.Face != 1 || e.RoomSplit != 0 || len(e.Abilities) != 1 || e.Abilities[0].OriginOK {
		t.Errorf("room:Bottomless Pool#11: face %d split %d abilities %d", e.Face, e.RoomSplit, len(e.Abilities))
	}
}

// TestRoomsNameReadRefusal: a card with a name-reading filter (Hare
// Apparent's "named<itself>", F2SameNameAsSource) is flagged by the build,
// a Room is not, and a matchup holding both refuses with a named reason
// (cardset.RoomNameRefusal through the generated roomNameGuard): the
// kernel reads names off CardIDs, which a Room's door-state defs split.
func TestRoomsNameReadRefusal(t *testing.T) {
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte("Bottomless Pool\nHare Apparent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, ratchetCards(t))
	if err != nil {
		t.Fatal(err)
	}
	all, bs, _, err := buildPool(lowerPool(p))
	if err != nil {
		t.Fatal(err)
	}
	var rooms, readers []string
	for i, c := range all {
		if bs[i].def.FaceMode == kop.FaceRoom && i < len(p.names) {
			rooms = append(rooms, c.Name)
		}
		if bs[i].readsName {
			readers = append(readers, c.Name)
		}
	}
	if len(rooms) != 1 || rooms[0] != "Bottomless Pool" || len(readers) != 1 || readers[0] != "Hare Apparent" {
		t.Fatalf("rooms %v, name readers %v", rooms, readers)
	}
	err = cardset.RoomNameRefusal(rooms, readers)
	if err == nil || !strings.HasPrefix(err.Error(), "Room name read: Hare Apparent reads a name and Bottomless Pool is a Room") {
		t.Fatalf("refusal: %v", err)
	}
	if cardset.RoomNameRefusal(rooms, nil) != nil || cardset.RoomNameRefusal(nil, readers) != nil {
		t.Fatal("a Room alone or a name reader alone refused")
	}
}

// TestRoomsHybridDoorRefusal: a Room door with a hybrid cost refuses (the
// unlock's pool pricing reads no hybrid symbol); the same Room with a
// plain cost lowers.
func TestRoomsHybridDoorRefusal(t *testing.T) {
	src := "Name:Probe Pool\nManaCost:U\nTypes:Enchantment Room\nAlternateMode:Split\n\nALTERNATE\n\nName:Probe Locker\nManaCost:4 COST\nTypes:Enchantment Room\n"
	for cost, want := range map[string]bool{"U": false, "WU": true} {
		card, diags := cards.ParseBytes("probe_pool.txt", []byte(strings.Replace(src, "COST", cost, 1)))
		if card == nil {
			t.Fatalf("parse: %v", diags)
		}
		if d := card.Link(); len(d) != 0 {
			t.Fatalf("link: %v", d)
		}
		p := &pool{names: []string{"Probe Pool"}, byName: map[string]*cards.Card{"Probe Pool": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
		c := lowerPool(p).cards[0]
		hit := false
		for _, r := range c.Refusals {
			hit = hit || r.Reason == "Room door hybrid cost"
		}
		if hit != want || c.FaceMode != kop.FaceRoom {
			t.Errorf("door cost %q: hybrid refusal %v (want %v), face mode %d, refusals %v", cost, hit, want, c.FaceMode, c.Refusals)
		}
	}
}
