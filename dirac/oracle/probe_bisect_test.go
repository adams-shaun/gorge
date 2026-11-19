package oracle

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TEMP PROBE (delete before commit): bisect the Bash-to-Bits flashback offer
// divergence on a live custom set. printedDelveSetIn builds the table from a
// name list; the games run gorge-vs-kernel with a cast-when-possible policy.
func probeBashSet(t *testing.T, extra []string) cardset.Set {
	t.Helper()
	names := []string{"Bash to Bits", "Mystic Gate", "Plains", "Vivid Meadow", "Mountain", "Forest",
		"Energy Refractor", "Bubbling Cauldron", "Cultist's Staff", "Orzhova, the Church of Deals",
		"Grizzly Bears", "Wind Drake"}
	names = append(names, extra...)
	dir, err := cardset.CorpusDir()
	if err != nil {
		t.Fatal(err)
	}
	return cardset.Raw(printedDelveSetIn(t, dir, [2][]string{names, names}))
}

func TestProbeBashBisect(t *testing.T) {
	probeBashLeg(t, 4, 4)
}

func TestProbeBashBisectNoGate(t *testing.T) {
	probeBashLeg(t, 4, 0)
}

func probeBashLeg(t *testing.T, refr, gate int) {
	t.Parallel()
	reg := synthReg(t)
	set := probeBashSet(t, nil)
	deck := func(refractor, gate int) []cardset.Entry {
		m := map[string]int{
			"Bash to Bits": 4, "Vivid Meadow": 4, "Mountain": 5, "Forest": 5,
			"Bubbling Cauldron": 3, "Cultist's Staff": 3, "Orzhova, the Church of Deals": 4,
			"Grizzly Bears": 2, "Wind Drake": 2,
		}
		m["Mystic Gate"] = gate
		if gate < 4 {
			m["Plains"] = 4 - gate
		}
		if refractor > 0 {
			m["Energy Refractor"] = refractor
		} else {
			m["Orzhov Signet"] = 4
		}
		var d []cardset.Entry
		for _, name := range []string{"Bash to Bits", "Mystic Gate", "Plains", "Vivid Meadow", "Mountain", "Forest",
			"Energy Refractor", "Orzhov Signet", "Bubbling Cauldron", "Cultist's Staff", "Orzhova, the Church of Deals",
			"Grizzly Bears", "Wind Drake"} {
			if c := m[name]; c > 0 {
				d = append(d, cardset.Entry{Name: name, Count: c})
			}
		}
		return d
	}
	var summary string
	{
		{
			kd := [2][]cardset.Entry{deck(refr, gate), {}}
			kd[1] = []cardset.Entry{{Name: "Mountain", Count: 20}, {Name: "Forest", Count: 20}}
			for _, e := range kd[0] {
				if _, ok := reg.Lookup(e.Name); !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
			}
			tbl, _, _, err := set.Matchup(kd[0], kd[1])
			if err != nil {
				t.Fatal(err)
			}
			var decks [2][]*cards.Card
			for s, d := range kd {
				for _, e := range d {
					c, ok := reg.Lookup(e.Name)
					if !ok {
						t.Fatalf("%s not in the corpus", e.Name)
					}
					for i := 0; i < e.Count; i++ {
						decks[s] = append(decks[s], c)
					}
				}
				if len(decks[s]) != 40 {
					t.Fatalf("deck %d: %d cards", s, len(decks[s]))
				}
			}
			var vivid, bashCast, bashFb int
			for seed := uint64(1); seed <= 30; seed++ {
				cfg := Config{GorgeAutopaySurface: true}
				m, err := NewMatchup(reg, [2]string{"pb-a", "pb-b"}, decks, 0)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				g, err := NewGorgeSide(m, seed, cfg, nil)
				if err != nil {
					t.Fatal(err)
				}
				e := g.E
				e.ManaAbilityHook = func(_ state.PlayerID, src state.ObjID, sa *cards.SA) {
					if sa.API == "Mana" && strings.Contains(sa.ParamStr(cards.PKCost), "SubCounter") {
						if o := e.G.Obj(src); o != nil && o.Face() != nil && o.Face().Name == "Vivid Meadow" {
							vivid++
						}
					}
				}
				res := LockstepLed(m, seed, g, k, castActivatePlayedPolicy(seed, map[string]int{}), RunConfig{Config: cfg, MaxDecisions: 2500, MaxTurns: 20})
				CloseSide(k)
				for _, ev := range g.E.L.Events {
					if ev.Obj == 0 {
						continue
					}
					o := g.E.G.Obj(ev.Obj)
					if o == nil || o.Face() == nil || o.Face().Name != "Bash to Bits" {
						continue
					}
					if ev.Kind == events.PutOnStack {
						bashCast++
					}
					if ev.Kind == events.CastInfo && strings.Contains(ev.Counter, "flashback") {
						bashFb++
					}
				}
				if d := res.Div; d != nil {
					summary += "\nrefractor=" + itoa0(refr) + " gate=" + itoa0(gate) + " seed " + itoa0(int(seed)) + ": divergence at " + itoa0(d.At) +
						" " + d.What + " " + d.Field + " ref=" + fmt.Sprint(d.RefOptions) + " ker=" + fmt.Sprint(d.KerOptions)
				}
			}
		t.Fatal("leg done refractor=" + itoa0(refr) + " gate=" + itoa0(gate) + ": vivid activations=" + itoa0(vivid) + " bash casts=" + itoa0(bashCast) + " flashbacks=" + itoa0(bashFb) + "\n" + summary)
	}
}
}

func itoa0(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
