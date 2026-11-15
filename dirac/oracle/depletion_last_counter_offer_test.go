package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// A depletion land on its last counter funds a cast (CR 605.3b: the mana
// ability's rider sacrifices it once no counter is left, and the activation
// is legal). The board is the one kfuzz pair 39 (dr-mbc-bb) reported: the
// active player holds a {3}{R}{R} spell (Earth Elemental here, a vanilla
// with the same cost) with Mountain, Mountain and Island untapped beside a
// Sandstone Needle holding one depletion counter, so the three lands make 3
// and only the Needle's {R}{R} completes the 5.
var (
	depletionLastCounterSpell = "Earth Elemental"
	depletionLastCounterDecks = [2][]string{
		append(deckOf(8, "Sandstone Needle"), append(deckOf(5, "Mountain"), append(deckOf(5, "Island"), deckOf(6, depletionLastCounterSpell)...)...)...),
		deckOf(24, "Plains"),
	}
)

// depletionLastCounterBoard reports whether d is the ticket's state: p's
// sorcery-speed priority, the spell in hand, exactly one untapped Sandstone
// Needle, on its last depletion counter, and exactly Mountain, Mountain and
// Island as p's other untapped lands.
func depletionLastCounterBoard(g *state.Game, d *decision.Decision) bool {
	p := d.Player
	if g.Active != p || !g.Step.IsMain() || len(g.Zone(state.ZStack, p)) != 0 {
		return false
	}
	inHand := false
	for _, id := range g.Zone(state.ZHand, p) {
		if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == depletionLastCounterSpell {
			inHand = true
		}
	}
	if !inHand {
		return false
	}
	needles, lastCounter := 0, false
	var others []string
	for _, id := range g.Zone(state.ZBattlefield, p) {
		o := g.Obj(id)
		if o == nil || o.Face() == nil || o.Tapped || o.Controller != p {
			continue
		}
		switch n := o.Face().Name; n {
		case "Sandstone Needle":
			needles++
			lastCounter = o.Counter("DEPLETION") == 1
		case "Mountain", "Island":
			others = append(others, n)
		}
	}
	slices.Sort(others)
	return needles == 1 && lastCounter && slices.Equal(others, []string{"Island", "Mountain", "Mountain"})
}

// TestDepletionLastCounterOffer plays seeded games of the Needle deck until
// the ticket's board is reached at least three times. At each such decision
// gorge's potential planner must price the cast (CR 605.3b) and the kernel's
// posed priority must list a cast of the same hand card; the lockstep itself
// requires every other decision's options to agree. It fails if no seed
// reaches the board, so it cannot pass vacuously.
func TestDepletionLastCounterOffer(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	set := printedDelveSet(t, depletionLastCounterDecks)
	name := "depletion_last_counter_offer"
	c := effectLongCase{name: name, prefer: []string{depletionLastCounterSpell}, decks: depletionLastCounterDecks, surface: "full"}
	witnessed := 0
	for seed := uint64(1); seed <= 400 && witnessed < 3; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{name + "_0", name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		tbl, _, _, err := set.Matchup(entryList(r.Cards[0]), entryList(r.Cards[1]))
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions = 2000
		rc.MaxTurns = 16
		rc.Check = func(ref *GorgeSide, k Side) *Divergence {
			d := ref.Pending()
			if d == nil || d.Kind != decision.KPriority || !depletionLastCounterBoard(ref.E.G, d) {
				return nil
			}
			gorgeOffers := false
			for _, pp := range ref.E.PotentialPaymentPlans(d.Player) {
				if pp.Action.Kind == "cast" && pp.Action.Label == "Cast "+depletionLastCounterSpell {
					gorgeOffers = true
				}
			}
			_, _, kopts := KernelStateOf(k)
			if kopts == nil {
				return &Divergence{What: "options", Field: "kernel", Detail: "not the real kernel side"}
			}
			kernelOffers := false
			for _, id := range ref.E.G.Zone(state.ZHand, d.Player) {
				o := ref.E.G.Obj(id)
				if o == nil || o.Face() == nil || o.Face().Name != depletionLastCounterSpell {
					continue
				}
				want := Pack(ActCast, ref.InstRef(id), RefNone, 0)
				for _, kc := range kopts {
					if kc.Kind() == want.Kind() && kc.Src() == want.Src() {
						kernelOffers = true
					}
				}
			}
			if !gorgeOffers || !kernelOffers {
				return &Divergence{What: "options", Field: "options",
					Detail: "Needle on its last counter: gorge offers the cast " + yesNo(gorgeOffers) + ", the kernel " + yesNo(kernelOffers)}
			}
			witnessed++
			return nil
		}
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, effectLongPolicy(m, c, seed), rc)
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	if witnessed < 3 {
		t.Fatalf("the Needle-on-its-last-counter board was reached %d times in 400 seeds, want 3", witnessed)
	}
	t.Logf("board reached %d times; both engines offered the cast each time", witnessed)
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
