package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestEffectRaiseCostReadsRememberedPlayers is Academy Loremaster's shape on
// the ff076d4 corpus: DB$ Effect with RememberObjects$ TriggeredPlayer
// delivers a RaiseCost whose Activator$ is Player.IsRemembered. The cost view
// must carry the effect's remembered players, or the {2} never applies.
func TestEffectRaiseCostReadsRememberedPlayers(t *testing.T) {
	t.Parallel()
	e := mayPlayBase(t)
	source := onBoardGrant(t, e, 0, "Name:Tax Source\nTypes:Creature\nPT:2/3\n"+
		"SVar:RaiseCost:Mode$ RaiseCost | ValidCard$ Card | Activator$ Player.IsRemembered | Type$ Spell | Amount$ 2\n"+
		"SVar:TrigDraw:DB$ Effect | StaticAbilities$ RaiseCost | RememberObjects$ TriggeredPlayer\n"+
		"Oracle:x\n")
	face := e.G.Obj(source).Face()
	ctx := &effects.Ctx{Source: source, Controller: 0, SVars: face.SVars}
	ctx.TriggerPlayer = state.Target{IsPlayer: true, Player: 1}
	effects.Resolve(e, ctx, cards.ResolveSVar(face.SVars, "TrigDraw"))
	spell := func(p state.PlayerID) state.ObjID {
		o := e.G.AddObject(card(t, "Name:Tax Bolt "+string(rune('A'+p))+"\nManaCost:R\nTypes:Instant\nOracle:x\n"), p)
		o.Zone = state.ZHand
		e.G.SetZone(state.ZHand, p, append(append([]state.ObjID(nil), e.G.Zone(state.ZHand, p)...), o.ID))
		return o.ID
	}
	s0, s1 := spell(0), spell(1)
	if got := e.costModifiers(1, s1, spellScope("")).Raises; len(got) != 1 || got[0] != 2 {
		t.Fatalf("remembered player's spell Raises = %v, want [2]", got)
	}
	if got := e.costModifiers(0, s0, spellScope("")).Raises; len(got) != 0 {
		t.Fatalf("other player's spell Raises = %v, want none", got)
	}
}
