package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestSnowTally: the kernel tallies the snow units of a player's pool beside
// the Treasure tally (gorge's state.Player.Snow beside Player.Pool; CR 106.3,
// 107.4h). gorge tags a unit snow when its source's types hold Snow and no
// typed word (effects.ManaProducerTag), and spends a plain unit of a colour
// before a tagged one (pay.takeUnit), so after any spend the snow units left
// in a colour are min(Snow[c], Pool[c]) — the same clamp treSync applies to
// the Treasure tally (dirac/mana_artifact.go), with the Treasure half reading
// the snow tally out of the slot first.
func snowScen(t *testing.T) (*scen, snowCards) {
	t.Helper()
	tbl := v.NewTable()
	c := snowCards{
		snowLand: CardID(len(tbl.Cards)),
		typed:    CardID(len(tbl.Cards) + 1),
		ask:      CardID(len(tbl.Cards) + 2),
	}
	// a snow basic land: its {W} is snow mana (CardDef.Snow, manaTyped false)
	// and its cost's {S} pip makes the prep set genPrep.anySnow
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Snow Land", Types: kop.TLand | kop.TBasic, Snow: true, SpentSnow: true, Timing: kop.TimingLand,
			Abilities: []kop.Ability{{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColW, ManaN: 1}}},
		// a snow artifact: its types hold a typed word, which wins over Snow
		// (gorge's ManaProducerTag), so its {W} tags typed and adds no tally
		kop.CardDef{Name: "Test Snow Relic", Cost: mc2(2, kop.W), Types: kop.TArtifact | kop.TCreature, Colors: kop.ColW, Power: 1, Toughness: 1,
			Abilities: []kop.Ability{{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColW, ManaN: 1}}},
		// a card whose activated ability asks {S} (Mana.Snow = 1): prep reads
		// it and sets genPrep.anySnow even though this ticket pays no {S}
		kop.CardDef{Name: "Test Snow Ask", Cost: mc2(2, kop.W), Types: kop.TCreature, Colors: kop.ColW, Power: 1, Toughness: 1,
			Abilities: []kop.Ability{{Kind: kop.AbActivated,
				Cost: kop.Cost{Mana: kop.ManaCost{Snow: 1}, Flags: kop.CostTap},
				Prog: []kop.Instr{{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1)}}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if sc.k.gen == nil {
		t.Fatal("precondition: the table has no generated extension")
	}
	if !sc.k.gen.anySnow {
		t.Fatal("precondition: genPrep.anySnow is not set")
	}
	if sx := coldSnow.of(sc.s); len(sx) != MaxSeats*kop.NumManaKinds {
		t.Fatalf("precondition: the snow tally is %d bytes, want %d", len(sx), MaxSeats*kop.NumManaKinds)
	}
	if !sc.k.cards[c.snowLand].manaSnow || sc.k.cards[c.typed].manaSnow {
		t.Fatal("precondition: the snow land is not manaSnow or the artifact is")
	}
	return sc, c
}

type snowCards struct{ snowLand, typed, ask CardID }

// mc2 builds a ManaCost of generic + one pip of colour c (vanilla's mc is not
// exported).
func mc2(generic int, c int) kop.ManaCost {
	var m kop.ManaCost
	m.Generic = uint8(generic)
	m.Pips[c]++
	return m
}

// tally reads seat p's snow tally of colour c through the State accessor.
func tally(s *State, p uint8, c int) uint8 {
	return s.snowAt(p, c)
}

func TestSnowTally(t *testing.T) {
	sc, c := snowScen(t)
	land := sc.perm(c.snowLand, 0)
	sl := sc.slot(land)

	t.Run("snow_unit_tags", func(t *testing.T) {
		if !sc.s.tapOneForMana(sc.k, 0, sl) {
			t.Fatal("the snow land does not tap for mana")
		}
		if got := sc.s.pl[0].pool[kop.W]; got != 1 {
			t.Fatalf("precondition: pool W %d, want 1", got)
		}
		if got := tally(sc.s, 0, kop.W); got != 1 {
			t.Fatalf("snow tally %d, want 1", got)
		}
		if sc.s.g.poolArt[0]&poolArtSnow == 0 {
			t.Fatal("poolArtSnow is not set")
		}
	})

	t.Run("plain_unit_spent_first", func(t *testing.T) {
		plains := sc.perm(v.Plains, 0)
		if !sc.s.tapOneForMana(sc.k, 0, sc.slot(plains)) {
			t.Fatal("the Plains does not tap for mana")
		}
		sc.s.payFromPool(0, &kop.ManaCost{Generic: 1}, 0)
		if got := sc.s.pl[0].pool[kop.W]; got != 1 {
			t.Fatalf("pool W %d, want 1", got)
		}
		if got := tally(sc.s, 0, kop.W); got != 1 {
			t.Fatalf("snow tally %d, want 1: the plain unit of the colour goes first", got)
		}
		if sc.s.g.poolArt[0]&poolArtSnow == 0 {
			t.Fatal("poolArtSnow is not set after the first spend")
		}
	})

	t.Run("last_unit_empties_the_tally", func(t *testing.T) {
		sc.s.payFromPool(0, &kop.ManaCost{Generic: 1}, 0)
		if got := sc.s.pl[0].pool[kop.W]; got != 0 {
			t.Fatalf("pool W %d, want 0", got)
		}
		if got := tally(sc.s, 0, kop.W); got != 0 {
			t.Fatalf("snow tally %d, want 0", got)
		}
		if sc.s.g.poolArt[0]&poolArtSnow != 0 {
			t.Fatal("poolArtSnow is still set")
		}
	})

	t.Run("typed_snow_source_tags_no_tally", func(t *testing.T) {
		relic := sc.perm(c.typed, 0)
		if !sc.s.tapOneForMana(sc.k, 0, sc.slot(relic)) {
			t.Fatal("the snow artifact does not tap for mana")
		}
		if got := sc.s.pl[0].pool[kop.W]; got != 1 {
			t.Fatalf("precondition: pool W %d, want 1", got)
		}
		if got := tally(sc.s, 0, kop.W); got != 0 {
			t.Fatalf("snow tally %d, want 0: the typed word wins over Snow", got)
		}
		if sc.s.g.poolArt[0]&poolArtSnow != 0 {
			t.Fatal("poolArtSnow is set")
		}
	})

	t.Run("treasure_unit_goes_first", func(t *testing.T) {
		land2 := sc.perm(c.snowLand, 0) // a fresh snow land: the first is still tapped
		sc.s.pl[0].pool[kop.W] = 0
		if !sc.s.tapOneForMana(sc.k, 0, sc.slot(land2)) {
			t.Fatal("the snow land does not tap for mana")
		}
		treasure := sc.perm(v.TreasureToken, 0)
		// the vanilla token carries no Treasure subtype (csTreasure needs a
		// cast-ring table's prep), so the test marks it the way a real
		// Treasure token's prep does (trig_validsa.go markTreasureSrc)
		sc.k.cards[treasure].castSrc |= csTreasure
		if !sc.s.tapOneForMana(sc.k, 0, sc.slot(treasure)) {
			t.Fatal("the Treasure does not tap for mana")
		}
		if got := sc.s.pl[0].pool[kop.W]; got != 2 {
			t.Fatalf("precondition: pool W %d, want 2", got)
		}
		if got := sc.s.g.poolTre[0][kop.W]; got != 1 {
			t.Fatalf("precondition: Treasure tally %d, want 1", got)
		}
		if got := tally(sc.s, 0, kop.W); got != 1 {
			t.Fatalf("precondition: snow tally %d, want 1", got)
		}
		sc.s.payFromPool(0, &kop.ManaCost{Generic: 1}, 0)
		if got := sc.s.pl[0].pool[kop.W]; got != 1 {
			t.Fatalf("pool W %d, want 1", got)
		}
		if got := sc.s.g.poolTre[0][kop.W]; got != 0 {
			t.Fatalf("Treasure tally %d, want 0: the Treasure unit of the colour goes before the snow one", got)
		}
		if got := tally(sc.s, 0, kop.W); got != 1 {
			t.Fatalf("snow tally %d, want 1", got)
		}
	})

	t.Run("end_step_empties_the_tally", func(t *testing.T) {
		sc.s.endStep(kop.StepMain2)
		if got := tally(sc.s, 0, kop.W); got != 0 {
			t.Fatalf("snow tally %d, want 0 after the step ends", got)
		}
		if sc.s.g.poolArt[0]&poolArtSnow != 0 {
			t.Fatal("poolArtSnow is still set after the step ends")
		}
	})

	t.Run("payment_allocates_nothing", func(t *testing.T) {
		land3 := sc.perm(c.snowLand, 0) // a fresh snow land: the others are tapped
		if !sc.s.tapOneForMana(sc.k, 0, sc.slot(land3)) {
			t.Fatal("the snow land does not tap for mana")
		}
		n := testing.AllocsPerRun(100, func() {
			sc.s.pl[0].pool[kop.W] = 2
			sc.s.snowSetAt(0, kop.W, 1)
			sc.s.g.poolArt[0] |= poolArtSnow
			sc.s.artTag(sc.k, 0, sl, kop.W)
			sc.s.payFromPool(0, &kop.ManaCost{Generic: 1}, 0)
		})
		if n != 0 {
			t.Fatalf("artTag/payFromPool allocates %.0f", n)
		}
		if got := tally(sc.s, 0, kop.W); got != 1 {
			t.Fatalf("snow tally %d, want 1", got)
		}
	})
}
