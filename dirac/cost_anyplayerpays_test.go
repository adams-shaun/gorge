package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// kfix-cast: "any player may pay" asks player 0 then player 1 (gorge
// unlessPayerTargets AliveFrom(0)); the first payer ends the walk.

// dashHopesAsks casts Dash Hopes for player 1 (at player 0's Grizzly Bears) and answers the any-player
// asks with ans (1 = pay, 0 = decline) in turn; it returns who was asked.
func dashHopesAsks(t *testing.T, ans ...uint8) (*scen, Obj, []uint8) {
	t.Helper()
	sc, id := selfCastScen(t, "Dash Hopes", "Grizzly Bears")
	if !sc.k.eachPay {
		t.Fatal("prep.eachPay not set")
	}
	sc.lands(0, id["Forest"], 2)
	sc.lands(1, id["Swamp"], 3)
	sc.castCard(t, sc.hand(id["Grizzly Bears"], 0))
	dh := sc.hand(id["Dash Hopes"], 1)
	for i := 0; i < 6; i++ {
		d := sc.s.Step()
		if d.Kind == DkPriority && d.Player == 1 {
			break
		}
		if d.Kind == DkPriority {
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		} else {
			sc.s.Apply(d.Options[0])
		}
	}
	sc.castCard(t, dh) // player 1 casts it: player 0 is still asked first
	sc.stepToTrigger(t)
	var asked []uint8
	for i := 0; i < 20 && len(asked) < len(ans); i++ {
		d := sc.s.Step()
		switch {
		case d.Kind == DkPriority:
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		case d.Kind == DkChoose && d.Sub == ChGenPayMode:
			a := ans[len(asked)]
			asked = append(asked, d.Player)
			sc.s.Apply(MakeAction(AkChoose, 0, 0, a))
		default:
			sc.s.Apply(d.Options[0])
		}
	}
	for i := 0; i < 6; i++ { // settle the resolution: a further ask is recorded, never answered
		d := sc.s.Step()
		if d.Kind == DkPriority {
			break
		}
		if d.Kind == DkChoose && d.Sub == ChGenPayMode {
			asked = append(asked, d.Player)
			break
		}
		sc.s.Apply(d.Options[0])
	}
	return sc, dh, asked
}

func TestEachPayerAskedInSeatOrder(t *testing.T) {
	sc, dh, asked := dashHopesAsks(t, 0, 0)
	if len(asked) != 2 || asked[0] != 0 || asked[1] != 1 {
		t.Fatalf("asked %v, want [0 1]", asked)
	}
	if z := sc.s.objs[dh].zone; z != kop.ZoneStack {
		t.Fatalf("both declined: Dash Hopes in zone %d, want the stack", z)
	}
}

func TestEachPayerSecondPays(t *testing.T) {
	sc, dh, asked := dashHopesAsks(t, 0, 1)
	if len(asked) != 2 || asked[1] != 1 {
		t.Fatalf("asked %v, want [0 1]", asked)
	}
	if z := sc.s.objs[dh].zone; z != kop.ZoneGraveyard {
		t.Fatalf("player 1 paid: Dash Hopes in zone %d, want the graveyard", z)
	}
	if sc.s.pl[0].life != 20 || sc.s.pl[1].life != 15 {
		t.Fatalf("life %d/%d, want 20/15", sc.s.pl[0].life, sc.s.pl[1].life)
	}
}

func TestEachPayerFirstPayStops(t *testing.T) {
	sc, dh, asked := dashHopesAsks(t, 1)
	if len(asked) != 1 || asked[0] != 0 {
		t.Fatalf("asked %v, want [0]", asked)
	}
	if z := sc.s.objs[dh].zone; z != kop.ZoneGraveyard {
		t.Fatalf("player 0 paid: Dash Hopes in zone %d, want the graveyard", z)
	}
	if sc.s.pl[0].life != 15 || sc.s.pl[1].life != 20 {
		t.Fatalf("life %d/%d, want 15/20", sc.s.pl[0].life, sc.s.pl[1].life)
	}
}
