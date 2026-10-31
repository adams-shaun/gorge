package lethalpressure

import (
	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/seat"
)

func init() {
	bots.Register(bots.Entry{Info: bots.Info{
		Name: "lethal-pressure", Label: "Lethal Pressure", Description: "The manual hosted bot policy; on auto-pay tables it uses its own decision rules rather than the production bot's combat-simulation arm.", Tier: bots.Experimental,
		Cost:    bots.Cost{Scope: "per decision", Note: "as bot", Source: "docs/design/2026-09-28-hosted-bot-packages.md §3.1"},
		Formats: []string{"constructed", "commander"}, Caretaker: "bot",
	}, New: newLethalPressure})
}

func newLethalPressure(o bots.Options) (seat.Seat, error) {
	b := heuristic.NewLethalPressureBot(o.Seed)
	if o.AutoPayMana {
		b.EnableAutoPayMana()
	}
	return b, nil
}
