package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Spectacle (CR 702.137a): "You may cast this spell for its spectacle cost
// rather than its mana cost if an opponent lost life this turn." Cold,
// reached only when the table has a card with printed spectacle
// (useSpectacle).
//
// gorge's hand walk (legal_walk_hand.go handAltCastRows) offers the
// "spectacle" cast option from the altCastModes row altSpectacle: a potential
// play (offered like dash and evoke, before the plain affordability gate)
// once opponentLostLifeThisTurn holds -- some living opponent of the caster
// has Player.LifeLostThisTurn > 0, walked in seat order, so at four seats any
// of the three opponents opens it. The kernel reads the same fact off the
// per-seat lifeLost tally over oppsFrom.
//
// The cast rides AkCast mode 0 with castSpectacleTgt in the target byte;
// legal.go sets genState.impCast and beginCast marks the build entry with
// modeImpended, which reads as "spectacled" on a HasSpectacle card (kgen
// refuses spectacle beside every other keyword that claims the bit, and on a
// modal spell). buildCost prices SpectacleCost and the cast is lowered like a
// dash cast (cast.go). A stack copy clears the bit (copy_gen.go), so a copy
// is not spectacled.
//
// The provenance a self-ETB trigger reads ("if its spectacle cost was paid":
// Card.Self+spectacle, Card.spectacle) is the permanent's kicked mark
// (mechKicked, F2Kicked): enterGen sets it from the resolving entry, as it
// does for Bargain and Teamwork. kgen gates the word on CardDef.HasSpectacle,
// where no kicker exists to share the mark. gorge stamps FlagAltCostPaid on
// the cast (altcast_modes.go), which altCostKeywordPaid reads on the face's
// K:Spectacle.

// useSpectacle marks a table with a printed-spectacle card.
var useSpectacle = newGenUse(func(t *kop.Table) bool {
	for i := range t.Cards {
		if t.Cards[i].HasSpectacle {
			return true
		}
	}
	return false
})

// KwSpectacle is the manifest's KwID for the keyword (dirac/manifest/KwID/
// KwSpectacle): the code this mechanic implements; the printed carrier is
// CardDef.HasSpectacle (no KwMask bit is free).
const _ = kop.KwSpectacle

// castSpectacleTgt in an AkCast's target byte (mode 0, from the hand) is the
// cast for the card's spectacle cost (offerSpectacleHand).
const castSpectacleTgt = 8

// FeatCastSpectacleTgt is the target byte of a spectacle cast (canon/translate.go).
const FeatCastSpectacleTgt = castSpectacleTgt

// DebugSpectacleAlwaysOpen drops the "an opponent lost life this turn" gate
// (TestSpectacleGateMutation: the lockstep must catch it).
var DebugSpectacleAlwaysOpen bool

// SpectacleStats counts what a lockstep run exercised (nil costs one test on
// this cold path).
type SpectacleStats struct {
	Offers int // spectacle casts offered
	Paid   int // permanents entered with the spectacle mark
}

// DebugSpectacleStats, when non-nil, tallies the spectacle offers and casts.
var DebugSpectacleStats *SpectacleStats

// spectacled reports whether e is a spell cast for its spectacle cost.
func (e *stackEntry) spectacled(k *prep) bool {
	return e.kind == seSpell && k.t.Cards[e.card].HasSpectacle && e.mode&(modeImpended|seFaceDown) == modeImpended
}

// spectacleOpen reports whether some living opponent of p lost life this turn
// (gorge opponentLostLifeThisTurn).
func (s *State) spectacleOpen(p uint8) bool {
	if DebugSpectacleAlwaysOpen {
		return true
	}
	q := s.oppsFrom(p)
	for i := uint8(0); i < q.n; i++ {
		if s.pl[q.s[i]].lifeLost > 0 {
			return true
		}
	}
	return false
}

