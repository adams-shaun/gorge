package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// typesall: PredTypesAll, the all-of type predicate (cold, out of line).
//
// A Forge filter clause naming two disjoint card types ("Artifact.Creature"
// = artifact creatures, "Land.Artifact" = artifact lands, "Creature.Artifact
// +YouCtrl") is, in gorge, the type head AND the type property
// (filter_word wordType: hasTypePredicateCtx on the current types), so a
// permanent needs EVERY type in the clause. kgen keeps the clause's any-of
// Filter.Types as the union (the hot pre-check stays a superset test) and
// emits PredTypesAll with PredN = the conjunction; the predicate decides
// the match exactly. The permanent path reads the DERIVED types (typesGen:
// the layer walk, so an Ensoul Artifact-animated artifact is an artifact
// creature); off the battlefield the printed types (prep.cards[c].types,
// as every other card-path type read).

// powerGEVFrom is PredPowerGEV (and PredToughnessLEV with PredN 3) for a filter read with a source permanent:
// the value is read in that source's context when it is not the resolving
// entry's source, as gorge evaluates a trigger's ValidCard$ with the
// trigger's host as the source (Ensign Harry Kim, Operations' "greater
// power than Ensign Kim" while another creature's spell resolves).
//
//go:noinline
func (s *State) powerGEVFrom(k *prep, fl *kop.Filter, sl, src Slot) bool {
	if s.xBoundPending(fl) {
		return true
	}
	ctx := s.ctxCur()
	if ctx.srcSl != src {
		ctx = evalCtx{src: s.perms[src].obj, srcSl: src, ctrl: s.perms[src].ctrl}
	}
	pw, t := s.ptGen(k, sl)
	v := s.evalV(k, fl.PredV, ctx)
	switch {
	case fl.Pred == kop.PredToughnessLEV:
		return t >= v // PredN 3: Hulkling's toughness > his own (toughnessGTZ)
	case fl.PredN == 3:
		return pw == v // power == the value (Amalia's powerEQ20)
	case fl.PredN == 2:
		return pw <= v // power <= the value (Legacy's Allure's powerLEX)
	}
	return pw >= v
}

