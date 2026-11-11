package kgen

// selfcond: a static whose condition reads the characteristics the static
// itself writes (Grond, the Gatebreaker counts Armies while it adds types
// to itself; Rusted Relic counts artifacts while it makes itself an
// artifact creature). Evaluated inside the kernel's layer walk, the
// condition re-enters the walk of every permanent it counts, the static's
// own source among them. The kernel's re-entry guard
// (dirac/static_reentryguard.go) answers that nested read with the
// static not applying; gorge answers it from its previous static list
// (engine/rules/layercache.go refreshStaticContinuous: a gate's nested
// Derived takes the list scanned at the previous event). The two agree
// exactly when the static's writes cannot change what its condition reads,
// which is what selfCondCheck proves (anything else is refused).

// condRd is what a condition reads of battlefield permanents.
type condRd struct {
	types uint16 // card types (supertypes ride in the same mask) its filters test
	sub   bool   // subtypes (a chosen type is a subtype read)
	col   bool   // colours (a chosen colour is a colour read)
	pt    bool   // power or toughness
	kw    bool   // keywords (layer 6)
	name  bool   // the layer-3 name
	all   bool   // something this check cannot bound
}

func (r *condRd) or(o condRd) {
	r.types |= o.types
	r.sub = r.sub || o.sub
	r.col = r.col || o.col
	r.pt = r.pt || o.pt
	r.kw = r.kw || o.kw
	r.name = r.name || o.name
	r.all = r.all || o.all
}

// filterRdFlags are the clause flags whose test reads no layer-3..7
// characteristic of the permanent tested (attack/tap/zone/identity state).
const filterRdFlags = fSelf | fOther | fToken | fNonToken | fAttacking | fBlocking | fTapped | fUntapped |
	fKicked | fCastByYou | fEquippedBy | fEnchantedBy | fEnteredThisTurn | fRemembered | fNotRemembered |
	fChosenCard | fNotChosenCard | fAttachedToParentTarget | fTargetedBy | fNotTriggerObj | fPromisedGift |
	fWasDamaged | fSaddled

// filterRdReads are the remaining clause flags: each names the
// characteristic it reads. A flag in neither set is unbounded (r.all).
const filterRdReads = fWithFlying | fWithoutFlying | fChosenType | fChosenColor | fNamedAsSource |
	fBasic | fLegendary | fSharesColorWithOtherRemembered

func filterRd(f *lFilter) condRd {
	var r condRd
	if f == nil || len(f.Clauses) == 0 {
		r.all = true
		return r
	}
	for _, c := range f.Clauses {
		r.types |= c.Types | c.TypesAll | c.NotTypes
		if c.Subtype != "" || c.NotSubtype != "" {
			r.sub = true
		}
		if c.Colors != "" || c.NotColors != "" {
			r.col = true
		}
		if c.Flags&(fWithFlying|fWithoutFlying) != 0 {
			r.kw = true
		}
		if c.Flags&fChosenType != 0 {
			r.sub = true // hasSubGen of the source's chosen creature type
		}
		if c.Flags&fChosenColor != 0 {
			r.col = true // colorsGen against the source's chosen colour
		}
		if c.Flags&fNamedAsSource != 0 || c.Name != "" {
			r.name = true // nameCardAt (layer 3)
		}
		if c.Flags&fBasic != 0 {
			r.types |= tBasic
		}
		if c.Flags&fLegendary != 0 {
			r.types |= tLegendary
		}
		if c.Flags&fSharesColorWithOtherRemembered != 0 {
			r.col = true
		}
		if c.Flags&^(filterRdFlags|filterRdReads) != 0 {
			r.all = true
		}
		switch c.CmpStat {
		case cmpPower, cmpToughness, cmpPowerLTToughness, cmpPowerGTBase:
			r.pt = true
		case cmpHasKw:
			r.kw = true
		case cmpMultiColor:
			r.col = true
		case cmpHistoric:
			r.types |= tArtifact | tLegendary
			r.sub = true // Saga
		case cmpNone, cmpCMC, cmpCounters, cmpHasX, cmpEquipped, cmpEnchanted, cmpAttached,
			cmpActiveCtrl, cmpModified, cmpHasCounters, cmpMonstrous:
			// no layer-3..7 characteristic: mana value, counters,
			// attachment, designation or the active player
		default:
			r.all = true
		}
	}
	return r
}

