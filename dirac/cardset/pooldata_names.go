package cardset

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// nameChoices fills Table.NameCards when a card of the matchup chooses a
// card name as it enters (ExtChooseName: Kai Winn, Faithless Leader): the
// real nonland cards of the matchup, ascending by CardID, which is gorge's
// sorted name list over the same cards. The chosen name is kept in the
// permanent's chosen byte as a CardID, so every id must fit below the byte's
// "none" value 255. A multi-face card (a split, adventure, double-faced or
// prepare card) has more nameable names than one CardID, so a matchup holding
// one beside a name chooser is refused, as roomNameGuard refuses a Room beside
// a name reader.
func (s dataSet) nameChoices(t *kop.Table, pool []int, id map[int]kop.CardID) error {
	chooses := false
	for i := range t.Cards {
		for _, ab := range t.Cards[i].Abilities {
			for _, in := range ab.Prog {
				chooses = chooses || in.Op == kop.OpExt && in.Ext() == kop.ExtChooseName
			}
		}
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
			return fmt.Errorf("matchup holds a card that chooses a card name and the multi-face card %q", cd.Name)
		}
		if cd.Types&kop.TLand != 0 {
			continue
		}
		if c >= 255 {
			return fmt.Errorf("matchup holds a card that chooses a card name and %q has CardID %d", cd.Name, c)
		}
		t.NameCards = append(t.NameCards, c)
	}
	return nil
}