// predPermCold is the predicate switch's default arm for a battlefield
// permanent: the later kinds, dispatched here so the hot
// switch table stays as it was.
//
//go:noinline
func (s *State) predPermCold(k *prep, fl *kop.Filter, sl Slot) bool {
	if fl.Pred == kop.PredTypesAll {
		m := kop.TypeMask(uint16(fl.PredN))
		return s.typesGen(k, sl)&m == m
	}
	if fl.Pred == kop.PredTypesAllCMCLE {
		// an all-of type conjunction and a constant mana-value bound: the
		// derived types and the current mana value (s.cmcPerm)
		m := kop.TypeMask(uint16(fl.PredN))
		return s.typesGen(k, sl)&m == m && s.cmcPerm(k, sl) <= int(fl.PredV.N)
	}
	if fl.Pred == kop.PredMonstrous {
		return s.monstrousAt(sl) // act_monstrosity.go
	}
	if (fl.Pred == kop.PredPowerGEV || fl.Pred == kop.PredToughnessLEV) && s.xBoundPending(fl) {
		return true
	}
	if fl.Pred == kop.PredPowerGEV {
		pw, _ := s.ptGen(k, sl) // power >= the value PredV; PredN 3: power == PredV; PredN 2: power <= PredV
		if fl.PredN == 3 {
			return pw == s.evalV(k, fl.PredV, s.ctxCur())
		}
		if fl.PredN == 2 {
			return pw <= s.evalV(k, fl.PredV, s.ctxCur())
		}
		return pw >= s.evalV(k, fl.PredV, s.ctxCur())
	}
	if fl.Pred == kop.PredToughnessLEV {
		_, t := s.ptGen(k, sl) // toughness <= the value PredV (current toughness, layers applied); PredN 3: >=
		if fl.PredN == 3 {
			return t >= s.evalV(k, fl.PredV, s.ctxCur())
		}
		return t <= s.evalV(k, fl.PredV, s.ctxCur())
	}
	if fl.Pred >= kop.PredNoCounters && fl.Pred <= kop.PredMonoColor {
		return s.predPermKvf(k, fl, sl) // filter_valueheads.go
	}
	if fl.Pred == kop.PredSameNameT0 {
		return s.sameNameT0(sl) // kw_madness.go
	}
	if fl.Pred == kop.PredCountersLT {
		n := int(fl.PredN)
		return s.perms[sl].ctrN(s, n&0xff) < n>>8
	}
	if fl.Pred == kop.PredTotalPT {
		pw, t := s.ptGen(k, sl) // gorge totalPT (current P + T)
		return kop.DmgGatePass(fl.PredN, pw+t)
	}
	if fl.Pred == kop.PredBlocked {
		// gorge "blocked": attacking with a blocker recorded
		// (pfBlocked survives the blocker leaving, gorge's tombstone)
		return s.perms[sl].flags&(pfAttacking|pfBlocked) == pfAttacking|pfBlocked
	}
	if fl.Pred == kop.PredSolved {
		// a Case's solved designation (OpSaddle C=1, the
		// cseenSaddled bit; kw_crewsaddle.go)
		return (s.perms[sl].cseen&cseenSaddled != 0) == (fl.PredN == 1)
	}
	if fl.Pred == kop.PredSuspected {
		return s.suspected(k, sl) == (fl.PredN == 1)
	}
	if fl.Pred == kop.PredRenowned {
		return s.renownedAt(k, sl) == (fl.PredN == 1) // act_renown.go
	}
	if fl.Pred == kop.PredEscaped {
		return s.mSl(k, mechEscaped, sl) == (fl.PredN == 1) // kw_escape.go
	}
	if fl.Pred == kop.PredTrigPlayerCtrl {
		return s.trigPlayerCtrl(sl)
	}
	if fl.Pred == kop.PredPermMark {
		return s.permMarkRead(k, fl, sl) // filter_permmarks.go
	}
	if fl.Pred == kop.PredExiledWith {
		return s.exwCard(s.perms[sl].obj)
	}
	if fl.Pred == kop.PredTriggerObj {
		return s.isTriggerObj(s.perms[sl].obj) // filter_predicates.go
	}
	if fl.Pred == kop.PredWasCastEnter {
		return s.wasCastPred(fl, s.perms[sl].obj, s.perms[sl].ctrl)
	}
	if fl.Pred == kop.PredAdventureCard {
		return adventureCard(k, s.perms[sl].card) // the object's own card
	}
	if fl.Pred == kop.PredWasCastSpell {
		// A PERMANENT side read: only an other-object ETB replacement's
		// entrant filter carries it (kgen refuses it elsewhere). Gorge
		// matches the entering object while the spell resolving into it --
		// when it IS one -- still resolves: wasCastSpell's spellEntryOf then
		// finds that entry and answers the entrant's own cast provenance,
		// the log read of gorge's host WasCastFromHand/WasCastFromExile
		// (host_cast_ledger.go: the bare word is "cast from a hand, any
		// caster", case 1 here); an object that entered any other way has no
		// entry and reads not cast, as gorge's dead ledger record does.
		return s.wasCastSpell(fl, s.perms[sl].obj, s.perms[sl].ctrl)
	}
	if fl.Pred == kop.PredNotF2 {
		return s.notF2Perm(k, fl, sl) // filter_negations.go
	}
	if fl.Pred == kop.PredFaceDown {
		return s.faceDownPred(k, sl) // filter_facedown.go
	}
	if fl.Pred == kop.PredWord7 {
		return s.wordPerm(k, fl, sl) // filter_props.go
	}
	if fl.Pred == kop.PredPowerGTBase {
		return s.powerGTBase(k, sl) // filter_powerbase.go
	}
	if fl.Pred == kop.PredHasManaAbility {
		// gorge FaceHasManaAbility's printed-face read (CardDef has it),
		// with a face-down permanent's no-abilities rule (CR 708.2); PredN
		// 1 is !hasManaAbility.
		return hasManaAbilityCard(k, s.cardAt(sl), s.faceDown(k, sl)) == (fl.PredN == 0)
	}
	panic("kernel: filter predicate not implemented: " + predName(fl.Pred))
}

