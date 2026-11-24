package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A token copy of an Unleash creature (The Scarab God reanimating
// Thrill-Kill Assassin) is asked its CR 702.86a counter election before it
// enters: gorge mints the copy outside every zone and parks its battlefield
// move on the ask (applyUnleashReplacement), so the kernel parks the minted
// copy (raGenTokChoice, unleashTokKind) and enters it from the answer. On the
// tail-60 evidence pool, which deals both cards.
func TestUnleashTokenCopyAskedBeforeEntry(t *testing.T) {
	for _, yes := range []bool{true, false} {
		set := cardsettest.Synth(t, "tail-60")
		for _, n := range []string{"The Scarab God", "Thrill-Kill Assassin", "Island"} {
			if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
				t.Fatalf("precondition: %s does not play in the tail-60 pool", n)
			}
		}
		deck := []cardset.Entry{{Name: "The Scarab God", Count: 1}, {Name: "Thrill-Kill Assassin", Count: 2}, {Name: "Island", Count: 20}}
		tbl, da, err := set.MatchupN([][]cardset.Entry{deck, deck})
		if err != nil {
			t.Fatal(err)
		}
		id := map[string]kop.CardID{}
		for p := range da {
			for _, c := range da[p] {
				id[tbl.Cards[c].Name] = c
			}
		}
		tka := id["Thrill-Kill Assassin"]
		if !tbl.Cards[tka].Unleash {
			t.Fatal("precondition: Thrill-Kill Assassin prints Unleash")
		}
		sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
		s := sc.s
		god := sc.perm(id["The Scarab God"], 0)
		victim := sc.grave(tka, 1, 1)[0]
		s.pl[0].pool[kop.U] = 2
		s.pl[0].pool[kop.B] = 2
		d := s.Step()
		var act Action
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(god) {
				act = a
				break
			}
		}
		if act == 0 {
			t.Fatal("precondition: the {2}{U}{B} ability is not offered")
		}
		s.Apply(act)
		sc.do(t, AkTarget, scAny, TCode(victim))
		asked := false
		for i := 0; i < 40 && !asked; i++ {
			d := s.Step()
			switch {
			case d.Kind == DkPriority:
				s.Apply(MakeAction(AkPass, 0, 0, 0))
			case d.Kind == DkChoose && d.Sub == ChGenUnleash:
				asked = true
				if n := len(sc.copiesOf(tka)); n != 0 {
					t.Fatalf("yes=%v: %d copies on the battlefield at the Unleash ask, want 0 (gorge parks the move on it)", yes, n)
				}
				if d.N != 2 {
					t.Fatalf("yes=%v: %d unleash options, want 2", yes, d.N)
				}
				tok := Obj(d.Options[0].Source())
				if s.objs[tok].of&ofToken == 0 || s.objs[tok].card != tka {
					t.Fatalf("yes=%v: the ask is about obj %d, not the minted copy", yes, tok)
				}
				idx := 1
				if yes {
					idx = 0
				}
				s.Apply(d.Options[idx])
			default:
				s.Apply(d.Options[0])
			}
		}
		if !asked {
			t.Fatalf("yes=%v: the copy token's Unleash election was never asked", yes)
		}
		sc.resolveAll(t)
		cps := sc.copiesOf(tka)
		if len(cps) != 1 || s.objs[s.perms[cps[0]].obj].of&ofToken == 0 {
			t.Fatalf("yes=%v: %d copies after the answer, want the one token", yes, len(cps))
		}
		want := 0
		if yes {
			want = 1
		}
		if n := s.perms[cps[0]].ctrN(s, int(kop.CtrP1P1)); n != want {
			t.Fatalf("yes=%v: the token has %d +1/+1 counters, want %d", yes, n, want)
		}
	}
}