// offerSpectacleHand offers hand card o's spectacle cast (gorge's "spectacle"
// option). The caller has checked the card's timing and the cast prohibition;
// it runs before the plain affordability gate, like dash's.
//
//go:noinline
func (s *State) offerSpectacleHand(k *prep, d *Decision, p uint8, o Obj, c CardID, av *avail) {
	if !s.spectacleOpen(p) {
		return
	}
	cd := &k.t.Cards[c]
	sc := s.adjustCost(k, p, c, o, cd.SpectacleCost) // the reduction applies to an alternative cost too (CR 601.2f)
	if len(k.gen.grantPC) != 0 && s.g.availX&availWalk != 0 {
		// gorge's potential walk counts a restricted mana ability only for a
		// payment its restriction admits (potentialRestrictionAdmits): a
		// hand-barred grant puts no cast on the walk grantCap reads
		var ha avail
		s.walkHandAvail(k, p, payDesc{class: pdSpell, card: c, src: NoSlot, og: s.restrOriginOfObj(o)}, &ha)
		av = &ha
	}
	if !av.affords(&sc, pipMaskOf(&sc), 0) && (!s.rbAny(p) && !k.restrSrc || !s.restrFallback(k, p, c, s.restrOriginOfObj(o), &sc, av)) {
		return
	}
	if !s.spellTargetsFeasible(k, c, p) || !s.spellCostsFeasible(k, c, p, o) || !s.altAddCastable(k, c, p, o, &sc, av) {
		return
	}
	if s.castHeldOut(o) {
		return
	}
	if st := DebugSpectacleStats; st != nil {
		st.Offers++
	}
	d.add(MakeAction(AkCast, TCode(o), castSpectacleTgt, 0))
}

// FeatCastAltTgt reports whether the target byte of a mode-0 hand cast names
// the cleave or spectacle cost, the two whose cast-mode word has no entry in
// the 16-entry table (canon/translate.go: CastModeOther).
func FeatCastAltTgt(tgt uint16) bool { return tgt == FeatCastCleaveTgt || tgt == FeatCastSpectacleTgt }

// surgeOrSpectacleCast reports whether AkCast a is the hand cast for the
// surge or the spectacle cost (the target-byte routes beginCast marks on the
// entry through genState.impCast).
func surgeOrSpectacleCast(k *prep, a Action) bool {
	if a.Mode() != 0 || k.gen == nil {
		return false
	}
	switch a.Target() {
	case castSurgeTgt:
		return k.gen.anySurge
	case castSpectacleTgt:
		return k.gen.uses.has(useSpectacle)
	}
	return false
}

// dashedOrSpectacled reports whether e is a spell cast for its dash or its
// spectacle cost (the two lowered as a potential play, cast.go).
func (e *stackEntry) dashedOrSpectacled(k *prep) bool {
	return k.gen.anyDash && e.dashed(k) || k.gen.uses.has(useSpectacle) && e.spectacled(k)
}

// surgeSpectacleCost is the alternative cost entry e's spell was cast for, when
// that is its surge or spectacle cost.
func surgeSpectacleCost(k *prep, e *stackEntry, cd *kop.CardDef) (kop.ManaCost, bool) {
	if cd.HasSurge && e.surged(k) {
		return surgeCostOf(cd), true // CR 702.117a; kw_surge.go
	}
	if cd.HasSpectacle && e.spectacled(k) {
		return cd.SpectacleCost, true // CR 702.137a
	}
	return kop.ManaCost{}, false
}

// enterRiotSpectacle runs at the top of enterGen's marks: a face-up entrant
// that prints Riot poses its ask (kw_riot.go), and a permanent a spectacle
// cast produced takes the kicked mark its self-ETB trigger reads.
func (s *State) enterRiotSpectacle(k *prep, sl Slot, o Obj, face uint8) {
	if face == 0 {
		s.riotEntryAsk(k, sl, o)
	}
	if k.gen.uses.has(useSpectacle) && s.res.active && s.res.e.kind == seSpell && s.res.e.obj == o && s.res.e.spectacled(k) {
		s.spectacleEnter(k, sl, &s.res.e)
	}
}

// spectacleEnter marks the permanent at sl that e's spectacle cast produced:
// the self-ETB trigger's "its spectacle cost was paid" reads the kicked mark.
func (s *State) spectacleEnter(k *prep, sl Slot, e *stackEntry) {
	s.mSlSet(k, mechKicked, sl)
	if st := DebugSpectacleStats; st != nil {
		st.Paid++
	}
}
