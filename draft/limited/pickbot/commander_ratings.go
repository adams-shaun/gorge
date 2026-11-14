package pickbot

import (
	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/engine/cards"
)

// CommanderValue is the override as a Commander-persona Value hook
// (CommanderDrafter.Value, CommanderBuilder.Value): the rated win rate is
// centred as RatingColour does ((v - RatingCentre) * RatingScale), then
// mapped onto Heuristic's points by inverting the heuristic bridge
// ((h - HeuristicCentre) * HeuristicScale), so the persona's commander,
// identity and option constants keep their meaning.
func (o *RatingsOverride) CommanderValue(t Tuning) func(*cards.Card, limited.CardRef) float64 {
	return func(_ *cards.Card, ref limited.CardRef) float64 {
		centred := (o.Value(ref.Name) - t.RatingCentre) * t.RatingScale
		return centred/t.HeuristicScale + t.HeuristicCentre
	}
}
