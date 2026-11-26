package kgen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// stackKwStatic builds one StatStackKw ability (lower_stackkw.go): the filter
// matches the spell being cast (a card in hand or on the stack), A is the
// keyword code, B the casualty threshold, and Zone the cast-origin marker
// (ZoneHand, ZoneExile, ZoneAny for !wasCastFromYourHand) or ZoneStack for an
// EffectZone$ Stack self grant. Split out of (*cardBuilder).static so that
// function keeps its length ceiling.
func (cb *cardBuilder) stackKwStatic(st *kop.Static, s lStatic) error {
	st.Kind = kop.StatStackKw
	if s.F != nil {
		f, err := cb.filter(s.F, kop.ZoneStack)
		if err != nil {
			return err
		}
		st.Filter = f
	}
	if len(s.Kw) != 1 {
		return fmt.Errorf("stack keyword grant with %d keywords", len(s.Kw))
	}
	head, param, _ := strings.Cut(s.Kw[0], ":")
	switch head {
	case "Cascade", "Improvise", "Convoke":
		if param != "" {
			return fmt.Errorf("stack keyword grant %q with a parameter", s.Kw[0])
		}
		st.A = kop.StackKwCascade
		switch head {
		case "Improvise":
			st.A = kop.StackKwImprovise
		case "Convoke":
			st.A = kop.StackKwConvoke
		}
	case "Lifelink", "Storm", "Split second", "Conspire", "Delve", "Demonstrate", "Rebound":
		if param != "" {
			return fmt.Errorf("stack keyword grant %q with a parameter", s.Kw[0])
		}
		st.A = kop.StackKwLifelink
		switch head {
		case "Rebound":
			st.A = kop.StackKwRebound
			j, ok := cb.abMap[s.GrantBody] // 0-based LIR index of the carrier's recast body
			if !ok || j+1 > 255 {
				return fmt.Errorf("stack keyword grant %q: the recast body was not emitted", s.Kw[0])
			}
			st.GrantBody = uint8(j + 1)
		case "Demonstrate":
			st.A = kop.StackKwDemonstrate
		case "Conspire":
			st.A = kop.StackKwConspire
		case "Storm":
			st.A = kop.StackKwStorm
		case "Split second":
			st.A = kop.StackKwSplitSecond
		case "Delve":
			st.A = kop.StackKwDelve
		}
	case "Web-slinging":
		cost, _, _ := strings.Cut(param, ":")
		if cost == "" {
			return fmt.Errorf("stack keyword grant %q without a cost", s.Kw[0])
		}
		st.A = kop.StackKwWebSling
		var err error
		if st.GrantCost, err = manaCostOf(s.Cost.Mana); err != nil {
			return err
		}
		if s.F2 != nil {
			if st.Filter2, err = cb.filter(s.F2, kop.ZoneStack); err != nil {
				return err
			}
		}
	case "Offspring":
		if param == "" {
			return fmt.Errorf("stack keyword grant %q without a cost", s.Kw[0])
		}
		st.A = kop.StackKwOffspring
		var err error
		if st.GrantCost, err = manaCostOf(s.Cost.Mana); err != nil {
			return err
		}
		j, ok := cb.abMap[s.GrantBody] // 0-based LIR index of the carrier's latent copy trigger
		if !ok || j+1 > 255 {
			return fmt.Errorf("stack keyword grant %q: the copy trigger was not emitted", s.Kw[0])
		}
		st.GrantBody = uint8(j + 1)
	case "Casualty":
		n, err := strconv.Atoi(param)
		if err != nil || n < 1 || n > 127 {
			return fmt.Errorf("stack keyword grant %q", s.Kw[0])
		}
		st.A, st.B = kop.StackKwCasualty, int16(n)
	default:
		return fmt.Errorf("stack keyword grant %q", s.Kw[0])
	}
	for _, fl := range s.Flags {
		switch fl {
		case "CastHand":
			st.Zone = kop.ZoneHand // wasCastFromHand / wasCastFromYourHand[ByYou]
		case "CastExile":
			st.Zone = kop.ZoneExile // wasCastFromExile
		case "CastNotHand":
			// !wasCastFromYourHand: anywhere but a hand. ZoneAny is not a
			// real zone here; it is the StatStackKw-only marker stackKwN
			// reads as "not from hand" (Zone is otherwise this static's
			// cast-origin restriction).
			st.Zone = kop.ZoneAny
		case "SelfStack":
			st.Zone = kop.ZoneStack // EffectZone$ Stack self grant (Bloodbraid Marauder)
		default:
			return fmt.Errorf("stack keyword grant flag %q", fl)
		}
	}
	return nil
}
