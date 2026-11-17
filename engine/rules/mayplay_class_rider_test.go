package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// Rogue Class's level 3 grant ("you may spend mana as though it were mana of
// any color to cast those spells", CR 716.2, 609.4b) is a Class level's
// AddStaticAbility$ static with no MayPlayPlayer$: no ContinuousEffect carries
// it, so mayPlayStaticRider is the only reader of its rider. It once skipped a
// static with no MayPlayPlayer$ and a white card exiled with the Class was
// never offered to a blue-black seat.
func TestMayPlayStaticRiderReadsAClassLevelGrant(t *testing.T) {
	const host state.ObjID = 7
	exiled := &state.Object{ID: 40, Zone: state.ZExile}
	grant := staticView{Source: host, Controller: 1, Params: map[string]string{
		"Mode": "Continuous", "MayPlay": "True", "MayPlayIgnoreType": "True",
		"Affected": "Card.ExiledWithSource", "AffectedZone": "Exile",
	}}
	statics := func(string) []staticView { return []staticView{grant} }
	granted := func(p state.PlayerID, id, h state.ObjID) bool { return p == 1 && id == exiled.ID && h == host }

	got := mayPlayStaticRider(statics, granted, exiled, 1, pipRider{})
	if !got.AnyType || got.AnyColor {
		t.Fatalf("the grant's rider = %+v, want AnyType alone", got)
	}
	if got := mayPlayStaticRider(statics, granted, exiled, 0, pipRider{}); got != (pipRider{}) {
		t.Fatalf("the other seat read the controller's rider: %+v", got)
	}
	hand := &state.Object{ID: 40, Zone: state.ZHand}
	if got := mayPlayStaticRider(statics, granted, hand, 1, pipRider{}); got != (pipRider{}) {
		t.Fatalf("a hand card read a grant that opens another zone: %+v", got)
	}
}
