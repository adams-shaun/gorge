package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

type damageSourceCharacteristics interface {
	inFlightDamageSource() state.ObjID
	actionCause() state.ObjID
	hasKeywordH(state.ObjID, kwHead) bool
}

func damageSourceHasDeathtouch(source damageSourceCharacteristics, sourceLKI map[state.ObjID]map[state.ObjID]effects.DamageSourceLKI, ev events.Event) bool {
	if ev.Kind != events.Damage {
		return false
	}
	sourceID := source.inFlightDamageSource()
	if sourceID == 0 {
		return false
	}
	if cause := source.actionCause(); cause != 0 {
		if bySource, ok := sourceLKI[cause]; ok {
			if lki, found := bySource[sourceID]; found {
				return lki.Deathtouch
			}
		}
	}
	return source.hasKeywordH(sourceID, kwhDeathtouch)
}

// excessBase is one recipient's pre-batch excess thresholds (CR 120.10):
// the creature lethal, the planeswalker loyalty and the battle defense, as
// they stood before the batch's first hit folded. A multi-type recipient's
// excess threshold is the SMALLEST applicable one (CR 120.4a's last
// sentence).
type excessBase struct {
	lethal, loyalty, defense int32
	creature, pw, battle     bool
}

func (b excessBase) hasType() bool { return b.creature || b.pw || b.battle }

// threshold is the recipient's excess threshold with deathtouch's CR 120.4a
// lethal clamp applied: one, without changing the fixed pre-batch baseline
// of any other applicable type.
func (b excessBase) threshold(deathtouch bool) int32 {
	th := int32(-1)
	pick := func(v int32) {
		if v < 0 {
			v = 0
		}
		if th < 0 || v < th {
			th = v
		}
	}
	if b.creature {
		l := b.lethal
		if deathtouch && l > 1 {
			l = 1
		}
		pick(l)
	}
	if b.pw {
		pick(b.loyalty)
	}
	if b.battle {
		pick(b.defense)
	}
	if th < 0 {
		return 0
	}
	return th
}

// captureExcessBaseline runs before Apply marks the damage. It remembers the
// first pre-fold thresholds for this recipient, not intermediate ones after
// earlier events in the same simultaneous batch. A planeswalker row is only
// useful to Mode$ ExcessDamage (Toralf, edh-trigB): damageTriggerAmount
// keeps Mode$ ExcessDamageAll on the creature-only threshold it has always
// compared, so the wider capture never widens the batch mode.
func captureExcessBaseline(baselines map[state.ObjID]excessBase, ev events.Event, board interface {
	Game() *state.Game
	Toughness(state.ObjID) int32
	IsCreature(state.ObjID) bool
}) map[state.ObjID]excessBase {
	if ev.Kind != events.Damage || ev.Obj == 0 || ev.Amount <= 0 {
		return baselines
	}
	id := ev.Obj
	o := board.Game().Obj(id)
	if o == nil || o.Zone != state.ZBattlefield {
		return baselines
	}
	if _, seen := baselines[id]; seen {
		return baselines
	}
	var b excessBase
	if board.IsCreature(id) {
		b.creature = true
		b.lethal = effects.LethalThreshold(board, id) - o.Damage
	}
	// Types mirror the Damage fold (engine/events/apply_combat.go) and
	// excessdamage_emit.go's noteExcessHit: the layer-derived creature type,
	// the printed planeswalker face whose loyalty the fold exchanges (a
	// DamageNotPlaneswalker hit is not one), the printed battle face.
	if f := o.Face(); f != nil {
		if f.IsPlaneswalker() && ev.Text != events.DamageNotPlaneswalker {
			b.pw = true
			b.loyalty = o.Counter("LOYALTY")
		}
		if f.IsBattle() {
			b.battle = true
			b.defense = o.Counter("DEFENSE")
		}
	}
	if !b.hasType() {
		return baselines
	}
	if baselines == nil {
		baselines = make(map[state.ObjID]excessBase)
	}
	baselines[id] = b
	return baselines
}

// damageTriggerAmount leaves ordinary modes unchanged. When the hit already
// meets the threshold, the whole hit is excess. Each simultaneous hit
// compares to the same base.
func damageTriggerAmount(baselines map[state.ObjID]excessBase, mode cards.TriggerMode, ev events.Event, deathtouch bool) int32 {
	if mode != cards.TriggerExcessDamage && mode != cards.TriggerExcessDamageAll {
		return ev.Amount
	}
	if ev.Obj == 0 || ev.Amount <= 0 {
		return 0
	}
	b, ok := baselines[ev.Obj]
	if !ok {
		return 0
	}
	// ExcessDamageAll ("whenever one or more CREATURES are dealt excess
	// damage") keeps its creature-only threshold: a planeswalker-only row
	// fires nothing, and a creature-planeswalker is compared by its creature
	// lethal alone, exactly as before the wider capture existed.
	if mode == cards.TriggerExcessDamageAll {
		if !b.creature {
			return 0
		}
		baseline := b.lethal
		if deathtouch && baseline > 1 {
			baseline = 1
		}
		if ev.Amount <= baseline {
			return 0
		}
		if baseline <= 0 {
			return ev.Amount
		}
		return ev.Amount - baseline
	}
	// ExcessDamage (CR 120.4a): the threshold is the SMALLEST applicable
	// one, deathtouch's lethal clamp included.
	baseline := b.threshold(deathtouch)
	if ev.Amount <= baseline {
		return 0
	}
	if baseline <= 0 {
		return ev.Amount
	}
	return ev.Amount - baseline
}

func setExcessTriggerAmount(pt *pendingTrigger, mode cards.TriggerMode, amount int32) {
	if mode == cards.TriggerExcessDamageAll || mode == cards.TriggerExcessDamage {
		pt.Ctx.TriggerContext.TriggerAmount = amount
	}
}
