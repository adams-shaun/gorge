package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestMustAttackOtherPodOffersPlayersOnly: a MustAttack$ Player.Other
// requirement with another legal player defender keeps the player pairs and
// withholds every non-player pair, as gorge's named-duty offer filter does
// (attack_cost.go SatisfiedByOffer: a walker or battle pair satisfies no
// named duty while a legal player pair satisfies one). The exclusion is the
// kernel static's B mark; the emblem is controlled by seat 0 and the
// attacker by seat 1, so seat 2's planeswalker is a legal non-player pair
// that must not be offered.
func TestMustAttackOtherPodOffersPlayersOnly(t *testing.T) {
	tbl := *v.Table
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Emblems = append([]kop.Emblem(nil), v.Table.Emblems...)
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
	creatures := kop.FilterID(len(tbl.Filters) - 1)
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test MustAttack", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{
			Kind:   kop.AbStatic,
			Static: kop.Static{Kind: kop.StatMustAttack, Filter: creatures, B: 1},
		}}})
	var decks [MaxSeats][]CardID
	var lib [20]CardID
	for i := range lib {
		lib[i] = v.Plains
	}
	for p := range decks {
		decks[p] = lib[:]
	}
	s := new(State)
	s.InitN(&tbl, decks, 4, 1, Config{NoElide: true})
	for p := uint8(0); p < 4; p++ {
		pl := &s.pl[p]
		pl.life = 20
		for pl.handN > 0 {
			o := pl.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pl.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 1, kop.StepMain1, 1, 0, ctlPriority
	sc := &scen{s: s, k: s.k()}
	// seat 0 owns the MustAttack$ Player.Other static
	sc.perm(CardID(len(tbl.Cards)-1), 0)
	bear := sc.perm(v.GrizzlyBears, 1)  // the affected attacker
	walker := sc.perm(v.EmberWalker, 2) // another opponent's planeswalker, a legal defender seat
	sc.passUntil(t, kop.StepAttackers)
	d := s.Step()
	if d.Kind != DkAttackers {
		t.Fatalf("decision %d, want attackers", d.Kind)
	}
	players, nonPlayers := 0, 0
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() != AkAttack || a.Source() != TCode(bear) {
			continue
		}
		if a.Target() == TCode(walker) {
			nonPlayers++
			continue
		}
		if a.Target() >= PlayerTarget0 {
			players++
		}
	}
	if players != 2 {
		t.Errorf("%d player pairs for the must-attack creature, want 2 (seats 2 and 3; seat 0 is excluded)", players)
	}
	if nonPlayers != 0 {
		t.Errorf("%d non-player pairs offered under MustAttack$ Player.Other, want 0", nonPlayers)
	}
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkAttack && a.Source() == TCode(bear) && a.Target() == PlayerCode(0) {
			t.Fatalf("the static controller's player pair was offered")
		}
	}
}

// TestMustAttackOtherPodSkipsBattleOffers: the pod offer walk is players only
// under the exclusion, so a legal battle pair is not added either (the
// attackOffersN playerOnly return gates attackerOptions' addBattleOffers).
func TestMustAttackOtherPodSkipsBattleOffers(t *testing.T) {
	var staticID, battleID, bearID CardID
	sc, _ := newSetScenTweak(t, "fdn", []cardset.Entry{{Name: "Plains", Count: 8}},
		func(tbl *kop.Table, ids map[string]CardID) {
			tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
			creatures := kop.FilterID(len(tbl.Filters) - 1)
			tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test MustAttack", Types: kop.TEnchantment,
				Abilities: []kop.Ability{{Kind: kop.AbStatic,
					Static: kop.Static{Kind: kop.StatMustAttack, Filter: creatures, B: 1}}}})
			staticID = CardID(len(tbl.Cards) - 1)
			tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
			bearID = CardID(len(tbl.Cards) - 1)
			tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Battle", Types: kop.TBattle, Defense: 3})
			battleID = CardID(len(tbl.Cards) - 1)
			ids["Test MustAttack"], ids["Test Bear"], ids["Test Battle"] = staticID, bearID, battleID
		})
	s, k := sc.s, sc.k
	if k.gen == nil {
		t.Fatal("precondition: no generated extension")
	}
	k.gen.anyBattle = true // the appended battle card may postdate the table's feature scan
	s.g.nseat = 4
	for p := uint8(0); p < 4; p++ {
		s.pl[p].life = 20
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 1, kop.StepMain1, 1, 0, ctlPriority
	sc.perm(staticID, 0)       // the static's controller, seat 0, is excluded
	bear := sc.perm(bearID, 1) // the affected attacker
	battle := sc.perm(battleID, 2)
	s.answerQueuedAsk(k, MakeAction(AkChoose, 0, 0, 0)) // seat 2's protector ask (face_battle.go): seat 0 protects
	sc.passUntil(t, kop.StepAttackers)
	d := s.Step()
	if d.Kind != DkAttackers {
		t.Fatalf("decision %d, want attackers", d.Kind)
	}
	players, battles := 0, 0
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() != AkAttack || a.Source() != TCode(bear) {
			continue
		}
		if a.Target() == TCode(battle) {
			battles++
			continue
		}
		if a.Target() >= PlayerTarget0 {
			players++
		}
	}
	if players != 2 {
		t.Errorf("%d player pairs, want 2 (seats 2 and 3; seat 0 is excluded)", players)
	}
	if battles != 0 {
		t.Errorf("%d battle pairs offered under MustAttack$ Player.Other, want 0", battles)
	}
}
