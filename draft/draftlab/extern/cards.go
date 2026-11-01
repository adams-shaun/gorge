package extern

import (
	"strings"

	"github.com/adams-shaun/gorge/draft/draftlab"
	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/server/protocol"
)

// draftCards converts a CardRef list to its wire shape, carrying the
// printed facts the draft UI renders: mana value, colours, cost and type
// line, read from the lab's corpus. A card the corpus does not know keeps
// only its ref fields. Colours are the card's colour identity — the union
// of every face's colours — as a subset of "WUBRG" in WUBRG order. This is
// eventhost's card projection (draft/eventhost/cards.go) copied, not imported:
// the extern tier must not import the host.
func draftCards(l *draftlab.Lab, refs []limited.CardRef) []protocol.DraftCard {
	if refs == nil {
		return nil
	}
	out := make([]protocol.DraftCard, len(refs))
	for i, ref := range refs {
		out[i] = protocol.DraftCard{
			ID: ref.ID, Name: ref.Name, Set: ref.Set, Number: ref.Number,
			Rarity: ref.Rarity.String(),
		}
		c, ok := l.Card(ref.Name)
		if !ok {
			continue
		}
		f := c.Faces[0]
		cost := f.ManaCost
		if strings.EqualFold(cost, "no cost") {
			cost = ""
		}
		out[i].ManaCost = cost
		out[i].ManaValue = int(f.ManaValue())
		out[i].Colors = wubrgOf(c.ColourIdentity())
		out[i].TypeLine = strings.Join(f.Types, " ")
	}
	return out
}

// draftCardLists maps draftCards over one list per pack (a pick item's
// seen packs).
func draftCardLists(l *draftlab.Lab, lists [][]limited.CardRef) [][]protocol.DraftCard {
	if lists == nil {
		return nil
	}
	out := make([][]protocol.DraftCard, len(lists))
	for i, ls := range lists {
		out[i] = draftCards(l, ls)
	}
	return out
}

// wubrgOf spells a colour-identity mask as a subset of "WUBRG", in WUBRG
// order.
func wubrgOf(mask uint8) string {
	const wubrg = "WUBRG"
	out := make([]byte, 0, 5)
	for i := 0; i < len(wubrg); i++ {
		if mask&(1<<i) != 0 {
			out = append(out, wubrg[i])
		}
	}
	return string(out)
}
