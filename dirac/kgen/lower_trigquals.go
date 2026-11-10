package kgen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// trigCmp reads a Compare$-style trigger operand "<OP><n>" (gorge
// CompareIntCount / ComparePresent: EQ NE GE LE GT LT) with a literal in
// 1..127, the shape the kernel packs into Trigger.N (kop.DmgGate; kernel
// trigCmpOK). Anything else is !ok and the caller refuses with its own
// message, so each carrier's refusal text is unchanged.
func trigCmp(v string) (cmp uint8, n int, ok bool) {
	if len(v) < 3 {
		return 0, 0, false
	}
	switch v[:2] {
	case "EQ":
		cmp = uint8(kop.DmgEQ)
	case "NE":
		cmp = uint8(kop.DmgNE)
	case "GE":
		cmp = uint8(kop.DmgGE)
	case "LE":
		cmp = uint8(kop.DmgLE)
	case "GT":
		cmp = uint8(kop.DmgGT)
	case "LT":
		cmp = uint8(kop.DmgLT)
	default:
		return 0, 0, false
	}
	n, err := strconv.Atoi(v[2:])
	if err != nil || n < 1 || n > 127 || strings.TrimSpace(v[2:]) != v[2:] {
		return 0, 0, false
	}
	return cmp, n, true
}

// trigCmpN packs a trigger count gate into Trigger.N. A zero cmp (the
// operator the kernel reads from a bare literal: GE for the attackers gate,
// EQ for the N-th spell gate) stays the plain literal, so every existing
// card's table is byte-identical.
func trigCmpN(cmp uint8, lit int) int16 {
	if cmp == 0 {
		return int16(lit)
	}
	return kop.DmgGate(kop.DmgCmp(cmp), lit)
}

// filterPlayerAlt is filterZones with a player alternative appended to the Or
// chain: an entry that never matches a permanent (FSelf with FOther) and
// carries FPlayer and the controller selector, which the kernel's
// castTargetsPlayer reads for a player target. zones[i] is clause i's zone
// (a TargetsValid$ zone-free alternative carries kop.ZoneAny).
func (cb *cardBuilder) filterPlayerAlt(f *lFilter, zones []kop.Zone, players uint8) (kop.FilterID, error) {
	if len(zones) != len(f.Clauses) {
		return 0, fmt.Errorf("TargetsValid player alternative zones")
	}
	alt := cb.pt.internFilter(kop.Filter{Ctrl: playerCtrl(players), Flags: kop.FPlayer | kop.FSelf | kop.FOther})
	var next kop.FilterID = alt
	for i := len(f.Clauses) - 1; i >= 0; i-- {
		kf, err := cb.clause(f.Clauses[i], zones[i])
		if err != nil {
			return 0, err
		}
		cb.markDefending(kf)
		kf.Or = next
		if kf.Flags2&kop.F2SameNameAsSource != 0 || kf.Pred == kop.PredNamed {
			cb.out.readsName = true
		}
		next = cb.pt.internFilter(kf)
	}
	return next, nil
}

// targetsValidZones classifies the object alternatives of a SpellCast
// TargetsValid$ spec v (lowered as f) into the per-clause zone filterPlayerAlt
// / filterZones read: kop.ZoneBattlefield for Card.Self or an explicit
// inZoneBattlefield alternative, kop.ZoneAny (kernel castTargetsAll's
// card-matcher read of an off-battlefield target) for a zone-free one, which
// gorge's MatchesSpec reads in any zone (Danitha, Spear of Agony). ok is false
// when a line mixes the two (the kernel would read the battlefield clause in
// any zone too), when the alternatives do not line up with f's clauses, or
// when a zone-free alternative carries a tapped/untapped/blocking predicate:
// the kernel's card matcher ignores those off the battlefield, gorge does not.
func targetsValidZones(v string, f *lFilter, isType func(string) bool) (zones []kop.Zone, ok bool) {
	anyZone, battlefield, n := false, false, 0
	for _, alt := range splitAlternatives(v, isType) {
		alt = strings.TrimSpace(alt)
		switch alt {
		case "You", "Opponent":
			continue
		}
		n++
		if alt == "Card.Self" || strings.Contains(alt, "inZoneBattlefield") {
			battlefield = true
		} else {
			anyZone = true
		}
	}
	if n != len(f.Clauses) || n == 0 || anyZone && battlefield {
		return nil, false
	}
	zone := kop.ZoneBattlefield
	if anyZone {
		zone = kop.ZoneAny
		for _, c := range f.Clauses {
			if c.Flags&(fTapped|fUntapped|fBlocking) != 0 {
				return nil, false
			}
		}
	}
	zones = make([]kop.Zone, len(f.Clauses))
	for i := range zones {
		zones[i] = zone
	}
	return zones, true
}
