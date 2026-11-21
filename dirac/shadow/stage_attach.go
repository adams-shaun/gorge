package shadow

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/view"
)

// tokenCard finds the table's token for a token the view shows. A token's
// view carries its name ("Goblin Token") and printed power and toughness; the table names its
// tokens by script ("r_1_1_goblin": colours, power, toughness, name). The
// match is the one table token whose script ends in the view's name and whose
// power and toughness are the printed ones, and none when there are zero or
// several: a token this package cannot name exactly is not staged.
func (b *builder) tokenCard(cv *view.CardView) (kop.CardID, bool) {
	return tableToken(b.in.Tbl, cv)
}

// tableToken is tokenCard over a table: the Layout names the tokens it has seen
// by it.
func tableToken(tbl *kop.Table, cv *view.CardView) (kop.CardID, bool) {
	want := "_" + strings.ReplaceAll(strings.ToLower(strings.TrimSuffix(cv.Name, " Token")), " ", "_")
	found := kop.CardID(0)
	n := 0
	for i := range tbl.Cards {
		d := &tbl.Cards[i]
		if d.Types&kop.TToken == 0 || !strings.HasSuffix(d.Name, want) || fmt.Sprintf("%d/%d", d.Power, d.Toughness) != cv.PrintedPT {
			continue
		}
		found = kop.CardID(i)
		n++
	}
	return found, n == 1
}

// markTokenAttach sets o's Token and attachment fields from the view of the
// card. A token is always on the battlefield (checkCard); an attachment is
// refused unless the card is an Aura or Equipment, and its bearer maps to an
// object index.
func (b *builder) markTokenAttach(o *dirac.WorldObj, cv *view.CardView, name string) error {
	o.Token = cv.IsToken
	if cv.AttachedTo == 0 {
		return nil
	}
	if b.in.Tbl.Cards[o.Card].Types&(kop.TAura|kop.TEquipment) == 0 {
		return bad(ReasonAttach, "%s is not an Aura or Equipment", name)
	}
	host, ok := b.in.ObjOf(int(cv.AttachedTo))
	if !ok {
		return bad(ReasonUnmapped, "%s: its bearer (%d)", name, cv.AttachedTo)
	}
	o.Attached, o.Host = true, host
	return nil
}

// ceased adds the tokens the caller says have ceased to exist: each takes its
// object index with no zone (dirac.WorldObj Zone ZoneNone) and carries no mark.
// The name is the table token's script name (the card tokenCard resolves).
func (b *builder) ceased() error {
	for _, c := range b.in.Hidden.Ceased {
		id, ok := b.tokenByName(c.Name)
		if !ok {
			return bad(ReasonToken, "ceased token %d: no table token %q", c.Obj, c.Name)
		}
		if int(c.Owner) >= len(b.in.View.Players) {
			return bad(ReasonSeats, "ceased token %d: seat %d", c.Obj, c.Owner)
		}
		b.all = append(b.all, placed{c.Obj, dirac.WorldObj{Card: id, Owner: c.Owner, Zone: kop.ZoneNone, Ctrl: c.Owner, Token: true}})
	}
	return nil
}

// tokenByName is the table token whose script name is name; the scripts of a
// table's tokens are unique.
func (b *builder) tokenByName(name string) (kop.CardID, bool) {
	for i := range b.in.Tbl.Cards {
		if d := &b.in.Tbl.Cards[i]; d.Types&kop.TToken != 0 && d.Name == name {
			return kop.CardID(i), true
		}
	}
	return 0, false
}
