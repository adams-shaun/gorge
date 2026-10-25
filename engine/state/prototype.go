package state

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// PrototypeParams reads a face's K:Prototype:<cost>:<P>:<T> line (CR
// 702.160a). The whole parameter is KeywordParam's remainder; Forge puts the
// inset cost first, then the power and toughness, with an optional "|…" tail
// that is cut. Every corpus line is numeric P/T and a plain mana cost, so a
// malformed or absent line fails closed rather than inventing values.
func PrototypeParams(f *cards.Face) (cost string, power, toughness int32, ok bool) {
	if f == nil {
		return "", 0, 0, false
	}
	raw, ok := f.KeywordParam("Prototype")
	if !ok {
		return "", 0, 0, false
	}
	if i := strings.IndexByte(raw, '|'); i >= 0 {
		raw = raw[:i]
	}
	parts := strings.Split(raw, ":")
	if len(parts) < 3 {
		return "", 0, 0, false
	}
	cost = strings.TrimSpace(parts[0])
	if cost == "" {
		return "", 0, 0, false
	}
	p, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return "", 0, 0, false
	}
	t, err := strconv.Atoi(strings.TrimSpace(parts[2]))
	if err != nil {
		return "", 0, 0, false
	}
	return cost, int32(p), int32(t), true
}

// prototypeIn reports the prototype inset values for o while it is the spell
// on the stack or the permanent on the battlefield. CR 718.3a: the
// designation applies only there; the card in every other zone has its
// printed characteristics.
func (o *Object) prototypeIn(zone Zone) (cost string, power, toughness int32, ok bool) {
	if o == nil || o.CastFlags&FlagPrototype == 0 || (zone != ZStack && zone != ZBattlefield) {
		return "", 0, 0, false
	}
	return PrototypeParams(o.Face())
}

// PrototypeCharacteristics is prototypeIn for o's current zone: the inset
// cost and P/T when o is a prototyped spell on the stack or a prototyped
// permanent on the battlefield.
func (o *Object) PrototypeCharacteristics() (cost string, power, toughness int32, ok bool) {
	if o == nil {
		return "", 0, 0, false
	}
	return o.prototypeIn(o.Zone)
}
