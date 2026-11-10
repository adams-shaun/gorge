package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// ccLower lowers an inline mount whose attack trigger runs `copy` (the
// CopyPermanent line of its one effect) and returns its refusals.
func ccLower(t *testing.T, trigger, copyLine string) []string {
	t.Helper()
	src := "Name:Mount\nManaCost:4 R R\nTypes:Legendary Creature Horse Mount\nPT:4/6\nK:Haste\n" +
		"T:" + trigger + "\n" +
		"SVar:TrigCopy:" + copyLine + "\n" +
		"K:Saddle:1\n"
	card, diags := cards.ParseBytes("copychoice_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Mount"}, byName: map[string]*cards.Card{"Mount": card}, nDecks: map[string]int{}, reg: cards.NewRegistry()}
	var got []string
	for _, r := range lowerRefusals(lowerCard(p, "Mount", card, false)) {
		got = append(got, r.Reason)
	}
	return got
}

const (
	ccAttack = "Mode$ Attacks | ValidCard$ Card.Self+IsSaddled | TriggerZones$ Battlefield | Execute$ TrigCopy | TriggerDescription$ x"
	ccCopy   = "DB$ CopyPermanent | Choices$ Creature.nonLegendary+SaddledThisTurn | TokenTapped$ True | TokenAttacking$ True | AtEOT$ Sacrifice"
)

// TestLowerCopypermChoicesAttacking: CopyPermanent Choices$ lowers to the
// controller's one pick (OpChooseCard over the battlefield filter) and
// ExtCopyPicked over the pick, with the tapped, attacking and end-step
// sacrifice riders; Calamity, Galloping Inferno (the copy and its repeat)
// builds that pair twice; every shape the op does not carry refuses by name.
func TestLowerCopypermChoicesAttacking(t *testing.T) {
	t.Run("inline shape lowers", func(t *testing.T) {
		if r := ccLower(t, ccAttack, ccCopy); len(r) != 0 {
			t.Fatalf("refused: %v", r)
		}
	})
	t.Run("Calamity, Galloping Inferno", func(t *testing.T) {
		all, bs := keclBuild(t, []string{"Calamity, Galloping Inferno"})
		i := keclIndex(t, all, "Calamity, Galloping Inferno")
		if r := lowerRefusals(all[i]); len(r) != 0 {
			t.Fatalf("refused: %v", r)
		}
		var picks, copies int
		for _, ab := range bs[i].def.Abilities {
			for n := range ab.Prog {
				in := &ab.Prog[n]
				switch {
				case in.Op == kop.OpChooseCard:
					picks++
					if in.F == 0 || in.C != 0 {
						t.Errorf("pick F %d C %d: want a filter and the mandatory code", in.F, in.C)
					}
				case in.Op == kop.OpExt && in.Ext() == kop.ExtCopyPicked:
					copies++
					if n == 0 || ab.Prog[n-1].Op != kop.OpChooseCard || in.Obj != kop.RefChosen {
						t.Errorf("copy at %d does not read the pick before it (Obj %v)", n, in.Obj)
					}
					want := kop.CpTapped | kop.CpSacAtEndDly | kop.CopyAttacking
					if got := int(uint16(in.B.N)); got != want {
						t.Errorf("B.N %#x, want %#x (tapped, sacrifice at end step, attacking)", got, want)
					}
					if in.F != 0 || in.C != 0 {
						t.Errorf("F %d C %d: no filter and no copy exception", in.F, in.C)
					}
				}
			}
		}
		if picks != 2 || copies != 2 {
			t.Errorf("%d picks and %d copies, want the copy and its repeat (2 and 2)", picks, copies)
		}
	})
	for _, v := range []struct{ name, trigger, line, want string }{
		{"num copies", ccAttack, ccCopy + " | NumCopies$ 2", "Choices$ beside NumCopies$"},
		{"target", ccAttack, ccCopy + " | ValidTgts$ Opponent", "Choices$ beside ValidTgts$"},
		{"defined", ccAttack, ccCopy + " | Defined$ Self", "Choices$ beside Defined$"},
		{"counters", ccAttack, ccCopy + " | WithCountersType$ P1P1", "Choices$ beside WithCountersType$"},
		{"remember", ccAttack, ccCopy + " | RememberTokens$ True", "Choices$ beside RememberTokens$"},
		{"controller", ccAttack, ccCopy + " | Controller$ Opponent", "Choices$ beside Controller$"},
		{"copy exception", ccAttack, ccCopy + " | NonLegendary$ True", "Choices$ beside copy exceptions"},
		{"attacking selector", ccAttack, strings.Replace(ccCopy, "TokenAttacking$ True", "TokenAttacking$ Opponent", 1), `Choices$ with TokenAttacking$ "Opponent"`},
		{"attacking outside the own attack trigger", "Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigCopy | TriggerDescription$ x", ccCopy, "TokenAttacking$ True in a trigger other than the controller's own attack"},
		{"end step trigger rider", ccAttack, ccCopy + " | AtEOTTrig$ Sacrifice", "Choices$ beside AtEOTTrig$"},
	} {
		t.Run(v.name, func(t *testing.T) {
			kdefWant(t, ccLower(t, v.trigger, v.line), v.want)
		})
	}
}
