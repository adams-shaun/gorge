package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The payment-history and chroma count heads. Each reads what a payment
// recorded (state.Object.ManaSpent / ManaColorSpent / the typed captures, or
// the TriggerPayingMana snapshot) or the mana-cost symbols of a named set of
// cards (cards.ManaCostChroma, CR 107.4 / 202.1):
//
//	Count$TotalManaSpent [<Type>]        mana spent (of a producer type) on the
//	                                      resolving spell or activated ability
//	Count$ManaColorsPaid                  distinct colours of that mana
//	Count$xColorPaid <Colour>             of that colour spent on {X}
//	Count$Chroma.<Colour>                 symbols among permanents you control
//	Count$ChromaInGrave.<Colour>          symbols among cards in your graveyard
//	Count$TriggeredPayingMana.<C>[.<C>]   mana of those colours paid by the
//	                                      triggering payment
//	Count$TriggeredManaCostDevotion.<C>   symbols in the triggering card's cost

// evalCountBodyPayment evaluates the heads above. matched is false when head
// is none of them, so the dispatcher hands the body on.
func evalCountBodyPayment(c *Ctx, g *state.Game, head, arg string) (int32, bool, bool) {
	switch evalCountBodyPaidCodes.Code(head) {
	case evalCountBodyPaidTotalManaSpent:
		// The bare form is the total spent; "<Type>" selects the share paid
		// with that producer type's mana (Treasure for Jetmir's Fixer),
		// through the same byTag read CastTotalManaSpent makes.
		if o := paymentRecordObject(g, c); o != nil {
			return manaSpentTotalsOf(o).byTag(arg), true, true
		}
		return 0, true, true
	case evalCountBodyPaidManaColorsPaid:
		var n int32
		if o := paymentRecordObject(g, c); o != nil {
			for i := state.MW; i <= state.MG; i++ {
				if o.ManaColorSpent[i] > 0 {
					n++
				}
			}
		}
		return n, true, true
	case evalCountBodyPaidXColorPaid:
		mask, ok := chromaColourMask(arg)
		if !ok {
			return 0, false, true
		}
		return xColorPaid(g, c, mask), true, true
	}
	if rest, ok := strings.CutPrefix(head, "ChromaInGrave"); ok {
		mask, okm := chromaSourceMask(rest)
		if !okm {
			return 0, false, true
		}
		return chromaZoneCount(g, state.ZGraveyard, c.Controller, mask), true, true
	}
	if rest, ok := strings.CutPrefix(head, "Chroma"); ok && strings.HasPrefix(rest, ".") {
		mask, okm := chromaSourceMask(rest)
		if !okm {
			return 0, false, true
		}
		return chromaZoneCount(g, state.ZBattlefield, c.Controller, mask), true, true
	}
	if rest, ok := strings.CutPrefix(head, "TriggeredManaCostDevotion"); ok {
		mask, okm := chromaSourceMask(rest)
		if !okm {
			return 0, false, true
		}
		return chromaObjectCount(g.Obj(c.TriggerCard), mask), true, true
	}
	if rest, ok := strings.CutPrefix(head, "TriggeredPayingMana"); ok {
		var n int32
		for _, tok := range strings.Split(strings.TrimPrefix(rest, "."), ".") {
			l := colourLetter(tok)
			if l == 0 {
				return 0, false, true
			}
			n += c.TriggerPayingMana[state.ManaIndex(l)]
		}
		return n, true, true
	}
	return 0, false, false
}

// paymentRecordObject is the object whose recorded payment a payment-history
// head reads: the resolving activated ability's own stack object when the
// activation stamped its spend there (rules' activation capture), else the
// spell or permanent the source's cast stamped (castProvenanceSource). An
// activation's payment is its own -- never the permanent's cast.
func paymentRecordObject(g *state.Game, c *Ctx) *state.Object {
	if so := g.Obj(c.ResolvingObj); so != nil && so.Ability != nil &&
		so.CastFlags&(state.FlagManaSpent|state.FlagManaColorSpent) != 0 {
		return so
	}
	return castProvenanceSource(g, c)
}

// xColorPaid is the mana of the queried colours spent on {X} of the resolving
// spell. The payment records the per-colour spend of the whole cost, not which
// pip each unit paid, so the share is attributed deterministically: units of
// the colour beyond the printed cost's own symbols of it go to X first, capped
// at the announced X (a player restricted to "spend only black and/or red
// mana on X" pays X from exactly those units).
func xColorPaid(g *state.Game, c *Ctx, mask uint8) int32 {
	o := paymentRecordObject(g, c)
	if o == nil {
		return 0
	}
	x := o.X
	if c.X != 0 || c.XAnnounced {
		x = c.X
	}
	var spent int32
	for i, bit := range [...]uint8{cards.ColourWhite, cards.ColourBlue, cards.ColourBlack, cards.ColourRed, cards.ColourGreen} {
		if mask&bit != 0 {
			spent += o.ManaColorSpent[i]
		}
	}
	spent -= chromaObjectCount(o, mask)
	return max(0, min(x, spent))
}

// chromaZoneCount counts the mana symbols of the queried colours in the mana
// costs of the objects in controller's zone: the permanents they control for
// Chroma, the cards in their graveyard for ChromaInGrave. A phased-out
// permanent is treated as though it does not exist (CR 702.26b).
func chromaZoneCount(g *state.Game, z state.Zone, controller state.PlayerID, mask uint8) int32 {
	if int(controller) < 0 || int(controller) >= len(g.Players) {
		return 0
	}
	var n int32
	for _, id := range g.Zone(z, controller) {
		o := g.Obj(id)
		if o == nil || o.PhasedOut {
			continue
		}
		n += chromaObjectCount(o, mask)
	}
	return n
}
