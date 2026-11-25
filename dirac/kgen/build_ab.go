package kgen

import (
	"fmt"
	"slices"
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
		ka, add, err := cb.staticFor(s)
		if err == nil {
			err = cb.grantStatic(&ka, s) // lower_grantedstatic.go
		}
		if err != nil {
			return bfail(cb.node, "%v", err)
		}
		d.Abilities = append(d.Abilities, ka)
		cb.out.stAddSub = append(cb.out.stAddSub, add)
		cb.out.origins = append(cb.out.origins, 0xFF)
	}
	for i, s := range c.GrantedStatics {
		cb.node = fmt.Sprintf("granted static[%d] %s", i, s.Kind)
		ka, _, err := cb.staticIn(s)
		if err != nil {
			return bfail(cb.node, "%v", err)
		}
		d.Granted = append(d.Granted, ka)
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
		if e.Forget != nil {
			f, ferr := cb.filter(e.Forget, kop.ZoneStack)
			if ferr != nil {
				return bfail(cb.node, "%v", ferr)
			}
			ke.Forget = f
		}
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
			ka, add, err := cb.staticFor(s)
			if err != nil {
				return bfail(cb.node, "%v", err)
			}
			if len(add) > 0 {
				return bfail(cb.node, "emblem static grants subtypes")
			}
			ke.Abilities = append(ke.Abilities, ka)
		}
		for _, r := range e.Repls {
			ka, err := cb.repl(r)
			if err != nil {
				return bfail(cb.node, "%v", err)
			}
			ke.Abilities = append(ke.Abilities, ka)
		}
		if len(e.Statics) == 0 && len(e.Repls) == 0 {
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
		case f.Players == plBoth && len(f.Clauses) == 1 && anyKeptObjectClause(f.Clauses[0]):
			// "target player or planeswalker" (Boros Charm, Viashino
			// Pyromancer) and Any.wasDealtDamageThisTurn (Needle Drop): the
			// "any target" walk with the object clause as the filter (see
			// anyKeptObjectClause).
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
		case f.Players == plBoth && anyOtherTarget(f):
			// "any other target" (Matoc, Lavamancer; Black Panther, Red
			// Hulk, the Invasions of Tarkir and Ulgrotha): the "any target"
			// walk with the object half not the source (gorge's .Other)
			ts.Kind = kop.TgtAny
			g := lFilter{Clauses: []lClause{{Flags: fOther}}}
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
	case !t.AtRes && !t.Unique && !t.SameCtrl && !t.DiffCtrl && !t.SharedCrType && !t.DistinctName && (t.Divided == 0 || t.Divided == kop.DivMaxV || t.Divided == kop.DivValue) && t.ParentPlus1 == 0 &&
		tgtBoundV(t.Min, true) && tgtBoundV(t.Max, false):
		// "up to" a value (TargetMax$ X over a count, X paid or the trigger
		// amount): evaluated as the slot is announced (gorge resolvedTargetBounds);
		// a mode's slot (Thraben Charm) is read when its mode is chosen (TgtModes).
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
	case vCountersOn:
		// the source's own counters (Vile Requiem: up to X targets, X the verse
		// counters on CARDNAME): gorge's bound context anchors Source at the
		// ability's object, the kernel's entry context reads it the same way
		return v.Ref.Kind == rSelf && v.S != "ALL"
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
	ka.SplitPrev = a.SplitPrev
	ka.FromAnyGrave = a.FromAnyGrave
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
		if a.Kind == akMana && exileGraveManaCostRefused(ka.Cost) {
			// gorge pays these through the mana-activation walk
			// (AskManaForage); the kernel's mana abilities have no cost picks
			return ka, fmt.Errorf("mana ability with a forage / exile-from-graveyard cost")
		}
		if a.Sorcery && !a.Loyalty {
			ka.Cost.Flags |= kop.CostSorcery
		}
		ka.CmdOnly = a.CmdOnly
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
		if err := trigWhoseScope(ka.Trig.Whose, a); err != nil {
			return ka, err
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
	if a.GenericChoice {
		if a.Kind != akTriggered || a.Modes == 0 {
			return ka, fmt.Errorf("GenericChoice mark on a non-trigger charm")
		}
		ka.AFlags |= kop.AfCharmTrigger
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

// manaReflected lowers a Special "Reflected" mana read (lower_reflectedmana.go).
// It reports whether m is one it lowered. A production read with a
// non-constant Amount$ (Incubation Druid), or one mana of any colour among
// the matched permanents' colours (Plaza of Heroes), or among what the
// matching permanents' mana abilities could produce (Ability.ManaProduce,
// every Valid$ alternative OppCtrl: Exotic Orchard). A Graveyard read (The
// Grey Havens) scans every alive seat's graveyard instead (Ability.ManaGrave;
// its spec compiles zone-free like the Count$ValidGraveyard precedent — the
// read is the zone).
func (cb *cardBuilder) manaReflected(m *lMana, ka *kop.Ability) (bool, error) {
	if m.Special != "Reflected" || m.Restrict != nil || m.RestrictAb != nil {
		return false, nil
	}
	if m.Produce && !m.Amount.isConst() {
		// (Incubation Druid) the production read with a non-constant Amount$
		// ("one mana ... add three of that type instead"): gorge's planner
		// defers the amount (pay.PaymentPlanReflectedShape rejects
		// mr.Amount.Present), so its census leaves the source a gap
		// (censusGapMana's reflected Amount$ shape, dirac mana_reflamt.go)
		// and the fold prices the amount open; the prefix search evaluates it
		// at the activation (kop.AfManaDynamic + ManaV). One mana of any
		// colour among the matched permanents' cards' producible colours
		// (Ability.ManaProduce; Ability.ManaProduceType admits {C} under
		// ColorOrType$ Type), scaled by the Amount$ expression evaluated with
		// the source
		id, err := cb.filter(m.SpecialF, kop.ZoneBattlefield)
		if err != nil {
			return true, err
		}
		v, err := cb.value(m.Amount)
		if err != nil {
			return true, err
		}
		ka.AFlags |= kop.AfManaChoice | kop.AfManaReflect | kop.AfManaDynamic
		ka.ManaProduce, ka.ManaProduceType = true, m.WidenType
		ka.ManaAmong = id
		ka.ManaColors = kop.ColAny
		ka.ManaN = 1
		ka.ManaV = v
		return true, nil
	}
	if !m.Amount.isConst() || m.Amount.N != 1 {
		return false, nil
	}
	zone := kop.ZoneBattlefield
	if m.Graveyard {
		zone = kop.ZoneNone
	}
	id, err := cb.filter(m.SpecialF, zone)
	if err != nil {
		return true, err
	}
	ka.AFlags |= kop.AfManaChoice | kop.AfManaReflect
	if m.Produce {
		ka.ManaProduce, ka.ManaProduceType = true, m.WidenType
	}
	ka.ManaGrave = m.Graveyard
	ka.ManaAmong = id
	ka.ManaColors = kop.ColAny
	ka.ManaN = 1
	return true, nil
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
	done, err := cb.manaReflected(m, ka)
	if err != nil || done {
		return err
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
	ka.ManaAskColour = m.AskColour
	ka.ManaNoCounter = m.NoCounter
	ka.ManaIdentityPotential = m.IdentityPotential
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
		ka.AFlags |= m.dynamicFlag() // lower_askcolourmana.go
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
		id, err := cb.manaRestrictAb(m)
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

// manaRestrictAb builds m's activated half: a battlefield-only clause keeps
// ZoneNone, a zone-free one carries kop.ZoneAny (kernel restrActMatch's
// source off the battlefield; lowerCtx.restrictAbZones).
func (cb *cardBuilder) manaRestrictAb(m *lMana) (kop.FilterID, error) {
	if len(m.RestrictAbZones) != len(m.RestrictAb.Clauses) {
		return 0, fmt.Errorf("mana restriction's activated half: %d zones for %d clauses", len(m.RestrictAbZones), len(m.RestrictAb.Clauses))
	}
	return cb.filterZones(m.RestrictAb, m.RestrictAbZones)
}

var stepMap = map[string]kop.Step{"Upkeep": kop.StepUpkeep, "End": kop.StepEnd, "BeginCombat": kop.StepBeginCombat,
	"Draw": kop.StepDraw, "Main1": kop.StepMain1, "Main2": kop.StepMain2, "EndCombat": kop.StepEndCombat}

var eventMap = map[string]kop.TrigEvent{"ETB": kop.TrigETB, "Dies": kop.TrigDies, "Leaves": kop.TrigLeaves, "ToGraveyard": kop.TrigToGraveyard, "Blocks": kop.TrigBlocks, "Taps": kop.TrigTaps, "Attacks": kop.TrigAttacks,
	"AttackersDeclared": kop.TrigAttackersDeclared, "Phase": kop.TrigPhase, "SpellCast": kop.TrigSpellCast,
	"LifeGained": kop.TrigLifeGained, "LifeLost": kop.TrigLifeLost, "DamageDone": kop.TrigDamageDone,
	"DamageDealtOnce": kop.TrigDamageDealtOnce, "Drawn": kop.TrigDrawn, "Discarded": kop.TrigDiscarded, "DiscardedAll": kop.TrigDiscardedAll, "AbilityCast": kop.TrigAbilityCast, "Scry": kop.TrigScry, "Surveil": kop.TrigSurveil, "Explores": kop.TrigExplores, "CollectEvidence": kop.TrigCollectEvidence, "Exploited": kop.TrigExploited,
	"LandPlayed": kop.TrigLandPlayed, "SearchedLibrary": kop.TrigSearchedLibrary, "Forage": kop.TrigForage, "Discover": kop.TrigDiscover, "ManifestDread": kop.TrigManifestDread,
	"CounterAddedOnce": kop.TrigCounterAddedOnce, "CounterAdded": kop.TrigCounterAddedOnce, "CounterAddedAll": kop.TrigCounterAddedAll, "Untaps": kop.TrigUntaps,
	"CounterRemoved": kop.TrigCounterRemoved, "CounterRemovedOnce": kop.TrigCounterRemovedOnce, // the card's own (lower_counterremoved.go; dirac/trig_counterremoved.go)
	"Sacrificed": kop.TrigSacrificed, "GiveGift": kop.TrigGiveGift, "Proliferate": kop.TrigProliferate, "FacesDilemma": kop.TrigFacesDilemma, "BecomeMonarch": kop.TrigBecomeMonarch, "TokenCreated": kop.TrigTokenCreated, "BecomeMonstrous": kop.TrigBecomeMonstrous, "ManaExpend": kop.TrigManaExpend, "AttackerBlocked": kop.TrigBecomesBlocked, "TurnFaceUp": kop.TrigTurnFaceUp, "Transformed": kop.TrigTransformed, "CommitCrime": kop.TrigCommitCrime, "UnlockDoor": kop.TrigUnlockDoor, "FullyUnlock": kop.TrigFullyUnlock, "RingTemptsYou": kop.TrigRingTemptsYou,
	"BecomesTarget":    kop.TrigBecomesTarget,    // the Valiant, watcher and self shapes (lower_trigger.go)
	"ClassLevelGained": kop.TrigCounterAddedOnce, // N = CtrLevel (lower_class.go)
	"BecomesPlotted":   kop.TrigBecomesPlotted,   // the card's own
	"BecomesSaddled":   kop.TrigBecomesSaddled,
	"Evolved":          kop.TrigEvolved,    // the source's own, after its evolve counter lands
	"Attached":         kop.TrigAttached,   // the attachment's own, Creature bearer (Assimilation Aegis)
	"Unattached":       kop.TrigUnattached, // the attachment's own, the FORMER BEARER (Captain's Hook family)
	"LeaveExile":       kop.TrigLeaveExile} // an imprinted card leaves exile (lower_changezonepairs.go; dirac/trig_leaveexile.go)

// trigQualifierFlags is trigger's qualifier-flag block: the Evolve, the
// SpellCast spend qualifiers (Increment, the mana-spend gate) and the split
// dies half's Tf2ToGraveHalf mark, each refused beside an incompatible event.
func (cb *cardBuilder) trigQualifierFlags(kt *kop.Trigger, t *lTrigger) error {
	if t.Evolve {
		if t.Event != "ETB" {
			return fmt.Errorf("Evolve on a %s trigger", t.Event)
		}
		kt.Flags2 |= kop.Tf2Evolve
	}
	if t.Increment {
		if t.Event != "SpellCast" || t.OrCopy || t.CastSnap || t.CastSpent || t.NthSpell != 0 || t.TargetsAll {
			return fmt.Errorf("Increment on a %s trigger", t.Event)
		}
		kt.Flags2 |= kop.Tf2Increment
	}
	if t.ToGraveHalf {
		if t.Event != "Dies" || t.Batch {
			return fmt.Errorf("ToGraveHalf on a %s trigger", t.Event)
		}
		kt.Flags2 |= kop.Tf2ToGraveHalf
	}
	if t.SpentGate != 0 {
		// Trigger.N is read on SpellCast only under
		// Tf2NthSpell (castTrigGen), so the gate rides in N with no flag.
		if t.Event != "SpellCast" || t.OrCopy || t.CastSnap || t.CastSpent || t.NthSpell != 0 || t.Increment {
			return fmt.Errorf("spend gate on a %s trigger", t.Event)
		}
		kt.N = t.SpentGate
	}
	return nil
}

// playerTriggerFilter builds a BecomesTarget player alternative: an FPlayer
// chain entry the kernel's player walk matches with castTargetsPlayer,
// beside the battlefield object clauses.
func (cb *cardBuilder) playerTriggerFilter(f *lFilter, t *lTrigger, self bool) (kop.FilterID, error) {
	if t.Event != "BecomesTarget" || !t.PlayerAlt || self {
		return 0, fmt.Errorf("player filter on a trigger object")
	}
	zones := make([]kop.Zone, len(f.Clauses))
	for i := range zones {
		zones[i] = kop.ZoneBattlefield
	}
	return cb.filterPlayerAlt(f, zones, f.Players)
}

// counterTriggerN returns the counter kind carried in Trigger.N for the
// CounterAdded and CounterRemoved trigger families: every kind for the "*"
// CounterAddedOnce form, else the named kind.
func counterTriggerN(t *lTrigger) (int16, bool, error) {
	switch t.Event {
	case "CounterAddedOnce", "CounterAdded":
		if t.Step == "*" {
			return kop.TrigNAnyCounter, true, nil
		}
	case "CounterAddedAll", "CounterRemoved", "CounterRemovedOnce":
	default:
		return 0, false, nil
	}
	k, err := counterKind(t.Step)
	if err != nil {
		return 0, false, err
	}
	return int16(k), true, nil
}

// batchEventAllowed reports the trigger events a "one or more" (Tf2Batch)
// line may name: the zone/entry batches and the effect-action batches.
func batchEventAllowed(ev string) bool {
	switch ev {
	case "ETB", "Dies", "Leaves", "ToGraveyard", "DamageDone", "Sacrificed", "TokenCreated", "Taps":
		return true
	}
	return false
}

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
		self, selfCreature := splitSelfClause(t.Event, &f)
		selfCast := false
		if !self && t.Event == "SpellCast" {
			sc, err := selfCastClause(&f)
			if err != nil {
				return kt, err
			}
			if sc {
				kt.Flags |= kop.TfSelf
				self = false // the rest is the spell's Filter, not an alternative
				selfCast = true
			}
		}
		if f.Players != 0 {
			kt.Filter, err = cb.playerTriggerFilter(&f, t, self)
			if err != nil {
				return kt, err
			}
		} else {
			switch {
			case self && len(f.Clauses) == 0:
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
				if len(f.Clauses) > 0 {
					zone := kop.ZoneBattlefield
					if t.Event == "SpellCast" {
						zone = kop.ZoneStack
						if t.NthFiltered {
							zone = kop.ZoneNone // it also tests the earlier pushes, wherever they now live
						}
					}
					if t.Event == "Discarded" || t.Event == "ToGraveyard" {
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
		case t.Event == "DamageDone" && t.F2.Players == plBoth && len(t.F2.Clauses) == 1 && dmgPlayerAltClause(t.F2.Clauses[0]):
			// ValidTarget$ Player,Planeswalker or Player,Battle: TfToPlayer beside a
			// Filter2, "a player or a matching permanent" (trig_damagedonce.go ddoRecipient)
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
	if n, ok, err := counterTriggerN(t); err != nil {
		return kt, err
	} else if ok {
		kt.N = n
	}
	if t.MinAttackers > 0 {
		kt.N = trigCmpN(t.AtkCmp, t.MinAttackers)
	}
	if t.Event == "AbilityCast" {
		kt.N = t.AbClass
	}
	if t.Event == "LandPlayed" || t.Event == "Leaves" || t.Event == "BecomeMonarch" || t.Event == "ToGraveyard" {
		kt.N = t.N // Leaves: the destination mask; BecomeMonarch: kop.TrigNMonarchBeganTurn; ToGraveyard: the origin mask
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
		if t.First && !selfPlain {
			// the Tf2FirstTime latch (trig_counterfirst.go) is keyed on the
			// trigger object; only the ungated self line queues on every
			// targeting gorge's per-object count sees (lower_trigger.go)
			return kt, fmt.Errorf("FirstTime on a gated BecomesTarget")
		}
		if !selfPlain && !(t.Watch && (kt.Whose == kop.WhoseOpp || kt.Whose == kop.WhoseAny || (t.SrcSpell || t.SrcAbility) && kt.Whose == kop.WhoseYou)) {
			return kt, fmt.Errorf("BecomesTarget without Valiant")
		}
		if t.SrcSpell {
			if !t.Watch {
				return kt, fmt.Errorf("spell source on a non-watcher BecomesTarget")
			}
			kt.N = kop.TrigNSrcSpell
			if t.SrcF != nil {
				if kt.Filter2, err = cb.filter(t.SrcF, kop.ZoneStack); err != nil {
					return kt, err
				}
			}
		}
		if err := btSrcN(t, &kt); err != nil {
			return kt, err
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
	if t.TapsForMana && (t.Event != "Taps" || kt.Flags&kop.TfSelf != 0 && kop.TapPlan(t.TapPlan) != kop.TapPlanDefer) {
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
		if t.Attacked&atkEnch != 0 {
			kt.Flags4 |= kop.Tf4DefEnch // fireAtkGroup's Attacks arm (trig_attacksenchanted.go)
		}
	}
	if t.PerCounter {
		if t.Event != "CounterAdded" && t.Event != "CounterRemoved" {
			return kt, fmt.Errorf("PerCounter on a %s trigger", t.Event)
		}
		kt.Flags4 |= kop.Tf4PerCounter
	}
	if err := cb.trigQualifierFlags(&kt, t); err != nil {
		return kt, err
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
		if !batchEventAllowed(t.Event) {
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
	if t.InstGate != 0 {
		// AtLeastOneInstance$ (lower_atleastone.go): a once-per-source
		// DamageDealtOnce line, whose Trigger.N nothing else reads
		if t.Event != "DamageDealtOnce" || t.PerRef || t.F2 != nil {
			return kt, fmt.Errorf("AtLeastOneInstance gate on a %s trigger", t.Event)
		}
		kt.N = t.InstGate
	}
	if t.PerRef {
		if t.Event != "DamageDone" || !t.Batch {
			return kt, fmt.Errorf("per-referent flag on a %s trigger", t.Event)
		}
		kt.Flags2 |= kop.Tf2PerReferent
		kt.N = t.DmgGate
	} else if t.DmgGate != 0 {
		// a plain DamageDone line's per-event DamageAmount$ gate, read by
		// ddoLine for the lines that read their recipient (lower_damageamount.go)
		if t.Event != "DamageDone" || kt.Filter2 == 0 && kt.Flags2&kop.Tf2ToYou == 0 {
			return kt, fmt.Errorf("damage-amount gate on a %s trigger without a recipient read", t.Event)
		}
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
		// the ordering ask; kgen already proved the body ask-free (the
		// SpellCast arm's rider shapes, or the eventless-body gate every
		// other mode lowers through)
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
	if t.CauseSpell != 0 {
		if t.Event != "LifeLost" && t.Event != "LifeLostAll" && t.Event != "TurnFaceUp" {
			return kt, fmt.Errorf("cause-spell flag on a %s trigger", t.Event)
		}
		kt.Flags4 |= kop.Tf4CauseSpellAb
		switch t.CauseSpell {
		case 2:
			kt.N = kop.TrigNCauseSpellYouCtrl
		case 3:
			kt.N = kop.TrigNCauseSpellOppCtrl
		}
	}
	if t.CounterCell {
		kt.Flags4 |= kop.Tf4CounterCell
	}
	if t.OptTrigPlayer {
		if !t.Optional || (t.Event != "Phase" && t.Event != "AttackersDeclared" && t.Event != "Dies") {
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
		if t.Event != "BecomesPlotted" && t.Event != "CounterRemoved" && t.Event != "CounterRemovedOnce" {
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

// splitSelfClause pulls a bare Card.Self (or Creature.Self) clause out of a
// trigger's object filter, reporting whether it was the self clause and
// whether it required a creature. An AttackersDeclared filter is
// ValidAttackers$, matched against each declared attacker (kop.Trigger.Filter,
// read by the kernel's attackersMatching): Card.Self there means the source is
// one of the attackers (CR 508.3a), not that the event object is the source,
// so it stays in the filter instead of becoming TfSelf, which that path never
// reads.
func splitSelfClause(event string, f *lFilter) (self, selfCreature bool) {
	if event == "AttackersDeclared" {
		return false, false
	}
	var rest []lClause
	for _, c := range f.Clauses {
		if c == (lClause{Flags: fSelf}) || c == (lClause{Types: tCreature, Flags: fSelf}) {
			self = true
			selfCreature = c.Types != 0
			continue
		}
		rest = append(rest, c)
	}
	f.Clauses = rest
	return self, selfCreature
}

// staticFor dispatches the static kinds whose build is its own helper
// (lower_offzone.go), else static.
func (cb *cardBuilder) staticFor(s lStatic) (kop.Ability, []uint8, error) {
	if s.Kind == "RetraceGrant" {
		return cb.offKwGrantAbility(s)
	}
	if s.Kind == "UnearthGrant" || s.Kind == "CyclingGrant" {
		ka, _, err := cb.offKwGrantAbility(s)
		if err != nil {
			return ka, nil, err
		}
		ka.Static.Kind = kop.StatUnearthGrant
		if s.Kind == "CyclingGrant" {
			ka.Static.Kind = kop.StatCyclingGrant
		}
		j, ok := cb.abMap[s.GrantBody] // 0-based LIR index, like s.Grant above
		if !ok {
			return ka, nil, fmt.Errorf("granted unearth body %d not emitted", s.GrantBody)
		}
		ka.Static.GrantBody = uint8(j + 1)
		return ka, nil, nil
	}
	if s.Kind == "NumLoyaltyAct" {
		return cb.numLoyaltyActStatic(s) // static_loyalty.go
	}
	if s.Kind == "IgnoreLandWalk" {
		return cb.ignoreLandWalkAbility(s) // static_ignorelandwalk.go
	}
	if s.Kind == "MustBlock" {
		return cb.mustBlockAbility(s) // static_mustblock.go
	}
	if s.Kind == "UnspentMana" {
		return cb.unspentManaAbility(s) // static_unspentmana.go
	}
	if s.Kind == "CanBlockIfShadow" {
		return cb.canBlockIfShadowAbility(s) // static_canblockifshadow.go
	}
	if s.Kind == "IgnoreLegendRule" {
		return cb.ignoreLegendRuleAbility(s) // static_ignorelegendrule.go
	}
	return cb.staticIn(s)
}

// combatAmountFilter builds the two combat-assignment statics (CR 510.1):
// the kind zeroes the matching creature's combat damage assignment, and the
// filter reads its creature on the battlefield.
func (cb *cardBuilder) combatAmountFilter(st *kop.Static, kind kop.StaticKind, f *lFilter) error {
	st.Kind = kind
	if f == nil {
		return nil
	}
	fid, err := cb.filter(f, kop.ZoneBattlefield)
	st.Filter = fid
	return err
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
		st.Filter2, err = filter(s.F2, zone)
		return err
	}
	var add []uint8
	switch s.Kind {
	case "FlipCoinMod":
		st.Kind = kop.StatFlipCoinMod // static_turnbits.go
	case "ActivationLimit":
		st.Kind = kop.StatActivationLimit // static_turnbits.go
	case "Continuous":
		zone := continuousKind(st, s.Flags, s.Zone) // lower_offzone.go
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
		st.StFlags |= gyContStFlag(s.Flags)
		if st.Filter, err = filter(s.F, zone); err != nil {
			return ka, nil, err
		}
		if err := amounts(s.Power, s.Tough); err != nil {
			return ka, nil, err
		}
		if err := continuousStFlags(s, st); err != nil {
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
		if st.Landwalk, st.GainsOf, err = continuousFilterPair(filter, &s, st); err != nil {
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
			rb, rf := reduceAbBits(s)
			st.B, st.StFlags = st.B|rb, st.StFlags|rf
			if s.F2 != nil {
				if err := targets2(); err != nil { // a target-gated reduction (the equip's Card.Self; Bladegraft's ability target)
					return ka, nil, err
				}
			}
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
		if s.Zone == zCommand {
			st.Zone = kop.ZoneCommand // a Vanguard's command-zone static (Sisay)
		}
		st.A, st.B = int16(s.TapCol), int16(s.TapAmt)
		for _, fl := range s.Flags {
			f, ok := tapExtraFlag(fl)
			if !ok {
				return ka, nil, fmt.Errorf("tap extra flag %q", fl)
			}
			st.StFlags |= f
		}
		if s.Amount.Kind != 0 {
			if st.VA, err = cb.value(s.Amount); err != nil {
				return ka, nil, err
			}
			st.StFlags |= kop.StTapDynamic
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
	case "CantTarget", "MustTarget":
		st.Kind = targetStaticKind(s.Kind) // lower_canttarget.go, static_musttarget.go
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
		if err := raiseCostStFlags(s, st); err != nil {
			return ka, nil, err
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
		st.Kind, st.Filter, err = cb.manaConvertOp(s) // lower_manaconvert.go (err is returned below)
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
		return ka, nil, cb.combatAmountFilter(st, kop.StatCombatDmgToughness, s.F)
	case "AssignNoCombatDamage":
		return ka, nil, cb.combatAmountFilter(st, kop.StatNoCombatDamage, s.F)
	case "AssignCombatDamageAsUnblocked":
		// the as-unblocked election's static (CR 510.1c): the filter reads its
		// creature on the battlefield, and the kernel asks the creature's
		// controller the election in the combat damage step (combat_asgngrant.go)
		return ka, nil, cb.combatAmountFilter(st, kop.StatAssignCombatDamageAsUnblocked, s.F)
	case "DisableTriggers":
		st.Kind = kop.StatDisableETB
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
	case "ProtYourColors":
		st.Kind = kop.StatProtYourColors // kw_protyours.go
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
		if err := atkScopeBits(st, "CantAttack", s.Flags); err != nil {
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
		if err := atkScopeBits(st, "CantAttackUnless", s.Flags); err != nil {
			return ka, nil, err
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
	case "ExtraBlock":
		st.Kind = kop.StatExtraBlock // combat_extrablock.go blockCap
		st.A = int16(s.BlockAmount)  // kop.ExtraBlocksAny: combat_blockany.go
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
	case "CanAttackDefender":
		st.Kind = kop.StatCanAttackDefender
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
	case "MustAttack":
		if err := mustAttackStatic(s, st, filter); err != nil {
			return ka, nil, err
		}
	case "AttackRequirement":
		// the whole-declaration closure (War's Toll): Filter is the members
		// ValidCard$ binds, Filter2 the ValidAttacker$ creatures whose
		// attacking turns the requirement on (dirac/combat_attackrequirement.go)
		st.Kind = kop.StatAttackRequirement
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
		if st.Filter2, err = filter(s.F2, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
	case "Goad":
		st.Kind = kop.StatGoad // lower_trigger.go's Continuous Goad$ True
		if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
			return ka, nil, err
		}
	case "CastWithFlash":
		st.Kind = kop.StatCastWithFlash
		st.StFlags |= s.flashCasterFlag()
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
				// carries no alternative-cost cast (altSacOK: a spell
				// ability with a non-mana cost, an either-or or printed
				// additional cost)
				return ka, nil, fmt.Errorf("alternative cost with an exile-from-hand part on a card the kernel offers no alternative cast for")
			}
			return ka, nil, fmt.Errorf("alternative cost with a forage / exile-from-graveyard part")
		}
		// a sacrifice (alone or with life), a return, a hand exile, a
		// discard, life alone or taps: the kernel's altPickCost picks, now
		// with the mana part beside them too (a Borderpost's {1} plus a
		// Return, Sephara's {W} plus taps; prepAltFree)
		sacAlt := altPickShape(&ka.Cost) && cb.altSacOK()
		if !sacAlt && (ka.Cost.Flags != 0 || ka.Cost.N != 0 || ka.Cost.Mana.X != 0) {
			// the kernel offers a mana-only alternative cost
			// (prepAltFree); a part the pick stages cannot carry would
			// never be offered
			return ka, nil, fmt.Errorf("alternative cost with a non-mana part (the kernel offers mana-only alternatives)")
		}
	case "NoMaxHandSize":
		st.Kind = kop.StatNoMaxHandSize
		if slices.Contains(s.Flags, "HandDelta") {
			// CR 402.2: the additive RaiseMaxHandSize$ statics apply on top of
			// the set maximum, never below zero (gorge maxHandSizeFor): A is
			// the signed delta, read after the set-maximum walk (kernel
			// maxHand, static_playerstats.go)
			if !s.Amount.isConst() || s.Amount.N < -255 || s.Amount.N > 255 {
				return ka, nil, fmt.Errorf("NoMaxHandSize delta %v", s.Amount)
			}
			st.StFlags |= kop.StHandDelta
			st.A = int16(s.Amount.N)
		} else {
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
		}
		for _, fl := range s.Flags {
			if fl != "HandOpp" && fl != "HandDelta" {
				return ka, nil, fmt.Errorf("NoMaxHandSize flag %q", fl)
			}
			if fl == "HandOpp" {
				st.StFlags |= kop.StHandOpp
			}
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
	case "PlayerShroud":
		// "You have shroud" (Ivory Mask, True Believer, Solitary
		// Confinement): A == 2 is the shroud read, the symmetric one of the
		// A == 1 protection bit (kernel static_playerstats.go playerShroud;
		// gorge playerShroudBlocksTarget). The static is unconditional: a
		// conditioned grant is refused at the lowering (the kernel's reader
		// consults no Cond2).
		if s.Cond.Kind != cNone || s.Amount.Kind != vNone {
			return ka, nil, fmt.Errorf("PlayerShroud beside %s", s.Cond)
		}
		st.Kind = kop.StatPlayerHexproof
		st.A = 2
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
		if err = panharmoniconFlags(st, s.Flags); err != nil {
			return ka, nil, err
		}
		if s.F2 != nil {
			// ValidCause$ (the entering permanent): kop.StPanhETB; the dies
			// form (A == PanhModeDies) reads Filter2 as the dying permanent
			if st.A != kop.PanhModeDies {
				st.StFlags |= kop.StPanhETB
			}
			if st.Filter2, err = filter(s.F2, kop.ZoneBattlefield); err != nil {
				return ka, nil, err
			}
		}
	case "StackKw":
		// AffectedZone$ Stack keyword grant (lower_stackkw.go)
		if err := cb.stackKwStatic(st, s); err != nil {
			return ka, nil, err
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
		// the chosen land-type setter's read (Convincing Mirage's AddType$
		// ChosenType): the static reads its source's perm.chosen
		// (act_choosetype.go chosenGrant); any other static kind's AddChosen
		// lowering is the Continuous case's
		st.AddChosen = s.AddChosen
	default:
		return ka, nil, fmt.Errorf("static kind %s", s.Kind)
	}
	return ka, add, err
}

// graveLibFlags is the Repl.C placement bits of a GraveToLibrary replacement:
// bottom instead of top, shuffle after the insert, and battlefield-only.
func graveLibFlags(r lRepl) int16 {
	var c int16
	if r.LibBottom {
		c |= kop.ReplLibBottom
	}
	if r.LibShuffle {
		c |= kop.ReplLibShuffle
	}
	if r.LibFromBattlefield {
		c |= kop.ReplLibBattlefield
	}
	return c
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
		if err := cb.replEntersWith(r, &ka); err != nil {
			return ka, err
		}
	case "OthersEnterWith":
		rp.Kind = kop.ReplOthersEnterProgram
		// the entrant filter may carry the cast-provenance words: gorge
		// matches the entering object while the spell that resolves into it
		// is still on the stack, so the cast-provenance read is live there
		// (cb.entrant widens build.go's battlefield refusal)
		save := cb.entrant
		cb.entrant = true
		rp.Filter, err = cb.filter(r.F, kop.ZoneBattlefield)
		cb.entrant = save
		if err != nil {
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
		if r.N.isConst() && r.N.N > 2 {
			rp.A = int16(r.N.N) // the line's own Thrice (City on Fire)
		}
		if r.F != nil { // a filtered doubler (lower_replmult.go); Twinflame keeps Filter 0, C 0
			if r.Bits&(kop.ReplDmgOppPlayer|kop.ReplDmgOppPerm|kop.ReplDmgPlayer|kop.ReplDmgPerm|
				kop.ReplDmgYou|kop.ReplDmgSelf) == 0 {
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
	case "DamageRedirect":
		return rdrRepl(ka, r, cb)
	case "PreventMillOpps":
		rp.Kind = kop.ReplPreventMillOpps
	case "PreventDamageCounters", "DamageHealFirst", "PreventDamageToSelf", "PreventDamageFilt", "PreventDamageN":
		if err = cb.selfDamageRepl(rp, r); err != nil { // lower_preventtoself.go
			return ka, err
		}
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
		rp.C = int16(r.Bits) // kop.ReplCopyFirstAttached: the Crown's IsPresent$ attachment
	case "DoubleCounters":
		rp.Kind = kop.ReplDoubleCounters
	case "GraveToLibrary":
		rp.Kind = kop.ReplGraveToLibrary
		rp.C |= graveLibFlags(r)
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
	case "DestroyRegenerate":
		rp.Kind = kop.ReplDestroyRegenerate // lower_destroyregen.go
	case "ExileOnLeave":
		if err := buildExileOnLeave(cb, r, rp, &ka); err != nil {
			return ka, err
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
		if rp.Filter, err = cb.addCountersFilter(r.F); err != nil { // lower_counterrepl.go
			return ka, err
		}
	case "TurnUpProgram":
		rp.Kind = kop.ReplTurnUpProgram // lower_turnfaceuprepl.go
		if ka.Prog, err = cb.prog(r.Prog); err != nil {
			return ka, err
		}
	case "SpellsCantBeCountered":
		return cantCounterRepl(ka, r, cb) // lower_uncounterable.go
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
	case "BeginPhaseSkip", "LifeFloor", "DamageMinus", "DamageSet":
		if err := cb.replRewrite(r, rp); err != nil {
			return ka, err
		}
	default:
		return ka, fmt.Errorf("replacement %s", r.Kind)
	}
	return ka, nil
}

// replEntersWith builds the EntersWith replacement: the two bare shapes
// (enters tapped, enters with counters) or the general program.
func (cb *cardBuilder) replEntersWith(r lRepl, ka *kop.Ability) (err error) {
	rp := &ka.Repl
	switch {
	case len(r.Prog) == 1 && r.Prog[0].Op == "EnterTapped" && r.Cond.Kind == cNone:
		rp.Kind = kop.ReplEntersTapped
	case len(r.Prog) == 1 && r.Prog[0].Op == "EnterWithCounters" && r.Prog[0].A.isConst() && r.Cond.Kind == cNone:
		rp.Kind = kop.ReplEntersWithCounters
		rp.A = int16(r.Prog[0].A.N)
		k, err := counterKind(r.Prog[0].CS)
		if err != nil {
			return err
		}
		rp.C = int16(k)
	default:
		rp.Kind = kop.ReplEntersProgram
		if r.S == "Updated" {
			rp.C = kop.ReplAfterEnter
		}
		if ka.Prog, err = cb.prog(r.Prog); err != nil {
			return err
		}
	}
	return nil
}

// replRewrite builds the T17c replacement modes' abilities: the phase skip
// (lower_beginphase.go), the life floor (lower_liferepl.go) and the damage
// minus and set rewrites (lower_replmult.go).
func (cb *cardBuilder) replRewrite(r lRepl, rp *kop.Repl) (err error) {
	switch r.Kind {
	case "BeginPhaseSkip":
		rp.Kind = kop.ReplBeginPhase               // lower_beginphase.go
		rp.A, rp.C = int16(r.Phase), int16(r.Bits) // kop.ReplPhYou: the controller's own turns only
	case "LifeFloor":
		rp.Kind = kop.ReplLifeReduced // lower_liferepl.go
		if !r.N.isConst() || r.N.N < 1 {
			return fmt.Errorf("life floor %v", r.N)
		}
		rp.A = int16(r.N.N)
	case "DamageMinus":
		rp.Kind = kop.ReplDamageMinus // lower_replmult.go
		if !r.N.isConst() || r.N.N < 1 {
			return fmt.Errorf("damage minus %v", r.N)
		}
		rp.A, rp.C = int16(r.N.N), int16(r.Bits)
		if r.F != nil {
			if rp.Filter, err = cb.filter(r.F, kop.ZoneBattlefield); err != nil {
				return err
			}
		}
	case "DamageSet":
		rp.Kind = kop.ReplDamageSet // lower_replmult.go
		if !r.N.isConst() || r.N.N < 0 || kop.ReplDmgSetGateOf(int16(r.Bits)) == 0 || r.Bits&^kop.ReplDmgSetGE(15) == 0 {
			return fmt.Errorf("damage set %v to %#x", r.N, r.Bits)
		}
		rp.A, rp.C = int16(r.N.N), int16(r.Bits)
		if r.F2 != nil {
			if rp.Filter2, err = cb.filter(r.F2, kop.ZoneBattlefield); err != nil {
				return err
			}
		}
	}
	return nil
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

// buildExileOnLeave builds a dies-to-exile replacement: the filter has no
// zone gate either way (ZoneBattlefield compiles to Zone none), C 1 is the
// any-zone flag read by exileAnyN, and A carries the counter the exiled card
// gets (kind in the low byte, count above it; dirac/repl_exilectr.go). The
// rider prog (r.Prog) rides the ability.
func buildExileOnLeave(cb *cardBuilder, r lRepl, rp *kop.Repl, ka *kop.Ability) error {
	rp.Kind = kop.ReplExileOnLeave
	if len(r.Prog) != 0 { // the rider that runs after the exile move (lower_exilectrrider.go)
		var err error
		if ka.Prog, err = cb.prog(r.Prog); err != nil {
			return err
		}
	}
	f, err := cb.filter(r.F, kop.ZoneBattlefield)
	if err != nil {
		return err
	}
	rp.Filter = f
	if r.AnyZone {
		rp.C = 1
	}
	if r.Ctr != "" {
		k, err := counterKind(r.Ctr)
		if err != nil {
			return err
		}
		rp.A = int16(k) | int16(r.CtrN)<<8
	}
	return nil
}

// instr lowers one LIR instruction to its kop form: the operands every op
// shares here, then the op's handler from instrOps (instr.go; one
// instr_<family>.go per family registers its ops in init()).
func (cb *cardBuilder) instr(in lInstr) (kop.Instr, error) {
	ic := &instrCtx{cb: cb, in: in, fl: flagsOf(in.CS)}
	ki := &ic.ki
	var err error
	if ki.Obj, err = cb.ref(in.Obj); err != nil {
		return *ki, err
	}
	if in.W.Kind != rNone {
		// the player side input (lir.go lInstr.W: a Tapper$ provenance or an
		// OpAttach player destination)
		if ki.W, err = cb.ref(in.W); err != nil {
			return *ki, err
		}
	}
	if in.Op != "Bite" && in.Op != "Fight" {
		if ki.A, err = cb.value(in.A); err != nil {
			return *ki, err
		}
		if ki.B, err = cb.value(in.B); err != nil {
			return *ki, err
		}
	}
	op := instrOps[in.Op]
	if op == nil {
		return *ki, fmt.Errorf("op %s has no kop form", in.Op)
	}
	if err := op(ic); err != nil {
		return *ki, err
	}
	if ic.in.F != nil && ki.F == 0 && !ic.skipFilterCheck {
		return *ki, fmt.Errorf("filter on %s not emitted", in.Op)
	}
	return *ki, nil
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
		// a permanent spell with no spell ability of its own (Hand of
		// Emrakul, a Borderpost, Force of Virtue): the alternative cast's
		// pick parts are the static's alone as well, and no target is asked.
		// A printed additional cost beside it would go unpaid: the pick
		// cost is the alternative's alone (buildPickCost).
		if c.SpellCost != nil {
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

// raiseCostStFlags maps a RaiseCost static's lowered flags onto its
// kop.StFlags bits and coloured pips (kernel cost_raise.go, cost_raiselife.go).
func raiseCostStFlags(s lStatic, st *kop.Static) error {
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
		case "RaiseColor":
			st.StFlags |= kop.StReduceColor // the packed coloured pips (cost_raise.go)
			st.B = int16(s.RaiseCol)
		default:
			return fmt.Errorf("raise cost flag %q", fl)
		}
	}
	return nil
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

// continuousStFlags maps a Continuous static's AffectedX convention onto its
// kop.StFlags bit: StPerAffected (the walk evaluates the dynamic amounts with
// each AFFECTED object as the reading source, gorge layers_static.go
// staticAmountOn's affected anchor; the flag aliases StOncePerTurn, a
// MayPlayFrom/TapPower-only bit).
func continuousStFlags(s lStatic, st *kop.Static) error {
	if s.PerAffected {
		st.StFlags |= kop.StPerAffected
	}
	for _, fl := range s.Flags {
		switch fl {
		case "FunctionsInGraveyard", "FunctionsInExile":
			// gyContStFlag sets the Graveyard bit above; a FunctionsInExile
			// on a Continuous stays silently dropped (this allow-list only
			// stops the refusal)
		case offZoneFlag:
			// continuousKind made it a StatOffZoneGrant (lower_offzone.go)
		default:
			return fmt.Errorf("continuous flag %q", fl)
		}
	}
	return nil
}

// anyKeptObjectClause reports the one-clause object halves of a player-or-
// permanent target that lower as the TgtAny walk with the clause kept as its
// filter: a planeswalker alone ("target player or planeswalker": Boros Charm),
// and a creature or planeswalker dealt damage this turn
// (Any.wasDealtDamageThisTurn: Needle Drop), whose flag the seat arm reads
// against the seat's per-turn damage record (legal.go targetOK,
// genState.plDmgT) and the object arm against dmgTurn.
func anyKeptObjectClause(c lClause) bool {
	return c == (lClause{Types: tPlaneswalker}) ||
		c == (lClause{Types: tCreature | tPlaneswalker, Flags: fWasDamaged})
}

// cantCounterRepl builds ReplSpellsCantBeCountered: the cast spell filter
// (Chimil, the Inner Sun, Sphinx of the Final Word).
func cantCounterRepl(ka kop.Ability, r lRepl, cb *cardBuilder) (kop.Ability, error) {
	ka.Repl.Kind = kop.ReplSpellsCantBeCountered
	ka.Repl.C = int16(r.Bits) // kop.ReplUncAnyCtrl: the line names no controller scope
	f, err := cb.filter(r.F, kop.ZoneStack)
	if err != nil {
		return ka, err
	}
	ka.Repl.Filter = f
	return ka, nil
}

// atkScopeBits folds a lowered combat static's Target$ scope flags into its
// StFlags bits (kop/attacktax.go): the CantAttack player/walker pair bits
// and the CantAttackUnless tax's (lower_attacktax.go). One flag per scope
// clause; an unknown flag is a lowering bug, refused.
func atkScopeBits(st *kop.Static, which string, flags []string) error {
	for _, fl := range flags {
		var bit kop.StaticFlags
		switch {
		case which == "CantAttack" && fl == "AtkPlayer":
			bit = kop.StCantAtkPlayer
		case which == "CantAttack" && fl == "AtkWalker":
			bit = kop.StCantAtkWalker
		case which == "CantAttack" && fl == "AtkUnlessDef":
			bit = kop.StCantAtkDef
		case which == "CantAttackUnless" && fl == "TaxPlayer":
			bit = kop.StAtkTaxPlayer
		case which == "CantAttackUnless" && fl == "TaxWalker":
			bit = kop.StAtkTaxWalker
		default:
			return fmt.Errorf("%s flag %q", which, fl)
		}
		st.StFlags |= bit
	}
	return nil
}

// panharmoniconFlags lowers a Panharmonicon static's mode flags (static_triggers.go
// modePanharmonicon) onto st: the one trigger mode in A and ValidPlayer$ in B.
func panharmoniconFlags(st *kop.Static, flags []string) error {
	for _, fl := range flags {
		switch fl {
		case "LifeGained":
			st.A = kop.PanhModeLifeGained
		case "Drawn":
			st.A = kop.PanhModeDrawn
		case "Dies":
			st.A = kop.PanhModeDies
		case "You":
			st.B = int16(kop.WhoseYou)
		default:
			return fmt.Errorf("Panharmonicon flag %s", fl)
		}
	}
	return nil
}

// trigWhoseScope refuses a trigger's player qualifier the kernel reads only
// on a printed battlefield listener of the events it handles.
func trigWhoseScope(whose kop.Whose, a *lAbility) error {
	if whose == kop.WhoseChosen && (a.Latent || a.Trigger.Event != "Phase" || a.Trigger.Zone != zBattlefield) {
		// the kernel reads it only for a printed battlefield Phase listener (chosenIs)
		return fmt.Errorf("Player.Chosen outside a printed battlefield Phase trigger")
	}
	if whose == kop.WhoseEnchanted && (a.Latent || a.Trigger.Event != "Phase" && a.Trigger.Event != "DamageDone" || a.Trigger.Zone != zNone && a.Trigger.Zone != zBattlefield) {
		// the kernel reads it only for a printed battlefield Phase or DamageDone listener (defEnchanted)
		return fmt.Errorf("Player.EnchantedBy outside a printed battlefield Phase or DamageDone trigger")
	}
	if whose == kop.WhoseMonarch && (a.Latent || a.Trigger.Event != "Phase" || a.Trigger.Zone != zNone && a.Trigger.Zone != zBattlefield) {
		// the kernel reads it only for a printed battlefield Phase listener (monarchTurnIs)
		return fmt.Errorf("Player.isMonarch outside a printed battlefield Phase trigger")
	}
	if whose == kop.WhoseEnchCtrl && (a.Latent || a.Trigger.Event != "Phase" || a.Trigger.Zone != zNone && a.Trigger.Zone != zBattlefield) {
		// the kernel reads it only for a printed battlefield Phase listener
		return fmt.Errorf("EnchantedController outside a printed battlefield Phase trigger")
	}
	return nil
}

// btSrcN sets a BecomesTarget watcher's ability-source (kop.TrigNSrcAbility)
// or source-controller (kop.TrigNSrcCtrl) marker in kt.N.
func btSrcN(t *lTrigger, kt *kop.Trigger) error {
	if t.SrcAbility {
		if !t.Watch || t.SrcSpell {
			return fmt.Errorf("ability source on a non-watcher BecomesTarget")
		}
		kt.N = kop.TrigNSrcAbility
	}
	if t.SrcCtrl {
		if !t.Watch || t.SrcSpell || t.SrcAbility || kt.N != 0 {
			return fmt.Errorf("source controller on a non-watcher BecomesTarget")
		}
		kt.N = kop.TrigNSrcCtrl
	}
	return nil
}

// mustAttackStatic lowers a MustAttack static: its creature filter, and the
// "other than you" flag (MustAttack$ Player.Other, lower_effectemblem.go) to
// kop.StatMustAttack's B: the requirement withholds the static controller as
// a defender while another living player is a legal one
// (dirac/combat.go mustAttackOther).
func mustAttackStatic(s lStatic, st *kop.Static, filter func(*lFilter, kop.Zone) (kop.FilterID, error)) error {
	var err error
	st.Kind = kop.StatMustAttack
	if st.Filter, err = filter(s.F, kop.ZoneBattlefield); err != nil {
		return err
	}
	for _, fl := range s.Flags {
		if fl != "MustAttackOther" {
			return fmt.Errorf("MustAttack flag %q", fl)
		}
		st.B = 1
	}
	return nil
}

// tapExtraFlag is a StatTapExtra static's named flag (lStatic.Flags), mapped
// once so the static builder's switch stays within its line ceiling.
func tapExtraFlag(name string) (kop.StaticFlags, bool) {
	switch name {
	case "TapOnC":
		return kop.StTapOnC, true
	case "TapReflect":
		return kop.StTapReflect, true
	case "TapAnyPlayer":
		return kop.StTapAnyPlayer, true
	case "TapChosen":
		return kop.StTapChosen, true
	case "TapDynamic":
		return kop.StTapDynamic, true
	case "TapAnyCol":
		return kop.StTapAnyCol, true
	}
	return 0, false
}

// dmgPlayerAltClause: the one-type permanent alternative a DamageDone
// ValidTarget$ lists beside Player (Planeswalker or Battle: a permanent
// combat damage can be assigned to as an attack's recipient). A creature
// alternative stays refused (TestDamageTargetPlayerOrPlaneswalkerBuilds).
func dmgPlayerAltClause(c lClause) bool {
	return c == (lClause{Types: tPlaneswalker}) || c == (lClause{Types: tBattle})
}
