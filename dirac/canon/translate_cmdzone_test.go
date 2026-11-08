package canon

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestCommanderZoneCanon: CR 903.9a's ask translates to DCommanderZone and
// each answer to ActCommanderZone with the commander's ref and the option
// index as mode (gorge oracle/canon.go's KCommanderZone packing).
func TestCommanderZoneCanon(t *testing.T) {
	const cmd = Ref(3)
	s := new(dirac.State)
	d := &dirac.Decision{Kind: dirac.DkChoose, Sub: dirac.ChCommanderZone, Player: 0, N: 2}
	d.Options[0] = dirac.MakeAction(dirac.AkChoose, 3, 0, 0)
	d.Options[1] = dirac.MakeAction(dirac.AkChoose, 3, 0, 1)
	tr := &Translator{S: s, D: d}
	if got := tr.DKind(); got != DCommanderZone {
		t.Fatalf("decision kind %v, want DCommanderZone", got)
	}
	for mode := uint8(0); mode < 2; mode++ {
		got := tr.Action(d.Options[mode])
		want := Pack(ActCommanderZone, cmd, RefNone, mode)
		if got != want {
			t.Fatalf("option %d = %#x (kind %d src %d tgt %d mode %d), want %#x", mode,
				got, got.Kind(), got.Src(), got.Tgt(), got.Mode(), want)
		}
	}
}
