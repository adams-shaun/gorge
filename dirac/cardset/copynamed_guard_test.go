package cardset

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCopyNamedGuard: a matchup table holding a card-name chooser beside a
// named-card copier is refused (a chooser narrows gorge's name universe to the
// matchup's cards, which the kernel's copy does not see); either card alone,
// and the pool table, are not.
func TestCopyNamedGuard(t *testing.T) {
	chooser := kop.CardDef{Name: "Chooser", Abilities: []kop.Ability{{Prog: []kop.Instr{{Op: kop.OpExt, A: kop.Const(int(kop.ExtChooseName))}}}}}
	copier := kop.CardDef{Name: "Copier", Abilities: []kop.Ability{{Prog: []kop.Instr{{Op: kop.OpExt, A: kop.Const(int(kop.ExtCopyNamed))}}}}}
	plain := kop.CardDef{Name: "Plain"}
	for _, c := range []struct {
		name  string
		cards []kop.CardDef
		want  bool
	}{
		{"both", []kop.CardDef{plain, chooser, copier}, true},
		{"chooser only", []kop.CardDef{plain, chooser}, false},
		{"copier only", []kop.CardDef{plain, copier}, false},
	} {
		err := copyNamedGuard(&kop.Table{Cards: c.cards})
		var ir *InteractionRefusal
		if got := errors.As(err, &ir); got != c.want {
			t.Errorf("%s: refused = %v (%v), want %v", c.name, got, err, c.want)
		}
		if c.want && (ir.Reason != "namechoice" || len(ir.Cards) != 2) {
			t.Errorf("%s: refusal %+v", c.name, ir)
		}
	}
}
