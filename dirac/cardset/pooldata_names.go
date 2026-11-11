package cardset

import (
	"fmt"
	"sort"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A card that chooses a card name as it enters (ExtChooseName: Kai Winn,
// Faithless Leader) keeps the choice in the permanent's chosen byte as the
// named card's CardID, so every nameable id must fit below the byte's "none"
// value 255 and a nameable card must have one name. A matchup table numbers
// its cards compactly, so that is about its own cards; the pool table
// numbers them by their stable PoolIDs, which a big pool outgrows.

// choosesName reports whether a table card's program opens an as-enters card
// name choice.
func choosesName(cd *kop.CardDef) bool {
	n, _ := choosesNameKind(cd)
	return n
}

// activatesOffBattlefield reports whether a table card has an activated ability
// whose source must be off the battlefield (cycling, channel, a graveyard
// ability). gorge's CantBeActivated reads no zone, so an any-card name forbids
// those too, which the kernel's battlefield-only check (actForbiddenAb) does
// not: a table holding such a card beside an any-card name chooser is refused.
func activatesOffBattlefield(cd *kop.CardDef) bool {
	for _, ab := range cd.Abilities {
		if ab.Kind == kop.AbActivated && ab.ActZone != kop.ZoneNone && ab.ActZone != kop.ZoneBattlefield {
			return true
		}
	}
	return false
}

// choosesNameKind is choosesName, and whether the choice is over every card's
// name, lands included (ExtChooseName B = 1 or 2: Sorcerous Spyglass).
func choosesNameKind(cd *kop.CardDef) (names, anyCard bool) {
	for _, ab := range cd.Abilities {
		for _, in := range ab.Prog {
			if in.Op == kop.OpExt && in.Ext() == kop.ExtChooseName {
				names = true
				anyCard = anyCard || in.B.N == 1 || in.B.N == 2
			}
		}
	}
	return names, anyCard
}

// poolNamesFit reports whether the pool table can hold a card name choice:
// no multi-face card (a split, adventure, double-faced or prepare card has
// more nameable names than one CardID) and every real nonland card's PoolID
// below 255.
func poolNamesFit(d *PoolData, anyCard bool) bool {
	for i := 0; i < d.NumCards; i++ {
		if len(d.Defs[i].Faces) > 0 || anyCard && activatesOffBattlefield(&d.Defs[i]) {
			return false
		}
		if (anyCard || d.Defs[i].Types&kop.TLand == 0) && d.PoolID[i] >= 255 {
			return false
		}
	}
	return true
}

// nameInertCards marks the cards that choose a card name when the pool table
// cannot hold the choice (poolNamesFit): Pool plays them as inert fallbacks,
// and the Set says so (Inert), as it does for a card it does not lower.
// nil when no card needs it.
func nameInertCards(d *PoolData) []bool {
	var out []bool
	var fit [2]int8 // by anyCard: -1 unmeasured, 0 no, 1 yes
	fit = [2]int8{-1, -1}
	for i := 0; i < d.NumCards; i++ {
		names, anyCard := choosesNameKind(&d.Defs[i])
		if !names {
			continue
		}
		f := 0
		if anyCard {
			f = 1
		}
		if fit[f] < 0 {
			fit[f] = 0
			if poolNamesFit(d, anyCard) {
				fit[f] = 1
			}
		}
		if fit[f] == 0 {
			if out == nil {
				out = make([]bool, d.NumCards)
			}
			out[i] = true
		}
	}
	return out
}

// nameChoices fills Table.NameCards when a card of the table chooses a card
// name as it enters: the real nonland cards of the table (a matchup's cards,
// or the whole pool for the pool table), ascending by CardID. gorge lists the
// same cards' names sorted; the lockstep compares the two as sets. A matchup
// holding a multi-face card or a nameable CardID of 255 or more beside a name
// chooser is refused, as roomNameGuard refuses a Room beside a name reader,
// with an InteractionRefusal of reason "namechoice" (kfuzz counts it as a
// refused pair: either card plays without the other); the pool table instead plays the chooser inert when it cannot fit
// (nameInertCards), so it never reaches here with one.
func (s dataSet) nameChoices(t *kop.Table, pool []int, id map[int]kop.CardID) error {
	chooses, anyCard := false, false
	for i := range t.Cards {
		n, a := choosesNameKind(&t.Cards[i])
		chooses, anyCard = chooses || n, anyCard || a
	}
	if !chooses {
		return nil
	}
	for _, i := range pool {
		if i >= s.d.NumCards {
			continue // a token has no name a player may choose
		}
		c := id[i]
		cd := &t.Cards[c]
		if len(cd.Faces) > 0 {
			return nameChoiceRefusal(t, cd.Name, "a nameable card has more than one name")
		}
		if anyCard && activatesOffBattlefield(cd) {
			return nameChoiceRefusal(t, cd.Name, "an any-card name forbids an ability activated from off the battlefield, which the kernel does not model")
		}
		if c >= 255 && (anyCard || cd.Types&kop.TLand == 0) {
			return nameChoiceRefusal(t, cd.Name, fmt.Sprintf("a nameable card has CardID %d, past the chosen byte", c))
		}
		if anyCard {
			t.NameCardsAny = append(t.NameCardsAny, c)
		}
		if cd.Types&kop.TLand != 0 {
			continue
		}
		t.NameCards = append(t.NameCards, c)
	}
	sort.Slice(t.NameCards, func(a, b int) bool { return t.NameCards[a] < t.NameCards[b] })
	sort.Slice(t.NameCardsAny, func(a, b int) bool { return t.NameCardsAny[a] < t.NameCardsAny[b] })
	return nil
}

// nameChoiceRefusal is nameChoices' refusal of t: its name choosers beside the
// nameable card named, which cannot share a table with them.
func nameChoiceRefusal(t *kop.Table, named, why string) error {
	var cards []string
	for i := range t.Cards {
		if choosesName(&t.Cards[i]) && t.Cards[i].Name != named {
			cards = append(cards, t.Cards[i].Name)
		}
	}
	return &InteractionRefusal{Reason: "namechoice", Why: why, Cards: append(cards, named)}
}

// copyNamedGuard refuses a matchup table holding a name chooser beside a card
// that copies a card by its name (kop.ExtCopyNamed, Mutable Explorer): gorge
// resolves that name in the game's name universe, which a name chooser
// narrows to the matchup's own cards (oracle NameUniverseFor), so the copy
// would be made by the kernel and not by gorge when the named card is not
// among them. Like nameChoices it runs on a matchup table, never the pool's.
func copyNamedGuard(t *kop.Table) error {
	chooser, copier := "", ""
	for i := range t.Cards {
		cd := &t.Cards[i]
		if choosesName(cd) && chooser == "" {
			chooser = cd.Name
		}
		for _, ab := range cd.Abilities {
			for _, in := range ab.Prog {
				if in.Op == kop.OpExt && in.Ext() == kop.ExtCopyNamed && copier == "" {
					copier = cd.Name
				}
			}
		}
	}
	if chooser == "" || copier == "" {
		return nil
	}
	return &InteractionRefusal{Reason: "namechoice", Why: "a copy of a named card reads the whole card universe, which a card-name chooser narrows to the matchup's cards", Cards: []string{chooser, copier}}
}