func valueRd(v lValue) condRd {
	var r condRd
	switch v.Kind {
	case vNone, vConst, vX, vCountGraveyard, vAttackedThisTurn, vLifeOf, vLibrarySizeOf, vStartingLife,
		vLifeGainedThisTurn, vOppLifeLostThisTurn, vCreaturesDied, vLifeLostThisTurn, vLifeLostLastTurn, vHandSizeOf,
		vOppsLostLife, vDrawnThisTurn, vGraveTypes, vGraveAgg, vYourSpeed, vHighestOppPoison,
		vKicked, vMorbid, vSum, vPromisedGift, vCompare, vBranches, vCommittedCrime, vCountersOn,
		vLowestLife, vOppsAttacked:
	case vCount, vCountAtLeast:
		r = filterRd(v.F)
	default:
		r.all = true
	}
	for _, s := range v.Sub {
		r.or(valueRd(s))
	}
	return r
}

func condRead(c lCond) condRd {
	var r condRd
	switch c.Kind {
	case cNone, cKicked, cThreshold, cYourTurn, cOpponentsTurn, cPromisedGift:
	case cAnd:
		for _, s := range c.Sub {
			r.or(condRead(s))
		}
	case cPresent:
		r = filterRd(c.F)
		r.or(valueRd(c.A))
	case cValue:
		r = valueRd(c.A)
		r.or(valueRd(c.B))
	default:
		r.all = true
	}
	return r
}

// selfCondCheck refuses a conditioned layer-4..7 static (an override, or a
// Continuous subtype grant) whose own writes can change what its
// condition reads. An added card type the source prints already is no
// change when the static reaches only its source.
func (lc *lowerCtx) selfCondCheck(st *lStatic) error {
	if st.Cond.Kind == cNone {
		return nil
	}
	var wTypes uint16
	var setTypes, wSub, wCol, wPT, wKw, wName bool
	if o := st.Override; o != nil {
		wTypes = o.AddTypes
		if filterSelfOnly(st.F) {
			wTypes &^= lc.out.Types
		}
		setTypes = o.HasSetTypes || o.SetTypes != 0
		wSub = o.HasSetSubtypes || len(o.SetSubtypes) > 0 || len(o.AddSubtypes) > 0 || o.AllCreatureTypes || o.RemoveCreatureSub
		wCol = o.HasSetColors
		wPT = o.HasSetPT
		wKw = o.LoseAbilities || len(o.AddKw) > 0 || len(o.RemoveKw) > 0
		wName = o.Name != ""
	}
	if len(st.AddSub) > 0 || st.AddChosen {
		wSub = true
	}
	if wTypes == 0 && !setTypes && !wSub && !wCol && !wPT && !wKw && !wName {
		return nil
	}
	r := condRead(st.Cond)
	switch {
	case r.all:
		return refuse("self-reading static condition: an unbounded read beside a characteristic write")
	case wTypes&r.types != 0 || setTypes && r.types != 0:
		return refuse("self-reading static condition: counts a card type the static adds or sets")
	case wSub && r.sub:
		return refuse("self-reading static condition: counts a subtype beside a subtype write")
	case wCol && r.col:
		return refuse("self-reading static condition: tests a colour the static sets")
	case wPT && r.pt:
		return refuse("self-reading static condition: tests a stat beside a set P/T")
	case wKw && r.kw:
		return refuse("self-reading static condition: tests a keyword the static grants, removes or loses")
	case wName && r.name:
		return refuse("self-reading static condition: tests a name the static sets")
	}
	return nil
}