// predCardCold is predPermCold for a card off the battlefield (printed
// types).
//
//go:noinline
func predCardCold(k *prep, fl *kop.Filter, c CardID, fd bool) bool {
	if fl.Pred == kop.PredTypesAll {
		m := kop.TypeMask(uint16(fl.PredN))
		return k.cards[c].types&m == m
	}
	if fl.Pred == kop.PredSuspected {
		return fl.PredN == 0 // off the battlefield nothing is suspected (gorge Object.Suspected clears on the move)
	}
	if fl.Pred == kop.PredRenowned {
		return fl.PredN == 0 // off the battlefield nothing is renowned (gorge Object.Renowned clears on the move)
	}
	if fl.Pred == kop.PredEscaped {
		return fl.PredN == 0 // off the battlefield nothing is escaped (gorge resets CastFlags on the move)
	}
	if fl.Pred == kop.PredPermMark {
		return fl.PredN&kop.PermMarkNot != 0 // a card off the battlefield has no mark
	}
	if fl.Pred == kop.PredSolved {
		return fl.PredN == 0 // off the battlefield nothing is solved (gorge clears Solved on the move)
	}
	if fl.Pred == kop.PredMonstrous || fl.Pred == kop.PredPowerGEV || fl.Pred == kop.PredToughnessLEV || fl.Pred == kop.PredTotalPT || fl.Pred == kop.PredBlocked {
		return false // only a battlefield permanent is monstrous; PredToughnessLEV is lowered for the battlefield only (PredPowerGEV off it: cardPowerGEV)
	}
	if fl.Pred >= kop.PredNoCounters && fl.Pred <= kop.PredMonoColor {
		return predCardKvf(k, fl, c, fd) // filter_valueheads.go
	}
	if fl.Pred == kop.PredSameNameT0 {
		return false // reached only with no object (the object form: sameNameT0Card)
	}
	if fl.Pred == kop.PredCountersLT {
		// off the battlefield a card has no counters (a stash mark is read
		// by the card path's PredCountersGE; no lowering pairs it with LT)
		return 0 < fl.PredN>>8
	}
	if fl.Pred == kop.PredAdventureCard {
		return adventureCard(k, c)
	}
	if fl.Pred == kop.PredFaceDown {
		return false // off the battlefield nothing is face down for gorge (wordFaceDown), a face-down spell included
	}
	if fl.Pred == kop.PredWord7 {
		return false // reached only with no object
	}
	if fl.Pred == kop.PredPowerGTBase {
		return false // off the battlefield power equals base power (gorge objectPower/objectBasePower)
	}
	if fl.Pred == kop.PredHasManaAbility {
		return hasManaAbilityCard(k, c, fd) == (fl.PredN == 0) // a face-down card has no abilities (CR 708.2)
	}
	panic("kernel: card predicate not implemented: " + predName(fl.Pred))
}

// hasManaAbilityCard is PredHasManaAbility's card read: the printed face's
// mana ability (CardDef.HasManaAbility, filled by kgen from
// effects.FaceHasManaAbility), with a face-down card or permanent answering
// false (CR 708.2). Negotiation is the caller's.
func hasManaAbilityCard(k *prep, c CardID, fd bool) bool {
	return !fd && c != NoCard && k.t.Cards[c].HasManaAbility
}

// cardMV is card c's mana value as an off-battlefield filter reads it: the
// printed total, with a face-down card at 0 (CR 708.4), a split card off the
// stack and battlefield summing its halves (CR 709.4b), and a spell's chosen
// X added on the stack (CR 202.3e). The CMC arm and PredTypesAllCMCLE share
// it.
func (s *State) cardMV(k *prep, c CardID, o Obj, fd bool) int {
	pc := &k.cards[c]
	mv := int(pc.total)
	if fd {
		return 0 // CR 708.4: a face-down spell has no mana cost
	}
	if k.gen != nil && k.gen.anySplit {
		mv = s.splitMV(k, c, o, mv) // face_castface.go
	}
	if pc.cost.X != 0 && o != NoObj {
		mv += s.stackXMV(o, pc.cost.X)
	}
	return mv
}

// typesAllCMCCard is PredTypesAllCMCLE off the battlefield: the printed type
// conjunction (exactly PredTypesAll's mask) and the printed mana value.
func (s *State) typesAllCMCCard(k *prep, fl *kop.Filter, c CardID, o Obj, fd bool) bool {
	m := kop.TypeMask(uint16(fl.PredN))
	if k.cards[c].types&m != m {
		return false
	}
	return s.cardMV(k, c, o, fd) <= int(fl.PredV.N)
}

// trigPlayerCtrl is kop.PredTrigPlayerCtrl: sl's controller is the trigger
// player of the current trigger entry (built or resolving), the damaged
// seat of a damage trigger. No trigger entry matches nothing.
func (s *State) trigPlayerCtrl(sl Slot) bool {
	if DebugTrigPlayerNextSeat {
		return s.perms[sl].ctrl != s.active // seat-ok: a test-only mutation, the two-seat OppCtrl reading
	}
	e := s.curEntry()
	return e != nil && e.kind == seTrigger && s.perms[sl].ctrl == e.trigPl
}

// DebugTrigPlayerNextSeat is a test-only mutation: PredTrigPlayerCtrl reads
// any player but the active one, the two-seat reduction.
var DebugTrigPlayerNextSeat bool
