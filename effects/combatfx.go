package effects

import (
	"fmt"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() {
	Register("Tap", effTap)
	Register("Pump", effPump)
	Register("PumpAll", effPumpAll)
	Register("Animate", effAnimate)
	Register("Protection", effProtection)
}

func effTap(h Host, c *Ctx, sa *cards.SA) {
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil || o.Zone != state.ZBattlefield || o.Tapped {
			continue
		}
		h.Emit(events.Event{Kind: events.Tap, Obj: o.ID})
	}
}

// effPump, effPumpAll, effAnimate and effProtection are all continuous
// effects: real CR behaviour needs a layer system to grant power/toughness,
// keywords, types or protection "until end of turn" and have it expire on
// schedule, which is Task 19's rules.Layers. M1 records the grant as a Note
// so the intent is visible in the log, but nothing downstream (combat math,
// keyword checks, filter predicates) sees it yet.
func effPump(h Host, c *Ctx, sa *cards.SA) {
	att := Num(h, c, sa, "NumAtt", 0)
	def := Num(h, c, sa, "NumDef", 0)
	kw := sa.Params["KW"]
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		h.Emit(events.Event{Kind: events.Note, Obj: o.ID,
			Text: fmt.Sprintf("pumped %+d/%+d%s until end of turn", att, def, kwText(kw))})
	}
}

func effPumpAll(h Host, c *Ctx, sa *cards.SA) {
	att := Num(h, c, sa, "NumAtt", 0)
	def := Num(h, c, sa, "NumDef", 0)
	kw := sa.Params["KW"]
	spec := sa.Params["ValidCards"]
	if spec == "" {
		spec = "Creature"
	}
	g := h.Game()
	for _, p := range g.AliveFrom(0) {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			if MatchesSpecFrom(g, spec, id, c.Controller, c.Source) {
				h.Emit(events.Event{Kind: events.Note, Obj: id,
					Text: fmt.Sprintf("pumped %+d/%+d%s until end of turn", att, def, kwText(kw))})
			}
		}
	}
}

func kwText(kw string) string {
	if kw == "" {
		return ""
	}
	return ", gains " + kw
}

// effAnimate does not require the target to already be on the battlefield --
// Forge's own Animate targets a creature card in a graveyard as often as a
// permanent -- so unlike Pump it has no zone guard beyond "the object still
// exists".
func effAnimate(h Host, c *Ctx, sa *cards.SA) {
	pw := Num(h, c, sa, "Power", 0)
	tf := Num(h, c, sa, "Toughness", 0)
	types := sa.Params["Types"]
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil {
			continue
		}
		h.Emit(events.Event{Kind: events.Note, Obj: o.ID,
			Text: fmt.Sprintf("becomes a %d/%d %s until end of turn", pw, tf, types)})
	}
}

func effProtection(h Host, c *Ctx, sa *cards.SA) {
	gains := sa.Params["Gains"]
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		h.Emit(events.Event{Kind: events.Note, Obj: o.ID,
			Text: "gains protection from " + gains + " until end of turn"})
	}
}
