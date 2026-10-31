package eventhost

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/booster"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/server/protocol"
)

// colourOf adapts the card corpus to booster's ColourOf: the card's colour
// identity as W/U/B/R/G bytes, in that order. A card the corpus does not
// know (or a colourless one) has no colours.
func colourOf(reg *cards.Registry) booster.ColourOf {
	const wubrg = "WUBRG"
	return func(name string) []byte {
		c, ok := reg.Lookup(name)
		if !ok {
			return nil
		}
		mask := c.ColourIdentity()
		if mask == 0 {
			return nil
		}
		out := make([]byte, 0, 5)
		for i := 0; i < len(wubrg); i++ {
			if mask&(1<<i) != 0 {
				out = append(out, wubrg[i])
			}
		}
		return out
	}
}

// openPod generates the event's whole pod from its seed: out[seat][pack].
// Regenerating is deterministic, so a run that crashed between its
// EventCreated and its last PackOpened resumes by reopening exactly the
// packs it has not logged yet.
func (r *run) openPod() ([][][]limited.CardRef, error) {
	return booster.OpenPod(r.spec, r.meta.Seed, draftSeats, draftRounds, &booster.IDs{}, colourOf(r.h.opts.Cards))
}

// draftSeats and draftRounds spell the pod shape locally.
const (
	draftSeats  = 4
	draftRounds = 3
)

// draftCards converts a CardRef list to its wire shape, carrying the
// printed facts the draft UI renders: mana value, colours, cost and
// type line, read from the corpus. A card the corpus does not know keeps
// only its ref fields. Colours are the card's colour identity — the union
// of every face's colours, exactly what booster's colourOf reads — as a
// subset of "WUBRG" in WUBRG order.
func (r *run) draftCards(refs []limited.CardRef) []protocol.DraftCard {
	if refs == nil {
		return nil
	}
	out := make([]protocol.DraftCard, len(refs))
	for i, ref := range refs {
		out[i] = protocol.DraftCard{
			ID: ref.ID, Name: ref.Name, Set: ref.Set, Number: ref.Number,
			Rarity: ref.Rarity.String(),
		}
		c, ok := r.h.opts.Cards.Lookup(ref.Name)
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

// wubrgOf spells a colour-identity mask as a subset of "WUBRG", in WUBRG
// order — the same order booster's colourOf reports colours in.
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

// pipsOf reports the coloured mana pips (W U B R G) of one maindeck card
// id: the pool card's faces' ManaCostChroma, exactly the way pickbot reads
// pips. An id the pool does not carry counts no pips.
func (r *run) pipsOf(seat int) func(id string) [5]int {
	return func(id string) [5]int {
		var out [5]int
		for _, ref := range r.ev.Pool(seat) {
			if ref.ID != id {
				continue
			}
			c, ok := r.h.opts.Cards.Lookup(ref.Name)
			if !ok {
				return out
			}
			for _, face := range c.Faces {
				for i := 0; i < 5; i++ {
					out[i] += int(cards.ManaCostChroma(face.ManaCost, 1<<i))
				}
			}
			return out
		}
		return out
	}
}

// packKey is the draft-pack change key a seat's subscribers are compared
// against: the pack on offer, or "waiting".
func (r *run) packKey(seat int) string {
	if r.ev.Phase() != event.PhaseDraft {
		return "waiting"
	}
	v, ok := r.ev.Next(seat)
	if !ok {
		return "waiting"
	}
	return fmt.Sprintf("r%dp%dn%d", v.Round, v.PickNo, len(v.Pack))
}
