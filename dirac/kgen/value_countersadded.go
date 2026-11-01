package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// ctrAddedValue lowers Count$CountersAddedThisTurn <Kind> You <Spec>
// (gorge evalCountBodyPaidCountersAddedThisTurn, rules.CountersAddedThisTurn;
// ) to ExCtrAdded. The kernel keeps presence per player, not
// amounts, so only the carriers' shape reads exactly: the kind is Any,
// P1P1 or LORE; the player is You; the spec is Creature (kind Any), Card.Self
// (Card.EffectSource refuses); and the head is compared >= 1 (cb.cond enforces
// that, buildCond's ctrGate).
func (lc *lowerCtx) ctrAddedValue(arg string) (lValue, error) {
	parts := strings.Fields(arg)
	if len(parts) != 3 {
		return lValue{}, refuse("Count$CountersAddedThisTurn %q", arg)
	}
	var kind int
	switch parts[0] {
	case "Any":
		kind = kop.CtrAddedKindAny
	case "P1P1":
		kind = kop.CtrAddedKindP1P1
	case "LORE":
		kind = kop.CtrAddedKindLore
	default:
		return lValue{}, refuse("Count$CountersAddedThisTurn kind %q", parts[0])
	}
	if parts[1] != "You" {
		return lValue{}, refuse("Count$CountersAddedThisTurn player %q", parts[1])
	}
	var form int
	switch parts[2] {
	case "Creature":
		if kind != kop.CtrAddedKindAny {
			return lValue{}, refuse("Count$CountersAddedThisTurn %s on Creature", parts[0])
		}
		form = kop.CtrAddedFormCreature
	case "Card.Self":
		form = kop.CtrAddedFormSelf
	case "Card.EffectSource":
		// Summon: Brynhildr's MayPlay gate lives in a CreateEmblem static, and
		// the kernel evaluates an emblem static with no source object unless
		// the emblem is host-bound (chars_gen.go eachStatic), so the Saga's
		// record cannot be keyed; its MayPlay of a remembered exile and the
		// turn-based lore counter beside it are unprobed too
		return lValue{}, refuse("Count$CountersAddedThisTurn Card.EffectSource: an Effect emblem static has no source object")
	default:
		return lValue{}, refuse("Count$CountersAddedThisTurn spec %q", parts[2])
	}
	return lValue{Kind: vCtrAdded, N: kind | form<<kop.CtrAddedFormShift}, nil
}
