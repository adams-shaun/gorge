package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAuralossWidenReadsNoRestrictedAbility: a permanent that lost all
// abilities (CR 613.1h) lost its printed restricted mana ability with them,
// so the widen record that joins a restricted ability to its census unit is
// dead. What survived is a granted ability (Honest Work's "{T}: Add {C}" on
// the enchanted land), and the restricted view must not widen it into the
// restricted ability's colours: the planner would otherwise price the lost
// source's Lesson-only any colour into a Lesson cast, gorge never lists it
// (its census holds no restricted source there). The std-tla lockstep pair
// plays the shape with the real cards: p0's earthbent White Lotus Hideout,
// p1's Honest Work on it, and Azula Always Lies in hand (its recording is
// aura_loss_widened_mana.repro.json).
//
// The scenario is built over the fdn table with the shape appended, as
// static_layerguards_test.go's do:
//   - Savannah Lions carries White Lotus Hideout's two abilities: the census
//     "{T}: Add {C}" and the restricted "{T}: Add one mana of any colour.
//     Spend this mana only to cast a Lesson or Shrine spell";
//   - Witness Protection's override (its Aura slot is the override's) is
//     Honest Work's: the enchanted creature loses all abilities, is a 1/1
//     Citizen, and gains the Aura's "{T}: Add {C}";
//   - a Test Lesson card stands in for Azula Always Lies: an instant Lesson
//     costing {1}{B}.
func TestAuralossWidenReadsNoRestrictedAbility(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Witness Protection", Count: 1}, {Name: "Savannah Lions", Count: 1},
		{Name: "Plains", Count: 10},
	}
	fdnSet := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := fdnSet.Lookup(e.Name); !ok || !fdnSet.Supported()[i] {
			t.Skipf("%s not in the fdn table", e.Name)
		}
	}
	tbl, da, _, err := fdnSet.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	l := &auraLossScen{tbl: *tbl, id: map[string]CardID{}}
	l.tbl.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	l.tbl.Filters = append([]kop.Filter(nil), tbl.Filters...)
	l.tbl.Overrides = append([]kop.Override(nil), tbl.Overrides...)
	l.tbl.Subtypes = append([]string(nil), tbl.Subtypes...)
	for _, c := range da {
		l.id[l.tbl.Cards[c].Name] = c
	}
	l.sub = map[string]int{}
	for i, s := range l.tbl.Subtypes {
		l.sub[s] = i
	}
	for _, s := range []string{"Citizen", "Lesson"} {
		if _, ok := l.sub[s]; !ok {
			l.tbl.Subtypes = append(l.tbl.Subtypes, s)
			l.sub[s] = len(l.tbl.Subtypes) - 1
		}
	}
	// White Lotus Hideout's two abilities on the enchanted permanent.
	l.tbl.Filters = append(l.tbl.Filters, kop.Filter{Sub: uint8(l.sub["Lesson"] + 1)})
	restr := kop.FilterID(len(l.tbl.Filters) - 1)
	lions := &l.tbl.Cards[l.id["Savannah Lions"]]
	lions.Abilities = append(lions.Abilities,
		kop.Ability{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColC, ManaN: 1},
		kop.Ability{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColAny, ManaN: 1, ManaRestrict: restr})
	// Honest Work's override on the Aura: the loss, the 1/1 Citizen and the
	// granted "{T}: Add {C}" (the Aura's own next ability, Grant index+1).
	auraCard := &l.tbl.Cards[l.id["Witness Protection"]]
	grant := uint8(len(auraCard.Abilities))
	auraCard.Abilities = append(auraCard.Abilities,
		kop.Ability{Kind: kop.AbMana, AFlags: kop.AfLatent, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColC, ManaN: 1})
	ov := l.override(kop.Override{HasPT: true, SetP: 1, SetT: 1, AddSub: l.mask("Citizen"), LoseAbilities: true, Grant: [2]uint8{grant + 1}})
	for i := range auraCard.Abilities {
		a := &auraCard.Abilities[i]
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatOverride {
			a.Static.Override = uint16(ov + 1)
			break
		}
	}
	// Azula Always Lies: an instant Lesson costing {1}{B}.
	lesson := CardID(len(l.tbl.Cards))
	l.tbl.Cards = append(l.tbl.Cards, kop.CardDef{Name: "Test Lesson", Cost: kop.ManaCost{Generic: 1, Pips: [6]uint8{0, 0, 1, 0, 0, 0}},
		Types: kop.TInstant, Subtypes: l.mask("Lesson")})

	l.start(t)
	host := l.perm(l.id["Savannah Lions"], 0)
	aura := l.wear(l.id["Witness Protection"], host)
	l.sc.s.objs[aura].setOwner(1) // the Aura enchants an opponent's creature
	l.sc.s.pl[0].pool[kop.G] = 1  // the {1} of {1}{B} the pool pays
	s, k := l.sc.s, l.sc.k

	// precondition: the Aura is attached, the host lost its abilities, its
	// census unit is the granted {T}: Add {C}, and the card's restricted
	// ability is the widen record's (Lesson or Shrine only)
	if s.perms[l.sc.slot(aura)].attached != l.sc.slot(host) {
		t.Fatal("the Aura is not attached to the host")
	}
	if !s.genLost(k, l.sc.slot(host)) {
		t.Fatal("the host has not lost its abilities: the board does not carry the loss")
	}
	own := s.manaPCGen(k, l.sc.slot(host))
	if own.manaAb < 0 || own.manaCol != kop.ColC || own.manaChosen {
		t.Fatalf("the host's census view is manaAb=%d manaCol=%#x, want the granted {T}: Add {C} (grantIdx=%d grantMay=%v grantPC=%d anyGrant=%v grantsLive=%v)",
			own.manaAb, own.manaCol, s.grantIdx(k, l.sc.slot(host)), s.grantMay(k, l.sc.slot(host)), len(k.gen.grantPC), k.gen.anyGrant, s.grantsLive(k))
	}
	if k.gen.flags[l.id["Savannah Lions"]]&gfRestrSrc == 0 || len(k.gen.rs.recs) == 0 {
		t.Fatal("the host holds no restricted-source record: the board does not carry the widen")
	}

	// the widen must be dead: the Lesson spell the planner prices over this
	// board sees no restricted source at the host, so {1}{B} is not payable
	// there (the pool's {G} pays only the {1})
	d := payDesc{class: pdSpell, card: lesson, src: NoSlot}
	if v, kind, _ := s.restrView(k, 0, l.sc.slot(host), own, &d); v != nil {
		t.Fatalf("the restricted view widened the lost host's census unit (kind=%d): the Lesson-only any colour prices a Lesson the board cannot pay", kind)
	}
	cost := kop.ManaCost{Generic: 1, Pips: [6]uint8{0, 0, 1, 0, 0, 0}} // {1}{B}
	if s.affordsUngrantedCast(k, 0, &cost, d) {
		t.Fatal("the ungranted census pays {1}{B}: the widen read made a black source of the granted {C}")
	}
}

// auraLossScen is static_layerguards_test.go's l7scen: a table over the fdn
// set with the shape appended, and a scenario at seat 0's main phase.
type auraLossScen struct {
	tbl kop.Table
	id  map[string]CardID
	sub map[string]int
	sc  *scen
}

func (l *auraLossScen) mask(subs ...string) kop.SubMask {
	var m kop.SubMask
	for _, s := range subs {
		m = m.With(l.sub[s])
	}
	return m
}

func (l *auraLossScen) override(o kop.Override) int {
	l.tbl.Overrides = append(l.tbl.Overrides, o)
	return len(l.tbl.Overrides) - 1
}

func (l *auraLossScen) start(t *testing.T) {
	l.sc = newScenTable(t, &l.tbl, Config{NoElide: true})
}

func (l *auraLossScen) perm(c CardID, p uint8) Obj { return l.sc.perm(c, p) }

func (l *auraLossScen) wear(aura CardID, host Obj) Obj { return l.sc.wear(aura, host) }
