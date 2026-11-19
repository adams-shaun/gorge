package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The cast-free ask's CR 601.2c feasibility sweep judges a candidate's targets
// with the CANDIDATE SPELL as the targeting source, not the resolving entry
// the ask rides on (CR 702.16b: protection from black stops a black spell's
// targets; gorge's census judges the spell being cast). Sword of Once and
// Future's "you may cast an instant or sorcery spell with mana value 2 or less
// from your graveyard" asks while the sword's own combat-damage trigger
// resolves: a black spell (Ghastly Demise) whose only potential target is the
// sword-equipped creature carrying protection from black has no legal target
// and must not be offered.
func TestCastFreeAskJudgesTargetsByTheSpell(t *testing.T) {
	for _, tc := range []struct {
		name      string
		protBlack bool
		wantOffer bool
	}{
		{"no protection", false, true},
		{"protection from black on the only target", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deck := []cardset.Entry{{Name: "Island", Count: 20}}
			set := cardsettest.Set(t, "freeplay")
			tbl, _, _, err := set.Matchup(deck, deck)
			if err != nil {
				t.Fatal(err)
			}
			prot := CardID(len(tbl.Cards))
			pcol := kop.ColorMask(0)
			if tc.protBlack {
				pcol = kop.ColB
			}
			tbl.Cards = append(tbl.Cards, kop.CardDef{
				Name:       "Probe Shield",
				Cost:       kop.ManaCost{Generic: 1, Pips: [kop.NumManaKinds]uint8{4: 1}},
				Types:      kop.TCreature,
				Colors:     kop.ColG,
				Power:      2,
				Toughness:  2,
				ProtColors: pcol, // printed protection from black
			})
			demise := CardID(len(tbl.Cards))
			creatureF := kop.FilterID(len(tbl.Filters))
			tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
			tbl.Cards = append(tbl.Cards, kop.CardDef{
				Name:   "Probe Demise",
				Cost:   kop.ManaCost{Generic: 1},
				Types:  kop.TInstant,
				Colors: kop.ColB,
				Abilities: []kop.Ability{{
					Kind:     kop.AbSpell,
					Targets:  [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtObject, Filter: creatureF, Min: 1, Max: 1}},
					NTargets: 1,
				}},
			})
			saF1 := kop.FilterID(len(tbl.Filters))
			tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TSorcery, Pred: kop.PredCMCLE, PredN: 2, Owner: kop.CtrlYou})
			saF0 := kop.FilterID(len(tbl.Filters))
			tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TInstant, Pred: kop.PredCMCLE, PredN: 2, Owner: kop.CtrlYou, Or: saF1})
			trig := CardID(len(tbl.Cards))
			tbl.Cards = append(tbl.Cards, kop.CardDef{
				Name:      "Probe Trigger",
				Types:     kop.TArtifact, // colourless, like the Equipment that asks
				Power:     1,
				Toughness: 1,
				Abilities: []kop.Ability{{Kind: kop.AbTriggered, Prog: []kop.Instr{
					{Op: kop.OpCastFree, C: 4 | 8 | 32 | 1024, F: saF0}, // grave, One, ReplaceGY, SpellMV
				}}},
			})

			var lib [20]CardID
			for i := range lib {
				lib[i] = CardID(0)
			}
			for i := range lib {
				for c := range tbl.Cards {
					if tbl.Cards[c].Name == "Island" {
						lib[i] = CardID(c)
					}
				}
			}
			sc := newScenTableLib(t, tbl, lib[0], Config{NoElide: true})
			trigPerm := sc.perm(trig, 1) // the resolving entry, on seat 1's battlefield
			shield := sc.perm(prot, 1)   // the Demise's only potential target
			d := sc.obj(demise, 1)
			sc.s.place(d, kop.ZoneGraveyard, false)

			e := stackEntry{kind: seTrigger, obj: trigPerm, card: trig, ab: 0, ctrl: 1, trigObj: NoObj}
			sc.s.sp = 1
			sc.s.stack[0] = e
			sc.s.res = resCtx{e: e, active: true, lastTok: NoObj, eachObj: NoObj}
			sc.s.runProg(sc.k)
			ask := sc.s.Step()
			offered := false
			if ask.Kind == DkChoose && ask.Sub == ChGenCastFree {
				for i := 0; i < int(ask.N); i++ {
					if ask.Options[i].Kind() == AkChoose && Obj(ask.Options[i].Source()) == d {
						offered = true
					}
				}
			} else if tc.wantOffer {
				t.Fatalf("decision kind %d sub %d, want the free-cast ask", ask.Kind, ask.Sub)
			}
			// an empty candidate list poses no ask (gorge's Note path)
			if offered != tc.wantOffer {
				t.Fatalf("shield=%v: the Demise offered=%v, want %v (options %d)", tc.protBlack, offered, tc.wantOffer, ask.N)
			}
			_ = shield
		})
	}
}
