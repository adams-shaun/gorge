package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestWebSlingingScarletSpiderEntersWithReturnedManaValueCounters pins
// "If Scarlet Spider was cast using web-slinging, he enters with X +1/+1
// counters, where X is the mana value of the returned creature": the entry
// replacement reads the cast's returned list (CastSA>Returned$CardManaCost).
func TestWebSlingingScarletSpiderEntersWithReturnedManaValueCounters(t *testing.T) {
	t.Parallel()

	cast := func(t *testing.T, web bool, mana, bearSrc string) (*Engine, state.ObjID) {
		e := handEngine(t, corpusCard(t, "Scarlet Spider, Ben Reilly"))
		sp := e.G.Zone(state.ZHand, 0)[0]
		var ret state.ObjID
		if web {
			ret = onBoard(t, e, 0, bearSrc)
			e.emit(events.Event{Kind: events.Tap, Obj: ret})
		}
		addMana(t, e, 0, mana)
		if web {
			submitChoices(t, e, castModeOption(t, e, sp, "web-slinging"))
		} else {
			idx := -1
			for _, o := range e.Pending().Options {
				if o.Kind == "cast" && o.Obj == sp {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("no hardcast option: %+v", e.Pending().Options)
			}
			submitChoices(t, e, idx)
		}
		for i := 0; i < 20; i++ {
			d := e.Pending()
			if d == nil || d.Kind != decision.KChoose {
				break
			}
			idx := -1
			for _, o := range d.Options {
				if o.Kind == "returncost" && web && o.Obj == ret {
					idx = o.Index
				}
			}
			if idx < 0 {
				for _, o := range d.Options {
					if o.Kind == "done" {
						idx = o.Index
					}
				}
			}
			if idx < 0 {
				t.Fatalf("unexpected KChoose: %+v", d)
			}
			submitChoices(t, e, idx)
		}
		passUntilStackEmpty(t, e, 60)
		if web && e.G.Obj(ret).Zone != state.ZHand {
			t.Fatalf("setup: returned creature is in zone %v, want hand", e.G.Obj(ret).Zone)
		}
		return e, sp
	}

	for _, tc := range []struct {
		name, src string
		want      int32
	}{
		{"web-slinging returns a MV-2 creature", "Name:Web Test Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n", 2},
		{"different MV differs", "Name:Web Test Ox\nManaCost:2 G\nTypes:Creature Ox\nPT:3/3\nOracle:x\n", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, sp := cast(t, true, "RG", tc.src)
			o := e.G.Obj(sp)
			if o.Zone != state.ZBattlefield || o.CastFlags&state.FlagWebSlinged == 0 {
				t.Fatalf("setup: Scarlet Spider zone=%v flags=%v, want battlefield + web-slinged", o.Zone, o.CastFlags)
			}
			if len(o.Counters) != 1 || o.Counters[0].Kind != "P1P1" || o.Counters[0].N != tc.want {
				t.Fatalf("counters = %+v, want one P1P1 x%d", o.Counters, tc.want)
			}
		})
	}

	t.Run("hardcast", func(t *testing.T) {
		t.Parallel()
		e, sp := cast(t, false, "1RG", "")
		o := e.G.Obj(sp)
		if o.Zone != state.ZBattlefield || o.CastFlags != 0 {
			t.Fatalf("setup: zone=%v flags=%v, want battlefield with no web-slinging", o.Zone, o.CastFlags)
		}
		if len(o.Counters) != 0 {
			t.Fatalf("hardcast counters = %+v, want none", o.Counters)
		}
	})
}
