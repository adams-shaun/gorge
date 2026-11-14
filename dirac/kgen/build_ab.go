package kgen

import (
	"fmt"
	"math/bits"
	"slices"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// abilities builds the card's abilities: keyword-shaped expansions fold back
// into the record (Equip, Enchant, Ward, Prowess), the rest become kop
// abilities in LIR order, then the statics and replacements.
func (cb *cardBuilder) abilities() error {
	c := cb.c
	d := &cb.out.def
	var keep []*lAbility
	for _, a := range c.Abilities {
		cb.node = fmt.Sprintf("ab[%d]", a.Index)
		folded, err := cb.fold(a)
		if err != nil {
			return err
		}
		if !folded {
			cb.abMap[a.Index] = len(keep)
			keep = append(keep, a)
		}
	}
	// Emblem triggers move into their emblems; they stay in the card's list
	// as latent abilities too (harmless, never active) so indexes hold.
	for _, a := range keep {
		cb.node = fmt.Sprintf("ab[%d]", a.Index)
		ka, err := cb.ability(a)
		if err != nil {
			return bfail(cb.node, "%v", err)
		}
		d.Abilities = append(d.Abilities, ka)
		o := uint8(0xFF)
		if a.OriginOK {
			o = uint8(a.Origin)
		}
		cb.out.origins = append(cb.out.origins, o)
	}
	cb.out.stAddSub = make([][]uint8, len(d.Abilities))
	for i, s := range c.Statics {
		cb.node = fmt.Sprintf("static[%d] %s", i, s.Kind)
		ka, add, err := cb.static(s)
		if err != nil {
			return bfail(cb.node, "%v", err)
		}
		d.Abilities = append(d.Abilities, ka)
		cb.out.stAddSub = append(cb.out.stAddSub, add)
		cb.out.origins = append(cb.out.origins, 0xFF)
	}
	for i, r := range c.Repls {
		cb.node = fmt.Sprintf("repl[%d] %s", i, r.Kind)
		ka, err := cb.repl(r)
		if err != nil {
			return bfail(cb.node, "%v", err)
		}
		d.Abilities = append(d.Abilities, ka)
		cb.out.stAddSub = append(cb.out.stAddSub, nil)
		cb.out.origins = append(cb.out.origins, 0xFF)
	}
	for i, e := range c.Emblems {
		cb.node = fmt.Sprintf("emblem[%d]", i)
		var ke kop.Emblem
		ke.UntilEOT = e.UntilEOT
		ke.UntilNextTurn = e.UntilNextTurn
		ke.WhileHost = e.WhileHost
		ke.CaptureLapse = e.CaptureLapse
		ke.CaptureRem, ke.Source, ke.NumAmount, ke.Once, ke.NotHere = e.CaptureRem, e.Source, e.NumAmount, e.Once, e.NotHere
		if e.CaptureSet {
			r, err := cb.ref(e.Capture)
			if err != nil {
				return bfail(cb.node, "%v", err)
			}
			if e.CaptureIsPlayer {
				ke.CapturePlayer = r
			} else {
				ke.Capture = r
			}
		}
		if e.Capture2Set {
			r, err := cb.ref(e.Capture2)
			if err != nil {
				return bfail(cb.node, "%v", err)
			}
			ke.Capture2 = r
		}
		for _, s := range e.Statics {
			ka, add, err := cb.static(s)
			if err != nil {
				return bfail(cb.node, "%v", err)
			}
			if len(add) > 0 {
				return bfail(cb.node, "emblem static grants subtypes")
			}
			ke.Abilities = append(ke.Abilities, ka)
		}
		if len(e.Statics) == 0 {
			j, ok := cb.abMap[e.Trigger]
			if !ok {
				return bfail(cb.node, "emblem trigger %d not emitted", e.Trigger)
			}
			ta := d.Abilities[j]
			ta.AFlags &^= kop.AfLatent
			ke.Abilities = append(ke.Abilities, ta)
		}
		cb.pt.emblems = append(cb.pt.emblems, ke)
		// Patch the CreateEmblem ops of this card to the pool emblem index.
		for ai := range d.Abilities {
			for ii := range d.Abilities[ai].Prog {
				in := &d.Abilities[ai].Prog[ii]
				if in.Op == kop.OpCreateEmblem && in.C == int16(i) && in.F == 0 {
					in.C = int16(len(cb.pt.emblems) - 1)
					in.F = 1 // marks the op patched (cleared below)
				}
			}
		}
	}
	for ai := range d.Abilities {
		for ii := range d.Abilities[ai].Prog {
			if in := &d.Abilities[ai].Prog[ii]; in.Op == kop.OpCreateEmblem {
				in.F = 0
			}
		}
	}
	return nil
}

// fold folds keyword-shaped abilities into the card record and reports
// whether a was folded.
func (cb *cardBuilder) fold(a *lAbility) (bool, error) {
	d := &cb.out.def
	c := cb.c
	switch {
	case a.Kind == akActivated && a.Name == "Equip":
		// Kept as gorge expands it (an activated Attach at sorcery speed,
		// AfEquip); CardDef.EquipCost records the cost for reference. A
		// restricted equip ("Equip Pirate {1}", CR 702.6a) keeps its
		// subtype target; the unrestricted one's cost is the reference.
		if len(a.Targets) != 1 || len(a.Prog) != 1 || a.Prog[0].Op != "Attach" || !a.Sorcery || !(isCreatureYouCtrl(a.Targets[0].F) || isSubtypeYouCtrl(a.Targets[0].F)) {
			return false, bfail(cb.node, "equip shape")
		}
		mc, err := manaCostOf(a.Cost.Mana)
		if err != nil {
			return false, err
		}
		if isCreatureYouCtrl(a.Targets[0].F) {
			d.EquipCost = mc
		}
		return false, nil
	case a.Kind == akSpell && hasSub(c.Subtypes, "Aura"):
		if len(a.Targets) != 1 || len(a.Prog) != 1 || a.Prog[0].Op != "Attach" {
			return false, bfail(cb.node, "aura spell shape")
		}
		ts, err := cb.target(a.Targets[0])
		if err != nil {
			return false, err
		}
		d.Enchant = ts
		return true, nil
	case a.Kind == akTriggered && a.Trigger != nil && a.Trigger.Event == "BecomesTargetByOpp":
		if len(a.Prog) != 1 || a.Prog[0].Op != "Ward" || !a.Prog[0].A.isConst() {
			return false, bfail(cb.node, "ward shape")
		}
		n := a.Prog[0].A.N
		if a.Prog[0].C != 0 && a.Prog[0].CS != "Mana" {
			return false, bfail(cb.node, "ward cost %s with C %d", a.Prog[0].CS, a.Prog[0].C)
		}
		switch a.Prog[0].CS {
		case "Mana":
			if a.Prog[0].B.N != 0 {
				return false, bfail(cb.node, "coloured ward cost")
			}
			d.WardN = uint8(n)
			if l := a.Prog[0].C; l != 0 {
				// {N} and pay L life: kernel wardMLBit
				if l < 0 || l > 31 || n < 1 || n > 31 {
					return false, bfail(cb.node, "ward mana-and-life cost")
				}
				d.WardLife = uint8(l)
			}
		case "PayLife":
			d.WardLife = uint8(n)
		case "CollectEvidence":
			d.WardEvidence = uint8(n) // kernel kw_wardcost.go
		case "Discard":
			if n != 1 {
				return false, bfail(cb.node, "ward cost Discard %d", n)
			}
			d.WardDiscard = 1 // kernel kw_wardcost.go
		case "Waterbend":
			if n < 1 || n > 31 {
				return false, bfail(cb.node, "ward cost waterbend %d", n)
			}
			d.WardWaterbend = uint8(n) // kernel cost_unlesswaterbend.go
		case "Poison":
			if n < 1 || n > 255 {
				return false, bfail(cb.node, "ward cost poison %d", n)
			}
			d.WardPoison = uint8(n) // kernel cost_restokens.go
		case "DiscardOrMana":
			if n < 1 || n > 255 {
				return false, bfail(cb.node, "ward cost discard or {%d}", n)
			}
			d.WardDiscOrN = uint8(n) // kernel kw_wardcost.go
		case "Sacrifice":
			if n != 1 {
				return false, bfail(cb.node, "ward cost Sacrifice %d", n)
			}
			f, err := cb.filter(a.Prog[0].F, kop.ZoneBattlefield)
			if err != nil {
				return false, err
			}
			if f == 0 {
				return false, bfail(cb.node, "ward cost Sacrifice without a filter")
			}
			d.WardSac = f // kernel kw_wardcost.go
		case "PayLifePower":
			d.WardLifePower = true // kernel kw_wardcost.go
		default:
			return false, bfail(cb.node, "ward cost %s", a.Prog[0].CS)
		}
		d.Keywords |= kop.KwWard
		return true, nil
	case isProwess(a):
		// The keyword bit says "has prowess"; the trigger stays the card's
		// own SpellCast ability, as gorge expands it (dirac/vanilla's
		// Jeskai Student).
		d.Keywords |= kop.KwProwess
		return false, nil
	}
	return false, nil
}

func isConstN(v lValue, n int) bool { return v.isConst() && v.N == n }

// kickedConstRange: min and max are both Count$Kicked.<kicked>.<unkicked>
// with these constants.
func kickedConstRange(lo, hi lValue, kicked, unkicked int) bool {
	for _, v := range [2]lValue{lo, hi} {
		if v.Kind != vKicked || v.Mul != 0 || v.Div != 0 || v.Add != 0 || v.Neg || len(v.Sub) != 2 || !isConstN(v.Sub[0], kicked) || !isConstN(v.Sub[1], unkicked) {
			return false
		}
	}
	return true
}

// kickedGuardedAsk: slot i's mid-resolution ask is the first instruction
// inside a SkipIf Kicked guard (the sub's Condition$ Kicked).
func kickedGuardedAsk(prog []lInstr, i int) bool {
	for j, in := range prog {
		if in.Op == "ChooseTargets" && in.C == i {
			return j > 0 && prog[j-1].Op == "SkipIf" && prog[j-1].Cond.Kind == cKicked && !prog[j-1].Cond.Neg
		}
	}
	return false
}

func hasSub(ss []string, w string) bool {
	for _, s := range ss {
		if s == w {
			return true
		}
	}
	return false
}

func isCreatureYouCtrl(f *lFilter) bool {
	return f != nil && f.Players == 0 && len(f.Clauses) == 1 && f.Clauses[0] == lClause{Types: tCreature, Ctrl: whoYou}
}

// isSubtypeYouCtrl reports a restricted equip's target: one subtype
// ("a Pirate you control"), optionally under a Creature head.
func isSubtypeYouCtrl(f *lFilter) bool {
	if f == nil || f.Players != 0 || len(f.Clauses) != 1 {
		return false
	}
	c := f.Clauses[0]
	return c.Subtype != "" && (c.Types == 0 || c.Types == tCreature) && c == lClause{Types: c.Types, Subtype: c.Subtype, Ctrl: whoYou}
}

// isProwess recognises the prowess expansion (or its printed equivalent).
func isProwess(a *lAbility) bool {
	t := a.Trigger
	if a.Kind != akTriggered || a.Latent || t == nil || t.Event != "SpellCast" || t.Whose != whoYou || t.Zone != zNone || t.Optional || t.Cond.Kind != cNone {
		return false
	}
	if t.F == nil || len(t.F.Clauses) != 1 || t.F.Clauses[0] != (lClause{NotTypes: tCreature}) || t.F.Players != 0 {
		return false
	}
	if len(a.Prog) != 1 || len(a.Targets) != 0 {
		return false
	}
	in := a.Prog[0]
	return in.Op == "PumpUntilEOT" && in.Obj.Kind == rSelf && isConstN(in.A, 1) && isConstN(in.B, 1)
}

// target builds a target spec.
func (cb *cardBuilder) target(t lTarget) (kop.TargetSpec, error) {
	var ts kop.TargetSpec
	f := t.F
	var err error
	switch {
	case t.GraveExileCard:
		ts.Kind = kop.TgtGraveExileCard
		zs := make([]kop.Zone, len(t.ClauseZones))
		for i, z := range t.ClauseZones {
			zs[i] = kop.Zone(z)
		}
		if ts.Filter, err = cb.filterZones(f, zs); err != nil {
			return ts, err
		}
	case t.PermOrGraveCard:
		ts.Kind = kop.TgtPermOrGraveCard
		// parsed in the graveyard: matchCardOneAt reads the zone, matchPerm ignores it
		if ts.Filter, err = cb.filter(f, kop.ZoneGraveyard); err != nil {
			return ts, err
		}
	case t.PermOrSpell:
		ts.Kind = kop.TgtPermOrSpell
		if ts.Filter, err = cb.filter(f, kop.ZoneBattlefield); err != nil {
			return ts, err
		}
	case t.Spell:
		ts.Kind = kop.TgtSpell
		if t.StackAb {
			ts.Kind = kop.TgtStackObj
			if t.AbOnly {
				ts.Kind = kop.TgtStackAb
				switch t.CopyAb {
				case 1:
					ts.Kind = kop.TgtCopyAb
				case 2:
					ts.Kind = kop.TgtCopyTrig
				}
			}
		}
		if t.TrigOnly {
			ts.Kind = kop.TgtStackObjTrig
		}
		if t.SingleTgt {
			ts.TFlags |= kop.TgfSingleTgt
		}
		if t.ActOnly {
			ts.TFlags |= kop.TgfActOnly
		}
		if t.VTgt != nil || t.VYou {
			// the filter names the battlefield object one of the spell's
			// targets must match (VYou: the chooser); SpellF is the spell side
			ts.Kind = kop.TgtSpellTgting
			if t.StackAb {
				ts.Kind = kop.TgtStackTgting // a stack ability's own targets count too
			}
			ts.VYou = t.VYou
			if t.VTgt != nil {
				if ts.Filter, err = cb.filter(t.VTgt, kop.ZoneBattlefield); err != nil {
					return ts, err
				}
			}
			if t.VSpellF != nil {
				if ts.SpellF, err = cb.filter(t.VSpellF, kop.ZoneStack); err != nil {
					return ts, err
				}
			}
			break
		}
		if ts.Filter, err = cb.filter(f, kop.ZoneStack); err != nil {
			return ts, err
		}
	case len(f.Clauses) == 0:
		ts.Kind = kop.TgtPlayer
		ts.Filter = cb.pt.internFilter(kop.Filter{Ctrl: playerCtrl(f.Players)})
	case f.Players != 0:
		switch {
		case f.Players == plBoth && len(f.Clauses) == 1 && f.Clauses[0] == (lClause{Types: tCreature | tPlaneswalker}):
			ts.Kind = kop.TgtAny
		case f.Players == plBoth && len(f.Clauses) == 1 && f.Clauses[0] == (lClause{Types: tPlaneswalker}):
			// "target player or planeswalker" (Boros Charm, Viashino
			// Pyromancer): a player matches the player
			// alternative, a permanent the planeswalker one. Same target
			// walk as "any target", with the object clause as the filter.
			ts.Kind = kop.TgtAny
			g := *f
			g.Players = 0
			if ts.Filter, err = cb.filter(&g, kop.ZoneBattlefield); err != nil {
				return ts, err
			}
		case f.Players == plOpp && len(f.Clauses) == 1 && f.Clauses[0] == (lClause{Types: tPlaneswalker}):
			// "target opponent or planeswalker" (Burning Sun's Avatar,
			// Rakdos's Return): the Boros Charm walk with the player
			// alternative narrowed to an opponent of the chooser.
			ts.Kind = kop.TgtAny
			ts.TFlags |= kop.TgfOppOnly
			g := *f
			g.Players = 0
			if ts.Filter, err = cb.filter(&g, kop.ZoneBattlefield); err != nil {
				return ts, err
			}
		default:
			return ts, fmt.Errorf("mixed player/object target %s", f)
		}
	case t.Zone == zBattlefield || t.Zone == zNone:
		ts.Kind = kop.TgtObject
		if ts.Filter, err = cb.filter(f, kop.ZoneBattlefield); err != nil {
			return ts, err
		}
	default:
		ts.Kind = kop.TgtCard
		if ts.Filter, err = cb.filter(f, zoneMap[t.Zone]); err != nil {
			return ts, err
		}
	}
	minC, maxC := t.Min.isConst(), t.Max.isConst()
	switch {
	case t.LKIDiv:
		ts.Divide = kop.DivLKIP1P1 // the kernel bounds the slot 0..X itself
	case t.Divided == kop.DivValue && minC && maxC && t.Min.N == 0 && t.Max.N >= 1 && t.Max.N < 255 && !t.AtRes:
		// "up to N targets" over a value total (Shatterskull Smashing's two,
		// the total X or 2X): the maximum is min(N, total), so the slot is
		// bounds-valued and tgtBoundsV clamps it to the total.
		var err error
		if ts.MinV, err = cb.value(t.Min); err != nil {
			return ts, err
		}
		if ts.MaxV, err = cb.value(t.Max); err != nil {
			return ts, err
		}
		ts.TFlags |= kop.TgfBoundsV
		ts.UpTo = true
	case minC && maxC && t.Min.N == 1 && t.Max.N == 1:
	case minC && maxC && t.Min.N == 0 && t.Max.N == 1:
		ts.UpTo = true
	case minC && maxC:
		ts.Min, ts.Max = uint8(t.Min.N), uint8(t.Max.N)
		// "up to N": castable with no target (Macabre Waltz)
		ts.UpTo = t.Min.N == 0
	case t.Min.Kind == vPromisedGift || t.Max.Kind == vPromisedGift:
		// Gift: the slot's count depends on the promise (Wear Down's one
		// or two, Long River's Pull's creature-spell or any-spell slot)
		if !cb.gift {
			return ts, fmt.Errorf("gift target range without a gift")
		}
		var lo, hi [2]int // [promised, not]
		for i, v := range [2]lValue{t.Min, t.Max} {
			out := &lo
			if i == 1 {
				out = &hi
			}
			switch {
			case v.isConst():
				out[0], out[1] = v.N, v.N
			case v.Kind == vPromisedGift && v.Mul == 0 && v.Div == 0 && v.Add == 0 && !v.Neg && len(v.Sub) == 2 && v.Sub[0].isConst() && v.Sub[1].isConst():
				out[0], out[1] = v.Sub[0].N, v.Sub[1].N
			default:
				return ts, fmt.Errorf("gift target range %s..%s", t.Min, t.Max)
			}
		}
		for i := 0; i < 2; i++ {
			if lo[i] < 0 || lo[i] > hi[i] || hi[i] > 8 {
				return ts, fmt.Errorf("gift target range %s..%s", t.Min, t.Max)
			}
		}
		ts.TFlags |= kop.TgfGiftRange
		ts.MinKicked, ts.MaxKicked = uint8(lo[0]), uint8(hi[0])
		ts.Min, ts.Max = uint8(lo[1]), uint8(hi[1])
	case t.Min.Kind == vKicked && t.Max.Kind == vKicked && (cb.kicked || cb.bargain) && kickedConstRange(t.Min, t.Max, 0, 1) && !t.AtRes:
		// Tear Asunder's first slot: one target unkicked, none kicked
		ts.TFlags |= kop.TgfKickedRange
		ts.Min, ts.Max = 1, 1
		ts.MinKicked, ts.MaxKicked = 0, 0
	case t.Min.Kind == vKicked && t.Max.Kind == vKicked && (cb.kicked || cb.bargain) && kickedConstRange(t.Min, t.Max, 1, 0) && !t.AtRes:
		// Tear Asunder's kicked slot: none unkicked, one kicked, announced on
		// cast (3ea231b90; CR 601.2c)
		ts.TFlags |= kop.TgfKickedRange
		ts.Min, ts.Max = 0, 0
		ts.MinKicked, ts.MaxKicked = 1, 1
	case t.Min.Kind == vKicked && t.Max.Kind == vKicked && (cb.kicked || cb.bargain):
		// Divine Resilience: one target, or any number when kicked.
		if len(t.Min.Sub) != 2 || len(t.Max.Sub) != 2 || !isConstN(t.Min.Sub[1], 1) || !isConstN(t.Max.Sub[1], 1) || !isConstN(t.Min.Sub[0], 0) || t.Max.Sub[0].Kind != vCount {
			return ts, fmt.Errorf("kicked target range shape")
		}
		ts.TFlags |= kop.TgfKickedRange
		ts.MinKicked, ts.MaxKicked = 0, 255
	case !t.AtRes && !t.Unique && !t.SameCtrl && !t.DiffCtrl && !t.SharedCrType && !t.DistinctName && (t.Divided == 0 || t.Divided == kop.DivValue) && t.Mode < 0 && t.ParentPlus1 == 0 &&
		plainX(t.Min) && plainX(t.Max):
		// "X target creatures": exactly the announced X (CR 601.2b, 601.2c).
		// gorge offers only an X the target ask can answer
		// (targetsAnnounceableAt), so the slot is never short; the kernel's
		// X ask filters the same way (addXOptions, xAnswerable)
		var err error
		if ts.MinV, err = cb.value(t.Min); err != nil {
			return ts, fmt.Errorf("target count %s..%s: %w", t.Min, t.Max, err)
		}
		ts.MaxV = ts.MinV
		ts.TFlags |= kop.TgfBoundsV
	case !t.AtRes && !t.Unique && !t.SameCtrl && !t.DiffCtrl && !t.SharedCrType && !t.DistinctName && (t.Divided == 0 || t.Divided == kop.DivMaxV || t.Divided == kop.DivValue) && t.Mode < 0 && t.ParentPlus1 == 0 &&
		tgtBoundV(t.Min, true) && tgtBoundV(t.Max, false):
		// "up to" a value (TargetMax$ X over a count, X paid or
		// the trigger amount): evaluated as the slot is announced (gorge
		// resolvedTargetBounds). Min 0 keeps the slot offerable with no target.
		var err error
		if ts.MinV, err = cb.value(t.Min); err != nil {
			return ts, fmt.Errorf("target count %s..%s: %w", t.Min, t.Max, err)
		}
		if ts.MaxV, err = cb.value(t.Max); err != nil {
			return ts, fmt.Errorf("target count %s..%s: %w", t.Min, t.Max, err)
		}
		ts.TFlags |= kop.TgfBoundsV
		ts.UpTo = true
	case !t.AtRes && !t.Unique && !t.SameCtrl && !t.DiffCtrl && !t.SharedCrType && !t.DistinctName && t.Divided == 0 && t.Mode < 0 && t.ParentPlus1 == 0 &&
		t.Min.isConst() && t.Min.N == 1 && !t.Max.isConst() && !plainX(t.Max) && tgtBoundReads(t.Max):
		// one target at least, up to a value read as the slot is announced
		// (Return to Dust: TargetMin$ 1, TargetMax$ the own-main-phase count).
		// The slot is mandatory: the kernel's bound check asks MinV
		// candidates (tgtBoundsFeasible) and the ask closes at MaxV.
		var err error
		if ts.MinV, err = cb.value(t.Min); err != nil {
			return ts, fmt.Errorf("target count %s..%s: %w", t.Min, t.Max, err)
		}
		if ts.MaxV, err = cb.value(t.Max); err != nil {
			return ts, fmt.Errorf("target count %s..%s: %w", t.Min, t.Max, err)
		}
		ts.TFlags |= kop.TgfBoundsV
	default:
		return ts, fmt.Errorf("target count %s..%s", t.Min, t.Max)
	}
	if t.Unique {
		ts.TFlags |= kop.TgfDistinct
	}
	if t.SameCtrl {
		ts.TFlags |= kop.TgfSameController
	}
	if t.DiffCtrl {
		ts.TFlags |= kop.TgfDiffController
	}
	if t.OppToActive {
		ts.TFlags |= kop.TgfOppToActive | kop.TargetFlags(t.ThanActive)<<kop.TgfThanShift
	}
	if t.TrigChooser {
		if ts.Min|ts.Max != 0 || ts.Divide != 0 {
			return ts, fmt.Errorf("trigger-player chooser on a several-object slot")
		}
		ts.TFlags |= kop.TgfTrigChooser
	}
	if err := tgtSetSpec(t, &ts); err != nil { // lower_targetset.go
		return ts, err
	}
	if t.Divided > 0 && !t.LKIDiv {
		if t.Divided == kop.DivValue {
			if t.DividedV.Kind == vNone {
				return ts, fmt.Errorf("divided slot without a value total")
			}
			if !divTotalReads(t.DividedV) {
				return ts, fmt.Errorf("divided total %s is not evaluable at the announcement", t.DividedV)
			}
			if ts.DivideV, err = cb.value(t.DividedV); err != nil {
				return ts, fmt.Errorf("divided total %s: %w", t.DividedV, err)
			}
		}
		ts.Divide = uint8(t.Divided)
	}
	if t.AtRes && ts.TFlags == kop.TgfGiftRange && ts.Divide == 0 && ts.Min == ts.Max && ts.MinKicked == ts.MaxKicked && ts.Max <= 1 && ts.MaxKicked <= 1 {
		// a gift-gated mid-resolution slot (Into the Flood Maw's "instead
		// return target nonland permanent"): one object under one election,
		// none under the other (OpChooseTargets)
		ts.TFlags |= kop.TgfAtResolution
		t.AtRes = false
	}
	if t.AtRes {
		if ts.Min|ts.Max != 0 || ts.TFlags != 0 || ts.Divide != 0 {
			return ts, fmt.Errorf("mid-resolution multi-object target")
		}
		ts.TFlags |= kop.TgfAtResolution
	}
	if t.Decl {
		ts.TFlags |= kop.TgfDecl
	}
	if t.SubDecl && !t.AtRes {
		// gorge castSubTargetsOwed: a link behind an unpaid Kicked or
		// OptionalCost announces nothing. Only a kicked-only slot (Tear
		// Asunder's) makes the kernel's copy ask skip it exactly.
		if t.SubCond == "optionalcost" || t.SubCond == "kicked" && !(ts.TFlags&kop.TgfKickedRange != 0 && ts.Min|ts.Max == 0 && ts.MinKicked > 0) {
			return ts, fmt.Errorf("chain link excused by an unpaid %s cost declares a copy target", t.SubCond)
		}
		ts.TFlags |= kop.TgfDeclSub
	}
	if t.ParentPlus1 != 0 {
		if ts.Kind != kop.TgtCard {
			return ts, fmt.Errorf("parent-restricted non-card target")
		}
		ts.ParentPlus1 = uint8(t.ParentPlus1)
	}
	return ts, nil
}

// tgtBoundV reports whether v is a bound kop.TgfBoundsV evaluates exactly: a
// Min of literal 0 (a Min over a value, Hide on the Ceiling's X..X, would
// refuse a cast whose candidates fall short: gorge aborts it, the lockstep
// has no card reaching that), or a Max that is a literal in range or any
// value cb.value lowers.
func tgtBoundV(v lValue, isMin bool) bool {
	switch {
	case isMin:
		return v.isConst() && v.N == 0
	case v.isConst():
		return v.N >= 1 && v.N < 255
	}
	return tgtBoundReads(v)
}

// tgtBoundReads reports whether v reads only what gorge's bound context
// (targetBoundCtx: the asking player, the source's SVars, the announced X,
// the trigger's amount) and the kernel's entry context hold alike: a battlefield
// or graveyard count, an aggregate, X, the trigger amount, and sums and
// products of those. Anything reading a chain's earlier result (the remembered
// set, a roll, excess damage), a target or the cast's payment is not evaluated
// the same at announcement.
// plainX reports whether v is the announced X itself.
func plainX(v lValue) bool {
	return v.Kind == vX && v.Mul == 0 && v.Div == 0 && v.Add == 0 && !v.Neg && len(v.Sub) == 0
}

// divTotalReads is tgtBoundReads widened to a divided total's
// "Count$Compare X LT6.X.Z" (Shatterskull Smashing's twice X from 6): a
// comparison of a bound-readable value whose two arms are bound-readable.
func divTotalReads(v lValue) bool {
	if v.Kind == vCompare {
		if len(v.Sub) != 2 || v.Sub[1].Kind != vBranches || len(v.Sub[1].Sub) != 2 {
			return false
		}
		return tgtBoundReads(v.Sub[0]) && divTotalReads(v.Sub[1].Sub[0]) && divTotalReads(v.Sub[1].Sub[1])
	}
	return tgtBoundReads(v)
}

func tgtBoundReads(v lValue) bool {
	switch v.Kind {
	case vConst, vX, vCount, vCountGraveyard, vValidAgg, vTriggerAmount, vOppsAttacked, vIfCastOwnMain, vTimesKicked:
		return true
	case vOppExtreme:
		// the asking player's living opponents (PlayerCountPlayers$Amount
		// is their count plus one): gorge's bound context holds the asker
		return v.N == kop.OppExtAmount
	case vSum, vProduct:
		for _, sv := range v.Sub {
			if !tgtBoundReads(sv) {
				return false
			}
		}
		return len(v.Sub) == 2
	}
	return false
}

func playerCtrl(p uint8) kop.CtrlSel {
	switch p {
	case plYou:
		return kop.CtrlYou
	case plOpp:
		return kop.CtrlOpp
	}
	return kop.CtrlAny
}

var abKinds = [...]kop.AbilityKind{akSpell: kop.AbSpell, akActivated: kop.AbActivated, akMana: kop.AbMana,
	akTriggered: kop.AbTriggered, akLatent: kop.AbTriggered}

func (cb *cardBuilder) ability(a *lAbility) (kop.Ability, error) {
	var ka kop.Ability
	ka.Kind = abKinds[a.Kind]
	var err error
	if a.Latent {
		ka.AFlags |= kop.AfLatent
		ka.MeleeGrant = a.Name == "GrantedMelee" // lower_meleegrant.go
	}
	if a.Kind == akSpell {
		ka.Cost = cb.spellCost
		if a.EvidenceTargets {
			ka.Evidence = kop.EvidenceTargetsMV // the targets' total mana value (cost_collectevidencex.go)
		}
		if a.ActCond.Kind != cNone {
			// a spell's cast restriction (IsPresent$; kernel castForbidden,
			// cast_spellcondition.go)
			if ka.Trig.Cond, err = cb.cond(a.ActCond); err != nil {
				return ka, err
			}
		}
	}
	if a.Latent && a.Name == "MayRaise" {
		if ka.Cost, err = cb.raiseCost(a.Cost); err != nil { // lower_mayraise.go
			return ka, err
		}
	}
	if a.Kind == akActivated || a.Kind == akMana {
		abCost := a.Cost
		abCost.EvidenceN = 0 // a table byte, not a cost flag (kop.Ability.Evidence)
		if a.Cost.EvidenceN > 0 && a.Kind == akMana {
			return ka, fmt.Errorf("evidence cost on a mana ability")
		}
		if ka.Cost, err = cb.cost(abCost); err != nil {
			return ka, err
		}
		if a.Kind == akMana && ka.Cost.Flags&(kop.CostForage|kop.CostExileGrave|kop.CostReturn) != 0 {
			// gorge pays these through the mana-activation walk
			// (AskManaForage); the kernel's mana abilities have no cost picks
			return ka, fmt.Errorf("mana ability with a forage / exile-from-graveyard cost")
		}
		if a.Sorcery && !a.Loyalty {
			ka.Cost.Flags |= kop.CostSorcery
		}
		if a.Loyalty {
			// CostLoyalty implies sorcery speed (dirac/vanilla's Ember Walker).
			ka.Cost.Flags |= kop.CostLoyalty
		}
		switch a.Limit {
		case 0:
		case 1:
			ka.Cost.Flags |= kop.CostOncePerGame
			ka.Once = uint8(a.Once)
		default:
			return ka, fmt.Errorf("activation limit %d", a.Limit)
		}
		ka.Exhaust = a.Exhaust
		ka.CantCopy = a.CantCopy
		ka.AnyActivator = a.AnyAct
		ka.Evidence = uint8(a.Cost.EvidenceN)
		ka.GrantedKw = a.Latent && strings.HasPrefix(a.Name, "GrantedFirebending")
		if a.PowerUp {
			if a.Latent {
				return ka, fmt.Errorf("power-up on a latent ability")
			}
			ka.Cost.Flags |= kop.CostPowerUp
		}
		if a.TurnOnce && (a.Latent || a.Zone != 0 && a.Zone != zBattlefield) {
			// the once-each-turn record (actUsed) is a battlefield permanent's
			return ka, fmt.Errorf("once-each-turn limit on a granted or non-battlefield ability")
		}
		exileGrant := a.Latent && a.Name == "GrantedExile" && a.Zone == zExile
		if a.ActCond.Kind != cNone && !exileGrant && (a.Latent || a.Zone != 0 && a.Zone != zBattlefield && a.Zone != zGraveyard) {
			// read by canActivate for a printed battlefield ability, and by
			// priorityGraveAct for a printed graveyard one
			return ka, fmt.Errorf("activation limit/condition on a granted or non-battlefield ability")
		}
		if a.ActCond.Kind != cNone && (a.Zone == zGraveyard || exileGrant) && !graveActCondOK(a.ActCond) {
			return ka, fmt.Errorf("activation condition %s on a graveyard ability", a.ActCond)
		}
		if a.Reduce != nil {
			if ka.OwnReduce, err = cb.value(*a.Reduce); err != nil {
				return ka, err
			}
		}
		if a.TurnOnce {
			if i := cb.abMap[a.Index]; i >= 8 {
				return ka, fmt.Errorf("once-each-turn ability index %d (the kernel tracks 8)", i)
			}
			ka.Cost.Flags |= kop.CostOncePerTurn
		}
		if a.ActCond.Kind != cNone {
			// a printed mana ability's gate rides Trig.Cond: the
			// kernel reads it in the planner/window variant, never as an
			// activation condition
			if a.Kind != akActivated && a.Kind != akMana || a.Trigger != nil {
				return ka, fmt.Errorf("activation condition on a non-activated ability")
			}
			if ka.Trig.Cond, err = cb.cond(a.ActCond); err != nil {
				return ka, err
			}
		}
		ka.ActZone = zoneMap[a.Zone]
		if a.Zone == zBattlefield {
			ka.ActZone = kop.ZoneNone
		}
	}
	if len(a.Targets) > kop.MaxTargets {
		return ka, fmt.Errorf("%d target slots", len(a.Targets))
	}
	for i, t := range a.Targets {
		if t.AtRes && cb.kicked && t.Min.Kind == vKicked && kickedConstRange(t.Min, t.Max, 1, 0) && kickedGuardedAsk(a.Prog, i) {
			// Tear Asunder's "if this spell was kicked, exile target
			// nonland permanent instead": a mid-resolution slot under the
			// sub's Condition$ Kicked guard, so asked only when kicked,
			// one object
			t.Min, t.Max = cst(1), cst(1)
		}
		if a.Kind == akActivated && len(a.Prog) == 1 && a.Prog[0].Op == "Attach" && t.F != nil {
			// gorge never offers an attach ability its own source as the
			// object (cast_targets.go excludeSelf, CR 701.3a): an Equipment
			// that is a creature is not an Equip target for itself. The
			// filter says so (fOther: matchPerm rejects src), so targetOK
			// needs no test of its own. Every any-of clause carries it.
			f := *t.F
			f.Clauses = append([]lClause(nil), f.Clauses...)
			for c := range f.Clauses {
				f.Clauses[c].Flags |= fOther
			}
			t.F = &f
		}
		if ka.Targets[i], err = cb.target(t); err != nil {
			return ka, err
		}
		if xt := &ka.Targets[i]; xt.TFlags&kop.TgfBoundsV != 0 && !xt.UpTo && ka.Kind != kop.AbSpell && !a.Cost.LoyaltyX && !xCountActivatedOK(a, &ka) {
			return ka, fmt.Errorf("target count X..X outside a spell")
		}
		if t.Mode >= 0 {
			ka.TgtModes[i] = 1 << t.Mode
		}
	}
	ka.NTargets = uint8(len(a.Targets))
	if a.Modes > 0 {
		ka.NModes, ka.ModesPick = uint8(a.Modes), uint8(a.ModesPick)
		ka.ModesFew = uint8(a.ModesPick - a.ModesMin)
		ka.ModeRestrict = a.ModeRestrict
		ka.ChainModes = a.ChainModes
		ka.RepMax, ka.RepCost, ka.RepBudget = a.RepMax, a.RepCost, a.RepBudget
	}
	if a.Mana != nil {
		if err := cb.mana(a.Mana, &ka); err != nil {
			return ka, err
		}
		if a.Kind == akMana && a.Cost.SubCtrX && ka.Cost.Flags&kop.CostLoyaltyX != 0 &&
			ka.AFlags&kop.AfManaDynamic != 0 && ka.ManaV.K == kop.VX {
			// Amount$ X on a SubCounter<X/KIND> mana ability is the number
			// of counters removed (CR 107.3, 118.3): the kernel's script
			// gap reads it off the source (mana_countercost.go).
			ka.ManaV = kop.ValueRef{K: kop.VCountersSelf, N: int16(ka.Cost.Ctr)}
		}
	}
	if a.Trigger != nil {
		if a.Trigger.OncePer && (a.Latent || a.Trigger.Zone == zCommand) && a.Trigger.Event != "CounterAddedOnce" {
			// the kernel's once-each-turn record is keyed by (object,
			// ability index); a granted or emblem trigger's index is the
			// granting card's
			return ka, fmt.Errorf("once-each-turn trigger on a granted or emblem ability")
		}
		if ka.Trig, err = cb.trigger(a.Trigger); err != nil {
			return ka, err
		}
		if ka.Trig.Whose == kop.WhoseChosen && (a.Latent || a.Trigger.Event != "Phase" || a.Trigger.Zone != zBattlefield) {
			// the kernel reads it only for a printed battlefield Phase listener (chosenIs)
			return ka, fmt.Errorf("Player.Chosen outside a printed battlefield Phase trigger")
		}
		if ka.Trig.Whose == kop.WhoseEnchCtrl && (a.Latent || a.Trigger.Event != "Phase" || a.Trigger.Zone != zNone && a.Trigger.Zone != zBattlefield) {
			// the kernel reads it only for a printed battlefield Phase listener
			return ka, fmt.Errorf("EnchantedController outside a printed battlefield Phase trigger")
		}
		if a.Name == "CheckOnTriggerOnly" {
			ka.AFlags |= kop.AfCheckOnTrigger
		}
		if a.Name == "Gift" {
			// a permanent's gift trigger (lowerGift): queued only when the
			// gift was promised, never checked again on resolution
			ka.AFlags |= kop.AfGift | kop.AfCheckOnTrigger
		}
	}
	if a.ResolveCount {
		// kernel countResolve runs in trigResolveOK before the
		// intervening-if: a trigger with one would count a resolution
		// gorge may not (refused). An activated ability counts as it
		// resolves (resolve.go), after the fizzle check, as gorge's Resolve
		// event does
		if a.Latent || a.Kind == akTriggered && (a.Trigger == nil || ka.Trig.Cond != 0) || a.Kind != akTriggered && a.Kind != akActivated {
			return ka, fmt.Errorf("Count$ResolvedThisTurn on a latent, conditional or non-printed-ability")
		}
		ka.AFlags |= kop.AfResolveCount
	}
	if a.Unless != nil {
		return ka, fmt.Errorf("ability-level unless cost")
	}
	if a.Name == "Equip" {
		ka.AFlags |= kop.AfEquip
	}
	if a.Cycling {
		ka.AFlags |= kop.AfCycling
	}
	if a.Station {
		ka.AFlags |= kop.AfStation
	}
	if a.Ninjutsu {
		ka.AFlags |= kop.AfNinjutsu
	}
	if a.Gate != nil {
		// the kernel reads a gate on a printed battlefield trigger or
		// activated ability only (kw_class.go, kw_saga.go)
		if a.Latent || a.Kind != akTriggered && a.Kind != akActivated || a.Kind == akActivated && a.Zone != 0 && a.Zone != zBattlefield && !(a.Gate.Speed && a.Zone == zGraveyard) ||
			a.Trigger != nil && a.Trigger.Zone != 0 && a.Trigger.Zone != zBattlefield || !a.Gate.Speed && (a.Gate.Level < 1 || a.Gate.Level > 256) {
			return ka, fmt.Errorf("gate on this ability")
		}
		if a.Gate.Speed {
			ka.Gate = kop.Gate{Ctr: kop.CtrMaxSpeed, N: 4}
		} else {
			switch a.Gate.Ctr {
			case "":
				ka.Gate = kop.Gate{Ctr: kop.CtrLevel, N: uint8(a.Gate.Level - 1), Eq: a.Gate.Eq}
			case "LORE":
				// a Saga chapter: a self trigger on a lore-counter event, gated
				// at its chapter number (CR 714.2b); it is never rechecked on
				// resolution, so a chapter ability already on the stack still
				// resolves after the lore counters change
				if a.Kind != akTriggered || a.Gate.Eq || a.Trigger == nil ||
					a.Trigger.Event != "CounterAddedOnce" || a.Trigger.Step != "LORE" {
					return ka, fmt.Errorf("lore gate on this ability")
				}
				ka.Gate = kop.Gate{Ctr: kop.CtrLore, N: uint8(a.Gate.Level)}
				ka.AFlags |= kop.AfCheckOnTrigger
			case "CHARGE":
				// a Station threshold's granted trigger (lower_stationgrant.go):
				// present while the source holds N charge counters, matched when
				// its event happens and not rechecked on resolution (CR 603.4
				// does not apply to a grant)
				if a.Kind != akTriggered || a.Gate.Eq || a.Gate.Level > 255 {
					return ka, fmt.Errorf("charge gate on this ability")
				}
				ka.Gate = kop.Gate{Ctr: kop.CtrCharge, N: uint8(a.Gate.Level)}
				ka.AFlags |= kop.AfCheckOnTrigger
			case "MODE":
				// a Siege's mode grant (lower_siegemode.go): present while the
				// source's chosen mode is Level-1, matched when its event
				// happens and not rechecked on resolution
				if a.Kind != akTriggered || a.Gate.Eq || a.Gate.Level > 2 {
					return ka, fmt.Errorf("mode gate on this ability")
				}
				ka.Gate = kop.Gate{Ctr: kop.CtrMode, N: uint8(a.Gate.Level - 1)}
				ka.AFlags |= kop.AfCheckOnTrigger
			default:
				return ka, fmt.Errorf("gate counter %q", a.Gate.Ctr)
			}
		}
	}
	if a.Trigger != nil && a.Trigger.CrossAt > 0 {
		// CounterAdded "when the Nth <kind> counter is put on": the kind's
		// count after the event must have just reached N (the Gate threshold,
		// read by counterAddedCrossed; trig_counteradded.go). Checked
		// when the event happens only, like a Saga chapter.
		k, err := counterKind(a.Trigger.Step)
		if err != nil {
			return ka, err
		}
		if a.Gate != nil || k == kop.CtrLevel || k == kop.CtrLore || a.Latent || a.Kind != akTriggered || a.Trigger.Zone != 0 && a.Trigger.Zone != zBattlefield {
			return ka, fmt.Errorf("counter crossing on this ability")
		}
		ka.Gate = kop.Gate{Ctr: k, N: uint8(a.Trigger.CrossAt)}
		ka.AFlags |= kop.AfCheckOnTrigger
	}
	if ka.Prog, err = cb.prog(a.Prog); err != nil {
		return ka, err
	}
	return ka, nil
}

func (cb *cardBuilder) mana(m *lMana, ka *kop.Ability) error {
	if m.Special == "EachColorAmong" && m.Restrict == nil && m.RestrictAb == nil && m.Amount.isConst() && m.Amount.N == 1 {
		// one mana of each colour among the matching permanents: gorge's
		// planner census cannot price it (a gap its potential plays reach
		// by a scripted activation), so it is no autopay source
		id, err := cb.filter(m.SpecialF, kop.ZoneBattlefield)
		if err != nil {
			return err
		}
		ka.AFlags |= kop.AfManaEachColor
		ka.ManaAmong = id
		ka.ManaN = 1
		return nil
	}
	if m.Special == "Reflected" && m.Restrict == nil && m.RestrictAb == nil && m.Amount.isConst() && m.Amount.N == 1 {
		// one mana of any colour among the matching permanents' colours
		// (Plaza of Heroes), or among what the matching permanents' mana
		// abilities could produce (Ability.ManaProduce, every Valid$ alternative
		// OppCtrl: Exotic Orchard; lower_reflectedmana.go)
		id, err := cb.filter(m.SpecialF, kop.ZoneBattlefield)
		if err != nil {
			return err
		}
		ka.AFlags |= kop.AfManaChoice | kop.AfManaReflect
		if m.Produce {
			ka.ManaProduce = true
		}
		ka.ManaAmong = id
		ka.ManaColors = kop.ColAny
		ka.ManaN = 1
		return nil
	}
	if m.Special == "ReflectedImprint" && m.Restrict == nil && m.RestrictAb == nil && m.Amount.isConst() && m.Amount.N == 1 {
		// (Chrome Mox) a printed AfManaReflect with no ManaAmong filter
		// reads the colours of the cards its source imprinted (kernel
		// mana_imprint.go; lower_imprintmox.go)
		ka.AFlags |= kop.AfManaChoice | kop.AfManaReflect
		ka.ManaColors = kop.ColAny
		ka.ManaN = 1
		return nil
	}
	if m.Special == "ReflectedIdentity" && m.Restrict == nil && m.RestrictAb == nil && m.Amount.isConst() && m.Amount.N == 1 {
		// (Arcane Signet) a printed AfManaReflect with no ManaAmong filter and
		// ManaIdentity reads the controller's commanders' colour identity
		// (kernel mana_reflected.go identityColours; lower_zone.go lowerMana)
		ka.AFlags |= kop.AfManaChoice | kop.AfManaReflect
		ka.ManaIdentity = true
		ka.ManaColors = kop.ColAny
		ka.ManaN = 1
		return nil
	}
	if m.Special == "ReflectedSelf" && m.Restrict == nil && m.RestrictAb == nil && m.Amount.isConst() && m.Amount.N == 1 {
		// a granted "any of this creature's colors" (Katilda): AfManaReflect
		// with no ManaAmong filter reads the grantee's own colours
		ka.AFlags |= kop.AfManaChoice | kop.AfManaReflect
		ka.ManaColors = kop.ColAny
		ka.ManaN = 1
		return nil
	}
	if m.Special != "" {
		return fmt.Errorf("mana %s has no kop form", m.Special)
	}
	if m.RestrictAb != nil && m.Restrict == nil && !m.NonSpell {
		// an activated-only restriction (Shang-Chi, Master of Kung Fu's
		// RestrictValid$ Activated.Creature): its spell half matches no
		// spell (the clause manaRestrict lowers Spell.isCastFaceDown to),
		// so ManaRestrict marks the source restricted and pays no spell
		m.Restrict = &lFilter{Clauses: []lClause{{NotTypes: tAnyCard}}}
	}
	if m.RestrictAb != nil && m.Restrict == nil {
		return fmt.Errorf("mana spend restriction on activated abilities has no kop form")
	}
	if m.Choice {
		ka.AFlags |= kop.AfManaChoice
	}
	ka.ManaEach = m.Each
	ka.ManaDeferred = m.Deferred
	ka.ManaRider = uint8(m.Rider)
	if m.RiderCtr != "" {
		k, err := counterKind(m.RiderCtr)
		if err != nil {
			return err
		}
		ka.ManaRiderCtr = k
	}
	if m.RiderN > 0 {
		if m.RiderN > 255 {
			return fmt.Errorf("mana rider count %d", m.RiderN)
		}
		ka.ManaRiderN = uint8(m.RiderN)
	}
	if sp := m.Split; sp != nil {
		ka.SplitGate, ka.SplitCols = sp.Gate, colorMask(sp.Cols)
		if sp.Ctr != "" {
			k, err := counterKind(sp.Ctr)
			if err != nil {
				return err
			}
			ka.SplitCtr = k
		}
	}
	// m.RestrictAb beside a spell restriction (Secluded Courtyard: "or
	// activate an ability of a creature of the chosen type") is the
	// activated half of the restriction (ManaRestrictAb); m.NonSpell adds
	// every payment that is not a spell (the Powerstone; AfManaNonSpell).
	switch m.Colors {
	case "Chosen":
		ka.AFlags |= kop.AfManaChosen
	case "WChosen", "UChosen", "BChosen", "RChosen", "GChosen":
		// Combo <C> Chosen (Citadel Gate, Sea Gate): the
		// printed colour or the permanent's chosen one (srcMaskAt)
		ka.AFlags |= kop.AfManaChosen
		ka.ManaColors = colorMask(strings.TrimSuffix(m.Colors, "Chosen"))
	case "WUBRG":
		ka.ManaColors = kop.ColAny
	default:
		ka.ManaColors = colorMask(m.Colors)
	}
	if m.Choice && len(m.Colors) > 1 &&
		strings.Trim(m.Colors, "WUBRG") == "" && !colorsInWUBRGOrder(m.Colors) {
		ka.ManaColOrd = manaColOrd(m.Colors)
	}
	if m.Amount.isConst() {
		ka.ManaN = uint8(m.Amount.N)
	} else {
		v, err := cb.value(m.Amount)
		if err != nil {
			return err
		}
		ka.AFlags |= kop.AfManaDynamic
		ka.ManaV = v
		ka.ManaN = 1
	}
	if m.Restrict != nil {
		id, err := cb.filter(m.Restrict, kop.ZoneStack)
		if err != nil {
			return err
		}
		ka.ManaRestrict = id
	}
	if m.RestrictAb != nil {
		id, err := cb.filter(m.RestrictAb, kop.ZoneNone)
		if err != nil {
			return err
		}
		ka.ManaRestrictAb = id
	}
	if m.NonSpell {
		ka.AFlags |= kop.AfManaNonSpell
	}
	if m.HasteValid != nil {
		// the kernel reads the rider off the tapped card's mana ability
		// (dirac/mana_spentrider.go): a second mana ability beside it would
		// carry the rider onto its own taps
		n := 0
		for _, a := range cb.c.Abilities {
			if a.Kind == akMana {
				n++
			}
		}
		if n != 1 {
			return fmt.Errorf("AddsKeywords$ mana rider beside another mana ability")
		}
		id, err := cb.filter(m.HasteValid, kop.ZoneStack)
		if err != nil {
			return err
		}
		ka.ManaHasteValid = id
	}
	return nil
}

var stepMap = map[string]kop.Step{"Upkeep": kop.StepUpkeep, "End": kop.StepEnd, "BeginCombat": kop.StepBeginCombat,
	"Draw": kop.StepDraw, "Main1": kop.StepMain1, "Main2": kop.StepMain2, "EndCombat": kop.StepEndCombat}

var eventMap = map[string]kop.TrigEvent{"ETB": kop.TrigETB, "Dies": kop.TrigDies, "Leaves": kop.TrigLeaves, "ToGraveyard": kop.TrigToGraveyard, "Blocks": kop.TrigBlocks, "Taps": kop.TrigTaps, "Attacks": kop.TrigAttacks,
	"AttackersDeclared": kop.TrigAttackersDeclared, "Phase": kop.TrigPhase, "SpellCast": kop.TrigSpellCast,
	"LifeGained": kop.TrigLifeGained, "LifeLost": kop.TrigLifeLost, "DamageDone": kop.TrigDamageDone,
	"DamageDealtOnce": kop.TrigDamageDealtOnce, "Drawn": kop.TrigDrawn, "Discarded": kop.TrigDiscarded, "DiscardedAll": kop.TrigDiscardedAll, "AbilityCast": kop.TrigAbilityCast, "Scry": kop.TrigScry, "Surveil": kop.TrigSurveil, "Explores": kop.TrigExplores, "CollectEvidence": kop.TrigCollectEvidence, "Exploited": kop.TrigExploited,
	"LandPlayed": kop.TrigLandPlayed, "SearchedLibrary": kop.TrigSearchedLibrary, "Forage": kop.TrigForage, "Discover": kop.TrigDiscover, "ManifestDread": kop.TrigManifestDread,
	"CounterAddedOnce": kop.TrigCounterAddedOnce, "CounterAdded": kop.TrigCounterAddedOnce, "Untaps": kop.TrigUntaps,
	"Sacrificed": kop.TrigSacrificed, "GiveGift": kop.TrigGiveGift, "Proliferate": kop.TrigProliferate, "FacesDilemma": kop.TrigFacesDilemma, "BecomeMonarch": kop.TrigBecomeMonarch, "TokenCreated": kop.TrigTokenCreated, "BecomeMonstrous": kop.TrigBecomeMonstrous, "ManaExpend": kop.TrigManaExpend, "AttackerBlocked": kop.TrigBecomesBlocked, "TurnFaceUp": kop.TrigTurnFaceUp, "Transformed": kop.TrigTransformed, "CommitCrime": kop.TrigCommitCrime, "UnlockDoor": kop.TrigUnlockDoor, "FullyUnlock": kop.TrigFullyUnlock,
	"BecomesTarget":    kop.TrigBecomesTarget,    // the Valiant, watcher and self shapes (lower_trigger.go)
	"ClassLevelGained": kop.TrigCounterAddedOnce, // N = CtrLevel (lower_class.go)
	"BecomesPlotted":   kop.TrigBecomesPlotted,   // the card's own
	"BecomesSaddled":   kop.TrigBecomesSaddled,
	"Evolved":          kop.TrigEvolved,    // the source's own, after its evolve counter lands
	"Attached":         kop.TrigAttached,   // the attachment's own, Creature bearer (Assimilation Aegis)
	"Unattached":       kop.TrigUnattached} // the attachment's own, the FORMER BEARER (Captain's Hook family)

func (cb *cardBuilder) trigger(t *lTrigger) (kop.Trigger, error) {
	var kt kop.Trigger
	ev, ok := eventMap[t.Event]
	if !ok {
		return kt, fmt.Errorf("trigger event %s", t.Event)
	}
	kt.Event = ev
	kt.Whose = kop.Whose(t.Whose)
	if t.Event == "Leaves" || t.Event == "ToGraveyard" {
		kt.From = zoneMap[t.From]
	}
	var err error
	if t.F != nil {
		f := *t.F
		var rest []lClause
		self, selfCast, selfCreature := false, false, false
		for _, c := range f.Clauses {
			if c == (lClause{Flags: fSelf}) || c == (lClause{Types: tCreature, Flags: fSelf}) {
				self = true
				selfCreature = c.Types != 0
				continue
			}
			rest = append(rest, c)
		}
		f.Clauses = rest
		if !self && t.Event == "SpellCast" {
			sc, err := selfCastClause(&f)
			if err != nil {
				return kt, err
			}
			if sc {
				kt.Flags |= kop.TfSelf
				rest = f.Clauses
				self = false // the rest is the spell's Filter, not an alternative
				selfCast = true
			}
		}
		if f.Players != 0 {
			return kt, fmt.Errorf("player filter on a trigger object")
		}
		switch {
		case self && len(rest) == 0:
			kt.Flags |= kop.TfSelf
			if selfCreature && t.Event == "Dies" {
				// a creature card that may leave as something else (CR 700.4,
				// Tom, Bert, and William returned as an artifact)
				kt.Filter, err = cb.filter(&lFilter{Clauses: []lClause{{Types: tCreature}}}, kop.ZoneBattlefield)
				if err != nil {
					return kt, err
				}
			}
		case self:
			kt.Flags2 |= kop.Tf2SelfOrFilter
			fallthrough
		default:
			if len(rest) > 0 {
				zone := kop.ZoneBattlefield
				if t.Event == "SpellCast" {
					zone = kop.ZoneStack
					if t.NthFiltered {
						zone = kop.ZoneNone // it also tests the earlier pushes, wherever they now live
					}
				}
				if t.Event == "Discarded" {
					zone = kop.ZoneGraveyard
				}
				old := cb.selfCast
				cb.selfCast = selfCast
				kt.Filter, err = cb.filter(&f, zone)
				cb.selfCast = old
				if err != nil {
					return kt, err
				}
			}
		}
	}
	if t.F2 != nil {
		if t.TargetsAll && len(t.F2Zones) != len(t.F2.Clauses) {
			return kt, fmt.Errorf("TargetsValid object alternative zones")
		}
		switch {
		case len(t.F2.Clauses) == 0 && t.F2.Players == plBoth:
			kt.Flags |= kop.TfToPlayer
		case len(t.F2.Clauses) == 0 && t.F2.Players == plOpp:
			kt.Flags |= kop.TfToPlayer
			kt.Flags2 |= kop.Tf2ToOpponent
		case len(t.F2.Clauses) == 0 && t.F2.Players == plYou && t.Event == "DamageDone":
			kt.Flags |= kop.TfToPlayer
			kt.Flags2 |= kop.Tf2ToYou
		case t.TargetsAll && (t.Event == "SpellCast" || t.Event == "AbilityCast") && len(t.F2.Clauses) > 0 && t.F2.Players == 0:
			// TargetsValid$ without a player alternative: the battlefield
			// clauses keep ZoneNone, a zone-free one carries kop.ZoneAny
			if kt.Filter2, err = cb.filterZones(t.F2, t.F2Zones); err != nil {
				return kt, err
			}
		case t.F2.Players == 0:
			if kt.Filter2, err = cb.filter(t.F2, kop.ZoneBattlefield); err != nil {
				return kt, err
			}
		case t.Event == "DamageDone" && t.F2.Players == plBoth && len(t.F2.Clauses) == 1 && t.F2.Clauses[0] == (lClause{Types: tPlaneswalker}):
			// ValidTarget$ Player,Planeswalker: TfToPlayer beside a Filter2
			// reads "a player, or a permanent matching Filter2"
			// (dirac/trig_damagedonce.go ddoRecipient)
			kt.Flags |= kop.TfToPlayer
			if kt.Filter2, err = cb.filter(&lFilter{Clauses: t.F2.Clauses}, kop.ZoneBattlefield); err != nil {
				return kt, err
			}
		case t.TargetsAll && (t.Event == "SpellCast" || t.Event == "AbilityCast") && len(t.F2.Clauses) > 0 && (t.F2.Players == plYou || t.F2.Players == plOpp || t.F2.Players == plBoth):
			// TargetsValid$ with a player alternative
			if kt.Filter2, err = cb.filterPlayerAlt(t.F2, t.F2Zones, t.F2.Players); err != nil {
				return kt, err
			}
		default:
			return kt, fmt.Errorf("trigger second filter %s", t.F2)
		}
	}
	if t.Event == "Phase" {
		s, ok := stepMap[t.Step]
		if !ok {
			return kt, fmt.Errorf("phase %s", t.Step)
		}
		kt.Step = s
	}
	if t.Event == "ClassLevelGained" {
		kt.N = int16(kop.CtrLevel)
	}
	if t.Step == "*" && (t.Event == "CounterAddedOnce" || t.Event == "CounterAdded") {
		kt.N = kop.TrigNAnyCounter
	} else if t.Event == "CounterAddedOnce" || t.Event == "CounterAdded" {
		k, err := counterKind(t.Step)
		if err != nil {
			return kt, err
		}
		kt.N = int16(k)
	}
	if t.MinAttackers > 0 {
		kt.N = trigCmpN(t.AtkCmp, t.MinAttackers)
	}
	if t.Event == "AbilityCast" {
		kt.N = t.AbClass
	}
	if t.Event == "LandPlayed" || t.Event == "Leaves" || t.Event == "BecomeMonarch" {
		kt.N = t.N // Leaves: the destination mask; BecomeMonarch: kop.TrigNMonarchBeganTurn
	}
	switch t.Combat {
	case 1:
		kt.Flags |= kop.TfCombat
	case -1:
		kt.Flags2 |= kop.Tf2NonCombat
	}
	if t.Optional {
		kt.Flags |= kop.TfOptional
	}
	if t.OncePer {
		kt.Flags |= kop.TfOncePerTurn
	}
	if t.First {
		kt.Flags2 |= kop.Tf2FirstTime
	}
	switch t.NthDraw {
	case 0:
	case 2:
		kt.Flags |= kop.TfSecondDraw
	default:
		// Number$ N past the second (Sneaky Snacker's third): the same
		// gate, N carrying the ordinal (kernel nthDrawOK)
		if t.NthDraw < 1 || t.NthDraw > 255 || t.Event != "Drawn" {
			return kt, fmt.Errorf("draw number %d", t.NthDraw)
		}
		kt.Flags |= kop.TfSecondDraw
		kt.N = int16(t.NthDraw)
	}
	if t.OnlyYourTurn {
		kt.Flags2 |= kop.Tf2YourTurn
	}
	if t.OnlyOppTurn {
		kt.Flags2 |= kop.Tf2OppTurn
	}
	if t.NthSpell > 0 {
		if t.Event != "SpellCast" || t.CastSnap {
			return kt, fmt.Errorf("N-th spell on a %s trigger", t.Event)
		}
		kt.Flags2 |= kop.Tf2NthSpell
		kt.N = trigCmpN(t.NthCmp, t.NthSpell)
		if t.NthFiltered {
			kt.Flags4 |= kop.Tf4NthFiltered
		}
	}
	if t.Event == "ManaExpend" {
		if t.Amount <= 0 {
			return kt, fmt.Errorf("expend %d", t.Amount)
		}
		kt.N = int16(t.Amount)
	}
	if t.Training {
		if t.Event != "Attacks" || kt.Flags&kop.TfSelf == 0 {
			return kt, fmt.Errorf("Training on a %s trigger", t.Event)
		}
		kt.N = kop.TrigNTraining
	}
	if t.Valiant {
		if t.Event != "BecomesTarget" || kt.Flags&kop.TfSelf == 0 {
			return kt, fmt.Errorf("Valiant on a %s trigger", t.Event)
		}
		kt.Whose = kop.WhoseYou
		kt.N = kop.TrigNValiant
	} else if t.Event == "BecomesTarget" {
		// A non-Valiant BecomesTarget line is a watcher. Its filter may be
		// the watcher's own object (ValidTarget$ Card.Self, the
		// "becomes the target of an opponent's spell or ability" shape,
		// ): the trigger builder set TfSelf only when the
		// whole filter was the bare self clause, so that flag is the
		// structural test -- no second combination can carry it.
		// a watcher (opponent-source gated or not) or the plain self form
		// (TfSelf, no gate: "becomes the target of a spell or ability")
		selfPlain := !t.Watch && kt.Flags&kop.TfSelf != 0 && kt.Whose == kop.WhoseAny
		if !selfPlain && !(t.Watch && (kt.Whose == kop.WhoseOpp || kt.Whose == kop.WhoseAny)) {
			return kt, fmt.Errorf("BecomesTarget without Valiant")
		}
		if t.SpellToo {
			if kt.Flags&kop.TfSelf != 0 {
				return kt, fmt.Errorf("SpellToo on a self BecomesTarget")
			}
			kt.Flags2 |= kop.Tf2SpellToo
		}
	}
	if t.TeamworkTap {
		if t.Event != "Taps" || kt.Flags&kop.TfSelf == 0 {
			return kt, fmt.Errorf("Teamwork on a %s trigger", t.Event)
		}
		kt.N = kop.TrigNTeamworkTap
	}
	if t.TapsForMana && (t.Event != "Taps" || kt.Flags&kop.TfSelf != 0) {
		return kt, fmt.Errorf("TapsForMana on a %s trigger", t.Event)
	}
	if t.BlockedBySelf {
		kt.Flags2 |= kop.Tf2BlockedBySelf
	}
	if t.Attacked != 0 {
		if t.Event != "Attacks" {
			return kt, fmt.Errorf("Attacked on a %s trigger", t.Event)
		}
		if t.Attacked&atkYou != 0 {
			kt.Flags2 |= kop.Tf2AtkYou
		}
		if t.Attacked&atkYourPW != 0 {
			kt.Flags2 |= kop.Tf2AtkYourPW
		}
		if t.Attacked&atkOpp != 0 {
			kt.Flags2 |= kop.Tf2AtkOpp
		}
	}
	if t.PerCounter {
		if t.Event != "CounterAdded" {
			return kt, fmt.Errorf("PerCounter on a %s trigger", t.Event)
		}
		kt.Flags4 |= kop.Tf4PerCounter
	}
	if t.Evolve {
		if t.Event != "ETB" {
			return kt, fmt.Errorf("Evolve on a %s trigger", t.Event)
		}
		kt.Flags2 |= kop.Tf2Evolve
	}
	if t.Increment {
		if t.Event != "SpellCast" || t.OrCopy || t.CastSnap || t.CastSpent || t.NthSpell != 0 || t.TargetsAll {
			return kt, fmt.Errorf("Increment on a %s trigger", t.Event)
		}
		kt.Flags2 |= kop.Tf2Increment
	}
	if t.SpentGate != 0 {
		// Trigger.N is read on SpellCast only under
		// Tf2NthSpell (castTrigGen), so the gate rides in N with no flag.
		if t.Event != "SpellCast" || t.OrCopy || t.CastSnap || t.CastSpent || t.NthSpell != 0 || t.Increment {
			return kt, fmt.Errorf("spend gate on a %s trigger", t.Event)
		}
		kt.N = t.SpentGate
	}
	if t.Alone {
		if t.Event != "Attacks" {
			return kt, fmt.Errorf("Alone on a %s trigger", t.Event)
		}
		kt.Flags2 |= kop.Tf2Alone
	}
	if t.Cycled {
		if t.Event != "Discarded" {
			return kt, fmt.Errorf("cycled self trigger on a %s trigger", t.Event)
		}
		kt.Flags2 |= kop.Tf2Cycled
	}
	if t.DefPlayer != 0 {
		if t.Event != "AttackersDeclared" {
			return kt, fmt.Errorf("defender-player flag on a %s trigger", t.Event)
		}
		kt.Flags2 |= kop.Tf2DefPlayer
		switch t.DefPlayer {
		case 2:
			kt.Flags2 |= kop.Tf2AtkOpp
		case 3:
			kt.Flags2 |= kop.Tf2AtkYou // AttackedTarget$ You: the defender is the controller (kernel fireDefPlayer)
		case 4:
			// AttackedTarget$ Player.EnchantedBy: the defender is enchanted
			// by any enchantment (gorge playerSpecQualifierEnchantedBy); the
			// kernel scans the player-attached auras' seats (fireDefPlayer)
			kt.Flags4 |= kop.Tf4DefEnch
		}
	}
	if t.OrCopy {
		if t.Event != "SpellCast" || t.CastSnap || t.NthSpell != 0 || t.TargetsAll {
			return kt, fmt.Errorf("or-copy flag on a %s trigger", t.Event)
		}
		kt.Flags2 |= kop.Tf2OrCopy
	}
	if t.ResolvedLimit {
		kt.Flags2 |= kop.Tf2ResolvedLimit
	}
	if t.XBound {
		// kxpaid (lower_xbound.go): the body's X is the trigger object's
		// spell's paid X, captured as it triggers
		if t.Event != "ETB" && t.Event != "SpellCast" && t.Event != "TurnFaceUp" && !(t.Event == "Discarded" && t.Cycled) {
			return kt, fmt.Errorf("bound X on a %s trigger", t.Event)
		}
		kt.Flags2 |= kop.Tf2XBound
	}
	if t.SpentBound {
		if t.Event != "ETB" || t.XBound {
			return kt, fmt.Errorf("bound spend on a %s trigger", t.Event)
		}
		kt.Flags2 |= kop.Tf2SpentBound
	}
	if t.Unblocked {
		if t.Event != "AttackerBlocked" || t.ByCreature || t.BlockedBySelf {
			return kt, fmt.Errorf("unblocked flag on a %s trigger", t.Event)
		}
		kt.Flags2 |= kop.Tf2Unblocked
	}
	if t.ByCreature {
		if t.Event != "AttackerBlocked" {
			return kt, fmt.Errorf("by-creature pairs on a %s trigger", t.Event)
		}
		kt.Flags2 |= kop.Tf2PerPair
	}
	if t.TargetsAll {
		if t.Event != "SpellCast" && t.Event != "AbilityCast" || kt.Filter2 == 0 {
			return kt, fmt.Errorf("TargetsValid on a %s trigger", t.Event)
		}
		kt.Flags2 |= kop.Tf2TargetsFilter2
	}
	if t.Batch {
		if t.Event != "ETB" && t.Event != "Dies" && t.Event != "Leaves" && t.Event != "DamageDone" && t.Event != "Sacrificed" && t.Event != "TokenCreated" && t.Event != "Taps" {
			return kt, fmt.Errorf("batch flag on a %s trigger", t.Event)
		}
		kt.Flags2 |= kop.Tf2Batch
	}
	if t.SrcInPlay {
		if t.Event != "DamageDone" {
			return kt, fmt.Errorf("source-in-play flag on a %s trigger", t.Event)
		}
		kt.Flags2 |= kop.Tf2SrcInPlay
	}
	if t.PerRef {
		if t.Event != "DamageDone" || !t.Batch {
			return kt, fmt.Errorf("per-referent flag on a %s trigger", t.Event)
		}
		kt.Flags2 |= kop.Tf2PerReferent
		kt.N = t.DmgGate
	}
	if t.NotToGrave {
		if t.Event != "Leaves" || !t.Batch {
			return kt, fmt.Errorf("not-to-graveyard on a %s trigger", t.Event)
		}
		kt.Flags |= kop.TfNotToGrave
	}
	if t.AlsoGrave {
		if t.Event != "Leaves" || !t.Batch || t.N == 0 {
			return kt, fmt.Errorf("graveyard-too flag on a %s trigger", t.Event)
		}
		kt.Flags2 |= kop.Tf2LeavesGrave
	}
	if t.CastSpent {
		switch {
		case t.Event != "SpellCast":
			return kt, fmt.Errorf("cast-spent snapshot on a %s trigger", t.Event)
		case t.Zone != zNone && t.Zone != zBattlefield:
			return kt, fmt.Errorf("TriggeredCard$CastTotalManaSpent off the battlefield")
		case kt.Flags&kop.TfSelf != 0:
			return kt, fmt.Errorf("TriggeredCard$CastTotalManaSpent on a self cast trigger")
		case t.OrCopy:
			return kt, fmt.Errorf("TriggeredCard$CastTotalManaSpent beside OrCopy")
		case t.CastSnap:
			return kt, fmt.Errorf("TriggeredCard$CastTotalManaSpent beside a cast-count snapshot")
		}
		kt.Flags2 |= kop.Tf2CastSpent
	}
	if t.ConvergeBound {
		// the entering spell's colours spent ride x
		if t.Event != "ETB" || kt.Flags&kop.TfSelf == 0 || t.Batch || t.XBound || t.SpentBound {
			return kt, fmt.Errorf("bound converge on a %s trigger", t.Event)
		}
		kt.Flags3 |= kop.Tf3ConvergeBound
	}
	if t.CastConverge {
		// the triggering spell's colours spent ride the amount
		switch {
		case t.Event != "SpellCast":
			return kt, fmt.Errorf("TriggeredCard$Converge on a %s trigger", t.Event)
		case t.Zone != zNone && t.Zone != zBattlefield:
			return kt, fmt.Errorf("TriggeredCard$Converge off the battlefield")
		case kt.Flags&kop.TfSelf != 0:
			return kt, fmt.Errorf("TriggeredCard$Converge on a self cast trigger")
		case t.OrCopy:
			return kt, fmt.Errorf("TriggeredCard$Converge beside OrCopy")
		case t.CastSpent:
			return kt, fmt.Errorf("TriggeredCard$Converge beside TriggeredCard$CastTotalManaSpent")
		case t.CastSnap:
			return kt, fmt.Errorf("TriggeredCard$Converge beside a cast-count snapshot")
		case t.Convoked:
			return kt, fmt.Errorf("TriggeredCard$Converge beside Convoked$Amount")
		}
		kt.Flags3 |= kop.Tf3CastConverge
	}
	if t.CastHand {
		// the cast-from-your-hand-by-you gate, read on the
		// stack entry (SpellCast: the spell; ETB: the resolving spell that
		// put the source onto the battlefield)
		switch {
		case t.Event == "SpellCast" && kt.Flags&kop.TfSelf == 0 && !t.OrCopy:
		case t.Event == "ETB" && kt.Flags&kop.TfSelf != 0:
		default:
			return kt, fmt.Errorf("cast-from-hand gate on a %s trigger", t.Event)
		}
		kt.Flags2 |= kop.Tf2CastFromHand
	}
	if t.Convoked {
		if t.Event != "ETB" || kt.Flags&kop.TfSelf == 0 {
			return kt, fmt.Errorf("Convoked$Amount on a %s trigger", t.Event)
		}
		kt.Flags2 |= kop.Tf2Convoked
	}
	if t.ManaFromSelf {
		if t.Event != "SpellCast" || kt.Flags&kop.TfSelf != 0 || t.OrCopy || t.CastSnap || t.NthSpell > 0 {
			return kt, fmt.Errorf("mana-from-source gate on a %s trigger", t.Event)
		}
		kt.Flags2 |= kop.Tf2ManaFromSelf
	}
	if t.MayPlaySrc {
		// ValidSA$ Spell.MayPlaySource, read off the
		// spell's stack entry as the trigger fires (a TfSelf carrier keeps
		// its empty filter, which fireSelfCast would strip anyway)
		if t.Event != "SpellCast" || t.OrCopy {
			return kt, fmt.Errorf("may-play-source gate on a %s trigger", t.Event)
		}
		kt.Flags3 |= kop.Tf3MayPlaySrc
	}
	if t.Static {
		// Static$ True resolves off the stack before
		// the ordering ask; kgen already proved the body ask-free
		if t.Event != "SpellCast" || t.OrCopy {
			return kt, fmt.Errorf("Static$ True on a %s trigger", t.Event)
		}
		kt.Flags3 |= kop.Tf3Static
	}
	if t.ManaFromTreasure {
		if t.Event != "SpellCast" || kt.Flags&kop.TfSelf != 0 || t.OrCopy {
			return kt, fmt.Errorf("mana-from-Treasure gate on a %s trigger", t.Event)
		}
		kt.Flags3 |= kop.Tf3ManaFromTreasure
	}
	if t.OrTargets {
		if t.Event != "SpellCast" || kt.Flags&kop.TfSelf != 0 || t.OrCopy || t.TargetsAll || kt.Filter == 0 || kt.Filter2 == 0 {
			return kt, fmt.Errorf("IsTargeting ValidSA on a %s trigger", t.Event)
		}
		kt.Flags3 |= kop.Tf3OrTargets
	}
	if t.FirstAttack {
		kt.Flags3 |= kop.Tf3FirstAttack
	}
	kt.Flags5 = drawStepFlags(t)
	if t.OppCause {
		if t.Event != "Discarded" || kt.Flags&kop.TfSelf == 0 || t.Cycled {
			return kt, fmt.Errorf("discard cause on a %s trigger", t.Event)
		}
		kt.Flags5 |= kop.Tf5OppCause
	}
	if t.GameOnce {
		kt.Flags3 |= kop.Tf3GameOnce
	}
	kt.TapPlan, kt.TapPainN = kop.TapPlan(t.TapPlan), t.TapPainN
	if t.SingleTarget {
		if t.Event != "SpellCast" {
			return kt, fmt.Errorf("single-target flag on a %s trigger", t.Event)
		}
		kt.Flags4 |= kop.Tf4SingleTarget
	}
	if t.CauseTarget {
		if t.Event != "DamageDone" {
			return kt, fmt.Errorf("cause-target flag on a %s trigger", t.Event)
		}
		kt.Flags4 |= kop.Tf4SingleCauseTarget
	}
	if t.CounterCell {
		kt.Flags4 |= kop.Tf4CounterCell
	}
	if t.OptTrigPlayer {
		if !t.Optional || (t.Event != "Phase" && t.Event != "AttackersDeclared") {
			return kt, fmt.Errorf("optional trigger player on a %s trigger", t.Event)
		}
		if t.Event == "Phase" && t.Whose != whoAny {
			// only the every-player's-turn shape is played against gorge
			// (Peace in Our Time); Apathy's ValidPlayer$ enchanted-controller
			// line has no lockstep yet
			return kt, fmt.Errorf("optional trigger player under ValidPlayer$")
		}
		kt.Flags4 |= kop.Tf4OptTrigPlayer
	}
	if t.OptSrcCtrl && (!t.Optional || t.Event != "DamageDone") {
		return kt, fmt.Errorf("optional source controller on a %s trigger", t.Event)
	}
	if t.NamedCast {
		if t.Event != "SpellCast" {
			return kt, fmt.Errorf("named-cast flag on a %s trigger", t.Event)
		}
		kt.Flags4 |= kop.Tf4NamedCast
	}
	if t.Outstanding {
		if t.Event != "CounterAddedOnce" && t.Event != "CounterAdded" || kt.Flags&kop.TfSelf == 0 {
			return kt, fmt.Errorf("outstanding latch on a %s trigger", t.Event)
		}
		kt.Flags4 |= kop.Tf4StateCtr
	}
	if t.CastSnap {
		if t.Event != "SpellCast" {
			return kt, fmt.Errorf("cast-count snapshot on a %s trigger", t.Event)
		}
		kt.Flags2 |= kop.Tf2CastSnapshot
	}
	switch t.Zone {
	case zNone, zBattlefield:
	case zGraveyard, zCommand:
		kt.Zone = zoneMap[t.Zone]
	case zStack:
		// a "when you cast this spell" trigger (Demonstrate): the kernel
		// fires it from the cast spell itself (cast.go onCast's self walk)
		if t.Event != "SpellCast" || kt.Flags&kop.TfSelf == 0 {
			return kt, fmt.Errorf("trigger zone %d", t.Zone)
		}
		kt.Zone = kop.ZoneStack
	case zExile:
		// "when this card becomes plotted": the plot action
		// fires it from exile (kw_plot.go)
		if t.Event != "BecomesPlotted" {
			return kt, fmt.Errorf("trigger zone %d", t.Zone)
		}
		kt.Zone = kop.ZoneExile
	default:
		return kt, fmt.Errorf("trigger zone %d", t.Zone)
	}
	if kt.Cond, err = cb.cond(t.Cond); err != nil {
		return kt, err
	}
	return kt, nil
}

// static builds a static ability and its granted subtypes.
func (cb *cardBuilder) static(s lStatic) (kop.Ability, []uint8, error) {
	ka := kop.Ability{Kind: kop.AbStatic}
	st := &ka.Static
	var err error
	if st.Cond2, err = cb.cond(s.Cond); err != nil {
		return ka, nil, err
	}
	st.OvGate = s.OvGate
	filter := func(f *lFilter, zone kop.Zone) (kop.FilterID, error) {
		if f == nil {
			return 0, nil
		}
		return cb.filter(f, zone)
	}
	amounts := func(a, b lValue) error {
		if (a.Kind == vNone || a.isConst()) && (b.Kind == vNone || b.isConst()) {
			st.A, st.B = int16(a.N), int16(b.N)
			return nil
		}
		st.StFlags |= kop.StDynamic
		if st.VA, err = cb.value(a); err != nil {
			return err
		}
		st.VB, err = cb.value(b)
		return err
	}
	// targets2 lowers a cost static's ValidTarget$ spec (ReduceCost, RaiseCost)
	// into Filter2 under StTargetsFilter2
	targets2 := func() error {
		if s.F2 == nil {
			return nil
		}
		st.StFlags |= kop.StTargetsFilter2
		// a Spell head (Out of Air's Spell.Creature) targets a spell:
		// the whole filter reads the stack (costTargetOK's stack
		// branch); a filter mixing it with a permanent head refuses
		zone, spells := kop.ZoneBattlefield, 0
		for _, c := range s.F2.Clauses {
			if c.SpellBase {
				spells++
			}
		}
		switch {
		case spells == 0:
		case spells == len(s.F2.Clauses) && s.F2.Players == 0:
			zone = kop.ZoneStack
		default:
			return refuse("ReduceCost ValidTarget$ mixes a Spell head with another")
		}
		if st.Filter2, err = filter(s.F2, zone); err != nil {
			return err
		}
		return nil
	}
	var add []uint8
	switch s.Kind {
	case "FlipCoinMod":
		st.Kind = kop.StatFlipCoinMod // static_turnbits.go
	case "ActivationLimit":
		st.Kind = kop.StatActivationLimit // static_turnbits.go
	case "Continuous":
		st.Kind = kop.StatContinuous
		if s.F != nil && s.F.Players != 0 {
			// "You and <objects> you control have hexproof" (Captain America,
			// Shalai, Sigarda): the controller half rides StYouHexproof (the
			// kernel's playerHexproof reads it under Cond2). Any other
			// player half was dropped by filter() before shield: refused.
			if s.F.Players != plYou || len(s.Kw) != 1 || s.Kw[0] != "Hexproof" || s.Power.Kind != vNone || s.Tough.Kind != vNone ||
				len(s.Grant) != 0 || len(s.AddSub) != 0 || s.AddChosen {
				return ka, nil, fmt.Errorf("continuous static affecting players %d (only \"you have hexproof\" is read)", s.F.Players)
			}
			st.StFlags |= kop.StYouHexproof
		}
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
		if err := amounts(s.Power, s.Tough); err != nil {
			return ka, nil, err
		}
		if st.Kw, err = kwMask(s.Kw); err != nil {
			return ka, nil, err
		}
		st.Ward = uint8(s.Ward)
		st.WardLife = uint8(s.WardLife)
		if add, err = cb.subIndexes(s.AddSub); err != nil {
			return ka, nil, err
		}
		st.AddChosen = s.AddChosen
		st.OutlivesLoss = s.OutlivesLoss
		if len(s.Grant) > 1 {
			return ka, nil, fmt.Errorf("static grants %d abilities", len(s.Grant))
		}
		if len(s.Grant) == 1 {
			j, ok := cb.abMap[s.Grant[0]]
			if !ok {
				return ka, nil, fmt.Errorf("granted ability %d not emitted", s.Grant[0])
			}
			st.Grant = uint8(j + 1)
		}
		if st.ProtFrom, err = filter(s.ProtFrom, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
		st.ProtChosen = s.ProtChosen
		if st.ProtExc, err = filter(s.ProtExc, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
		if st.Landwalk, err = filter(s.Landwalk, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
		st.Evasion, st.Infect = s.Evasion, s.Infect // lower_kwmark.go
		if s.Override != nil {
			// RemoveKeyword$ beside the Continuous parts (layer 6)
			i, err := cb.override(s.Override)
			if err != nil {
				return ka, nil, err
			}
			st.Override = uint16(i + 1)
		}
	case "CDAPT":
		st.Kind = kop.StatCDAPT
		st.StFlags |= kop.StDynamic
		if st.VA, err = cb.value(s.Power); err != nil {
			return ka, nil, err
		}
		if st.VB, err = cb.value(s.Tough); err != nil {
			return ka, nil, err
		}
	case "ReduceCost":
		if s.AbilityRed != 0 {
			// activated abilities of battlefield permanents (lower_reduceab.go)
			st.Kind = kop.StatReduceAbility
			if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
				return ka, nil, err
			}
			if err := amounts(s.Amount, lValue{}); err != nil {
				return ka, nil, err
			}
			if s.AbilityRed == 2 {
				st.B = 1
			}
			if s.AbilityRed == 3 {
				st.B |= kop.ReduceAbUnlock
			}
			if s.AbilityRed == 4 {
				st.B |= kop.ReduceAbPowerUp
			}
			if s.AbilityRed == 5 {
				st.B |= kop.ReduceAbEquip
				if err := targets2(); err != nil { // the equip's target (Cloud's Card.Self)
					return ka, nil, err
				}
			}
			st.B |= int16(s.MinMana) << kop.ReduceAbMinShift
			break
		}
		st.Kind = kop.StatReduceCost
		if st.Filter, err = filter(s.F, kop.ZoneStack); err != nil {
			return ka, nil, err
		}
		if err := amounts(s.Amount, lValue{}); err != nil {
			return ka, nil, err
		}
		if err := targets2(); err != nil {
			return ka, nil, err
		}
		if s.Zone == zAny {
			st.Zone = kop.ZoneCommand
		}
		for _, fl := range s.Flags {
			switch fl {
			case "Bargained", "OptPaid":
				st.StFlags |= kop.StBargained // lower_reducespell.go, lower_optcost.go
			case "ReduceColor":
				st.StFlags |= kop.StReduceColor
				st.B = int16(s.RedCol)
			case "AnyCaster":
				st.StFlags |= kop.StReduceAny
			case "EmbNum":
				// the amount is the emblem's frozen
				// SetChosenNumber$, read by adjustCostGen from embN
				st.StFlags |= kop.StEmbNum
			default:
				return ka, nil, fmt.Errorf("reduce cost flag %q", fl)
			}
		}
	case "CantBeBeamedUp":
		st.Kind = kop.StatCantBeBeamedUp
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
		for _, fl := range s.Flags {
			if fl != "BeamGranted" {
				return ka, nil, fmt.Errorf("CantBeBeamedUp flag %q", fl)
			}
			st.StFlags |= kop.StBeamGranted
		}
	case "BeamGrant":
		st.Kind = kop.StatBeamGrant
	case "FbGrant":
		st.Kind = kop.StatFbGrant // kw_fbgrant.go
		if st.Filter, err = filter(s.F, kop.ZoneGraveyard); err != nil {
			return ka, nil, err
		}
		for _, fl := range s.Flags {
			if fl != "OwnCost" {
				return ka, nil, fmt.Errorf("FbGrant flag %q", fl)
			}
			st.StFlags |= kop.StFbOwnCost
			if st.GrantCost, err = manaCostOf(s.Cost.Mana); err != nil {
				return ka, nil, err
			}
		}
	case "TapPower":
		// a Mode$ TapPowerValue static: Filter is ValidCard$, AddSub the
		// host subtype its ValidSA$ qualifier names, A the literal bonus or
		// TapPowToughness, and VA the tapped creature's own value under
		// StDynamic (kop.StatTapPower; kernel value_tappower.go)
		st.Kind = kop.StatTapPower
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
		if s.F2 != nil {
			if st.Filter2, err = filter(s.F2, kop.ZoneBattlefield); err != nil {
				return ka, nil, err
			}
		}
		for _, fl := range s.Flags {
			switch fl {
			case "Crew":
				st.StFlags |= kop.StTapPowerCrew
			case "Saddle":
				st.StFlags |= kop.StTapPowerSaddle
			case "Station":
				st.StFlags |= kop.StTapPowerStation
			case "Toughness":
				st.StFlags |= kop.StTapPowerTough
			default:
				return ka, nil, fmt.Errorf("tap power flag %q", fl)
			}
		}
		if s.Amount.Kind != vNone {
			if s.Amount.isConst() {
				st.A = int16(s.Amount.N)
			} else {
				st.StFlags |= kop.StDynamic
				if st.VA, err = cb.value(s.Amount); err != nil {
					return ka, nil, err
				}
			}
		}
		if s.Amount.Kind != vNone && st.StFlags&kop.StTapPowerTough != 0 {
			return ka, nil, fmt.Errorf("tap power static with both a value and Toughness")
		}
	case "WarpGrant":
		st.Kind = kop.StatWarpGrant // kw_warpgrant.go
		if st.Filter, err = filter(s.F, kop.ZoneHand); err != nil {
			return ka, nil, err
		}
		if st.GrantCost, err = manaCostOf(s.Cost.Mana); err != nil {
			return ka, nil, err
		}
	case "EscapeGrant":
		// an AffectedZone$ Graveyard AddKeyword$ Escape static: each matching
		// card in its controller's graveyard has escape at its mana cost plus
		// N other cards from the same graveyard (kop.StatEscapeGrant;
		// kw_escape.go; CR 702.42a; Underworld Breach). Filter is the affected
		// cards; the ability's own Cost is the fodder part, as an
		// AlternativeCost static's is: CostExileGrave of N cards matching
		// Cost.Filter (the grant's ExileFromGrave spec), a table cost the
		// kernel's cost builder returns in place. The mana part is the card's
		// own cost, priced at the offer like any spell's.
		st.Kind = kop.StatEscapeGrant // kw_escape.go
		if st.Filter, err = filter(s.F, kop.ZoneGraveyard); err != nil {
			return ka, nil, err
		}
		if s.Cost.ExileG == nil || s.Cost.ExileGN < 1 || s.Cost.ExileGN > 15 {
			return ka, nil, fmt.Errorf("escape grant without an exile cost")
		}
		if ka.Cost, err = cb.cost(s.Cost); err != nil {
			return ka, nil, err
		}
		if ka.Cost.Flags != kop.CostExileGrave || ka.Cost.Mana != (kop.ManaCost{}) {
			return ka, nil, fmt.Errorf("escape grant cost beside its exile part")
		}
	case "TapExtra":
		st.Kind = kop.StatTapExtra
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
		st.A, st.B = int16(s.TapCol), int16(s.TapAmt)
		for _, fl := range s.Flags {
			switch fl {
			case "TapOnC":
				st.StFlags |= kop.StTapOnC
			case "TapReflect":
				st.StFlags |= kop.StTapReflect
			case "TapAnyPlayer":
				st.StFlags |= kop.StTapAnyPlayer
			default:
				return ka, nil, fmt.Errorf("tap extra flag %q", fl)
			}
		}
	case "CantBeCast":
		st.Kind = kop.StatCantBeCast
		if s.F != nil { // nil: the card's own static (Card.Self | EffectZone$ All)
			if st.Filter, err = filter(s.F, kop.ZoneStack); err != nil {
				return ka, nil, err
			}
		}
		if s.Zone == zAny {
			st.Zone = kop.ZoneCommand
		}
		for _, fl := range s.Flags {
			switch fl {
			case "ActivatorYou":
				st.StFlags |= kop.StRaiseYou
			case "ActivatorOpp":
				st.StFlags |= kop.StRaiseOpp
			case "CasterActive":
				st.StFlags |= kop.StCasterActive
			case "NamedCard":
				st.B |= kop.CantCastNamed
			case "SorcerySpeed":
				st.B |= kop.CantCastSorcery
			default:
				return ka, nil, fmt.Errorf("cant be cast flag %q", fl)
			}
		}
		if s.Amount.Kind != vNone {
			if !s.Amount.isConst() {
				return ka, nil, fmt.Errorf("cant be cast limit %v", s.Amount)
			}
			st.A = int16(s.Amount.N)
		}
	case "CantBeActivated":
		st.Kind = kop.StatCantActivate // lower_cantact.go
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
		for _, fl := range s.Flags {
			switch fl {
			case "ActivatorYou":
				st.StFlags |= kop.StRaiseYou
			case "ActivatorOpp":
				st.StFlags |= kop.StRaiseOpp
			case "SpareMana":
				st.A |= 1
			case "LoyaltyOnly":
				st.A |= 2
			case "NamedCard":
				st.A |= kop.CantActNamed
			default:
				return ka, nil, fmt.Errorf("cant be activated flag %q", fl)
			}
		}
	case "CantTgtGrave":
		// Ground Seal's class (lower_canttarget.go): every card in every
		// graveyard untargetable (kw_canttgtgrave.go). No operands.
		st.Kind = kop.StatCantTgtGrave
	case "CantTarget":
		st.Kind = kop.StatCantTarget // lower_canttarget.go
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
		if st.Filter2, err = filter(s.F2, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
		for _, fl := range s.Flags {
			switch fl {
			case "ActivatorYou":
				st.StFlags |= kop.StRaiseYou
			case "ActivatorOpp":
				st.StFlags |= kop.StRaiseOpp
			case "SASpell":
				st.A |= kop.CantTgtSpell
			case "SAActivated":
				st.A |= kop.CantTgtActivated
			case "SATriggered":
				st.A |= kop.CantTgtTriggered
			default:
				return ka, nil, fmt.Errorf("cant target flag %q", fl)
			}
		}
	case "RaiseCost":
		st.Kind = kop.StatIncreaseCost
		if st.Filter, err = filter(s.F, kop.ZoneStack); err != nil {
			return ka, nil, err
		}
		if err := targets2(); err != nil {
			return ka, nil, err
		}
		if err := amounts(s.Amount, lValue{}); err != nil {
			return ka, nil, err
		}
		for _, fl := range s.Flags {
			switch fl {
			case "ActivatorYou":
				st.StFlags |= kop.StRaiseYou
			case "ActivatorOpp":
				st.StFlags |= kop.StRaiseOpp
			case "RaiseLife":
				st.StFlags |= kop.StRaiseLife // Static.A is life (kernel cost_raiselife.go)
			case "RaisePerTarget":
				st.StFlags |= kop.StRaisePerTarget
				st.B = int16(s.RaiseCol) // the coloured pips per charge (cost_raise.go)
			default:
				return ka, nil, fmt.Errorf("raise cost flag %q", fl)
			}
		}
		if s.Zone == zAny {
			st.Zone = kop.ZoneCommand
		} else if s.Zone == zStack {
			st.Zone = kop.ZoneStack // the spell's own stack-zone raise (lower_raisecost.go)
		}
	case "CantBlock":
		st.Kind = kop.StatCantBlock
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
	case "Lure":
		st.Kind = kop.StatLure // combat_lure.go isLure
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
	case "CantPhaseIn":
		st.Kind = kop.StatCantPhaseIn // kw_phasing.go phaseBlocked
	case "CantSacrifice":
		st.Kind = kop.StatCantSacrifice // static_cantsacrifice.go
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
	case "CantPutCounter":
		st.Kind = kop.StatCantPutCounter
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
		st.A = s.Ctr
	case "UntapOtherPlayer":
		st.Kind = kop.StatUntapOther
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
	case "CantGainLife":
		st.Kind = kop.StatCantGainLife
		if err := banOppFlag(s.Flags, st); err != nil {
			return ka, nil, err
		}
	case "ActivateAbilityAsIfHaste":
		st.Kind = kop.StatAsIfHaste // static_asifhaste.go
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
	case "CantBeSuspected":
		st.Kind = kop.StatCantBeSuspected // act_suspect.go suspectOne
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
	case "ManaConvert":
		st.Kind = kop.StatAnyTypeCreatures // lower_manaconvert.go
	case "CantDraw":
		st.Kind = kop.StatCantDraw
		if err := banOppFlag(s.Flags, st); err != nil {
			return ka, nil, err
		}
	case "CantPreventDamage":
		st.Kind = kop.StatCantPreventDamage // static_cantprevent.go
		for _, fl := range s.Flags {
			switch fl {
			case "NoPrevCombat":
				st.StFlags |= kop.StNoPrevCombat
			case "NoPrevSpellSelf":
				st.StFlags |= kop.StNoPrevSpellSelf
			default:
				return ka, nil, fmt.Errorf("CantPreventDamage flag %q", fl)
			}
		}
	case "MinMaxBlocker":
		st.Kind = kop.StatMaxBlockers
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
	case "CombatDamageToughness":
		st.Kind = kop.StatCombatDmgToughness
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
	case "DisableTriggers":
		st.Kind = kop.StatDisableETB
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
	case "HexproofOwnColors":
		st.Kind = kop.StatHexproofOwnColors
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
	case "CantBlockBy":
		st.Kind = kop.StatCantBlockBy
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
		if st.Filter2, err = filter(s.F2, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
		st.PwVsSrc = s.PwVsSrc
	case "CantAttack":
		st.Kind = kop.StatCantAttack
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
	case "CantAttackUnless":
		st.Kind = kop.StatAttackTax // lower_attacktax.go
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
		if err := amounts(s.Amount, lValue{}); err != nil {
			return ka, nil, err
		}
		for _, fl := range s.Flags {
			switch fl {
			case "TaxPlayer":
				st.StFlags |= kop.StAtkTaxPlayer
			case "TaxWalker":
				st.StFlags |= kop.StAtkTaxWalker
			default:
				return ka, nil, fmt.Errorf("CantAttackUnless flag %q", fl)
			}
		}
	case "CantBlockUnless":
		st.Kind = kop.StatBlockTax // lower_blocktax.go
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
		if s.F2 != nil {
			if st.Filter2, err = filter(s.F2, kop.ZoneBattlefield); err != nil {
				return ka, nil, err
			}
		}
		if err := amounts(s.Amount, lValue{}); err != nil {
			return ka, nil, err
		}
	case "OffZonePT":
		// AffectedZone$ All P/T grant (lower_offzonept.go): a marker the
		// kernel's off-battlefield P/T readers stall on (static_offzonept.go)
		st.Kind = kop.StatOffZonePT
		if err := amounts(s.Power, s.Tough); err != nil {
			return ka, nil, err
		}
	case "CanAttackIfHaste":
		st.Kind = kop.StatCanAttackIfHaste // static_canattackifhaste.go
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
		for _, fl := range s.Flags {
			if fl != "OppDef" {
				return ka, nil, fmt.Errorf("CanAttackIfHaste flag %q", fl)
			}
			st.StFlags |= kop.StHasteOppDef
		}
	case "CanAttackDefender":
		st.Kind = kop.StatCanAttackDefender
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
	case "MustAttack":
		st.Kind = kop.StatMustAttack
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
	case "Goad":
		st.Kind = kop.StatGoad // lower_trigger.go's Continuous Goad$ True
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
	case "CastWithFlash":
		st.Kind = kop.StatCastWithFlash
		if st.Filter, err = filter(s.F, kop.ZoneHand); err != nil {
			return ka, nil, err
		}
		if st.Filter2, err = filter(s.F2, kop.ZoneStack); err != nil { // an emblem's ForgetOnCast$ (lower_effectemblem.go)
			return ka, nil, err
		}
	case "AlternativeCost":
		st.Kind = kop.StatAlternativeCost
		if ka.Cost, err = cb.cost(s.Cost); err != nil {
			return ka, nil, err
		}
		if ka.Cost.Flags&(kop.CostForage|kop.CostExileGrave) != 0 && !cb.altHandExileOK(&ka.Cost) {
			if altHandExileShape(&ka.Cost) {
				// the hand-exile shape lowers (the Force cycle); the card
				// carries no alternative-cost cast (Force of Virtue: an
				// enchantment, where altSacOK prices instants, sorceries
				// and creatures alone)
				return ka, nil, fmt.Errorf("alternative cost with an exile-from-hand part on a card the kernel offers no alternative cast for (instants, sorceries and creatures alone)")
			}
			return ka, nil, fmt.Errorf("alternative cost with a forage / exile-from-graveyard part")
		}
		sacAlt := ka.Cost.Flags == kop.CostSac && ka.Cost.N > 0 && ka.Cost.Mana == (kop.ManaCost{}) && cb.altSacOK()
		// the return alternative (Gush's "return two Islands you control to
		// their owner's hand rather than pay this spell's mana cost"): the
		// sacrifice shape with the permanents returned (kernel cast_altfree.go
		// altReturnCost, the cast's pkReturn picks)
		sacAlt = sacAlt || ka.Cost.Flags == kop.CostReturn && ka.Cost.N > 0 && ka.Cost.Mana == (kop.ManaCost{}) && cb.altSacOK()
		if !sacAlt && !cb.altHandExileOK(&ka.Cost) && !(altDiscardTwoShape(&ka.Cost) && cb.altSacOK()) && (ka.Cost.Flags != 0 || ka.Cost.N != 0 || ka.Cost.Mana.X != 0) {
			// the kernel offers a mana-only alternative cost
			// (prepAltFree); a Return / Sac / PayLife / exile-from-hand
			// part (Daze, Force of Will) would never be offered
			return ka, nil, fmt.Errorf("alternative cost with a non-mana part (the kernel offers mana-only alternatives)")
		}
	case "NoMaxHandSize":
		st.Kind = kop.StatNoMaxHandSize
		switch {
		case s.Amount.Kind == vNone: // Unlimited: A 0
		case s.Amount.isConst() && s.Amount.N >= 0 && s.Amount.N <= 255:
			st.A = int16(s.Amount.N + 1) // a numeric maximum: A = N+1
		case len(s.Flags) == 1 && s.Flags[0] == "HandOpp":
			// the opponents' maximum from an SVar (Winter): VA, read per
			// cleanup under Cond2 (kernel maxHand)
			st.StFlags |= kop.StDynamic
			if st.VA, err = cb.value(s.Amount); err != nil {
				return ka, nil, err
			}
		default:
			return ka, nil, fmt.Errorf("NoMaxHandSize amount %v", s.Amount)
		}
		for _, fl := range s.Flags {
			if fl != "HandOpp" {
				return ka, nil, fmt.Errorf("NoMaxHandSize flag %q", fl)
			}
			st.StFlags |= kop.StHandOpp
		}
	case "ExtraLandPlay":
		st.Kind = kop.StatExtraLandPlay
		if err := amounts(s.Amount, lValue{}); err != nil {
			return ka, nil, err
		}
	case "PlayerHexproof":
		st.Kind = kop.StatPlayerHexproof
		switch {
		case s.Amount.Kind == vNone:
		case s.Amount.isConst() && s.Amount.N == 1:
			st.A = 1 // also protection from each opponent
		default:
			return ka, nil, fmt.Errorf("PlayerHexproof amount %v", s.Amount)
		}
	case "PlayerProtAll":
		st.Kind = kop.StatPlayerProtAll // the controller has protection from everything (static_plprot.go)
	case "MayPlayFrom":
		st.Kind = kop.StatMayPlayFrom
		st.Zone = zoneMap[s.Zone]
		fz := st.Zone
		if slices.Contains(s.Flags, "AlsoSideboard") || slices.Contains(s.Flags, "FreeOnly") {
			fz = kop.ZoneNone // the filter also reads a sideboard card; the static's zones are tested apart (mayPlayFromZone)
		}
		if st.Filter, err = filter(s.F, fz); err != nil {
			return ka, nil, err
		}
		// the count of "RemoveAnyCounter<N/Any/Creature>" (RaiseCostRemove6...)
		if err := amounts(s.Amount, lValue{}); err != nil {
			return ka, nil, err
		}
		for _, fl := range s.Flags {
			switch fl {
			case "OncePerTurn":
				st.StFlags |= kop.StOncePerTurn
			case "AnyManaType":
				st.StFlags |= kop.StAnyManaType
			case "WithoutManaCost":
				st.StFlags |= kop.StWithoutManaCost
			case "SeatActive":
				st.StFlags |= kop.StSeatActive
			case "FunctionsInGraveyard":
				st.StFlags |= kop.StFunctionsInGraveyard
			case "FunctionsInExile":
				st.StFlags |= kop.StFunctionsInExile
			case "AlsoSideboard":
				st.StFlags |= kop.StAlsoSideboard
			case "FreeOnly":
				st.StFlags |= kop.StFreeOnly
			case "MayPayLife":
				st.StFlags |= kop.StMayPayLife
			case "RaiseCostRemove6CountersFromCreatures":
				st.StFlags |= kop.StRemove6Counters
			case "MayRaise":
				st.StFlags |= kop.StMayRaise
				j, ok := cb.abMap[s.RaiseAb-1]
				if !ok {
					return ka, nil, fmt.Errorf("MayRaise ability %d not emitted", s.RaiseAb-1)
				}
				st.B = int16(j + 1)
			default:
				return ka, nil, fmt.Errorf("MayPlay flag %s", fl)
			}
		}
	case "Control":
		st.Kind = kop.StatControl
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
	case "Panharmonicon":
		st.Kind = kop.StatPanharmonicon
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
		for _, fl := range s.Flags {
			switch fl {
			case "LifeGained":
				st.A = kop.PanhModeLifeGained
			case "Drawn":
				st.A = kop.PanhModeDrawn
			case "You":
				st.B = int16(kop.WhoseYou)
			default:
				return ka, nil, fmt.Errorf("Panharmonicon flag %s", fl)
			}
		}
		if s.F2 != nil {
			// ValidCause$ (the entering permanent): kop.StPanhETB
			st.StFlags |= kop.StPanhETB
			if st.Filter2, err = filter(s.F2, kop.ZoneBattlefield); err != nil {
				return ka, nil, err
			}
		}
	case "StackKw":
		// AffectedZone$ Stack keyword grant (lower_stackkw.go): the filter
		// matches the spell being cast (a card in hand or on the stack)
		st.Kind = kop.StatStackKw
		if st.Filter, err = filter(s.F, kop.ZoneStack); err != nil {
			return ka, nil, err
		}
		if len(s.Kw) != 1 {
			return ka, nil, fmt.Errorf("stack keyword grant with %d keywords", len(s.Kw))
		}
		head, param, _ := strings.Cut(s.Kw[0], ":")
		switch head {
		case "Cascade", "Improvise", "Convoke":
			if param != "" {
				return ka, nil, fmt.Errorf("stack keyword grant %q with a parameter", s.Kw[0])
			}
			st.A = kop.StackKwCascade
			switch head {
			case "Improvise":
				st.A = kop.StackKwImprovise
			case "Convoke":
				st.A = kop.StackKwConvoke
			}
		case "Lifelink", "Storm", "Split second", "Conspire", "Delve", "Demonstrate":
			if param != "" {
				return ka, nil, fmt.Errorf("stack keyword grant %q with a parameter", s.Kw[0])
			}
			st.A = kop.StackKwLifelink
			switch head {
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
		case "Casualty":
			n, err := strconv.Atoi(param)
			if err != nil || n < 1 || n > 127 {
				return ka, nil, fmt.Errorf("stack keyword grant %q", s.Kw[0])
			}
			st.A, st.B = kop.StackKwCasualty, int16(n)
		default:
			return ka, nil, fmt.Errorf("stack keyword grant %q", s.Kw[0])
		}
		for _, fl := range s.Flags {
			if fl != "CastHand" {
				return ka, nil, fmt.Errorf("stack keyword grant flag %q", fl)
			}
			st.Zone = kop.ZoneHand // wasCastFromYourHand
		}
	case "Override":
		st.Kind = kop.StatOverride
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
		i, err := cb.override(s.Override)
		if err != nil {
			return ka, nil, err
		}
		st.Override = uint16(i + 1)
	default:
		return ka, nil, fmt.Errorf("static kind %s", s.Kind)
	}
	return ka, add, nil
}

func (cb *cardBuilder) repl(r lRepl) (kop.Ability, error) {
	ka := kop.Ability{Kind: kop.AbReplacement}
	rp := &ka.Repl
	var err error
	if rp.Cond, err = cb.cond(r.Cond); err != nil {
		return ka, err
	}
	switch r.Kind {
	case "EntersWith":
		switch {
		case len(r.Prog) == 1 && r.Prog[0].Op == "EnterTapped" && r.Cond.Kind == cNone:
			rp.Kind = kop.ReplEntersTapped
		case len(r.Prog) == 1 && r.Prog[0].Op == "EnterWithCounters" && r.Prog[0].A.isConst() && r.Cond.Kind == cNone:
			rp.Kind = kop.ReplEntersWithCounters
			rp.A = int16(r.Prog[0].A.N)
			k, err := counterKind(r.Prog[0].CS)
			if err != nil {
				return ka, err
			}
			rp.C = int16(k)
		default:
			rp.Kind = kop.ReplEntersProgram
			if r.S == "Updated" {
				rp.C = kop.ReplAfterEnter
			}
			if ka.Prog, err = cb.prog(r.Prog); err != nil {
				return ka, err
			}
		}
	case "OthersEnterWith":
		rp.Kind = kop.ReplOthersEnterProgram
		if rp.Filter, err = cb.filter(r.F, kop.ZoneBattlefield); err != nil {
			return ka, err
		}
		if ka.Prog, err = cb.prog(r.Prog); err != nil {
			return ka, err
		}
	case "CantBeCountered":
		rp.Kind = kop.ReplCantBeCountered
	case "DoesntUntap":
		rp.Kind = kop.ReplDoesntUntap
	case "MayNotUntap":
		rp.Kind = kop.ReplMayNotUntap
	case "OthersDoesntUntap":
		rp.Kind = kop.ReplOthersDoesntUntap
		if rp.Filter, err = cb.filter(r.F, kop.ZoneBattlefield); err != nil {
			return ka, err
		}
	case "AnyUntap":
		rp.Kind = kop.ReplAnyUntap
		if rp.Filter, err = cb.filter(r.F, kop.ZoneBattlefield); err != nil {
			return ka, err
		}
	case "YouCantLose":
		rp.Kind = kop.ReplCantLoseGame
		rp.C = int16(r.Bits) // 0: any loss; kop.CantLoseLife: only the loss for 0 or less life
	case "OppsCantWin":
		rp.Kind = kop.ReplOppsCantWinGame
	case "DoubleDamageToOpps":
		rp.Kind = kop.ReplDoubleDamageToOpps
		if r.F != nil { // a filtered doubler (lower_replmult.go); Twinflame keeps Filter 0, C 0
			if r.Bits&(kop.ReplDmgOppPlayer|kop.ReplDmgOppPerm|kop.ReplDmgPlayer|kop.ReplDmgPerm) == 0 {
				return ka, fmt.Errorf("damage doubler to %#x", r.Bits)
			}
			rp.C = int16(r.Bits)
			if rp.Filter, err = cb.filter(r.F, kop.ZoneBattlefield); err != nil {
				return ka, err
			}
		}
	case "DamagePlus":
		rp.Kind = kop.ReplDamagePlus
		if !r.N.isConst() || r.N.N < 0 ||
			r.N.N == 0 && r.Bits&(kop.ReplDmgRaise|kop.ReplDmgCtrHost|kop.ReplDmgPowHost) == 0 || r.Bits == 0 {
			return ka, fmt.Errorf("damage plus %d to %#x", r.N.N, r.Bits)
		}
		rp.A, rp.C = int16(r.N.N), int16(r.Bits)
		if r.Bits&kop.ReplDmgCtrHost != 0 {
			k, err := counterKind(r.Ctr)
			if err != nil {
				return ka, err
			}
			rp.A = int16(k)
		}
		if rp.Filter, err = cb.filter(r.F, kop.ZoneBattlefield); err != nil {
			return ka, err
		}
	case "PreventNoncombatToOthers":
		rp.Kind = kop.ReplPreventNoncombatToOthers
	case "PreventMillOpps":
		rp.Kind = kop.ReplPreventMillOpps
	case "PreventDamageCounters":
		rp.Kind = kop.ReplPreventDamageCounters
	case "DamageHealFirst":
		rp.Kind = kop.ReplDamageHealFirst
	case "DoubleTokens":
		rp.Kind = kop.ReplDoubleTokens
		if r.N.isConst() && r.N.N > 2 {
			rp.A = int16(r.N.N) // the multiplier (0: twice)
		}
		if r.F != nil {
			if rp.Filter, err = cb.filter(r.F, kop.ZoneBattlefield); err != nil {
				return ka, err
			}
		}
	case "RewriteTokens":
		rp.Kind = kop.ReplRewriteTokens
		idx, ok := cb.pt.tokenIdx[r.S]
		if !ok {
			return ka, fmt.Errorf("token %q", r.S)
		}
		rp.C = int16(idx) // a pool token index; the matchup remaps it
	case "AddToken":
		rp.Kind = kop.ReplAddToken
		idx, ok := cb.pt.tokenIdx[r.S]
		if !ok {
			return ka, fmt.Errorf("token %q", r.S)
		}
		rp.C = int16(idx) // a pool token index; the matchup remaps it
		if r.F != nil {
			if rp.Filter, err = cb.filter(r.F, kop.ZoneBattlefield); err != nil {
				return ka, err
			}
		}
	case "CopyFirstToken":
		rp.Kind = kop.ReplCopyFirstToken
	case "DoubleCounters":
		rp.Kind = kop.ReplDoubleCounters
	case "GraveToLibrary":
		rp.Kind = kop.ReplGraveToLibrary
	case "DiscardOppToBattlefield":
		rp.Kind = kop.ReplDiscardOppToBattlefield
	case "PayBeforeETB":
		rp.Kind = kop.ReplPayBeforeETB
		if ka.Prog, err = cb.prog(r.Prog); err != nil {
			return ka, err
		}
	case "DestroyHeal":
		rp.Kind = kop.ReplDestroyHeal
		if rp.Filter, err = cb.filter(r.F, kop.ZoneBattlefield); err != nil {
			return ka, err
		}
	case "ExileOnLeave":
		rp.Kind = kop.ReplExileOnLeave
		// the filter has no zone gate either way (ZoneBattlefield compiles to
		// Zone none); C 1 = any zone, read by exileAnyN
		if rp.Filter, err = cb.filter(r.F, kop.ZoneBattlefield); err != nil {
			return ka, err
		}
		if r.AnyZone {
			rp.C = 1
		}
	case "OnePoison":
		rp.Kind = kop.ReplOnePoison // lower_delayed.go
	case "DoubleCountersYou":
		rp.Kind = kop.ReplDoubleCountersYou // lower_delayed.go
	case "DoubleCountersOn":
		rp.Kind = kop.ReplDoubleCountersOn
		k, err := replCtrKind(r) // lower_counterrepl.go
		if err != nil {
			return ka, err
		}
		rp.C = k
		if r.Putter && r.Ctr != "" {
			rp.A = 1 // one kind, only the controller's own placements (kop.ReplDoubleCountersOn)
		}
		if rp.Filter, err = cb.filter(r.F, kop.ZoneBattlefield); err != nil {
			return ka, err
		}
	case "AddCountersOn":
		rp.Kind = kop.ReplAddCountersOn
		if !r.N.isConst() || r.N.N == 0 {
			return ka, fmt.Errorf("counter plus %v", r.N)
		}
		k, err := replCtrKind(r)
		if err != nil {
			return ka, err
		}
		rp.A, rp.C = int16(r.N.N), k
		if rp.Filter, err = cb.filter(r.F, kop.ZoneBattlefield); err != nil {
			return ka, err
		}
	case "TurnUpProgram":
		rp.Kind = kop.ReplTurnUpProgram // lower_turnfaceuprepl.go
		if ka.Prog, err = cb.prog(r.Prog); err != nil {
			return ka, err
		}
	case "SpellsCantBeCountered":
		rp.Kind = kop.ReplSpellsCantBeCountered // lower_uncounterable.go
		if rp.Filter, err = cb.filter(r.F, kop.ZoneStack); err != nil {
			return ka, err
		}
	case "CantTurnUp":
		rp.Kind = kop.ReplCantTurnUp // lower_turnfaceuprepl.go
		if rp.Filter, err = cb.filter(r.F, kop.ZoneBattlefield); err != nil {
			return ka, err
		}
	case "DrawMore":
		rp.Kind = kop.ReplDrawMore // lower_drawrepl.go
		if !r.N.isConst() || r.N.N < 2 {
			return ka, fmt.Errorf("draw more %v", r.N)
		}
		rp.A, rp.C = int16(r.N.N), int16(r.Bits)
	case "LifeDouble":
		rp.Kind = kop.ReplLifeDouble
	case "LifeLossDouble":
		rp.Kind = kop.ReplLifeLossDouble // (Bloodletter of Aclazotz)
	case "LifePlus":
		rp.Kind = kop.ReplLifePlus
		if !r.N.isConst() || r.N.N <= 0 {
			return ka, fmt.Errorf("life plus %v", r.N)
		}
		rp.A = int16(r.N.N)
	default:
		return ka, fmt.Errorf("replacement %s", r.Kind)
	}
	return ka, nil
}

// prog builds an op program (one kop instruction per LIR instruction, so
// skip counts carry over unchanged).
func (cb *cardBuilder) prog(p []lInstr) ([]kop.Instr, error) {
	if len(p) == 0 {
		return nil, nil
	}
	if kvDiscoverTailRefused(p) {
		// The kernel runs a Discover's tail before the free cast; see
		// value_modifiers.go and kernel-discover-freecast-order.
		return nil, refuse("Discover free cast with a program tail (the kernel defers the cast past the tail)")
	}
	out := make([]kop.Instr, 0, len(p))
	for i, in := range p {
		ki, err := cb.instr(in)
		if err != nil {
			return nil, fmt.Errorf("instr %d %s: %v", i, in.Op, err)
		}
		out = append(out, ki)
	}
	return out, nil
}

func flagsOf(cs string) map[string]string {
	m := map[string]string{}
	for _, f := range strings.Split(cs, "|") {
		if f == "" {
			continue
		}
		k, v, _ := strings.Cut(f, "=")
		m[k] = v
	}
	return m
}

func (cb *cardBuilder) instr(in lInstr) (kop.Instr, error) {
	var ki kop.Instr
	var err error
	if ki.Obj, err = cb.ref(in.Obj); err != nil {
		return ki, err
	}
	if in.W.Kind != rNone {
		// the player side input (lir.go lInstr.W: a Tapper$ provenance or an
		// OpAttach player destination)
		if ki.W, err = cb.ref(in.W); err != nil {
			return ki, err
		}
	}
	if in.Op != "Bite" && in.Op != "Fight" {
		if ki.A, err = cb.value(in.A); err != nil {
			return ki, err
		}
		if ki.B, err = cb.value(in.B); err != nil {
			return ki, err
		}
	}
	fl := flagsOf(in.CS)
	skip := func() kop.ValueRef { return kop.Const(in.Skip) }
	objFilter := func(zone kop.Zone) error {
		ki.F, err = cb.filter(in.F, zone)
		return err
	}
	moveFlags := func() int {
		m := 0
		if _, ok := fl["Remember"]; ok {
			m |= kop.MvRemember
		}
		if _, ok := fl["ImprintMove"]; ok {
			m |= kop.MvImprint // lower_imprintzone.go
		}
		if _, ok := fl["Tapped"]; ok {
			m |= kop.MvTapped
		}
		if _, ok := fl["UnderYourControl"]; ok {
			m |= kop.MvUnderYourControl
		}
		if _, ok := fl["Transformed"]; ok {
			m |= kop.MvTransformed
		}
		if _, ok := fl["ForgetOther"]; ok {
			m |= kop.MvForgetOther
		}
		if _, ok := fl["LKI"]; ok {
			m |= kop.MvLKI
		}
		if _, ok := fl["Attacking"]; ok {
			m |= kop.MvAttacking
		}
		if _, ok := fl["AlsoSelf"]; ok {
			m |= kop.MvAlsoSelf
		}
		if _, ok := fl["ForgetChanged"]; ok {
			m |= kop.MvForgetChanged
		}
		return m
	}
	counters := func() error {
		if k, ok := fl["counters"]; ok {
			if strings.Contains(k, ",") {
				return cb.counterPair(&ki, in, k) // lower_counterlist.go
			}
			ck, err := counterKind(k)
			if err != nil {
				return err
			}
			if in.A.Kind == vNone {
				ki.A = kop.Const(1)
			}
			ki.B = kop.Const(int(ck))
		}
		return nil
	}
	switch in.Op {
	case "SkipIf":
		ki.Op = kop.OpSkipUnless
		id, err := cb.cond(in.Cond)
		if err != nil {
			return ki, err
		}
		ki.C, ki.A = int16(id), skip()
	case "Charm":
		ki.Op, ki.C, ki.A = kop.OpCharm, int16(in.C), skip()
	case "Branch":
		ki.Op, ki.C, ki.A = kop.OpBranch, int16(in.C), skip()
	case "RepeatEachCard":
		ki.Op, ki.B = kop.OpRepeatEach, skip() // act_repeatcards.go
		if in.F != nil {
			if err := objFilter(zBattlefield); err != nil {
				return ki, err
			}
		}
	case "RepeatEach":
		ki.Op, ki.A, ki.C = kop.OpRepeatEachPlayer, skip(), int16(in.C)
		if in.F != nil {
			if err := objFilter(zBattlefield); err != nil {
				return ki, err
			}
		}
	case "CopySpell":
		ki.Op, ki.C = kop.OpCopySpell, int16(in.C)
	case "Demonstrate":
		ki.Op = kop.OpDemonstrate
	case "Ninjutsu":
		ki.Op, ki.C = kop.OpNinjutsu, int16(in.C) // C 1: the plain put-self-from-hand (lower_ninjutsu.go putSelfAbility)
	case "Repeat":
		ki.Op, ki.B = kop.OpRepeat, skip() // A: the pass count, read as the loop starts
	case "RepeatWhile":
		id, err := cb.cond(in.Cond)
		if err != nil {
			return ki, err
		}
		if id == 0 {
			return ki, fmt.Errorf("RepeatWhile without a condition")
		}
		ki.Op, ki.B, ki.C = kop.OpRepeatWhile, skip(), int16(id)
	case "GenericChoice":
		ki.Op, ki.C = kop.OpChooseBranch, int16(in.C) // C 1: no payability filter (lower_genericchoice.go genericChoiceYou)
	case "UnlessPaid", "IfPaid", "UnlessSwitched":
		ki.Op = kop.OpUnlessPaid
		if in.Op == "IfPaid" {
			ki.Op = kop.OpIfPaid
		} else if in.Op == "UnlessSwitched" {
			ki.Op = kop.OpUnlessSwitched
		}
		pk := map[string]int{"Mana": kop.PayMana, "Discard": kop.PayDiscard, "DiscardRandom": kop.PayDiscardRandom, "DiscardHand": kop.PayDiscardHand, "Sacrifice": kop.PaySac, "Blight": kop.PaySac, "PayLife": kop.PayLife, "PayEnergy": kop.PayEnergy, "Return": kop.PayReturn, "Forage": kop.PayForage, "SacrificeSelf": kop.PaySacSelf, "SacrificeSelfMust": kop.PaySacSelf, "SacrificeHost": kop.PaySacSelf, "Draw": kop.PayDraw, "DamageYou": kop.PayDraw, "SubCounter": kop.PaySubCounter, "TapX": kop.PayTapX, "TapXVar": kop.PayTapX, "Waterbend": kop.PayTapX, "AddCounter": kop.PayAddCounter, "ExileGrave": kop.PayExileGrave, "ExileTrigNew": kop.PayExileGrave, "ExileTrigNewEvidence": kop.PayExileGrave, "EvidenceX": kop.PayManaX, "ExileGraveChain": kop.PayExileGrave, "ExileStackSelf": kop.PayExileGrave, "ExileHand": kop.PayExileGrave, "ManaX": kop.PayManaX, "Evidence": kop.PayManaX, "Reveal": kop.PayDiscard, "Behold": kop.PayDiscard}[in.CS]
		if pk == 0 {
			return ki, fmt.Errorf("pay kind %q", in.CS)
		}
		if in.Skip < 0 || in.Skip > 2047 {
			return ki, fmt.Errorf("pay skip %d", in.Skip)
		}
		ki.C = int16(pk | in.Skip<<4)
		if in.CS == "AddCounter" {
			// PayAddCounter is not a C-nibble kind (it is appended past the
			// 15 the nibble holds): the instruction rides PayTapX with B.N
			// packing the counter kind and the recipient (kop.PayAddCounterB).
			// The kernel widens it to its own pay kind
			// (cost_unlessaddcounter.go).
			r, err := cb.ref(in.Obj2)
			if err != nil {
				return ki, err
			}
			n, ok := kop.PayAddCounterB(kop.CounterKind(in.B.N), r)
			if !ok {
				return ki, fmt.Errorf("AddCounter recipient ref %d", r)
			}
			ki.C = int16(kop.PayTapX | in.Skip<<4)
			ki.B = kop.Const(n)
		}
		if in.CS == "Blight" {
			ki.B = kop.Const(kop.PayBSubBlight) // a Blight, not a sacrifice
		}
		if in.CS == "Evidence" {
			ki.B = kop.Const(kop.PayBSubEvidence) // CollectEvidence, not {X}: PayEvidence's nibble value is PayManaX's
		}
		if in.CS == "EvidenceX" {
			ki.B = kop.Const(kop.PayBSubEvidenceX) // the announced X, not {X} (cost_collectevidencex.go)
		}
		if in.CS == "ExileTrigNewEvidence" {
			ki.B = kop.Const(kop.PayBSubTrigNewEvidence) // the triggered card, then evidence A (cost_collectevidencex.go)
		}
		if in.CS == "SacrificeHost" {
			ki.B = kop.Const(kop.PayBSubSacHost) // the granting host, not the source
		}
		if in.CS == "SacrificeSelfMust" {
			ki.B = kop.Const(kop.PayBSubSacSelfMust) // Cost$ Mandatory: no pay/decline ask (cost_mandatorysac.go)
		}
		if in.CS == "ExileTrigNew" {
			ki.B = kop.Const(kop.PayBSubTrigNew) // ExileAnyGrave<1/Card.TriggeredNewCard>, not a filter exile
		}
		if in.CS == "ExileGraveChain" {
			ki.B = kop.Const(kop.PayBSubChain) // the exile half of a mana-and-exile cost (cost_exilepool.go)
		}
		if in.CS == "ExileStackSelf" {
			ki.B = kop.Const(kop.PayBSubStackSelf) // the resolving spell too, from the stack
		}
		if in.CS == "DamageYou" {
			ki.B = kop.Const(kop.PayBSubDamage) // the resolving source deals A damage to the payer, not a draw (cost_restokens.go)
		}
		if in.CS == "TapXVar" {
			ki.B = kop.Const(kop.PayBSubTapXVar) // tap any number, X the count (cost_tapxvar.go)
		}
		if in.CS == "Waterbend" {
			ki.B = kop.Const(kop.PayBSubWaterbend) // an unless Waterbend<N>, not a tapXType (cost_unlesswaterbend.go)
		}
		if in.CS == "ExileHand" {
			ki.B = kop.Const(kop.PayBSubExileHand) // ExileFromHand<N/Card>, not a graveyard exile
		}
		if in.CS == "Reveal" {
			ki.B = kop.Const(kop.PayBReveal) // a reveal, not a discard: nothing moves
		}
		if in.CS == "Behold" {
			ki.B = kop.Const(kop.PayBBehold) // a behold, not a discard: nothing moves
		}
		if pk == kop.PaySac && strings.HasPrefix(in.Note, noteRememberSacrificed) {
			// a trigger's own IfPaid Sacrifice whose characteristic a value
			// head reads: B carries the remember bit and the sum bit
			// (kernel sacRemember's IfPaid arm)
			ki.B = kop.Const(1 + remLKBits(in.Note))
		}
		z := kop.ZoneBattlefield
		if pk == kop.PayExileGrave {
			z = kop.ZoneGraveyard
		}
		if in.CS == "ExileHand" {
			z = kop.ZoneHand
		}
		if in.CS == "Reveal" {
			z = kop.ZoneHand
		}
		if err := objFilter(z); err != nil {
			return ki, err
		}
		if pk == kop.PayMana && (in.C > 0 || len(in.Hyb) != 0) {
			// PayMana's F is the life paid beside the mana (a trigger's own
			// mana-and-life cost, Zoraline) and its hybrid pips (Emiel the
			// Blessed's {G/W}; kop.PayManaF)
			f, ok := payManaF(in.C, in.Hyb) // lower_trighybrid.go
			if in.F != nil || !ok {
				return ki, fmt.Errorf("mana-and-life pay shape")
			}
			ki.F = f
		}
	case "DealDamage":
		ki.Op = kop.OpDealDamage
		if in.F != nil {
			if err := objFilter(zBattlefield); err != nil {
				return ki, err
			}
		}
		c := 0
		if _, ok := fl["TrackExcess"]; ok {
			c |= kop.DmgTrackExcess
		}
		if _, ok := fl["ExileIfDiesThisTurn"]; ok {
			c |= kop.DmgExileIfDiesThisTurn
		}
		if _, ok := fl["Remember"]; ok {
			c |= kop.DmgRemember
		}
		if _, ok := fl["ExileIfDealt"]; ok {
			c |= kop.DmgExileIfDealt
		}
		if _, ok := fl["NoRegen"]; ok {
			c |= kop.DmgNoRegen
		}
		if _, ok := fl["NoRegenTarget"]; ok {
			c |= kop.DmgNoRegenTarget
		}
		if strings.HasPrefix(in.Note, "damage map") || cb.inDamageMap {
			c |= kop.DmgMapped
			cb.inDamageMap = true
		}
		if c != 0 {
			ki.Op, ki.C = kop.OpDealDamageEx, int16(c)
		}
	case "SweepFrom":
		// OpExt ExtSweepFrom over Obj, B the amount, C&1 the
		// source's own slot excluded
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtSweepFrom)), int16(in.C)
		if err := objFilter(kop.ZoneBattlefield); err != nil {
			return ki, err
		}
	case "DamageAll":
		switch {
		case in.F != nil && len(in.F.Clauses) == 0 && in.F.Players == plBoth:
			// "N damage to each player".
			ki.Op, ki.Obj = kop.OpDealDamage, kop.RefEachPlayer
		case in.F != nil && len(in.F.Clauses) == 0 && in.F.Players == plOpp:
			// "N damage to each opponent" (Grab the Prize's ValidPlayers$
			// Player.Opponent; seat-exact: the kernel deals each opponent)
			ki.Op, ki.Obj = kop.OpDealDamage, kop.RefEachOpponent
		case in.F != nil && in.F.Players == 0:
			ki.Op = kop.OpDamageAll
			if err := objFilter(kop.ZoneBattlefield); err != nil {
				return ki, err
			}
			// a planeswalker or battle clause ("each creature and each
			// planeswalker that player controls", Incinerator of the
			// Guilty): B.N bit 4, the filter's types choose (gorge
			// effDamageAll damages every matching permanent; CR 120.3c)
			for _, c := range in.F.Clauses {
				if c.Types&(tPlaneswalker|tBattle) != 0 {
					ki.B = kop.ValueRef{N: 4}
				}
			}
		case in.F != nil && in.F.Players == plOpp && len(in.F.Clauses) > 0:
			// "each opponent and each creature and planeswalker they
			// control" (End the Festivities): the permanents, then the
			// opponent (B.N bit 2); bit 4 when a clause names a
			// non-creature type, so the filter's types choose
			f := *in.F
			f.Players = 0
			in.F = &f
			ki.Op = kop.OpDamageAll
			if err := objFilter(kop.ZoneBattlefield); err != nil {
				return ki, err
			}
			b := int16(2)
			for _, c := range f.Clauses {
				if c.Types&^tCreature != 0 || c.Types == 0 {
					b |= 4
				}
			}
			ki.B = kop.ValueRef{N: b}
		case in.F != nil && in.F.Players == plBoth && len(in.F.Clauses) > 0:
			// "each creature and each player" (Encounter the Unknown): the
			// creatures, then each player, the controller first (B.N bit 1)
			for _, c := range in.F.Clauses {
				if c.Types != tCreature {
					return ki, fmt.Errorf("DamageAll filter %s: each player beside a non-creature clause", in.F)
				}
			}
			f := *in.F
			f.Players = 0
			in.F = &f
			ki.Op = kop.OpDamageAll
			if err := objFilter(kop.ZoneBattlefield); err != nil {
				return ki, err
			}
			ki.B = kop.ValueRef{N: 1}
		default:
			return ki, fmt.Errorf("DamageAll filter %s", in.F)
		}
		if ki.Op == kop.OpDealDamage {
			return ki, nil
		}
	case "ResolveDamageMap":
		ki.Op = kop.OpResolveDamageMap
		cb.inDamageMap = false
	case "DealDamageDivided":
		ki.Op = kop.OpDealDamageDivided
		if _, ok := fl["DivideEvenly"]; ok {
			ki.C = kop.DmgDivideEvenly
		}
	case "DealDamageFrom":
		ki.Op = kop.OpDealDamageFrom
		if in.CS == "players" {
			// each-opponent recipients (lower_damagesourceplayers.go): B.N 1 is the
			// kernel's player mode (dirac/act_damagefrom.go)
			ki.C, ki.B = int16(kop.RefEachOpponent), kop.Const(1)
			break
		}
		if in.CS == "defending" {
			// the Attacks trigger's defending player (lower_damagesourceplayers.go):
			// the player mode over RefDefendingPlayer (dirac eachPlayers -> oneOpp)
			ki.C, ki.B = int16(kop.RefDefendingPlayer), kop.Const(1)
			break
		}
		if in.CS == "anytgt" {
			// the slot may name a player or a permanent (lower_damagesourceplayers.go)
			r, err := cb.ref(lRef{Kind: rTarget, N: in.C})
			if err != nil {
				return ki, err
			}
			ki.C, ki.B = int16(r), kop.Const(1)
			break
		}
		r, err := cb.ref(lRef{Kind: rTarget, N: in.C})
		if err != nil {
			return ki, err
		}
		ki.C = int16(r)
	case "Bite", "Fight":
		ki.Op = kop.OpBite
		if in.Op == "Fight" {
			ki.Op = kop.OpFight
			if _, ok := fl["TrackExcess"]; ok {
				ki.A = kop.Const(1) // ExcessSVar$: record ExExcessDamage for the hit dealt to C (dirac/value_fightexcess.go)
			}
		}
		r, err := cb.ref(lRef{Kind: rTarget, N: in.C})
		if err != nil {
			return ki, err
		}
		ki.C = int16(r)
	case "Destroy":
		ki.Op, ki.C = kop.OpDestroy, int16(in.C)
	case "Regenerate":
		ki.Op = kop.OpRegenerate
	case "DestroyAll":
		ki.Op = kop.OpDestroyAll
		ki.C = int16(in.C & 3)
		if err := objFilter(kop.ZoneBattlefield); err != nil {
			return ki, err
		}
	case "Exile":
		ki.Op = kop.OpExile
		if m := moveFlags(); m != 0 || fl["counters"] != "" {
			ki.Op, ki.C = kop.OpExileEx, int16(m)
			if err := counters(); err != nil {
				return ki, err
			}
		}
		if (in.Obj.Kind == rSelf || in.Obj.Kind == rTriggered || in.Obj.Kind == rTriggeredSource) && fl["from"] == "Battlefield" {
			// a triggering permanent that left before the exile resolved is
			// a new object this ability cannot find (CR 400.7, 608.2b)
			ki.C |= kop.MvFromBattlefield
		}
		if (in.Obj.Kind == rTriggered || in.Obj.Kind == rTriggeredNew) && fl["from"] == "Graveyard" {
			ki.C |= kop.MvFromGraveyard
		}
	case "Bounce":
		ki.Op = kop.OpBounce
		// RememberChanged$: MvRemember; RememberLKI$: MvRemember|MvLKI
		// (zone_rememberparams.go#bounceRemember reads those two bits and
		// MvForgetChanged).
		if m := moveFlags(); m != 0 {
			if m&^(kop.MvRemember|kop.MvLKI|kop.MvForgetChanged) != 0 {
				return ki, fmt.Errorf("bounce move flags %#x", m)
			}
			ki.C = int16(m)
		}
		if fl["from"] == "Library" {
			// Origin$ Library: a library card only, then the owner's shuffle
			ki.C |= kop.BncFromLibrary
			if _, ok := fl["ShuffleOwed"]; ok {
				ki.C |= kop.BncShuffleOwed
			}
		}
		if _, ok := fl["PermOrSpell"]; ok {
			ki.C |= kop.MvPermOrSpell
		}
		if _, ok := fl["DelayedMany"]; ok {
			ki.C = kop.BncDelayedMany // the delayed many-object return (dirac/trig_delayedmany.go)
		}
	case "ToLibrary":
		ki.Op, ki.C = kop.OpToLibrary, int16(in.C)
		if (in.C == 0 || in.C == 1) && (in.Obj.Kind == rTriggered || in.Obj.Kind == rTriggeredNew) && fl["from"] == "Graveyard" {
			ki.C = kop.LibTopGrave + int16(in.C) // dirac/zone_tolibrarygrave.go
		}
		if _, rem := fl["Remember"]; rem && in.C == 1 && in.Obj.Kind == rDelayedRemembered && fl["from"] == "Battlefield" {
			ki.C = kop.LibBottomRememberBF // dirac/zone_tolibrarybfremember.go
		}
		if _, rem := fl["Remember"]; rem && in.C == 0 && in.Obj.Kind == rSelf && fl["from"] == "Battlefield" {
			ki.C = kop.LibTopRememberBF // dirac/zone_tolibrarybfremember.go
		}
		if _, ok := fl["TopShuffle"]; ok {
			ki.C = kop.LibTopShuffle
			if (in.Obj.Kind == rTriggered || in.Obj.Kind == rTriggeredNew) && fl["from"] == "Graveyard" {
				ki.C = kop.LibTopShuffleGrave
			}
		}
		if _, ok := fl["TopShuffleSelf"]; ok {
			ki.C = kop.LibTopShuffleSelf
		}
		if _, ok := fl["BottomShuffle"]; ok {
			ki.C = kop.LibBottomShuffle
		}
		if in.C == 3 {
			ki.Op, ki.C = kop.OpToLibraryChoice, 0
		}
		if in.C == 4 {
			ki.Op, ki.C = kop.OpToLibraryChoice, kop.LibChoiceSecond
		}
		if in.C == libChoiceCtrlGrave { // lower_destalt.go
			ki.Op, ki.C = kop.OpToLibraryChoice, kop.LibChoiceCtrlGrave
		}
	case "ToGraveyard":
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtToGraveyard))
	case "MayShuffleMove":
		ki.Op, ki.C = kop.OpMayShuffleMove, int16(in.C)
	case "Return":
		ki.Op, ki.C = kop.OpReturn, int16(zoneMap[in.C])
		_, underOpp := fl["UnderOpponent"]
		_, attachTo := fl["AttachTo"]
		if m := moveFlags(); m != 0 || fl["counters"] != "" || fl["from"] == "Library" || underOpp || attachTo {
			ki.Op = kop.OpReturnEx
			ki.C = int16(int(zoneMap[in.C]) | m<<3)
			if err := counters(); err != nil {
				return ki, err
			}
		}
		if underOpp {
			// GainControl$ ChosenPlayer (one opponent): B.N rides beside the library bits
			if fl["counters"] != "" || fl["from"] == "Library" {
				return ki, fmt.Errorf("Return under the opponent with counters or from the library")
			}
			ki.B = kop.Const(kop.RetUnderOpponent)
		}
		if attachTo {
			// AttachedTo$: the bearer's Ref rides
			// B bits 8..10 beside RetAttachTo (kop.RetAttachShift)
			if fl["counters"] != "" || underOpp || fl["from"] == "Library" {
				return ki, fmt.Errorf("Return attached with counters, UnderOpponent or from the library")
			}
			var r kop.Ref
			if in.F != nil {
				// a filter bearer (One Last Job): the controller
				// chooses among the legal permanents it matches, so F rides
				// the instruction and the Ref stays zero
				if err := objFilter(kop.ZoneBattlefield); err != nil {
					return ki, err
				}
			} else if r, err = cb.ref(in.Obj2); err != nil {
				return ki, err
			}
			if r > 7 {
				return ki, fmt.Errorf("attach bearer ref %d", r)
			}
			ki.B = kop.Const(kop.RetAttachTo | int(r)<<kop.RetAttachShift)
		}
		if ki.Op == kop.OpReturn {
			ki.B = kop.Const(originMask(fl["from"]))
			if _, ok := fl["DelayedMany"]; ok {
				ki.B = kop.Const(originMask(fl["from"]) | kop.RetDelayedMany) // the delayed many-object return (dirac/trig_delayedmanyreturn.go)
			}
		}
		if fl["from"] == "Library" {
			// Origin$ Library: the move takes a library card only, then its
			// owner's library is shuffled (the kernel otherwise skips a library
			// object)
			if fl["counters"] != "" {
				return ki, fmt.Errorf("Return from the library with counters")
			}
			if _, ok := fl["ForgetOther"]; ok {
				return ki, fmt.Errorf("Return from the library with ForgetOtherRemembered$")
			}
			if _, ok := fl["AlsoSelf"]; ok {
				return ki, fmt.Errorf("Return from the library with ThisDefinedAndTgts$")
			}
			ki.B = kop.Const(kop.RetFromLibrary)
			if _, ok := fl["ShuffleOwed"]; ok {
				ki.B = kop.Const(kop.RetFromLibrary | kop.RetShuffleOwed)
			}
		}
	case "ExileUntilLeaves":
		ki.Op = kop.OpExileUntilLeaves
	case "Search", "ChooseMove":
		ki.Op = kop.OpSearchEx
		zone := kop.ZoneLibrary
		sf := 0
		if in.Op == "ChooseMove" {
			ki.Op = kop.OpChooseMove
			zone = kop.ZoneGraveyard
			switch fl["from"] {
			case "Hand":
				zone = kop.ZoneHand
			case "Battlefield":
				zone = kop.ZoneBattlefield
			case "Exile":
				zone = kop.ZoneExile // every seat's exile (zone_exiledwithpick.go)
			case "Library":
				zone = kop.ZoneLibrary // a hidden pick, not a search (Sanar's per-colour picks)
			case "Graveyard,Exile":
				// graveexile: a game-wide pool over both zones; the walk in
				// dirac/value_tappower.go restricts the zones itself.
				zone = kop.ZoneNone
				sf |= kop.SrGraveExile
			case "Hand,Graveyard":
				// the fetching player's hand then
				// graveyard; SrHandGrave rides in B (below)
				zone = kop.ZoneNone
			}
		}
		for k, b := range map[string]int{"Tapped": kop.SrTapped, "ThenTop": kop.SrThenTop, "Mandatory": kop.SrMandatory, "Confirm": kop.SrConfirm, "ChooserYou": kop.SrChooserYou, "Reveal": kop.SrReveal, "OwnZone": kop.SrOwnZone, "ForgetOther": kop.SrForgetOther, "UnderYourControl": kop.SrUnderYourControl} {
			if _, ok := fl[k]; ok {
				sf |= b
			}
		}
		if _, ok := fl["Remember"]; ok {
			// RememberChanged$: the moved cards join the remembered set
			// (gorge zone_search eventRemember / Ctx.Remembered; kernel
			// moveFound). moveFound remembers a card put onto the
			// battlefield, into a hand or graveyard, or into a library from
			// a hand (gorge handLibraryTail: the bottom move); a library
			// SEARCH's moveFound did not remember, and neither does the
			// ThenTop path: refuse those.
			_, noShuf := fl["NoShuffle"]
			handToLib := in.Op == "ChooseMove" && zone == kop.ZoneHand && zoneMap[in.C] == kop.ZoneLibrary
			if zoneMap[in.C] == kop.ZoneLibrary && !noShuf && !handToLib || sf&kop.SrThenTop != 0 {
				return ki, fmt.Errorf("RememberChanged$ on a search into a library")
			}
			sf |= kop.SrRemember
		}
		ki.C = int16(int(zoneMap[in.C]) | sf<<3)
		// SrForgetChanged rides in B too; SrAttacking / SrDifferentNames ride in B (C>>3 is full); B.N bit 0
		// is the UntilLeaves hand-exile link
		sb := 0
		if _, ok := fl["Attacking"]; ok {
			sb |= kop.SrAttacking
		}
		if _, ok := fl["DifferentNames"]; ok {
			sb |= kop.SrDifferentNames
		}
		if _, ok := fl["NoShuffle"]; ok {
			if in.Op != "Search" && zone != kop.ZoneLibrary {
				return ki, fmt.Errorf("NoShuffle on a %s", in.Op)
			}
			sb |= kop.SrNoShuffle
		}
		if _, ok := fl["Imprint"]; ok {
			if in.Op != "ChooseMove" || zone != kop.ZoneLibrary || zoneMap[in.C] != kop.ZoneExile {
				return ki, fmt.Errorf("Imprint on a %s from %s", in.Op, fl["from"])
			}
			sb |= kop.SrImprint
		}
		if _, ok := fl["Random"]; ok {
			if in.Op != "ChooseMove" || zone != kop.ZoneGraveyard && zone != kop.ZoneExile {
				return ki, fmt.Errorf("Random on a %s", in.Op)
			}
			sb |= kop.SrRandom // Tersa Lightshatter
		}
		if _, ok := fl["GameGrave"]; ok {
			// A game-wide graveyard pick (gorge effHiddenPick gameWide: no
			// ValidTgts$, no DefinedPlayer$): the pool is every player's
			// graveyard in seat order; picked cards enter their owner's zone.
			// Only a graveyard origin poses it; the pick's destination rides
			// in C. Counters never co-occur (a counter rider is refused
			// before the pick lowers).
			if in.Op != "ChooseMove" || zone != kop.ZoneGraveyard {
				return ki, fmt.Errorf("GameGrave on a %s from %s", in.Op, fl["from"])
			}
			sb |= kop.SrGameGrave
		}
		if _, ok := fl["Bottom"]; ok {
			// LibraryPosition$ -1 on a hand move: the picked cards go to the
			// bottom of the library (gorge handLibraryTail's
			// libraryOrderPlacement bottom=true). Only a hand-to-library
			// ChooseMove carries it.
			if in.Op != "ChooseMove" || zone != kop.ZoneHand || zoneMap[in.C] != kop.ZoneLibrary {
				return ki, fmt.Errorf("Bottom on a %s from %s", in.Op, fl["from"])
			}
			sb |= kop.SrHandBottom
		}
		if _, ok := fl["PosAsk"]; ok {
			// the controller's top-or-bottom ask after a hand put-back
			// (Dream Cache; lower_destalt.go)
			if in.Op != "ChooseMove" || zone != kop.ZoneHand || zoneMap[in.C] != kop.ZoneLibrary {
				return ki, fmt.Errorf("PosAsk on a %s from %s", in.Op, fl["from"])
			}
			sb |= kop.SrHandPosAsk
		}
		if _, ok := fl["DestAltGrave"]; ok {
			// the searcher's hand-or-graveyard ask after the pick (Dina's
			// Guidance; lower_destalt.go)
			if in.Op != "Search" || zoneMap[in.C] != kop.ZoneHand {
				return ki, fmt.Errorf("DestAltGrave on a %s", in.Op)
			}
			sb |= kop.SrDestAltGrave
		}
		if _, ok := fl["ShareLandType"]; ok {
			if in.Op != "Search" {
				return ki, fmt.Errorf("ShareLandType on a %s", in.Op)
			}
			sb |= kop.SrShareLandType
		}
		if _, ok := fl["ForgetChanged"]; ok {
			if in.Op != "Search" {
				return ki, fmt.Errorf("ForgetChanged on a %s", in.Op)
			}
			sb |= kop.SrForgetChanged
		}
		if az, ok := fl["alt"]; ok {
			// OriginAlternative$: the library
			// search's zone election; the alternative zone rides B bits 5..7
			_, altAll := fl["altAll"]
			_, noShuf := fl["NoShuffle"]
			libPut := altAll && noShuf && zoneMap[in.C] == kop.ZoneLibrary // Doomsday (lower_libgrave.go)
			if in.Op != "Search" || zoneMap[in.C] != kop.ZoneHand && zoneMap[in.C] != kop.ZoneBattlefield && !libPut {
				return ki, fmt.Errorf("alt origin on a %s to zone %d", in.Op, zoneMap[in.C])
			}
			var altZ kop.Zone
			switch az {
			case "Graveyard":
				altZ = kop.ZoneGraveyard
			case "Hand":
				altZ = kop.ZoneHand
			case "Graveyard,Hand":
				// the pair occupies the zone code the library can never use:
				// the election offers the library, graveyard and hand.
				altZ = kop.Zone(kop.SrAltGraveHand)
			default:
				return ki, fmt.Errorf("alt origin %q", az)
			}
			sb |= kop.SrAltOrigin | int(altZ)<<kop.SrAltZoneShift
			if _, ok := fl["altAll"]; ok {
				// Origin$ Library,Graveyard (lower_libgrave.go): every zone, no election
				if altZ != kop.ZoneGraveyard {
					return ki, fmt.Errorf("altAll with alt origin %q", az)
				}
				sb |= kop.SrAltAll
			}
			zone = kop.ZoneNone // the filter matches cards of every elected zone
		}
		if sb, zone, err = sideSearchFlags(in.Op, fl, sb, zone); err != nil {
			return ki, err
		}
		if _, ok := fl["AuraEntry"]; ok {
			// the hand move of one card whose filter may
			// admit an Aura (CR 303.4f/g, kernel zone_handmoveaura.go)
			if in.Op != "ChooseMove" || zone != kop.ZoneHand || zoneMap[in.C] != kop.ZoneBattlefield {
				return ki, fmt.Errorf("AuraEntry on a %s", in.Op)
			}
			sb |= kop.SrAuraEntry
		}
		if _, ok := fl["UntilLeaves"]; ok {
			// Duration$ UntilHostLeavesPlay on a hand exile
			// (Deep-Cavern Bat): B.N bit 0, the kernel's hand-return link
			if in.Op != "ChooseMove" || zone != kop.ZoneHand || zoneMap[in.C] != kop.ZoneExile {
				return ki, fmt.Errorf("UntilLeaves on a %s", in.Op)
			}
			sb |= 1
		}
		if in.Op == "ChooseMove" && fl["from"] == "Hand,Graveyard" {
			for _, bad := range []string{"Random", "UntilLeaves", "NoShuffle", "ForgetChanged"} {
				if _, ok := fl[bad]; ok {
					return ki, fmt.Errorf("%s on a Hand,Graveyard pick", bad)
				}
			}
			sb |= kop.SrHandGrave
			if k, ok := fl["counters"]; ok {
				ck, err := counterKind(k)
				if err != nil {
					return ki, err
				}
				if int(ck)+1 > 0xFF {
					return ki, fmt.Errorf("Hand,Graveyard pick counter kind %d", ck)
				}
				sb |= (int(ck) + 1) << 4
			}
		}
		if in.Op == "ChooseMove" && fl["from"] == "Graveyard" && fl["counters"] != "" {
			// WithCountersType$ on a plain graveyard pick
			// (Scavenger's Talent): the kind rides B bits 4..11, the same
			// field as Hand,Graveyard (SrHandGrave stays clear; bit 2
			// distinguishes the two pool shapes). The kernel arms exactly
			// one entry counter (handGraveArm).
			for _, bad := range []string{"Random", "UntilLeaves", "ForgetChanged"} {
				if _, ok := fl[bad]; ok {
					return ki, fmt.Errorf("%s on a Graveyard pick", bad)
				}
			}
			ck, err := counterKind(fl["counters"])
			if err != nil {
				return ki, err
			}
			if int(ck)+1 > 0xFF {
				return ki, fmt.Errorf("Graveyard pick counter kind %d", ck)
			}
			sb |= (int(ck) + 1) << 4
		}
		if sb != 0 {
			ki.B = kop.Const(sb)
		}
		if err := objFilter(zone); err != nil {
			return ki, err
		}
	case "ChangeZoneAll":
		from := zoneNames[fl["from"]]
		if fl["from"] == "Graveyard,Exile" {
			// graveexile: every remembered match in each player's Graveyard then
			// Exile (gorge changeZoneAllPlayers over both zones).
			if err := objFilter(kop.ZoneNone); err != nil {
				return ki, err
			}
			ki.Op = kop.OpChangeZoneAllEx
			ki.C = int16(int(zoneMap[in.C]) | (moveFlags()|kop.MvGraveExile)<<3)
			break
		}
		if b, ok := czallTwoZoneB(fl["from"]); ok {
			// two origins swept zone-major over every seat; the second zone
			// (or a Cz* selector) rides in B, ZoneNone lets the owner role build
			if err := objFilter(kop.ZoneNone); err != nil {
				return ki, err
			}
			ki.Op = kop.OpChangeZoneAllEx
			ki.C = int16(int(zoneMap[in.C]) | (moveFlags()|kop.MvGraveExile)<<3)
			if _, ok := fl["ShuffleEach"]; ok {
				b |= kop.CzShuffleEach // the kernel shuffles both libraries after the sweep
			}
			ki.B = kop.Const(b)
			break
		}
		if err := objFilter(zoneMap[from]); err != nil {
			return ki, err
		}
		ki.Op, ki.C = kop.OpChangeZoneAll, int16(zoneMap[in.C])
		_, untilLeaves := fl["UntilLeaves"]
		if m := moveFlags(); m != 0 || in.Obj.Kind != rNone || untilLeaves {
			ki.Op = kop.OpChangeZoneAllEx
			ki.C = int16(int(zoneMap[in.C]) | m<<3)
		}
		if fl["counters"] != "" || fl["static"] != "" || fl["atEOT"] != "" {
			// WithCountersType$ / StaticEffect$ on a sweep onto the
			// battlefield: A = constant count | counter kind<<8, B = override
			// index+1 (the index is pool-wide, so B gets all 16 bits); the
			// kernel applies both per entrant (kop.MvEntryRiders)
			if zoneMap[in.C] != kop.ZoneBattlefield {
				return ki, fmt.Errorf("ChangeZoneAll entry riders to zone %d", in.C)
			}
			ki.Op = kop.OpChangeZoneAllEx
			ki.C = int16(int(zoneMap[in.C]) | moveFlags()<<3)
			ki.A, ki.B = kop.Const(0), kop.Const(0)
			if k, ok := fl["counters"]; ok {
				ck, err := counterKind(k)
				if err != nil {
					return ki, err
				}
				if !in.A.isConst() || in.A.N < 1 || in.A.N > 255 {
					return ki, fmt.Errorf("ChangeZoneAll counter count is not a constant 1..255")
				}
				ki.A = kop.Const(int(ck)<<8 | in.A.N)
			}
			if idx, ok := fl["static"]; ok {
				n, err := strconv.Atoi(idx)
				if err != nil {
					return ki, fmt.Errorf("ChangeZoneAll static=%q", idx)
				}
				ov, err := cb.override(cb.c.overrides[n])
				if err != nil {
					return ki, err
				}
				if ov+1 > 0xffff {
					return ki, fmt.Errorf("override index %d above 65534", ov)
				}
				ki.B = kop.Const(ov + 1)
			}
			if a, ok := fl["atEOT"]; ok {
				// one next-end-step promise per entrant,
				// kop.EntryAtEOT in A's high bit (kind <= kop.EntryKindMask), the act in
				// kop.EntryAtEOTExile
				act, err := strconv.Atoi(a)
				if err != nil || act < 0 || act > 1 {
					return ki, fmt.Errorf("ChangeZoneAll atEOT=%q", a)
				}
				if ck := int(uint16(ki.A.N)) >> 8; ck > kop.EntryKindMask {
					return ki, fmt.Errorf("ChangeZoneAll counter kind %d collides with the AtEOT bits", ck)
				}
				ki.A.N |= entryAtEOTBits(act)
			}
			// MvEntryRiders is bit 12, so its shifted C image is the sign bit:
			// shift a variable (the constant 1<<15 does not fit int16).
			er := int16(kop.MvEntryRiders)
			ki.C |= er << 3
		}
		if _, forget := fl["ForgetOther"]; forget || untilLeaves {
			if err := czallBuild3(&ki, zoneMap[in.C], forget, untilLeaves); err != nil { // lower_changezoneall.go
				return ki, err
			}
		}
	case "Draw":
		ki.Op = kop.OpDraw
		if err := remember3(in.CS, &ki); err != nil {
			return ki, err
		}
	case "Discard":
		ki.Op = kop.OpDiscard
		switch in.CS {
		case "DiscarderChooses":
			ki.C = kop.ChooseSelf
		case "YouChoose":
			ki.C = kop.ChooseOpp
		case "Random":
			ki.C = kop.ChooseRandom
		default:
			return ki, fmt.Errorf("discard chooser %q", in.CS)
		}
		ki.C |= int16(in.C) // in.C: kop.OptAsk / OptAny / OptAll
		if in.Note == noteRememberDiscarded {
			ki.B = kop.Const(1) // kop.DiscRemember
		}
		if err := objFilter(kop.ZoneHand); err != nil {
			return ki, err
		}
	case "UntapChoose":
		ki.Op, ki.C = kop.OpUntapChoose, int16(in.C)
		if err := objFilter(kop.ZoneBattlefield); err != nil {
			return ki, err
		}
	case "DiscardUnless":
		ki.Op = kop.OpDiscardUnless
		if err := objFilter(kop.ZoneHand); err != nil {
			return ki, err
		}
	case "Mill":
		for k := range fl {
			switch k {
			case "Remember":
				ki.C |= kop.MvRemember
			case "Optional":
				ki.C |= kop.MvMayMill
			case "ImprintMove":
				ki.C |= kop.MvImprint
			default:
				return ki, fmt.Errorf("mill %s", in.CS)
			}
		}
		ki.Op = kop.OpMill
	case "Surveil":
		ki.Op = kop.OpSurveil
		if in.CS == "RememberKept" {
			ki.C |= kop.SurveilRememberKept
		} else if err := remember3(in.CS, &ki); err != nil {
			return ki, err
		}
	case "Scry":
		ki.Op = kop.OpScry
	case "PutCounterAll":
		ki.Op = kop.OpPutCounterAll
		k, err := counterKind(in.CS)
		if err != nil {
			return ki, err
		}
		ki.C = int16(k)
		if err := objFilter(kop.ZoneBattlefield); err != nil {
			return ki, err
		}
	case "UntapAll":
		ki.Op = kop.OpUntapAll
		if err := objFilter(kop.ZoneBattlefield); err != nil {
			return ki, err
		}
	case "Amass":
		ki.Op = kop.OpAmass
		idx, ok := cb.pt.tokenIdx[in.CS]
		if !ok {
			return ki, fmt.Errorf("token %q", in.CS)
		}
		ki.C = int16(idx)
		i, err := cb.override(cb.c.overrides[in.C])
		if err != nil {
			return ki, err
		}
		if i+1 > 0xfff {
			return ki, fmt.Errorf("amass override index %d", i)
		}
		ki.B = kop.Const(i + 1)
		if in.B.N != 0 {
			ki.B.N |= -0x8000 // bit 15: RememberAmass$
		}
	case "Empower":
		ki.Op = kop.OpEmpower
		idx, ok := cb.pt.tokenIdx[in.CS]
		if !ok {
			return ki, fmt.Errorf("token %q", in.CS)
		}
		ki.C = int16(idx)
		i, err := cb.override(cb.c.overrides[in.C])
		if err != nil {
			return ki, err
		}
		if i+1 > 0xfff {
			return ki, fmt.Errorf("empower override index %d", i)
		}
		ki.B = kop.Const(i + 1)
	case "Manifest":
		ki.Op = kop.OpManifest
		if in.C != 0 {
			ki.B = kop.Const(in.C) // CfManifestRemembered
		}
	case "Cloak":
		ki.Op = kop.OpCloak
		ki.B = kop.Const(in.C)
	case "ManifestDread":
		ki.Op = kop.OpManifestDread
		ki.B = kop.Const(in.C)
	case "Incubate":
		ki.Op = kop.OpIncubate
		idx, ok := cb.pt.tokenIdx[in.CS]
		if !ok {
			return ki, fmt.Errorf("token %q", in.CS)
		}
		ki.C = int16(idx)
	case "Seek":
		ki.Op, ki.C = kop.OpSeek, int16(in.C)
		if err := objFilter(kop.ZoneLibrary); err != nil {
			return ki, err
		}
	case "TapAll":
		ki.Op, ki.C = kop.OpTapAll, int16(in.C) // C 1: TapperController$ True (act_sweeps.go tapAll)
		if err := objFilter(kop.ZoneBattlefield); err != nil {
			return ki, err
		}
	case "GainLife":
		ki.Op = kop.OpGainLife
	case "LoseLife":
		ki.Op = kop.OpLoseLife
	case "PumpUntilEOT":
		ki.Op = kop.OpPumpUntilEOT
	case "GrantKeyword":
		ki.Op = kop.OpGrantKeyword
		m, err := kwMask(strings.Split(in.CS, "&"))
		if err != nil {
			return ki, err
		}
		ki.C = int16(uint16(m))
		ki.B = kop.ValueRef{K: kop.VConst, N: int16(uint16(m >> 16))}
	case "PumpAll":
		ki.Op = kop.OpPumpAll
		if in.CS != "" {
			m, err := kwMask(strings.Split(in.CS, "&"))
			if err != nil {
				return ki, err
			}
			if m>>16 != 0 {
				return ki, fmt.Errorf("PumpAll keyword above bit 15")
			}
			ki.C = int16(uint16(m))
		}
		if err := objFilter(kop.ZoneBattlefield); err != nil {
			return ki, err
		}
	case "GrantKwAll":
		// OpExt ExtGrantKwAll: a keyword-only PumpAll above bit 15 (lower_pumpallhighkw.go)
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtGrantKwAll))
		if ki.C, ki.B, err = kwMaskOperands(in.CS); err != nil {
			return ki, err
		}
		if err := objFilter(kop.ZoneBattlefield); err != nil {
			return ki, err
		}
	case "PumpAllEach":
		ki.Op, ki.Obj = kop.OpPumpAllEach, kop.Ref(in.C) // kop.PumpEach* bits
		if in.CS != "" {
			m, err := kwMask(strings.Split(in.CS, "&"))
			if err != nil {
				return ki, err
			}
			if m>>16 != 0 {
				return ki, fmt.Errorf("PumpAllEach keyword above bit 15")
			}
			ki.C = int16(uint16(m))
		}
		if err := objFilter(kop.ZoneBattlefield); err != nil {
			return ki, err
		}
	case "DoubleCountersAll":
		ki.Op = kop.OpDoubleCountersAll
		if in.CS == "All" {
			ki.C = 255
		} else {
			k, err := counterKind(in.CS)
			if err != nil {
				return ki, err
			}
			ki.C = int16(k)
		}
		if err := objFilter(kop.ZoneBattlefield); err != nil {
			return ki, err
		}
	case "PutCounter", "RemoveCounter", "EnterWithCounters":
		ki.Op = map[string]kop.Op{"PutCounter": kop.OpPutCounter, "RemoveCounter": kop.OpRemoveCounter, "EnterWithCounters": kop.OpEnterWithCounters}[in.Op]
		if in.Op == "RemoveCounter" && in.C == 1 {
			ki.Op = kop.OpRemoveCounterRem
		}
		k, err := counterKind(in.CS)
		if err != nil {
			return ki, err
		}
		ki.C = int16(k)
	case "Bolster":
		ki.Op = kop.OpBolster
		k, err := counterKind(in.CS)
		if err != nil {
			return ki, err
		}
		ki.C = int16(k)
	case "Support":
		ki.Op = kop.OpSupport
		k, err := counterKind(in.CS)
		if err != nil {
			return ki, err
		}
		ki.C = int16(k) | int16(in.C)<<8 // C=1: other than the source
		if in.C&2 != 0 {                 // the remove form (2 remove, 4 exact, 8 remember amount) elects over F
			if err := objFilter(kop.ZoneBattlefield); err != nil {
				return ki, err
			}
		}
	case "Monstrosity":
		ki.Op = kop.OpMonstrosity
	case "Earthbend":
		ki.Op = kop.OpEarthbend
		i, err := cb.override(cb.c.overrides[in.C])
		if err != nil {
			return ki, err
		}
		ki.C = int16(i)
	case "DoubleCounters":
		ki.Op = kop.OpDoubleCounters
		if in.CS == "All" {
			ki.C = 255
		} else {
			k, err := counterKind(in.CS)
			if err != nil {
				return ki, err
			}
			ki.C = int16(k)
		}
	case "CreateToken":
		ki.Op = kop.OpCreateTokenEx
		idx, ok := cb.pt.tokenIdx[in.CS]
		if !ok {
			return ki, fmt.Errorf("token %q", in.CS)
		}
		ki.C = int16(idx)
		if in.C&(6|24|32|64|128|256|512|1024|2048) == 0 { // tokListBit rides the core op
			// the core's CreateToken: B.N = TokenRider flags.
			ki.Op = kop.OpCreateToken
			b := 0
			if in.C&1 != 0 {
				b |= kop.TrTapped
			}
			if in.C&tokListBit != 0 {
				b |= kop.TrList
			}
			if b != 0 {
				ki.B = kop.Const(b)
			}
			break
		}
		tf := 0
		if in.C&2 != 0 {
			tf |= kop.TokRemember
		}
		if in.C&4 != 0 {
			tf |= kop.TokRememberObj
		}
		if in.C&1 != 0 {
			tf |= kop.TokTapped
		}
		if in.C&2048 != 0 {
			tf |= kop.TokImprintObj // lower_imprintzone.go
		}
		if in.C&8 != 0 {
			tf |= kop.TokSacAtEnd
		}
		if in.C&16 != 0 {
			tf |= kop.TokExileAtEnd
		}
		if in.C&512 != 0 {
			tf |= int(kop.TokAtEOTDestroy) << 8
		}
		if in.C&1024 != 0 {
			tf |= int(kop.TokAtEOTSacCombat) << 8
		}
		if in.C&64 != 0 {
			tf |= kop.TokAttacking
		}
		if in.C&128 != 0 {
			tf |= kop.TokAttackAsk
		}
		if in.C&32 != 0 {
			r, err := cb.ref(in.Obj2)
			if err != nil {
				return ki, err
			}
			tf |= kop.TokAttach
			ki.B = kop.Const(tf | int(kop.TokAttachN(r)))
			break
		}
		ki.B = kop.Const(tf)
	case "GiveGift":
		ki.Op = kop.OpGiveGift
	case "GainEnergy":
		ki.Op = kop.OpGainEnergy
	case "Proliferate":
		ki.Op = kop.OpProliferate
	case "Saddle":
		ki.Op, ki.C = kop.OpSaddle, int16(in.C) // C = 1: Solved
	case "DigCounters":
		ki.Op, ki.C = kop.OpDigCounters, int16(in.C)
	case "SearchCounters":
		// (Captain Jean-Luc Picard) OpExt ExtSearchCounters: B the amount
		// (0 clears), C the kind; kgen emits it around the Search.
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtSearchCounters)), int16(in.C)
	case "TakeInitiative":
		ki.Op = kop.OpTakeInitiative
		idx, ok := cb.pt.tokenIdx[in.CS]
		if !ok {
			return ki, fmt.Errorf("dungeon %q", in.CS)
		}
		ki.C = int16(idx)
	case "Goad":
		ki.Op = kop.OpGoad
		if in.F != nil {
			if err := objFilter(kop.ZoneBattlefield); err != nil {
				return ki, err
			}
		}
	case "Prepare":
		ki.Op, ki.C = kop.OpPrepare, int16(in.C) // C=1: unprepare (Activate$ False)
	case "ChangeSpeed":
		ki.Op, ki.C, ki.B = kop.OpChangeSpeed, int16(in.C), kop.Const(in.B.N)
	case "Transform":
		ki.Op = kop.OpTransform
	case "ClassLevelUp":
		if ki.Obj != kop.RefSelf || !in.A.isConst() {
			return ki, fmt.Errorf("ClassLevelUp on %v", in.Obj)
		}
		ki.Op = kop.OpClassLevelUp
	case "Tap":
		ki.Op = kop.OpTap
	case "Untap":
		ki.Op = kop.OpUntap
	case "EnterTapped":
		ki.Op = kop.OpEnterTapped
	case "Counter":
		ki.Op, ki.C = kop.OpCounter, int16(in.C)
		if in.B.Kind != vNone {
			ki.B = kop.Const(in.B.N) // CtrRemMV / CtrRemSpell
		}
	case "Unearth":
		ki.Op = kop.OpUnearth
	case "EnterUntapped":
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtEnterUntapped))
	case "EnterAttacking":
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtEnterAttacking))
	case "ExchangeControl":
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtExchangeControl)), int16(in.C)
	case "EachDamage":
		// (Case of the Gateway Express) OpExt ExtEachDamage
		// C: the recipient Ref (low byte) | the damager target slots << 8
		// (0 = the F sweep)
		r, err := cb.ref(in.Obj2)
		if err != nil {
			return ki, err
		}
		if err := objFilter(kop.ZoneBattlefield); err != nil {
			return ki, err
		}
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtEachDamage)), int16(r)|int16(in.C)<<8
	case "Cumulative":
		// (Mystic Remora) OpExt ExtCumulative, C the instructions skipped
		// when the source is gone (lower_cumulative.go)
		if in.C < 1 || in.C > 3 {
			return ki, fmt.Errorf("Cumulative skip %d", in.C)
		}
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtCumulative)), int16(in.C)
	case "NumTimes":
		// (Tranquil Frillback) OpExt ExtNumTimes, B the unit's packed
		// pips; C = the unit's generic | the announcement maximum << 4 | the
		// decline jump << 7
		if ki.A.K != kop.VConst || in.C < 1 || in.C > 7 || ki.A.N < 0 || ki.A.N > 15 || in.Skip < 0 || in.Skip > 255 {
			return ki, fmt.Errorf("NumTimes shape")
		}
		ki.C = ki.A.N | int16(in.C)<<4 | int16(in.Skip)<<7
		if strings.Contains(in.CS, "open") {
			ki.F = 1 // no AnnounceMax$: stall loud past the maximum
		}
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtNumTimes))
	case "PutCounterExisting":
		// (Dathon, His Heart Open) OpExt ExtPutCounterExisting; Obj and B
		// are converted above
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtPutCounterExisting))
	case "RemoveCounterAny":
		// (Glissa Sunslayer) OpExt ExtRemoveCounterAny
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtRemoveCounterAny)), int16(in.C)
	case "DistCounters":
		// (Vastwood Hydra) OpExt ExtDistCounters: B the total, F the pool
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtDistCounters))
		if err := objFilter(kop.ZoneBattlefield); err != nil {
			return ki, err
		}
	case "TurnFaceUp":
		// (CR 708.6; gorge effSetState Mode$ TurnFaceUp) OpExt ExtTurnFaceUp:
		// C 0 the target in Obj, C 1 the up-to-one pick over F, C 2 the
		// mandatory Transform pick over F (lower_setstate.go)
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtTurnFaceUp)), int16(in.C)
		if in.F != nil {
			if err := objFilter(kop.ZoneBattlefield); err != nil {
				return ki, err
			}
		}
	case "UnlockDoor":
		// (CR 709.5f-g; gorge effUnlockDoor) OpExt
		// ExtUnlockDoor over the Room target slot, C 0 unlock / 1 lock-or-unlock
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtUnlockDoor)), int16(in.C)
	case "MayPlayGrave":
		// (Case of the Uneaten Feast) OpExt ExtMayPlayGrave
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtMayPlayGrave))
		if err := objFilter(kop.ZoneGraveyard); err != nil {
			return ki, err
		}
	case "ManaChoice":
		// (Chandra, Hope's Beacon) OpExt ExtManaChoice, B the amount,
		// C the colour mask (bit 8: Produced$ Any, one pick for the whole amount; bit 9: PersistentMana$ True)
		v, err := cb.value(in.A)
		if err != nil {
			return ki, err
		}
		ki.Op, ki.A, ki.B, ki.C = kop.OpExt, kop.Const(int(kop.ExtManaChoice)), v, int16(int(colorMask(in.CS))|in.C<<8)
		if isConstN(in.B, 2) {
			ki.C |= 0x200 // PersistentMana$ True: each pick is tallied in State.pm (mana_persistent.go)
		}
		if in.F != nil {
			// RestrictValid$ Spell.<spec>: each pick floats as restricted mana
			if ki.F, err = cb.filter(in.F, kop.ZoneStack); err != nil {
				return ki, err
			}
		}
	case "LibSweep":
		// (Teferi, Temporal Pilgrim) OpExt ExtLibSweep, F the swept permanents
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtLibSweep))
		if err := objFilter(kop.ZoneBattlefield); err != nil {
			return ki, err
		}
	case "ChooseColorRes":
		// (Skrelv) OpExt ExtChooseColorRes
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtChooseColorRes))
	case "Hideaway":
		// (Collector's Cage) OpExt ExtHideaway, B the looked-at count (C 0)
		v, err := cb.value(in.A)
		if err != nil {
			return ki, err
		}
		ki.Op, ki.A, ki.B, ki.C = kop.OpExt, kop.Const(int(kop.ExtHideaway)), v, int16(in.C)
	case "ExileExMark":
		// OpExt ExtExileExMark over the target slot Obj;
		// C the Fight's fighter ref (RefNone for a Pump)
		r, err := cb.ref(in.Obj2)
		if err != nil {
			return ki, err
		}
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtExileExMark)), int16(r)
	case "ExHaste":
		// Agility Bobblehead: OpExt ExtExHaste, C the power form (Instr.C is int16)
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtExHaste)), int16(in.C)
	case "AsgnGrant":
		// (Plagon, Outmaneuver) OpExt ExtAsgnGrant, C 0 toughness / 1 as-unblocked
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtAsgnGrant)), int16(in.C)
	case "ColorGrant":
		// (Skrelv) OpExt ExtColorGrant, B the toxic amount
		v, err := cb.value(in.A)
		if err != nil {
			return ki, err
		}
		ki.Op, ki.A, ki.B, ki.C = kop.OpExt, kop.Const(int(kop.ExtColorGrant)), v, int16(in.C)
		if in.F != nil {
			// (Akroma's Blessing) the sweep's filter
			if err := objFilter(kop.ZoneBattlefield); err != nil {
				return ki, err
			}
		}
	case "Endure":
		// OpExt ExtEndure, B the Spirit token, C the N/N
		// override (its PA is N)
		idx, ok := cb.pt.tokenIdx[in.CS]
		if !ok {
			return ki, fmt.Errorf("token %q", in.CS)
		}
		i, err := cb.override(cb.c.overrides[in.C])
		if err != nil {
			return ki, err
		}
		ki.Op, ki.A, ki.B, ki.C = kop.OpExt, kop.Const(int(kop.ExtEndure)), kop.Const(idx), int16(i)
	case "TokenPT":
		// OpExt ExtTokenPT right before the CreateToken, C the
		// layer-7b set override (its PA/TA are the X/X, read before the mint)
		i, err := cb.override(cb.c.overrides[in.C])
		if err != nil {
			return ki, err
		}
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtTokenPT)), int16(i)
	case "SpellToLib":
		// OpExt ExtSpellToLib: the resolving spell to its owner's library, shuffled
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtSpellToLib))
	case "LibPlace":
		// OpExt ExtLibPlace, C the arm (top / bottom) or the ask (lower_libplace.go)
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtLibPlace)), int16(in.C)
	case "AttacksIfAble":
		// OpExt ExtAttacksIfAble, Obj the target that attacks this turn if able
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtAttacksIfAble)), int16(in.C)
	case "BlocksIfAble":
		// OpExt ExtBlocksIfAble, Obj the target that blocks this turn if able
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtBlocksIfAble))
	case "KwMark":
		// OpExt ExtKwMark, C the keywords (kop.KwMarkIntimidate |
		// kop.KwMarkInfect), Obj the subject (RefNone with F: every
		// battlefield permanent matching F; lower_kwmark.go)
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtKwMark)), int16(in.C)
		if in.F != nil {
			if err := objFilter(kop.ZoneBattlefield); err != nil {
				return ki, err
			}
		}
	case "ChangeText":
		// OpExt ExtChangeText, Obj the one target, C the word list (textreach.go)
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtChangeText)), int16(in.C)
	case "RemoveFromCombat":
		// OpExt ExtRemoveFromCombat, Obj the one object removed (lower_removefromcombat.go)
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtRemoveFromCombat))
	case "MustBeBlocked":
		// OpExt ExtMustBeBlocked, Obj the subject (RefNone
		// with F: every battlefield permanent matching F)
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtMustBeBlocked))
		if in.C == kop.MustBeBlockedLure {
			ki.C = kop.MustBeBlockedLure // the lure mark (a single Obj)
		}
		if in.F != nil {
			if err := objFilter(kop.ZoneBattlefield); err != nil {
				return ki, err
			}
		}
	case "Suspect":
		// OpExt ExtSuspect, Obj the subject (RefNone with F: every
		// battlefield permanent matching F), C bit 0 the Activate$ False removal
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtSuspect)), int16(in.C&1)
		if in.F != nil {
			if err := objFilter(kop.ZoneBattlefield); err != nil {
				return ki, err
			}
		}
	case "CopyAllEach":
		// OpExt ExtCopyAllEach, F the Defined$ Valid set,
		// C the copy exceptions' override index+1 (as CopyAsToken)
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtCopyAllEach))
		ki.B = kop.Const(0)
		if in.C > 0 {
			i, err := cb.override(cb.c.overrides[in.C-1])
			if err != nil {
				return ki, err
			}
			ki.C = int16(i + 1)
		}
		if err := objFilter(kop.ZoneBattlefield); err != nil {
			return ki, err
		}
	case "AddPhase":
		// OpExt ExtAddPhase, B the count, C the kind | splice point
		ki.Op, ki.B, ki.C = kop.OpExt, ki.A, int16(in.C)
		ki.A = kop.Const(int(kop.ExtAddPhase))
	case "SkipPhase":
		// OpExt ExtSkipPhase, Obj the skipping player (dirac/act_skipphase.go)
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtSkipPhase))
	case "BecomeMonarch":
		// OpExt ExtBecomeMonarch, Obj the player (dirac/act_monarch.go)
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtBecomeMonarch))
	case "PlotGrant":
		// (Kellan Joins Up) OpExt ExtPlotGrant, Obj the
		// Remembered ref
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtPlotGrant))
	case "EffEtbCounters":
		// OpExt ExtEffEtbCtr, B the count (SetChosenNumber$),
		// Obj the triggering spell (the kernel reads the entry's trigObj)
		ki.Op, ki.B = kop.OpExt, ki.A
		ki.A = kop.Const(int(kop.ExtEffEtbCtr))
	case "StackExileMark":
		// OpExt ExtStackExileMark, C the kop.StackExile* bits (repl_stackexile.go)
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtStackExileMark)), int16(in.C)
	case "DmgMark":
		// OpExt ExtDmgMark, Obj the damaged player, or C kop.DmgMarkBySource
		// for the resolving controller's sources (dirac/repl_dmgmark.go)
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtDmgMark)), int16(in.C)
	case "EffFinalityCtr":
		// OpExt ExtFinalityCtr, Obj the triggering spell
		// (the kernel reads the entry's trigObj); the spell enters with one
		// FINALITY counter per applied grant
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtFinalityCtr))
	case "CopyAbility":
		// OpExt ExtCopyAbility, B the copy count (Amount$, default 1), C bit 0
		// the MayChooseTarget$ election and C>>1 the target slot (not Obj: a
		// target Ref would run the program once per target)
		slot, ok := tgtSlotOfRef(ki.Obj)
		if ki.Obj == kop.RefTriggerObj {
			slot, ok = kop.CopyAbTriggering, true // the ability that fired an AbilityCast trigger
		}
		if !ok {
			return ki, fmt.Errorf("CopyAbility Obj %d", ki.Obj)
		}
		n := ki.A
		ki.Op, ki.A, ki.B, ki.C = kop.OpExt, kop.Const(int(kop.ExtCopyAbility)), n, int16(in.C&1)|int16(slot)<<1
		ki.Obj = kop.RefSelf
	case "DividedCounters":
		// (Quirion Beastcaller) OpExt ExtDividedCounters, Obj the target slot
		// (C, not Obj: a target Ref would run the program once per target)
		slot, ok := tgtSlotOfRef(ki.Obj)
		if !ok {
			return ki, fmt.Errorf("DividedCounters Obj %d", ki.Obj)
		}
		ki.Op, ki.A, ki.C, ki.Obj = kop.OpExt, kop.Const(int(kop.ExtDividedCounters)), int16(slot), kop.RefSelf
	case "DrainMana":
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtDrainMana))
	case "CounterLose":
		// (Tishana's Tidebinder) OpExt ExtCounterLose, C the
		// host-bound LoseAbilities override
		i, err := cb.override(cb.c.overrides[in.C])
		if err != nil {
			return ki, err
		}
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtCounterLose)), int16(i)
	case "BreachChoose":
		// (Breach the Multiverse) OpExt ExtBreachChoose
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtBreachChoose))
	case "BreachReturn":
		// (Breach the Multiverse) OpExt ExtBreachReturn
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtBreachReturn))
	case "ExileLandChoose":
		// (Ensign Pavel Chekov) OpExt ExtExileLandChoose
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtExileLandChoose))
	case "Airbend":
		// OpExt ExtAirbend over Obj, or F (Obj RefNone) less
		// target slot C-1
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtAirbend)), int16(in.C)
		if in.F != nil {
			if err := objFilter(kop.ZoneBattlefield); err != nil {
				return ki, err
			}
		}
	case "ChangeTargets":
		// OpExt ExtChangeTargets over Obj (the
		// TgtStackObj slot, or the triggering spell); C bit 0: Optional$
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtChangeTargets)), int16(in.C)
	case "CounterAll":
		// OpExt ExtCounterAll (no operands)
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtCounterAll))
	case "Phases":
		// (Spectral Adversary) OpExt ExtPhases, Obj the permanent or target slot
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtPhases))
		if in.F != nil { // AllValid$: Obj RefNone, every battlefield permanent matching F
			if err := objFilter(kop.ZoneBattlefield); err != nil {
				return ki, err
			}
		}
	case "GrantExile":
		// (Q, Stacking the Deck) OpExt ExtGrantExile: every remembered exiled
		// card gains the latent activated ability C (table index)
		j, ok := cb.abMap[in.C]
		if !ok {
			return ki, fmt.Errorf("granted exile ability %d not emitted", in.C)
		}
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtGrantExile)), int16(j)
	case "GrantPanharm":
		// (Lwaxana Troi) OpExt ExtGrantPanharm on the target
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtGrantPanharm))
	case "LockActs":
		// (Braided Net) OpExt ExtLockActs on the target
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtLockActs))
	case "CloneCopy":
		// OpExt ExtCloneCopy; B.N the kop.Clone* flags, C the
		// copy exceptions' override index+1, F the Choices$ candidates (the
		// pick) or the Valid set of becomers
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtCloneCopy))
		if in.C > 0 {
			i, err := cb.override(cb.c.overrides[in.C-1])
			if err != nil {
				return ki, err
			}
			ki.C = int16(i + 1)
		}
		if in.F != nil {
			if err := objFilter(kop.ZoneBattlefield); err != nil {
				return ki, err
			}
		}
	case "AegisClone":
		// (Assimilation Aegis) OpExt ExtAegisClone
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtAegisClone))
	case "TwoPilesSac":
		// (Liliana of the Veil) OpExt ExtTwoPilesSac
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtTwoPilesSac))
		if err := objFilter(kop.ZoneBattlefield); err != nil {
			return ki, err
		}
	case "ExileEmbRem":
		// OpExt ExtExileEmbRem over the firing emblem's remembered set (lower_effectremember.go)
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtExileEmbRem))
	case "SacrificeDelayed":
		// OpExt ExtSacDelayed over the group's survivors
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtSacDelayed))
		if in.CS == "Many" {
			ki.C = 1 // the DpMany program's up to four objects (Mobilize 3-4)
		}
	case "PreventColor":
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtPreventColor)) // (Prismatic Strands)
	case "PreventDamageScope":
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtPreventDamageScope)), int16(in.C)
	case "Renown":
		// OpExt ExtRenown on Obj (the source), B the counter count (dirac/act_renown.go)
		v, err := cb.value(in.A)
		if err != nil {
			return ki, err
		}
		ki.Op, ki.A, ki.B = kop.OpExt, kop.Const(int(kop.ExtRenown)), v
	case "PreventNext":
		// OpExt ExtPreventNext on Obj, B the amount, C 1 for combat damage only (dirac/repl_prevnext.go)
		v, err := cb.value(in.A)
		if err != nil {
			return ki, err
		}
		ki.Op, ki.A, ki.B, ki.C = kop.OpExt, kop.Const(int(kop.ExtPreventNext)), v, int16(in.C&1)
	case "CounterPlusEOT":
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtCounterPlusEOT))
	case "TurnRepl":
		// OpExt ExtTurnRepl, C the kop.TurnRepl* kind (dirac/repl_turnrepl.go)
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtTurnRepl)), int16(in.C)
	case "Vote":
		// OpExt ExtVote, F the ballot (battlefield permanents), C bit 0
		// exile the most-voted
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtVote)), int16(in.C&1)
		if err := objFilter(kop.ZoneBattlefield); err != nil {
			return ki, err
		}
	case "ChoosePlayerRes":
		// OpExt ExtChoosePlayerRes, C bit 0 the Player.!IsRemembered pool
		// (act_chooseplayerres.go)
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtChoosePlayerRes)), int16(in.C&kop.ChoosePlayerFresh)
	case "DrawUpto":
		// OpExt ExtDrawUpto: Obj the drawer, B the most cards (act_drawupto.go)
		v, err := cb.value(in.A)
		if err != nil {
			return ki, err
		}
		ki.Op, ki.A, ki.B = kop.OpExt, kop.Const(int(kop.ExtDrawUpto)), v
	case "Recruit":
		// OpExt ExtRecruit. The token is a fixed corpus
		// script (the lowering registered it in lc.out.Tokens); the kernel
		// finds its table CardID by that name, so no matchup remap is
		// needed and every other generated pool stays byte-identical.
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtRecruit))
	case "SkipNextUntap":
		ki.Op = kop.OpSkipNextUntap
		if in.F != nil {
			// the sweep form (Sleep): every battlefield
			// permanent matching F, the player Obj's when Obj is set
			if err := objFilter(kop.ZoneBattlefield); err != nil {
				return ki, err
			}
		}
	case "AtkPWGuard":
		ki.Op = kop.OpAtkPWGuard
	case "LookAck":
		ki.Op = kop.OpLookAck
		if in.CS == "Library" {
			ki.C = 1 // the top A cards of the library (Mishra's Bauble)
		}
	case "ToLibraryPair":
		ki.Op = kop.OpToLibraryPair
	case "MayAsk":
		ki.Op, ki.C, ki.A = kop.OpMayAsk, int16(in.C), skip()
	case "AtEOT":
		ki.Op, ki.C = kop.OpAtEOT, int16(in.C)
	case "Sacrifice":
		ki.Op, ki.C = kop.OpSacrifice, int16(in.C) // in.C: kop.OptAsk
		if strings.HasPrefix(in.Note, noteRememberSacrificed) {
			ki.B = kop.Const(1 + remLKBits(in.Note)) // RememberSacrificed$; the LKI sum bits (value_remlki.go)
		}
		if err := objFilter(kop.ZoneBattlefield); err != nil {
			return ki, err
		}
	case "SacrificeAll":
		ki.Op = kop.OpSacrificeAll
		if err := remember3(in.CS, &ki); err != nil {
			return ki, err
		}
		if err := objFilter(kop.ZoneBattlefield); err != nil {
			return ki, err
		}
	case "Attach":
		ki.Op = kop.OpAttach
		switch in.CS {
		case "":
		case "Remember":
			ki.C = int16(kop.MvRemember) // RememberAttached$
		default:
			return ki, fmt.Errorf("Attach CS %q", in.CS)
		}
	case "TapOrUntap":
		ki.Op = kop.OpTapOrUntap
	case "TapOrUntapAll":
		// OpExt ExtTapOrUntapAll over the sweep filter (act_taporuntapall.go)
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtTapOrUntapAll))
		if err := objFilter(kop.ZoneBattlefield); err != nil {
			return ki, err
		}
	case "Explore":
		ki.Op = kop.OpExplore
	case "Poison":
		ki.Op = kop.OpPoison
	case "Blight":
		ki.Op = kop.OpBlight
		if err := objFilter(kop.ZoneBattlefield); err != nil {
			return ki, err
		}
	case "Extort":
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtExtort)) // dirac/act_extort.go
	case "Exploit":
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtExploit)) // dirac/kw_exploit.go
	case "Fog":
		ki.Op = kop.OpFog
	case "LoseGame":
		ki.Op, ki.C = kop.OpLoseGame, int16(in.C) // in.C 1: WinsGame (kernel winGame)
	case "Shuffle":
		ki.Op, ki.C = kop.OpShuffle, int16(in.C) // in.C 1: every remembered owner (lower_shuffleremowners.go)
	case "SetLife":
		ki.Op = kop.OpSetLife
	case "Rearrange":
		ki.Op, ki.C = kop.OpRearrange, int16(in.C)
	case "MoveCounter":
		ki.Op = kop.OpMoveCounter
		k, err := counterKind(in.CS)
		if err != nil {
			return ki, err
		}
		ki.B = kop.Const(int(k))
		r, err := cb.ref(in.Obj2)
		if err != nil {
			return ki, err
		}
		ki.C = int16(r)
	case "Discover":
		ki.Op, ki.C = kop.OpDiscover, int16(in.C)
	case "SkipUntap":
		ki.Op, ki.C = kop.OpSkipUntap, int16(in.C)
	case "AttachObj":
		ki.Op = kop.OpAttachObj
		r, err := cb.ref(in.Obj2)
		if err != nil {
			return ki, err
		}
		ki.C = int16(r)
		if in.Note == attachRememberNote {
			ki.B = kop.Const(1) // remember each object attached
		}
	case "ClearCounters":
		ki.Op = kop.OpClearCounters
	case "RemoveCounterAll":
		ki.Op = kop.OpRemoveCounterAll
		k, err := counterKind(in.CS)
		if err != nil {
			return ki, err
		}
		ki.C = int16(k)
		if err := objFilter(kop.ZoneBattlefield); err != nil {
			return ki, err
		}
	case "PutCountersFrom":
		ki.Op = kop.OpPutCountersFrom
		r, err := cb.ref(in.Obj2)
		if err != nil {
			return ki, err
		}
		ki.C = int16(r)
	case "Dig":
		ki.Op = kop.OpDigEx
		rest := zoneNames[fl["rest"]]
		df := int(zoneMap[rest])
		for k, b := range map[string]int{"Optional": kop.DigOptional, "RestRandomOrder": kop.DigRandomOrder, "Reveal": kop.DigReveal,
			"RememberChanged": kop.DigRemember, "ForceRevealToController": kop.DigRevealToCtrl, "Tapped": kop.DigTapped, "RestTop": kop.DigRestTop, "RestTopAsk": kop.DigRestTopAsk, "ExileFaceDown": kop.DigFaceDown, "ExilerOther": kop.DigExilerOther} {
			if _, ok := fl[k]; ok {
				df |= b
			}
		}
		if p, ok := fl["pos"]; ok {
			switch {
			case p == "0" && in.C == zLibrary:
				df |= kop.DigTakeToTop
			case p == "-1" && rest == zLibrary:
				// the rest to the bottom: the default
			default:
				return ki, fmt.Errorf("dig position %s", p)
			}
		}
		ki.C = int16(int(zoneMap[in.C]) | df<<3)
		if err := objFilter(kop.ZoneLibrary); err != nil {
			return ki, err
		}
	case "DigUntil":
		ki.Op = kop.OpDigUntil
		rev := zoneNames[fl["revealed"]]
		df := int(zoneMap[rev])
		if p, ok := fl["pos"]; ok {
			if p != "-1" {
				return ki, fmt.Errorf("dig-until position %s", p)
			}
			df |= kop.DuRestBottom
		}
		if _, ok := fl["RandomOrder"]; ok {
			df |= kop.DuRandomOrder
		}
		if _, ok := fl["FoundWithRevealed"]; ok {
			df |= kop.DuFoundWithRevealed
		}
		if _, ok := fl["Tapped"]; ok {
			df |= kop.DuFoundTapped
		}
		if _, ok := fl["Attacking"]; ok {
			df |= kop.DuFoundAttacking
		}
		if _, ok := fl["RememberFound"]; ok {
			df |= kop.DuRememberFound
		}
		if _, ok := fl["DugMark"]; ok {
			df |= kop.DuDugMark
		}
		if _, ok := fl["RememberRevealed"]; ok {
			df |= kop.DuRememberRevealed
		}
		if _, ok := fl["ImprintFound"]; ok {
			df |= kop.DuImprintFound
		}
		if _, ok := fl["MinTotalCMC"]; ok {
			df |= kop.DuMinTotalCMC
		}
		ki.C = int16(int(zoneMap[in.C]) | df<<3)
		if err := objFilter(kop.ZoneLibrary); err != nil {
			return ki, err
		}
	case "Peek":
		ki.Op = kop.OpPeek
		if _, ok := fl["MayReveal"]; ok {
			ki.C = int16(kop.PeekMayReveal)
			if err := objFilter(kop.ZoneLibrary); err != nil {
				return ki, err
			}
		}
		if _, ok := fl["Public"]; ok {
			ki.C = int16(kop.PeekPublic)
			if _, ok := fl["Remember"]; ok {
				ki.C |= int16(kop.PeekRemember)
			}
		}
	case "TwoPiles":
		ki.Op, ki.C = kop.OpTwoPiles, int16(zoneMap[in.C])
	case "ChooseCard":
		ki.Op = kop.OpChooseCard
		if err := objFilter(zoneMap[in.C]); err != nil {
			return ki, err
		}
		switch in.CS {
		case "Optional":
			ki.C = 1 // up to one: the ask carries a Done
		case "Remember":
			ki.C = 2 // RememberChosen$: the chosen card joins the remembered set
		case "LibraryUpTo":
			ki.C = 3 // up to B picks among the top A library cards
		case "AttachPick":
			ki.C = 4 // chooseAttachPick: the object-side Attach pick, mandatory
		case "AttachPickOpt":
			ki.C = 5 // chooseAttachPickOpt: the same pick, up to one
		case "AttachDest":
			ki.C = 10 // chooseAttachDest: the destination-side Attach pick over the remembered group (Fumble)
		case "PutCounterPick":
			ki.C = 6 // choosePutCounterPick
		case "FaceDownPick":
			ki.C = 7 // chooseFaceDownPick: the cloak-from-hand pick (gorge's face_down_choice)
		case "CopyPick":
			ki.C = 8 // chooseCopyPick: the CopyPermanent Choices$ pick (gorge's copypermanent_choice)
		case "PopulatePick":
			ki.C = 9 // choosePopulatePick: the Populate$ token pick, a lone candidate taken unasked
		}
	case "ChooseEachType":
		ki.Op = kop.OpChooseEachType
		if in.CS != "Artifact & Battle & Creature & Enchantment & Land & Planeswalker" {
			return ki, fmt.Errorf("ChooseEach %q", in.CS)
		}
	case "CastFree":
		// C: zone | AmountOne<<3 | ForgetPlayed<<4 | ReplaceGraveyard<<5 | CopyCard<<6 | Repeat<<7 | Paid<<8 | UpTo<<9
		// (Repeat: A is the copy count, AllowRepeats$ with Amount$ X)
		// Obj: RefRemembered (P1), a target slot (P2), or
		// RefNone with F (P3/P4 filter population).
		ki.Op = kop.OpCastFree
		ki.C = int16(zoneMap[in.C])
		if _, ok := fl["One"]; ok {
			ki.C |= 1 << 3
		}
		if _, ok := fl["Forget"]; ok {
			ki.C |= 1 << 4
		}
		if _, ok := fl["ReplaceGY"]; ok {
			ki.C |= 1 << 5
		}
		if _, ok := fl["Copy"]; ok {
			ki.C |= 1 << 6 // Play CopyCard$ True
		}
		if _, ok := fl["Repeat"]; ok {
			ki.C |= 1 << 7 // AllowRepeats$ True with Amount$ X: A copies offered
		}
		if _, ok := fl["Paid"]; ok {
			ki.C |= 1 << 8 // the copy pays its mana cost (an exile-granted ability's Play)
		}
		if _, ok := fl["UpTo"]; ok {
			ki.C |= 1 << 9 // a literal Amount$ N: the ask closes after A picks
		}
		if err := objFilter(zoneMap[in.C]); err != nil {
			return ki, err
		}
	case "MayPlay":
		ki.Op, ki.C = kop.OpMayPlay, int16(zoneMap[in.C])
		switch in.CS {
		case "EOT":
			ki.A = kop.Const(kop.DurEOT)
		case "EndOfYourNextTurn":
			ki.A = kop.Const(kop.DurYourNextTurn)
		case "WhileExiled":
			ki.A = kop.Const(kop.DurPermanent) // for as long as it remains exiled
		case "WhileSrc":
			ki.A = kop.Const(kop.DurWhileSrcMay) // for as long as the Effect's source is on the battlefield
		case "WhileValid":
			ki.A = kop.Const(kop.DurWhileValid) // while exiled, and while the controller controls a match of in.F
			if err := objFilter(kop.ZoneBattlefield); err != nil {
				return ki, err
			}
		default:
			return ki, fmt.Errorf("may-play duration %q", in.CS)
		}
	case "CreateEmblem":
		ki.Op, ki.C = kop.OpCreateEmblem, int16(in.C)
	case "RollDie":
		ki.Op, ki.C = kop.OpRollDie, int16(in.C)
	case "OverrideAll":
		ki.Op = kop.OpOverrideAll
		i, err := cb.override(cb.c.overrides[in.C])
		if err != nil {
			return ki, err
		}
		ki.C = int16(i)
		switch in.CS {
		case "EOT":
			ki.A = kop.Const(kop.DurEOT)
		case "Permanent":
			ki.A = kop.Const(kop.DurPermanent)
		case "UntilYourNextTurn":
			ki.A = kop.Const(kop.DurUntilYourNextTurn)
		default:
			return ki, fmt.Errorf("override duration %q", in.CS)
		}
		if err := objFilter(kop.ZoneBattlefield); err != nil {
			return ki, err
		}
	case "Override":
		ki.Op = kop.OpOverride
		i, err := cb.override(cb.c.overrides[in.C])
		if err != nil {
			return ki, err
		}
		ki.C = int16(i)
		if _, ok := fl["chosenColor"]; ok {
			// C names the FIRST of five records
			// differing only in SetColors (W, U, B, R, G in order); only
			// the first has an op, so intern the other four here. B marks
			// the op for the kernel's C + res.chosen - 1 record pick.
			for n := 1; n < 5; n++ {
				if _, err := cb.override(cb.c.overrides[in.C+n]); err != nil {
					return ki, err
				}
			}
			ki.B = kop.Const(kop.OvChosenColor)
			if err := cb.chosenColorOverridesOK(i); err != nil {
				return ki, err
			}
		}
		if _, ok := fl["enterWith"]; ok {
			ki.B = kop.Const(kop.OvEnterWith)
		}
		switch strings.SplitN(in.CS, "|", 2)[0] {
		case "EOT":
			ki.A = kop.Const(kop.DurEOT)
		case "Permanent":
			ki.A = kop.Const(kop.DurPermanent)
		case "UntilYourNextTurn":
			ki.A = kop.Const(kop.DurUntilYourNextTurn)
		case "WhileHost":
			ki.A = kop.Const(kop.DurPermanent) // Override.WhileHost bounds it by the host
		default:
			return ki, fmt.Errorf("override duration %q", in.CS)
		}
	case "CopyAsToken", "CopyNamed", "CopyPicked", "CopyExiledWith":
		ki.Op = kop.OpCopyAsTokenEx
		cf := 0
		if _, ok := fl["SacAtEndStep"]; ok {
			cf |= kop.CpSacAtEnd
		}
		if _, ok := fl["ExileAtEndStep"]; ok {
			cf |= kop.CpExileAtEnd
		}
		if _, ok := fl["SacAtEndDly"]; ok {
			cf |= kop.CpSacAtEndDly
		}
		if _, ok := fl["SacAtUpkeep"]; ok {
			cf |= kop.CpSacAtUpkeep
		}
		if _, ok := fl["Remember"]; ok {
			cf |= kop.CpRemember
		}
		if _, ok := fl["Tapped"]; ok {
			cf |= kop.CpTapped
		}
		if _, ok := fl["NonRemCtrl"]; ok {
			cf |= kop.CopyNonRemCtrl
		}
		if _, ok := fl["Attacking"]; ok {
			cf |= kop.CopyAttacking // ExtCopyPicked only (lower_copypicked.go)
		}
		pk, ok := fl["Pump"]
		pe, eot := fl["PumpEOT"]
		if eot {
			pk, ok = pe, true
			cf |= kop.CopyPumpEOT
		}
		if ok {
			// PumpKeywords$: a non-copy keyword grant; the kernel finds a
			// permanent one's keyword-only override record by its mask
			m, err := kwMask([]string{pk})
			if err != nil {
				return ki, err
			}
			if !eot {
				if _, err := cb.override(&lOverride{AddKw: []string{pk}, Dedup: true}); err != nil {
					return ki, err
				}
			}
			cf |= (bits.TrailingZeros32(uint32(m)) + 1) << kop.CopyPumpShift
		}
		ki.B = kop.Const(cf)
		if in.C > 0 {
			// the copy exceptions' override (CR 707.9b)
			i, err := cb.override(cb.c.overrides[in.C-1])
			if err != nil {
				return ki, err
			}
			ki.C = int16(i + 1)
		}
		if in.Op == "CopyPicked" {
			// kop.ExtCopyPicked: Obj the picked card, B and C as above
			ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtCopyPicked))
		}
		if in.Op == "CopyNamed" {
			// kop.ExtCopyNamed: B holds the named card's pool index (the
			// matchup remaps it to its CardID), C the flags and exception;
			// A held the count (one), now the sub-op
			idx, ok := cb.pt.tokenIdx[copyCardKey+fl["card"]]
			if !ok {
				return ki, fmt.Errorf("copied card %q", fl["card"])
			}
			if cf := int(uint16(ki.B.N)); cf>>8 != 0 || int(ki.C) > 0xff {
				return ki, fmt.Errorf("named copy flags %#x or exception override %d past a byte", cf, ki.C)
			}
			ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtCopyNamed))
			ki.B, ki.C = kop.Const(idx), int16(int(ki.C)<<kop.CopyNamedExcShift|int(uint16(ki.B.N)))
		}
		if in.Op == "CopyExiledWith" {
			// kop.ExtCopyExiledWith: F the exiled-with set, B and C as above
			ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtCopyExiledWith))
			if err := objFilter(kop.ZoneExile); err != nil {
				return ki, err
			}
		}
	case "CopyAsTokenCtr":
		// dirac/copy_counters.go: Obj the source, B the
		// amount (set above), C the kind (not the copy-exception index: kgen
		// refuses an exception beside the rider)
		k, err := counterKind(in.CS)
		if err != nil {
			return ki, err
		}
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtCopyTokenCtr)), int16(k)
	case "BecomeCopy":
		ki.Op = kop.OpBecomeCopy
		if err := objFilter(kop.ZoneBattlefield); err != nil {
			return ki, err
		}
		if in.B.Kind != vNone {
			if !isConstN(in.B, int(kop.BecomeCopyCtr)) {
				return ki, fmt.Errorf("BecomeCopy B %v", in.B)
			}
			ki.B = kop.Const(int(kop.BecomeCopyCtr))
		}
		ki.C = int16(in.C & 1)
		if x := in.C >> 1; x > 0 {
			i, err := cb.override(cb.c.overrides[x-1])
			if err != nil {
				return ki, err
			}
			if i+1 > 0x3fff {
				return ki, fmt.Errorf("copy exception override %d", i)
			}
			ki.C |= int16(i+1) << 1
		}
		ki.A = kop.ValueRef{}
		if in.A.Kind != vNone {
			// the AddAbilities$ grant (lower_cloneentergrant.go)
			if !in.A.isConst() || in.A.N < 1 || in.A.N > len(cb.c.overrides) {
				return ki, fmt.Errorf("BecomeCopy A %v", in.A)
			}
			i, err := cb.override(cb.c.overrides[in.A.N-1])
			if err != nil {
				return ki, err
			}
			ki.A = kop.Const(i + 1)
		}
	case "Connive":
		ki.Op = kop.OpConnive
	case "ConniveCounters":
		ki.Op = kop.OpConniveCounters
	case "GainControlEOT":
		if in.CS != "Untap|kw=Haste" {
			return ki, fmt.Errorf("gain control riders %q", in.CS)
		}
		ki.Op = kop.OpGainControlEOT
	case "GainControl":
		ki.Op = kop.OpGainControl
		var rid int
		for _, f := range strings.Split(in.CS, "|") {
			switch {
			case f == "Untap":
				rid |= kop.GcUntap
			case f == "kw=Haste" && fl["dur"] == "EOT":
				rid |= kop.GcHasteEOT
			case f == "Remember":
				rid |= kop.GcRemember
			case strings.HasPrefix(f, "dur="):
			default:
				return ki, fmt.Errorf("gain control rider %q", f)
			}
		}
		ki.B = kop.Const(rid)
		switch fl["dur"] {
		case "EOT":
			ki.A = kop.Const(kop.DurEOT)
		case "Permanent":
			ki.A = kop.Const(kop.DurPermanent)
		case "WhileSource":
			ki.A = kop.Const(kop.DurWhileSource)
		case "WhileControlSource":
			ki.A = kop.Const(kop.DurWhileControlSource)
		case "YourNextTurn":
			ki.A = kop.Const(kop.DurYourNextTurn)
		default:
			return ki, fmt.Errorf("gain control duration %q", fl["dur"])
		}
		if in.C != 0 {
			r, err := cb.ref(lRef{Kind: refKind(in.C >> 8), N: in.C & 0xFF})
			if err != nil {
				return ki, err
			}
			if _, isTgt := tgtSlotOfRef(r); r != kop.RefOpponent && r != kop.RefEachOpponent && !isTgt {
				return ki, fmt.Errorf("gain control new controller ref %d", r)
			}
			ki.C = int16(r)
		}
	case "DelayLeaveReturn":
		// OpDelayedTrigger C = the kernel's internal leave-return act
		// (dirac/trig_leavereturn.go opLeaveReturn)
		ki.Op, ki.C = kop.OpDelayedTrigger, kernelOpLeaveReturn
	case "DelayedTrigger", "ReflexiveTrigger":
		ki.Op = kop.OpDelayedProgram
		if in.Op == "ReflexiveTrigger" {
			ki.Op = kop.OpReflexiveTrigger
		} else {
			switch in.CS {
			case "NextEndStep":
			case "YourNextEndStep":
				ki.A = kop.Const(kop.DelayYourEndStep) // the controller's next end step (dirac/trig_delayedtiming.go)
			case "NextUpkeep":
				ki.A = kop.Const(kop.DelayUpkeep)
			case "YourNextUpkeep":
				ki.A = kop.Const(kop.DelayYourUpkeep)
			case "NextEndCombat":
				ki.A = kop.Const(kop.DelayEndCombat)
			case "NextUpkeepOfRemPlayer":
				ki.A = kop.Const(kop.DelayRemUpkeep) // dirac/trig_delayremupkeep.go
			case "NextUpkeepOfPlayer":
				ki.A = kop.Const(kop.DelayPlayerUpkeep) // dirac/trig_delayplayerupkeep.go
			case "NextBeginCombat":
				ki.A = kop.Const(kop.DelayBeginCombat) // dirac/trig_delaybegincombat.go
			case "NextSpellCast":
				ki.A = kop.Const(kop.DelaySpellCast)
			case "NextSpellCastUncounter":
				ki.A = kop.Const(kop.DelaySpellUncounter) // dirac/repl_uncounterable.go
			case "DiesThisTurn":
				ki.A = kop.Const(kop.DelayDiesThisTurn) // dirac/trig_delayeddies.go
			default:
				return ki, fmt.Errorf("delayed trigger %q", in.CS)
			}
		}
		if in.Op == "DelayedTrigger" && in.B.Kind == vConst && in.B.N == 1 {
			ki.B = kop.Const(kop.DpGroup) // Mobilize's group program (dirac/kw_mobilize.go)
		}
		if in.Op == "DelayedTrigger" && in.B.Kind == vConst && in.B.N == 3 {
			ki.B = kop.Const(kop.DpMany) // a many-object remembered set (dirac/trig_delayedmany.go)
		}
		if in.Op == "DelayedTrigger" && in.B.Kind == vConst && in.B.N == 2 {
			ki.B = kop.Const(kop.DpTrack) // Decayed's incarnation-tracked promise (dirac/kw_decayed.go)
		}
		j, ok := cb.abMap[in.C]
		if !ok {
			return ki, fmt.Errorf("latent ability %d not emitted", in.C)
		}
		ki.C = int16(j)
	case "ChooseTargets":
		ki.Op = kop.OpChooseTargets
		ki.C = int16(in.C)
	case "DigBottom":
		// the DigUntil's cards still in exile to the library bottom in a
		// random order (lower_digimprint.go; kernel act_discoverbottom.go)
		ki.Op, ki.Obj, ki.C = kop.OpDiscover, kop.RefController, kop.DiscoverDugBottom
	case "RemBottom":
		// the remembered cards still in the library to its bottom in a
		// random order (lower_playlibrary.go; kernel act_discoverbottom.go)
		ki.Op, ki.Obj, ki.C = kop.OpDiscover, kop.RefController, kop.DiscoverRemBottom
	case "ClearRemembered":
		ki.Op = kop.OpClearRemembered
	case "ClearChosen":
		ki.Op = kop.OpClearChosen
	case "ClearImprinted":
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtClearImprinted))
	case "DigImprint":
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtDigImprint)) // marks the Dig before it (zone_digimprint.go)
	case "RememberSearcher":
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtRememberSearcher)) // zone_remembersearcher.go
	case "RememberAll":
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtRememberAll)) // lower_rememberwritersmass.go
		if in.CS == "ObjOrder" {
			ki.C = kop.RememberAllObjOrder // lower_attachgroup.go
		}
		if err := objFilter(kop.ZoneBattlefield); err != nil {
			return ki, err
		}
	case "ExtJoinForces":
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtJoinForces)), int16(in.C) // lower_storesvar.go
	case "RememberImprinted":
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtRememberImprinted)) // Tainted Pact
	case "AddTurn":
		ki.Op = kop.OpAddTurn
	case "EndTurn":
		ki.Op = kop.OpEndTurn
	case "AddMana":
		ki.Op, ki.C = kop.OpAddMana, int16(colorMask(in.CS))
		if isConstN(in.B, 1) {
			ki.B = kop.Const(1) // firebending: the red units are tallied in State.fbR (kw_firebend.go)
		} else if isConstN(in.B, 2) {
			ki.B = kop.Const(2) // PersistentMana$ True: tallied in State.pm (mana_persistent.go)
		} else if isConstN(in.B, 3) {
			ki.B = kop.Const(3) // Effect UnspentMana Red: no units, keeps the controller's red (dirac/value_fightexcess.go)
		}
		if in.F != nil {
			// a loyalty ability's spend restriction: the units float as
			// restricted mana (mana_resolverestriction.go)
			if ki.F, err = cb.filter(in.F, kop.ZoneStack); err != nil {
				return ki, err
			}
		}
	case "ChooseColor":
		ki.Op, ki.C = kop.OpChooseColor, int16(in.C) // in.C: the Exclude$ colours (W U B R G = bits 0..4)
	case "ChooseType":
		ki.Op, ki.C = kop.OpChooseType, int16(in.C) // C: a card-type category's offered types (lower_choosetype.go)
	case "ChooseName":
		buildChooseName(&ki, in.B.N) // an as-enters card name (lower_choosename.go)
	case "ChooseCardEnter":
		buildChooseCardEnter(&ki) // an as-enters object choice (lower_choosecardenter.go)
		if err := objFilter(kop.ZoneBattlefield); err != nil {
			return ki, err
		}
	case "ChoosePlayerEnter":
		// an as-enters opponent (lower_chooseplayerenter.go)
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtChoosePlayer))
	case "ChoosePlayerMid":
		// a mid-resolution opponent choice (lower_chooseplayermid.go)
		ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtChoosePlayerMid))
	case "ChooseNumber":
		// an as-enters number from B to C (lower_choosenumber.go)
		ki.Op, ki.A, ki.B, ki.C = kop.OpExt, kop.Const(int(kop.ExtChooseNumber)), kop.Const(in.B.N), int16(in.C)
	case "ChooseNumberMid":
		// a resolution-time number (lower_choosenumber.go): B the Max$
		// value, C the Min$ floor plus the has-Max / Random flags
		ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtChooseNumberMid)), int16(in.C)
	default:
		return ki, fmt.Errorf("op %s has no kop form", in.Op)
	}
	if in.F != nil && ki.F == 0 {
		return ki, fmt.Errorf("filter on %s not emitted", in.Op)
	}
	_ = strconv.Itoa
	return ki, nil
}

