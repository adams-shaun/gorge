package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A graveyard mana ability whose {1} fee the floating pool pays (Jack-o'-
// Lantern: "{1}, Exile Jack-o'-Lantern from your graveyard: Add one mana of
// any color", CR 605.1a, 605.2a; kop.CardDef.GraveMana). gorge lists it as a
// payment-window option only while mana floats (the probe
// engine/rules TestGraveManaGorgeProbe), after every battlefield and hand
// source, so the kernel's window takes it where the battlefield and the hand
// run dry (windowTapWalk), and it is an untapped mana source to the same two
// gates the hand guides are (hasUntappedMana, windowUnpricedOpenIf).
//
// Activating it pays the {1} fee from the pool's first non-empty kind in C W U
// B R G order (the fee is unasked: gorge's activation asks no fee pick), moves
// the card to exile (the CARDNAME exile is unasked too) and adds one mana of
// the colour the window's cost still owes. Only a cast's or an activation's
// own payment window lists it, as it lists a hand guide (windowHandMana): the
// unless, ward and trigger-cost windows list the battlefield alone.

// useGraveMana: the table holds a card with a graveyard mana ability.
var useGraveMana = newGenUse(func(t *kop.Table) bool {
	for i := range t.Cards {
		if t.Cards[i].GraveMana != 0 {
			return true
		}
	}
	return false
})

// graveManaOn reports whether the table holds a graveyard mana source.
func graveManaOn(k *prep) bool {
	return k.gen != nil && k.gen.uses.has(useGraveMana) && !DebugNoGraveMana
}

// DebugNoGraveMana leaves every graveyard mana source out of the windows and
// gates (mutation testing only: the lockstep must catch it).
var DebugNoGraveMana bool

// graveManaPick is the first card in p's graveyard, in graveyard order, with a
// graveyard mana ability, and the colours it adds. NoObj when none.
func (s *State) graveManaPick(k *prep, p uint8) (o Obj, mask uint8) {
	pl := &s.pl[SeatIx(p)]
	for i := 0; i < int(pl.graveN); i++ {
		if m := k.t.Cards[s.objs[pl.grave[i]].card].GraveMana; m != 0 && !s.graveManaLost(k, pl.grave[i]) {
			return pl.grave[i], m
		}
	}
	return NoObj, 0
}

// graveManaAny reports whether p can activate a graveyard mana source: its fee
// needs one floating mana (gorge lists the activation only while one floats).
func (s *State) graveManaAny(k *prep, p uint8) bool {
	if !graveManaOn(k) || poolTotal(&s.pl[SeatIx(p)]) == 0 {
		return false
	}
	o, _ := s.graveManaPick(k, p)
	return o != NoObj
}

// graveManaFee pays the {1} fee from p's pool: the first non-empty kind in C
// W U B R G order (the fee is unasked: gorge's activation asks no fee pick,
// and its generic spend runs colourless first -- the order the filter fee's
// own spend reads, mana_mixgaps.go filtFeeSpend; a mixed {C}{U} pool pays the
// {C}, gravemana lockstep seed 16's turn-5 payment).
func (s *State) graveManaFee(p uint8) {
	pl := &s.pl[SeatIx(p)]
	for _, c := range [6]int{kop.C, kop.W, kop.U, kop.B, kop.R, kop.G} {
		if pl.pool[c] != 0 {
			pl.pool[c]--
			return
		}
	}
}

// graveManaCol is the colour of mask the window's answer takes: the first
// colour the cost still owes after the fee (gorge's askManaColor lists the
// owed colours first, CR 601.2g; windowDemand is that shortfall), else the
// lowest colour of mask. owed is nil for a window whose ask has no clamp
// (anyColor, or none).
func (s *State) graveManaCol(k *prep, p uint8, mask uint8, owed *kop.ManaCost) int {
	mask &= 0x1f
	if owed != nil {
		if m := mask & s.windowDemand(k, p, owed); m != 0 {
			return bits.TrailingZeros8(m)
		}
	}
	return bits.TrailingZeros8(mask)
}

// windowGraveMana is windowTapWalk's pick after the hand's: once neither a
// battlefield source nor a hand guide is listed, the window's answer takes the
// first graveyard source it lists, and reports whether one paid. It is
// withheld, as a hand guide is, when the clamp is decided and no pip colour is
// still owed (windowUseful: nothing it adds pays anything).
func (s *State) windowGraveMana(k *prep, p uint8, useful *kop.ManaCost, clamp, anyColor bool) bool {
	if !s.graveManaAny(k, p) || !s.buildLive() {
		return false
	}
	clamped := clamp && !anyColor
	if clamped {
		if want, decided := s.windowUseful(k, p, useful); decided && want == 0 {
			return false
		}
	} else {
		useful = nil
	}
	o, mask := s.graveManaPick(k, p)
	s.graveManaFee(p)
	s.exileFromGrave(o)
	s.poolAdd(p, s.graveManaCol(k, p, mask, useful), 1)
	return true
}

// The prefix search (filtSearch) funds through a graveyard source after the
// battlefield's and the hand's (gorge's scriptSearch.activations walks the raw
// priority's activate options, which list a graveyard source last and only
// while mana floats: TestGraveManaGorgeProbe Q1, Q1b). The search runs only
// where another gap (a filter) already starts it: gorge's planner prices no
// graveyard source, so Jack alone never makes a cast affordable
// (TestGraveManaGorgeProbe Q2) and no search is begun for it.
//
// Like a hand guide's, the activation is a covered funding step (funded+1, no
// gap used) whose mana the raw priority list counts and the planner did not,
// so fs.hands is held over its subtree (terminal, try). Unlike it, the
// activation asks a colour (askManaColor lists WUBRG): the ask is one node of
// the search and each colour answer is one branch, the colours the play needs
// first (scriptOrderAnswers; needFirst).
//
// An unless or ward census pay search and an activation's own payment
// (fs.noHand: actScriptPays, gorge's planner prices from the census) fund
// through none, as they fund through no hand guide: those windows list the
// battlefield alone and the census prices no graveyard source.

// DebugNoGraveFund leaves the graveyard sources out of the prefix search's
// funding pass (mutation testing only: the lockstep must catch it).
var DebugNoGraveFund bool

// graveFundDo plays a graveyard source's funding step on s (filtDo): the fee,
// the exile, then the answered colour.
func (s *State) graveFundDo(k *prep, p uint8, st filtStep) {
	pl := &s.pl[SeatIx(p)]
	if int(st.gidx) >= int(pl.graveN) || st.ans == 0 {
		return
	}
	o := pl.grave[st.gidx]
	if k.t.Cards[s.objs[o].card].GraveMana == 0 || s.graveManaLost(k, o) {
		return
	}
	s.graveManaFee(p)
	if k.artMana {
		s.artSync(p) // a slot the fee emptied no longer holds an artifact's mana
	}
	s.exileFromGrave(o)
	s.poolAdd(p, int(st.ans-1), 1)
}

// fundOffField is visit's funding pass over the zones beside the battlefield:
// the hand's guides (mana_handguide.go), then the graveyard's sources, in the
// order gorge's raw priority lists their activations. The unless / ward census
// prices no hand mana (unlessWindowUnits) and no graveyard mana, so a census
// pay search never reaches either: gorge offers decline where only a guide or
// Jack could pay. A plain window's pay probe keeps them.
func (fs *filtSearch) fundOffField(k *prep, t *State, funded int, gapUsed bool) bool {
	if fs.census && fs.pay != nil || funded >= fs.fees {
		return false
	}
	return handManaOn(k) && fs.handFund(k, t, funded, gapUsed) || graveManaOn(k) && fs.graveFund(k, t, funded, gapUsed)
}

// graveFund is the prefix search's funding pass over the payer's graveyard:
// each graveyard mana source in graveyard order is one funding activation,
// its colour ask answered in turn.
func (fs *filtSearch) graveFund(k *prep, t *State, funded int, gapUsed bool) bool {
	if DebugNoGraveFund || fs.noHand || !t.graveManaAny(k, fs.p) {
		return false
	}
	pl := &t.pl[SeatIx(fs.p)]
	for i := 0; i < int(pl.graveN); i++ {
		mask := k.t.Cards[t.objs[pl.grave[i]].card].GraveMana & 0x1f
		if mask == 0 || t.graveManaLost(k, pl.grave[i]) {
			continue
		}
		var order [5]uint8
		n := 0
		for c := uint8(0); c < 5; c++ {
			if mask&(1<<c) != 0 {
				order[n] = c
				n++
			}
		}
		needFirst(order[:n], fs.need)
		if n > 1 && !fs.tick() { // the colour ask is a node of the search
			return false
		}
		fs.hands++
		for _, c := range order[:n] {
			if fs.try(k, t, filtStep{sl: NoSlot, grave: true, gidx: uint8(i), ans: 1 + c}, funded+1, gapUsed) {
				fs.hands--
				return true
			}
		}
		fs.hands--
	}
	return false
}

// graveManaLost reports whether graveyard card o lost its printed mana
// ability (Yixlid Jailer; CR 613.1f): gorge's mana walk takes a lost card's
// printed mana abilities in every zone (mana_activation.go lossStamp), so the
// window and the funding pass list no such source (static_graveabilityloss.go).
func (s *State) graveManaLost(k *prep, o Obj) bool {
	return graveLossOn(k) && s.gravePrintedHidden(k, o)
}
