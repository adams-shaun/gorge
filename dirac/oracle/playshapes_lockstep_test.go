package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The paid-play lockstep (dirac/kgen/lower_zone.go play's paid arm, the
// kernel freeQ drain's paid route in dirac/turn_gen.go): a Play without
// WithoutManaCost$ True begins the ordinary cast paying its printed cost --
// Vaan, Street Thief's dug card from exile (with ForgetPlayed$ and the
// chained Treasure gate) and Chandra, Acolyte of Flame's targeted graveyard
// instant/sorcery (with ReplaceGraveyard$ Exile and the cmcLE3 ValidSA$
// bound). gorge asks the same "play" decision the free casts do; the answer
// begins a cast with a mana window (no plan mid-resolution, CR 601.2g), and
// an unpayable cost reverses it (CR 733.1). Run as its own shard
// (-run '^TestPlayShapesLockstep$').
func playShapesDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Vaan, Street Thief", Count: 3},
		{Name: "Chandra, Acolyte of Flame", Count: 3},
		{Name: "Shock", Count: 4},             // MV 1: Chandra's -2 casts it paid from a graveyard
		{Name: "Opt", Count: 4},               // MV 1
		{Name: "Divination", Count: 2},        // MV 3: the bound's edge
		{Name: "Baral's Expertise", Count: 2}, // MV 5: beyond the cmcLE3 bound, never offered
		{Name: "Grizzly Bears", Count: 4},
		{Name: "Island", Count: 8},
		{Name: "Mountain", Count: 6},
		{Name: "Swamp", Count: 2},
		{Name: "Forest", Count: 2},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

// playShapesPolicy casts, activates, plays lands and attacks whenever it can
// (so Vaan deals combat damage and Chandra reaches her -2), takes a play
// ask's cast over its Done, and counts the begun paid and free plays from
// gorge's pending decision.
func playShapesPolicy(t *psTally) func(uint64) Policy {
	return func(seed uint64) Policy {
		rnd := RandomPolicy(seed)
		return PolicyFunc(func(c *PickCtx) Canon {
			buf := make([]Canon, 0, 8)
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast, ActCastFace, ActActivate, ActPlayLand, ActAttack, ActMode:
					buf = append(buf, a)
				}
			}
			var a Canon
			if len(buf) > 0 && len(buf) < len(c.D.Options) {
				d := *c.D
				d.Options = buf
				a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			} else {
				a = rnd.Pick(c)
			}
			if c.G != nil {
				playShapesCount(c, a, t)
			}
			return a
		})
	}
}

// psTally counts the plays gorge's "play" asks begin, paid (no
// WithoutManaCost$ True) and free, with the carrier names they began.
type psTally struct {
	paid, free int
	paidNames  []string
}

// playShapesCount classifies one answer against the pending gorge decision
// (m11Count's shape, freeplay_test.go).
func playShapesCount(c *PickCtx, a Canon, t *psTally) {
	pd := c.G.Pending()
	if pd == nil || pd.ResumeKind != "play" || pd.ResumeSA == nil {
		return
	}
	opt, ok := c.G.GorgeOption(a)
	if !ok || opt.Kind != "mode" || opt.Obj == 0 {
		return
	}
	ob := c.G.E.G.Obj(opt.Obj)
	if ob == nil || ob.Face() == nil {
		return
	}
	switch ob.Zone {
	case state.ZExile, state.ZGraveyard, state.ZHand, state.ZLibrary:
		if strings.EqualFold(strings.TrimSpace(pd.ResumeSA.ParamStr(cards.PKWithoutManaCost)), "True") {
			t.free++
		} else {
			t.paid++
			t.paidNames = append(t.paidNames, ob.Face().Name)
		}
	}
}

func TestPlayShapesLockstep(t *testing.T) {
	t.Parallel()
	tally := new(psTally)
	lockstepRunPolicy(t, "freeplay", playShapesDecks(), 60, true, nil, nil, playShapesPolicy(tally))
	t.Logf("paid plays %d (%v), free plays %d", tally.paid, tally.paidNames, tally.free)
	if tally.paid < 15 {
		t.Errorf("paid plays begun %d, want at least 15; the lockstep proves nothing", tally.paid)
	}
	if tally.free < 5 {
		t.Errorf("free plays begun %d, want at least 5", tally.free)
	}
}
