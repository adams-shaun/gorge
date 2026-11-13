package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSelfCondReadKindsGuard drives the layer walk through the re-entry guard
// (dirac/static_reentryguard.go) for every characteristic a static's
// condition can read: type, subtype, colour, chosen type, chosen colour,
// power/toughness and keyword. Each case is one synthetic self static whose
// condition counts the permanents its own write can change:
//
//   - the overlap cases write the characteristic the condition reads, so the
//     guard's "not applying" answer makes the condition false and the static
//     stays off (unguarded, the walk recursed until the stack overflowed);
//   - the separate-characteristic cases read something the write cannot
//     change, so the walk must still apply the static (the guard's answer is
//     exact, not just terminating).
//
// The card is the only permanent on the battlefield, so every count is the
// source's own read.
func TestSelfCondReadKindsGuard(t *testing.T) {
	set := cardsettest.Set(t, "selfcond")
	tbl0, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	lib, ok := set.Lookup("Forest")
	if !ok {
		t.Fatal("Forest not in the selfcond pool")
	}
	libID := CardID(set.PoolID(lib))
	sub := func(name string) uint8 {
		for i, s := range tbl0.Subtypes {
			if s == name {
				return uint8(i)
			}
		}
		t.Fatalf("subtype %q not in the selfcond pool", name)
		return 0
	}
	golem, army := sub("Golem"), sub("Army")

	cases := []struct {
		name string
		// the synthetic card
		types  kop.TypeMask
		subs   []uint8
		colors kop.ColorMask
		kw     kop.KwMask
		p, tt  int8
		chosen int // -1 none
		// its condition filter and its own write
		filter kop.Filter
		ov     kop.Override
		check  func(t *testing.T, s *State, k *prep, sl Slot)
	}{
		{
			name:  "type read, type write (overlap)",
			types: kop.TArtifact, p: 4, tt: 4, chosen: -1,
			filter: kop.Filter{Types: kop.TCreature},
			ov:     kop.Override{AddTypes: kop.TCreature},
			check: func(t *testing.T, s *State, k *prep, sl Slot) {
				if s.typesAt(k, sl)&kop.TCreature != 0 {
					t.Errorf("guard did not suppress the type write: types %#x", s.typesAt(k, sl))
				}
			},
		},
		{
			name:  "type read, subtype write",
			types: kop.TArtifact, p: 4, tt: 4, chosen: -1,
			filter: kop.Filter{Types: kop.TArtifact},
			ov:     kop.Override{AddSub: kop.SubMask{}.With(int(army))},
			check: func(t *testing.T, s *State, k *prep, sl Slot) {
				if !s.hasSubGen(k, sl, army) {
					t.Errorf("subtype write did not apply: %v", s.typesAt(k, sl))
				}
			},
		},
		{
			name:  "subtype read, subtype write (overlap)",
			types: kop.TArtifact, p: 4, tt: 4, chosen: -1,
			filter: kop.Filter{Sub: golem + 1},
			ov:     kop.Override{AddSub: kop.SubMask{}.With(int(golem))},
			check: func(t *testing.T, s *State, k *prep, sl Slot) {
				if s.hasSubGen(k, sl, golem) {
					t.Errorf("guard did not suppress the subtype write")
				}
			},
		},
		{
			name:  "subtype read, type write",
			types: kop.TArtifact, subs: []uint8{golem}, p: 4, tt: 4, chosen: -1,
			filter: kop.Filter{Sub: golem + 1},
			ov:     kop.Override{AddTypes: kop.TCreature},
			check: func(t *testing.T, s *State, k *prep, sl Slot) {
				if s.typesAt(k, sl)&kop.TCreature == 0 {
					t.Errorf("type write did not apply: types %#x", s.typesAt(k, sl))
				}
			},
		},
		{
			name:  "colour read, colour write (overlap)",
			types: kop.TArtifact, colors: 0, p: 4, tt: 4, chosen: -1,
			filter: kop.Filter{Colors: kop.ColW},
			ov:     kop.Override{HasColors: true, SetColors: kop.ColW},
			check: func(t *testing.T, s *State, k *prep, sl Slot) {
				if s.colorsAt(k, sl) != 0 {
					t.Errorf("guard did not suppress the colour write: %#x", s.colorsAt(k, sl))
				}
			},
		},
		{
			name:  "colour read, type write",
			types: kop.TArtifact, colors: kop.ColW, p: 4, tt: 4, chosen: -1,
			filter: kop.Filter{Colors: kop.ColW},
			ov:     kop.Override{AddTypes: kop.TCreature},
			check: func(t *testing.T, s *State, k *prep, sl Slot) {
				if s.typesAt(k, sl)&kop.TCreature == 0 {
					t.Errorf("type write did not apply: types %#x", s.typesAt(k, sl))
				}
			},
		},
		{
			name:  "chosen type read, subtype write (overlap)",
			types: kop.TArtifact, p: 4, tt: 4, chosen: int(golem),
			filter: kop.Filter{Flags2: kop.F2ChosenType},
			ov:     kop.Override{AddSub: kop.SubMask{}.With(int(golem))},
			check: func(t *testing.T, s *State, k *prep, sl Slot) {
				if s.hasSubGen(k, sl, golem) {
					t.Errorf("guard did not suppress the chosen-type read's write")
				}
			},
		},
		{
			name:  "chosen type read, type write",
			types: kop.TArtifact, subs: []uint8{golem}, p: 4, tt: 4, chosen: int(golem),
			filter: kop.Filter{Flags2: kop.F2ChosenType},
			ov:     kop.Override{AddTypes: kop.TCreature},
			check: func(t *testing.T, s *State, k *prep, sl Slot) {
				if s.typesAt(k, sl)&kop.TCreature == 0 {
					t.Errorf("type write did not apply: types %#x", s.typesAt(k, sl))
				}
			},
		},
		{
			name:  "chosen colour read, colour write (overlap)",
			types: kop.TArtifact, colors: 0, p: 4, tt: 4, chosen: 0,
			filter: kop.Filter{Flags2: kop.F2ChosenColor},
			ov:     kop.Override{HasColors: true, SetColors: kop.ColW},
			check: func(t *testing.T, s *State, k *prep, sl Slot) {
				if s.colorsAt(k, sl) != 0 {
					t.Errorf("guard did not suppress the colour write: %#x", s.colorsAt(k, sl))
				}
			},
		},
		{
			name:  "chosen colour read, type write",
			types: kop.TArtifact, colors: kop.ColW, p: 4, tt: 4, chosen: 0,
			filter: kop.Filter{Flags2: kop.F2ChosenColor},
			ov:     kop.Override{AddTypes: kop.TCreature},
			check: func(t *testing.T, s *State, k *prep, sl Slot) {
				if s.typesAt(k, sl)&kop.TCreature == 0 {
					t.Errorf("type write did not apply: types %#x", s.typesAt(k, sl))
				}
			},
		},
		{
			name:  "PT read, PT write (overlap)",
			types: kop.TArtifact, p: 4, tt: 4, chosen: -1,
			filter: kop.Filter{Pred: kop.PredPowerGE, PredN: 5},
			ov:     kop.Override{HasPT: true, SetP: 5, SetT: 5},
			check: func(t *testing.T, s *State, k *prep, sl Slot) {
				if p, tt := s.basePTGen(k, sl); p != 4 || tt != 4 {
					t.Errorf("guard did not suppress the P/T write: %d/%d", p, tt)
				}
			},
		},
		{
			name:  "PT read, type write",
			types: kop.TArtifact, p: 4, tt: 4, chosen: -1,
			filter: kop.Filter{Pred: kop.PredPowerGE, PredN: 4},
			ov:     kop.Override{AddTypes: kop.TCreature},
			check: func(t *testing.T, s *State, k *prep, sl Slot) {
				if s.typesAt(k, sl)&kop.TCreature == 0 {
					t.Errorf("type write did not apply: types %#x", s.typesAt(k, sl))
				}
			},
		},
		{
			name:  "keyword read, keyword write (overlap)",
			types: kop.TArtifact, p: 4, tt: 4, chosen: -1,
			filter: kop.Filter{Flags2: kop.F2WithFlying},
			ov:     kop.Override{AddKw: kop.KwFlying},
			check: func(t *testing.T, s *State, k *prep, sl Slot) {
				if s.kwGen(k, sl)&kop.KwFlying != 0 {
					t.Errorf("guard did not suppress the keyword write")
				}
			},
		},
		{
			name:  "keyword read, type write",
			types: kop.TArtifact, kw: kop.KwFlying, p: 4, tt: 4, chosen: -1,
			filter: kop.Filter{Flags2: kop.F2WithFlying},
			ov:     kop.Override{AddTypes: kop.TCreature},
			check: func(t *testing.T, s *State, k *prep, sl Slot) {
				if s.typesAt(k, sl)&kop.TCreature == 0 {
					t.Errorf("type write did not apply: types %#x", s.typesAt(k, sl))
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tbl := *tbl0
			tbl.Cards = append([]kop.CardDef(nil), tbl0.Cards...)
			tbl.Filters = append([]kop.Filter(nil), tbl0.Filters...)
			tbl.Conds = append([]kop.Cond(nil), tbl0.Conds...)
			tbl.Overrides = append([]kop.Override(nil), tbl0.Overrides...)

			tbl.Filters = append(tbl.Filters, c.filter)
			condID := kop.CondID(len(tbl.Conds))
			tbl.Conds = append(tbl.Conds, kop.Cond{Kind: kop.CondPresent, F: kop.FilterID(len(tbl.Filters) - 1), Op: kop.CmpGE, A: kop.Const(1)})
			tbl.Overrides = append(tbl.Overrides, c.ov)
			selfID := kop.FilterID(len(tbl.Filters))
			tbl.Filters = append(tbl.Filters, kop.Filter{Flags: kop.FSelf})

			cd := kop.CardDef{
				Name: "Test Self Condition", Types: c.types, Colors: c.colors,
				Keywords: c.kw, Power: c.p, Toughness: c.tt,
				Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{
					Kind: kop.StatOverride, Filter: selfID,
					Override: uint16(len(tbl.Overrides)), Cond2: condID}}},
			}
			for _, i := range c.subs {
				cd.Subtypes = cd.Subtypes.With(int(i))
			}
			id := CardID(len(tbl.Cards))
			tbl.Cards = append(tbl.Cards, cd)

			sc := newScenTableLib(t, &tbl, libID, Config{NoElide: true})
			sl := sc.slot(sc.perm(id, 0))
			if c.chosen >= 0 {
				sc.s.perms[sl].chosen = objWord(uint8(c.chosen))
				sc.s.refreshTy(sc.k) // the chosen value is read by the condition
			}
			if sc.k.gen.fxNeed&fxNeedGlobal == 0 {
				t.Fatal("the synthetic self override did not make every slot walk")
			}
			c.check(t, sc.s, sc.k, sl)
		})
	}
}