// selfCastClause lowers a SpellCast "Card.Self+<property>" trigger object
// (kfix-cast; Sowing Mycospawn's "Card.Self+kicked", Approach of the
// Second Sun's "Card.Self+named<itself>") to the self cast flag (TfSelf,
// fired from the stack by the kernel's fireSelfCast) plus the remaining
// properties as the Filter, which fireSelfCast reads against the spell.
// The spell naming itself is always true for its own cast trigger, so it
// drops. Any other property beside Self is refused: fireSelfCast
// matches the kick and sneak flags.
func selfCastClause(f *lFilter) (bool, error) {
	self := false
	for i := range f.Clauses {
		c := &f.Clauses[i]
		if c.Flags&fSelf == 0 {
			continue
		}
		if len(f.Clauses) != 1 {
			return false, fmt.Errorf("SpellCast Card.Self+property inside an alternative")
		}
		rest := *c
		rest.Flags &^= fSelf | fNamedAsSource
		if rest.Types == tCreature {
			rest.Types = 0
		}
		if rest != (lClause{Flags: rest.Flags}) || rest.Flags&^(fKicked|fSneaked) != 0 {
			return false, fmt.Errorf("SpellCast Card.Self+property beyond kicked or sneaked")
		}
		self = true
		if rest.Flags == 0 {
			f.Clauses = nil
		} else {
			f.Clauses[0] = rest
		}
	}
	return self, nil
}

