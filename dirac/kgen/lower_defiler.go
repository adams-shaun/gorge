package kgen

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Defiler cycle's board OptionalCost static (Defiler of Dreams, Faith,
// Flesh, Instinct, Vigor): S:Mode$ OptionalCost | ValidCard$ Permanent.<Colour>
// | ValidSA$ Spell | Activator$ You | Cost$ PayLife<N> | ReduceColor$ <C>.
// "As an additional cost to cast <colour> permanent spells, you may pay N
// life. Those spells cost {C} less to cast if you paid life this way. This
// effect reduces only the amount of <colour> mana you pay." gorge poses one
// yes/no per applicable static (rules defiler_reducecolor.go defilerAsk);
// the kernel keeps the static as kop.StatDefiler and asks at bsDefiler
// (dirac/cost_defiler.go). Any other board OptionalCost (another cost part, an
// Activator$ other than You, a ReduceColor$ outside WUBRG) refuses by name;
// the card's own OptionalCost (Card.Self, EffectZone$ All) stays in
// lower_optcost.go.

var defilerLife = regexp.MustCompile(`^PayLife<([1-9][0-9]?)>$`)

// defilerShape reports whether the OptionalCost static is a board static
// with Activator$ or ReduceColor$ (not the card's own optional cost).
func defilerShape(s cards.Static) bool {
	_, act := s.Params["Activator"]
	_, red := s.Params["ReduceColor"]
	return strings.TrimSpace(s.Params["ValidCard"]) != "Card.Self" && (act || red)
}

// defilerStatic lowers the board OptionalCost static into lc.out.Statics.
func (lc *lowerCtx) defilerStatic(s cards.Static) error {
	for _, k := range sortedParamKeys(s.Params) {
		switch k {
		case "Mode", "ValidCard", "ValidSA", "Activator", "Cost", "ReduceColor", "Description":
		default:
			return refuse("OptionalCost board static with %s$", k)
		}
	}
	if a := strings.TrimSpace(s.Params["Activator"]); a != "You" {
		return refuse("OptionalCost board static Activator %q (only You)", a)
	}
	col := strings.TrimSpace(s.Params["ReduceColor"])
	ci := strings.Index("WUBRG", col)
	if len(col) != 1 || ci < 0 {
		return refuse("OptionalCost board static ReduceColor %q (only W, U, B, R or G)", col)
	}
	m := defilerLife.FindStringSubmatch(strings.TrimSpace(s.Params["Cost"]))
	if m == nil {
		return refuse("OptionalCost board static Cost %q (only PayLife<N>)", s.Params["Cost"])
	}
	if strings.TrimSpace(s.Params["ValidSA"]) != "Spell" {
		return refuse("OptionalCost board static ValidSA %q (only Spell)", s.Params["ValidSA"])
	}
	n, _ := strconv.Atoi(m[1])
	f, err := lc.filter(strings.TrimSpace(s.Params["ValidCard"]))
	if err != nil {
		return err
	}
	lc.out.Statics = append(lc.out.Statics, lStatic{Kind: "Defiler", F: f, Amount: cst(n), Defiler: uint8(ci) + 1})
	return nil
}

func init() {
	registerStaticBuild("Defiler", cbDefilerStatic)
}

// cbDefilerStatic builds kop.StatDefiler: Filter the spells it prices (read
// as a spell, as a ReduceCost's), A the life, B the colour index.
func cbDefilerStatic(cb *cardBuilder, s lStatic, st *kop.Static) ([]uint8, error) {
	var err error
	st.Kind = kop.StatDefiler
	if st.Filter, err = cb.staticFilter(s.F, kop.ZoneStack); err != nil {
		return nil, err
	}
	st.A = int16(s.Amount.N)
	st.B = int16(s.Defiler - 1)
	return nil, nil
}
