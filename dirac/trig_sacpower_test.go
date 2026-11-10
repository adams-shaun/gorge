package dirac

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Ruthless Technomancer's enters trigger ("you may sacrifice another creature
// you control. If you do, create Treasures equal to that creature's power")
// counts the sacrificed creature's last known power (CR 701.21, 608.2h), not
// the Technomancer's own, which the queued trigger's record named
// (trig_sacrificedlki.go). One candidate takes the unasked payment
// (autoPayPick), two ask for the pick (payCommitSacs).

func sacPowerScen(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Set(t, "xminsacx")
	var deck []cardset.Entry
	for _, n := range []string{"Ruthless Technomancer", "Memnite", "Ornithopter", "Swamp"} {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s not supported in the xminsacx table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	return newScenTableLib(t, tbl, id["Swamp"], Config{NoElide: true}), id
}

// treasures counts the battlefield Treasure tokens player p controls.
func (sc *scen) treasures(p uint8) (n int) {
	for o := Obj(0); int(o) < int(sc.s.nobj); o++ {
		ob := &sc.s.objs[o]
		if ob.zone == kop.ZoneBattlefield && strings.Contains(sc.k.t.Cards[ob.card].Name, "treasure") &&
			sc.s.perms[ob.slot].ctrl == p {
			n++
		}
	}
	return n
}

// sacPowerRun casts the Technomancer with the creatures in play, pays the
// enters trigger's sacrifice with pick (or the lone candidate) and returns the
// Treasures made.
func sacPowerRun(t *testing.T, creatures []string, pick string) (made, techPower, pickPower int) {
	t.Helper()
	sc, id := sacPowerScen(t)
	for i := 0; i < 4; i++ { // {2}{B}{B}
		sc.perm(id["Swamp"], 0)
	}
	objs := map[string]Obj{}
	for _, n := range creatures {
		objs[n] = sc.perm(id[n], 0)
	}
	tech := sc.hand(id["Ruthless Technomancer"], 0)
	sc.castCard(t, tech)
	sawPay, sawPick := false, false
	for i := 0; i < 80; i++ {
		d := sc.s.Step()
		if d.Kind == DkPriority && sc.s.sp == 0 && sc.s.npend == 0 {
			break
		}
		switch {
		case d.Kind == DkChoose && d.Sub == ChGenPay:
			sawPay = true
			sc.s.Apply(MakeAction(AkChoose, 0, 0, 1)) // pay
		case d.Kind == DkChoose && d.Sub == ChGenPayPick:
			sawPick = true
			applied := false
			for j := 0; j < int(d.N); j++ {
				if o := d.Options[j]; Obj(o.Source()) == objs[pick] {
					sc.s.Apply(o)
					applied = true
					break
				}
			}
			if !applied {
				t.Fatalf("pick %s not offered: %v", pick, d.Options[:d.N])
			}
		case d.Kind == DkPriority:
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		default:
			sc.s.Apply(d.Options[0])
		}
	}
	if !sawPay {
		t.Fatal("the enters trigger's pay/decline was never asked")
	}
	if wantPick := len(creatures) > 1; sawPick != wantPick {
		t.Fatalf("pick asked %v, want %v", sawPick, wantPick)
	}
	sacd := pick
	if sacd == "" {
		sacd = creatures[0]
	}
	if sc.onBF(objs[sacd]) {
		t.Fatalf("%s was not sacrificed", sacd)
	}
	ps := sc.k.cards[sc.s.objs[objs[sacd]].card]
	techPower = int(sc.k.cards[sc.s.objs[tech].card].power)
	pickPower = int(ps.power)
	return sc.treasures(0), techPower, pickPower
}

func TestTechnomancerTreasuresFromSacrificedPower(t *testing.T) {
	for _, c := range []struct {
		name      string
		creatures []string
		pick      string
		want      int
	}{
		{"lone power-1 candidate, unasked", []string{"Memnite"}, "", 1},
		{"lone power-0 candidate, unasked", []string{"Ornithopter"}, "", 0},
		{"picked power-1 of two", []string{"Ornithopter", "Memnite"}, "Memnite", 1},
		{"picked power-0 of two", []string{"Memnite", "Ornithopter"}, "Ornithopter", 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, techPw, pickPw := sacPowerRun(t, c.creatures, c.pick)
			if techPw == pickPw {
				t.Fatalf("Technomancer power %d equals the sacrificed %d: the test cannot tell them apart", techPw, pickPw)
			}
			if pickPw != c.want {
				t.Fatalf("fixture power %d, want %d", pickPw, c.want)
			}
			if got != c.want {
				t.Fatalf("%d Treasures, want %d (sacrificed power %d, Technomancer power %d)", got, c.want, pickPw, techPw)
			}
		})
	}
}