// altSacOK: a sacrifice-only alternative cost (Fireblast's "sacrifice two
// Mountains rather than pay this spell's mana cost") is modelled for an
// instant or sorcery whose spell ability has no cost of its own beyond
// mana and no either-or additional cost: the alternative cast's pick parts
// are then the static's alone (kernel prepAltFree, buildPickCost).
func (cb *cardBuilder) altSacOK() bool {
	c := cb.c
	if c.AltAddCost != nil {
		return false
	}
	if kop.TypeMask(c.Types)&(kop.TInstant|kop.TSorcery) == 0 {
		// a creature spell with no spell ability of its own (Hand of
		// Emrakul): the alternative cast's pick parts are the static's alone
		// as well, and no target is asked
		if kop.TypeMask(c.Types)&kop.TCreature == 0 {
			return false
		}
		for _, a := range c.Abilities {
			if a.Kind == akSpell {
				return false
			}
		}
		return true
	}
	n := 0
	for _, a := range c.Abilities {
		if a.Kind == akSpell {
			n++
			if !a.Cost.pureMana() {
				return false
			}
		}
	}
	return n == 1
}

// originMask is OpReturn's B.N: the zones of an Origin$ list, as 1<<kop.Zone bits;
// 0 for a move naming none.
func originMask(from string) int {
	m := 0
	if from != "" {
		for _, z := range strings.Split(from, ",") {
			for i, l := range zoneLabel {
				if l == z {
					m |= 1 << zoneMap[i]
				}
			}
		}
	}
	return m
}
