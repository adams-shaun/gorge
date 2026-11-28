package oracle

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A planned cast with an optional target, part-funded by sacrificing that
// target: both engines pose the target ask before anything is paid (CR
// 601.2c: Captivating Gyre's "up to three target creatures" may be declined,
// so the seat chooses), then Ashnod's Altar sacrifices the announced creature
// (CR 601.2g-h) and the spell, its only target illegal, does not resolve (CR
// 608.2b). Lockstep compares the two engines' options at every decision.
//
// The recording is tail-71 pair 73, seed 10402651278048062844, surface
// autopay: two altars and one creature, Flowstone Shambler. It offers the cast
// at decision 336 and does not take it; this test serves it up to there and
// takes the cast.
func TestAltarVictimTwoAltarsTakeCastLockstep(t *testing.T) {
	t.Parallel()
	const castAt = 336
	r, err := ReadRepro(filepath.Join("testdata", "repro", "altar_victim_two_altars_one_target_cast_offer.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Actions) <= castAt {
		t.Fatalf("recording has %d actions, want more than %d", len(r.Actions), castAt)
	}
	script := &ScriptPolicy{Actions: r.Actions[:castAt]}
	var gyre, shambler state.ObjID
	var tappedAtCast []state.ObjID
	var targetAsked, paid, left bool
	pol := PolicyFunc(func(c *PickCtx) Canon {
		if c.Seq < castAt {
			return script.Pick(c)
		}
		g := c.G.E.G
		d := c.G.Pending()
		// tapped is the caster's tapped permanents.
		tapped := func() (ids []state.ObjID) {
			for _, id := range g.Zone(state.ZBattlefield, g.Obj(gyre).Controller) {
				if g.Obj(id).Tapped {
					ids = append(ids, id)
				}
			}
			return ids
		}
		// option is the first action whose gorge option satisfies ok.
		option := func(ok func(Canon, decision.Option) bool) (Canon, bool) {
			for _, a := range c.D.Options {
				if o, found := c.G.GorgeOption(a); found && ok(a, o) {
					return a, true
				}
			}
			return Canon(0), false
		}
		switch {
		case c.Seq == castAt:
			if script.next != castAt || script.Skipped != 0 {
				t.Errorf("recording served %d of %d actions before the cast (%d skipped): its game path has drifted", script.next, castAt, script.Skipped)
			}
			cast, ok := option(func(a Canon, o decision.Option) bool {
				return a.Kind() == ActCast && c.G.M.InstanceCard(int(a.Src())) == "Captivating Gyre"
			})
			if !ok {
				t.Errorf("decision %d offers no Captivating Gyre cast: %v", c.Seq, c.D.Options)
				break
			}
			o, _ := c.G.GorgeOption(cast)
			gyre = o.Obj
			var creatures, altars []string
			for _, id := range g.Zone(state.ZBattlefield, d.Player) {
				switch p := g.Obj(id); {
				case p.Face().Name == "Ashnod's Altar":
					altars = append(altars, p.Face().Name)
				case slices.Contains(p.Face().Types, "Creature"):
					creatures = append(creatures, p.Face().Name)
					shambler = id
				}
			}
			if len(altars) != 2 || !slices.Equal(creatures, []string{"Flowstone Shambler"}) {
				t.Errorf("at the cast the caster holds altars %v and creatures %v, want two altars and only Flowstone Shambler", altars, creatures)
			}
			tappedAtCast = tapped()
			return cast
		case c.Seq == castAt+1:
			if d.Kind != decision.KTarget {
				t.Errorf("decision %d after the cast is %v, want the target ask", c.Seq, d.Kind)
				break
			}
			// CR 601.2c comes before 601.2g-h: nothing is paid yet.
			if z := g.Obj(shambler).Zone; z != state.ZBattlefield || !slices.Equal(tapped(), tappedAtCast) {
				t.Errorf("at the target ask the victim is in %v and tapped permanents are %v (were %v): paid before the targets", z, tapped(), tappedAtCast)
			}
			if _, ok := option(func(a Canon, _ decision.Option) bool { return a.Kind() == ActDone }); !ok {
				t.Errorf("the optional-target ask offers no way to decline: %v", c.D.Options)
			}
			pick, ok := option(func(a Canon, o decision.Option) bool { return a.Kind() == ActTarget && o.Obj == shambler })
			if !ok {
				t.Errorf("the target ask does not offer the Shambler: %v", c.D.Options)
				break
			}
			targetAsked = true
			return pick
		case targetAsked && !paid && d.Kind == decision.KTarget:
			// "up to three" with one creature: gorge asks again, done only.
			if len(c.D.Options) != 1 || c.D.Options[0].Kind() != ActDone {
				t.Errorf("second target ask offers %v, want done only", c.D.Options)
			}
		case targetAsked && !paid:
			paid = true
			victim, spell, now := g.Obj(shambler).Zone, g.Obj(gyre).Zone, tapped()
			if victim != state.ZGraveyard || spell != state.ZStack || len(now) <= len(tappedAtCast) {
				t.Errorf("after the targets: victim in %v, spell in %v, tapped %v (were %v); want the victim sacrificed and lands tapped for a spell still on the stack", victim, spell, now, tappedAtCast)
			}
		case paid && !left && g.Obj(gyre).Zone != state.ZStack:
			left = true
			if spell, victim := g.Obj(gyre).Zone, g.Obj(shambler).Zone; spell != state.ZGraveyard || victim != state.ZGraveyard {
				t.Errorf("the spell left the stack for %v with its target in %v, want both in the graveyard", spell, victim)
			}
			for _, ev := range c.G.E.L.Events {
				if ev.Kind == events.Resolve && ev.Obj == gyre {
					t.Errorf("the spell resolved with its only target gone: %+v", ev)
				}
			}
		}
		return c.D.Options[0] // done at the second ask, then pass
	})
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, pol)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v %s\nref %v\nker %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail, res.Div.RefOptions, res.Div.KerOptions)
	}
	stallCheck(t, r, res, false)
	if !targetAsked || !paid || !left {
		t.Fatalf("target asked %v, paid %v, spell left the stack %v (stall %q): want all three", targetAsked, paid, left, res.Stall)
	}
}
