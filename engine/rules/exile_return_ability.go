package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// findReturnSA walks an ability chain for the ChangeZone line that carries
// ReturnAbility$ (Lagrella, the Magpie's exile).
func findReturnSA(sa *cards.SA) *cards.SA {
	for d := 0; sa != nil && d < 16; d, sa = d+1, sa.Sub {
		if sa.HasParam(cards.PKReturnAbility) {
			return sa
		}
	}
	return nil
}

// runExileReturnAbility implements SpellAbilityEffect's ReturnAbility$ /
// ReturnValid$ rider for a Duration$ UntilHostLeavesPlay exile: after a card
// returns, if it matches ReturnValid$ (the returned card bound as the
// trigger-remembered object, "you" the host's last controller), the named
// SVar ability runs with the returned card as the triggered card. Only
// entries marked Ret at exile time reach here.
func (e *Engine) runExileReturnAbility(source, returned state.ObjID) {
	src := e.G.Obj(source)
	if src == nil || src.Face() == nil || e.G.Obj(returned) == nil {
		return
	}
	f := src.Face()
	var sa *cards.SA
	for _, a := range f.Abilities {
		if sa = findReturnSA(a); sa != nil {
			break
		}
	}
	for i := 0; sa == nil && i < len(f.Triggers); i++ {
		sa = findReturnSA(f.Triggers[i].Effect)
	}
	if sa == nil {
		return
	}
	body := cards.ResolveSVar(f.SVars, sa.ParamStr(cards.PKReturnAbility))
	if body == nil {
		return
	}
	if spec := sa.ParamStr(cards.PKReturnValid); spec != "" {
		var sc effects.SpecContext
		sc.You, sc.Source = src.Controller, source
		sc.DelayedRemembered = []state.Target{{Obj: returned}}
		if !e.matchesSpec(spec, returned, sc) {
			return
		}
	}
	ctx := effects.NewCtxPtr(source, src.Controller, effects.CtxInit{SVars: f.SVars})
	ctx.TriggerCard = returned
	ctx.Remembered = []state.Target{{Obj: returned}}
	effects.Resolve(e, ctx, body)
}
