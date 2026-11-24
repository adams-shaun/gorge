package pay

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// ForageExileCount is the exile arm's card count (CR 701.61a: "exile three
// cards from your graveyard").
const ForageExileCount = 3

// ForageExilePickAsk builds the exile arm's pick (CR 701.61a, 601.2h): exactly
// three of p's graveyard cards, in zone order, as "exilecost" options (the
// cast path's Exile cost-part kind). It returns nil when the pick is forced
// (the graveyard holds three cards or fewer): the settle then exiles the
// graveyard's three in zone order.
func ForageExilePickAsk(e Engine, p state.PlayerID, source state.ObjID) *decision.Decision {
	grave := e.Game().Zone(state.ZGraveyard, p)
	if len(grave) <= ForageExileCount {
		return nil
	}
	d := &decision.Decision{Player: p, Kind: decision.KChoose, Min: ForageExileCount, Max: ForageExileCount,
		Prompt: "Choose three cards to exile from your graveyard (forage)", Source: source}
	for _, id := range grave {
		label := ""
		if o := e.Game().Obj(id); o != nil && o.Face() != nil {
			label = o.Face().Name
		}
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "exilecost", Obj: id, Label: label})
	}
	return d
}

// ForageExilePicked validates an answer to ForageExilePickAsk (the asker knows
// the pick is pending; the kind is not re-read): three distinct cards, each
// still in p's graveyard. It returns the picked cards, or nil
// when the answer cannot pay.
func ForageExilePicked(e Engine, p state.PlayerID, chosen []decision.Option) []state.ObjID {
	if len(chosen) != ForageExileCount {
		return nil
	}
	grave := e.Game().Zone(state.ZGraveyard, p)
	out := make([]state.ObjID, 0, ForageExileCount)
	for _, opt := range chosen {
		if !slices.Contains(grave, opt.Obj) || slices.Contains(out, opt.Obj) {
			return nil
		}
		out = append(out, opt.Obj)
	}
	return out
}
