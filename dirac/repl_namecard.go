package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The mid-resolution card-name choice (gorge effNameCard, engine/effects/
// cardflow.go; Demonic Consultation's `SP$ NameCard | Defined$ You | SubAbility$
// DBExile`, Cheering Fanatic's trigger-body `DB$ NameCard | Defined$ You |
// SubAbility$ DBEffect`): the resolving controller chooses the name of one
// real card of the matchup (a ChGenName decision, mode = the card's CardID,
// Table.NameCardsAny or Table.NameCards ascending, gorge's sorted-name list
// over the matchup's name universe -- oracle NameUniverseFor narrows the
// universe to the matchup's own cards whenever one of them asks, so both
// sides offer the same names in the same order). gorge keeps the answer on
// the source object as Object.ChosenName (events.Choose Counter "name");
// the kernel keeps it in a battlefield source's chosen byte as that CardID
// (the as-enters choice's slot, repl_choosename.go), or in the resolution's
// own answer for a source still on the stack (gorge clears the name when an
// object leaves the battlefield, events apply_zone.go, so a spell's name
// needs nothing past its chain). ExtNameCard asks; raGenChooseName's answer
// stores; ExtClearNamedCard (gorge `DB$ Cleanup | ClearNamedCard$ True`)
// empties the name so the next ask asks again, as gorge's wiped source does
// (Cheering Fanatic attacks a second time).
//
// Card.NamedCard (kop.PredNamedCard) reads the name: a battlefield source's
// chosen byte through the walk's source context, a stack source's through
// this resolution's answer, and a host-bound emblem's frozen capture
// (embNamed, static_emblemcold.go) once a ClearNamedCard$ chain wiped the
// source after the Effect registered -- gorge freezes the chosen name on
// state.ContinuousEffect (ChosenNameFrozen) for exactly that reader.

// raGenChooseName is ExtNameCard's ask: one ChGenName option per nameable
// card of the matchup, mode = its CardID, Table.NameCardsAny (or
// Table.NameCards when B.N = 1, gorge ValidCards$ Card.nonLand), ascending
// -- gorge's sorted names (the oracle canon compares by the name's value).
// A name choice with no eligible name asks nothing (gorge effNameCard
// returns on an empty list).
const raGenChooseName = raGenManifestDread + 85

// resChosenName marks a resolution whose ExtNameCard answer is res.nc (the
// named CardID plus one): the stack source's chosen name (gorge
// Object.ChosenName) while its chain runs. res.bad's flag bits are all
// taken (0x1f the target-illegality bits, 0x20 chosen number, 0x40 chosen
// card type, 0x80 chosen type), so the answer rides resCtx.nc, 0 = none.

// nameAskSource is the slot of the resolving source when it is on the
// battlefield as the entry's object, NoSlot otherwise.
func (s *State) nameAskSource(k *prep) Slot {
	e := s.curEntry()
	if e == nil {
		return NoSlot
	}
	return s.objRef(k, kop.RefSelf)
}

// namedCardHeld reports the chosen name the resolving source already
// carries (gorge effNameCard's `o.ChosenName != ""` guard: the ask is a
// no-op), false when it asks anew. A battlefield source's chosen byte; a
// stack source's this resolution's answer.
func (s *State) namedCardHeld(k *prep) bool {
	if sl := s.nameAskSource(k); sl != NoSlot {
		return s.perms[sl].chosen != 255
	}
	return s.res.nc != 0
}

// storeChosenName keeps the answered name on the resolving source.
//
// DebugNameCardNoStore is TestNameCardLowers's mutation switch: the answer
// is dropped, so a DigUntil Valid$ Card.NamedCard of the same chain finds
// nothing where gorge finds the named card.
var DebugNameCardNoStore bool

func (s *State) storeChosenName(k *prep, c CardID) {
	if DebugNameCardNoStore {
		return
	}
	if sl := s.nameAskSource(k); sl != NoSlot {
		s.perms[sl].chosen = objWord(c)
		s.touchSl(sl)
		return
	}
	s.res.nc = uint8(c) + 1
}

// clearChosenName empties the resolving source's chosen card name (gorge
// `DB$ Cleanup | ClearNamedCard$ True`, Cheering Fanatic's DBCleanup): the
// next ask asks anew, as gorge's wiped Object.ChosenName does.
func (s *State) clearChosenName(k *prep) {
	if sl := s.nameAskSource(k); sl != NoSlot {
		s.perms[sl].chosen = 255
		s.touchSl(sl)
		return
	}
	s.res.nc = 0
}

// chosenNameCard is the chosen name the filter context ctx reads (nil: the
// resolving or building entry's), NoCard when there is none. A battlefield
// source's chosen byte; a stack source's this resolution's answer. A source
// with no chosen name matches nothing, as gorge's empty ChosenName does.
// The emblem statics' frozen read goes through srcNamedCard, which follows
// the static's own source.
func (s *State) chosenNameCard(ctx *evalCtx) CardID {
	cx := s.ctxCur()
	if ctx != nil {
		cx = *ctx
	}
	if cx.srcSl != NoSlot && s.occ.has(cx.srcSl) {
		if s.perms[cx.srcSl].chosen != 255 {
			return CardID(s.perms[cx.srcSl].chosen)
		}
		if ctx != nil {
			// an explicit context names a foreign source (the SpellCast
			// trigger's listener, castFilterOK), not this resolution's
			// entry: gorge namePredicate reads only that source's
			// ChosenName, so a source with no chosen name matches nothing
			// -- res.nc is this resolution's own answer, another entry's
			return NoCard
		}
	}
	if s.res.active && s.res.nc != 0 {
		return CardID(s.res.nc - 1)
	}
	return NoCard
}

// filterNamedCard reports whether some alternative of filter f reads the
// chosen card name (kop.PredNamedCard).
func filterNamedCard(t *kop.Table, f kop.FilterID) bool {
	for n := 0; f != 0 && int(f) < len(t.Filters) && n < 64; n++ {
		if t.Filters[f].Pred == kop.PredNamedCard {
			return true
		}
		f = t.Filters[f].Or
	}
	return false
}

// srcNamedCard is the chosen name the static with source slot src reads
// (NoSlot: this resolution's answer), its emblem handle emb (0 = not an
// emblem) first: the frozen capture a ClearNamedCard$ chain wiped the
// source after (gorge state.ContinuousEffect.ChosenName +
// ChosenNameFrozen; Cheering Fanatic's ReduceCost ValidCard$ Card.NamedCard).
func (s *State) srcNamedCard(src Slot, emb uint8) CardID {
	if emb != 0 {
		if n := s.embNamed(emb>>4, int(emb&15)-1); n != 0 {
			return CardID(n - 1)
		}
		return NoCard
	}
	if src != NoSlot {
		if s.perms[src].chosen != 255 {
			return CardID(s.perms[src].chosen)
		}
		return NoCard
	}
	if s.res.active && s.res.nc != 0 {
		return CardID(s.res.nc - 1)
	}
	return NoCard
}

// nameCardExt is OpExt ExtNameCard: the mid-resolution card-name choice
// (gorge effNameCard). A source that already carries a chosen name keeps it
// (gorge's ChosenName guard), and an ask with no eligible name asks nothing
// (gorge returns on an empty list).
//
//kmanifest:ext ExtNameCard
func (s *State) nameCardExt(k *prep, in *kop.Instr) (ok, done bool) {
	if s.namedCardHeld(k) {
		return true, true
	}
	names := k.t.NameCardsAny
	if in.B.N == 1 {
		names = k.t.NameCards // gorge ValidCards$ Card.nonLand
	}
	if len(names) == 0 {
		return true, true
	}
	s.res.ask, s.res.askP = raGenChooseName, s.res.e.ctrl
	return false, true
}

// clearNamedCardExt is OpExt ExtClearNamedCard: gorge `DB$ Cleanup |
// ClearNamedCard$ True` (Cheering Fanatic's next attack asks anew).
//
//kmanifest:ext ExtClearNamedCard
func (s *State) clearNamedCardExt(k *prep, in *kop.Instr) (ok, done bool) {
	s.clearChosenName(k)
	return true, true
}

// nameCardOptions is raGenChooseName's options: one per nameable card of the
// matchup, mode = its CardID ascending (gorge's sorted name list over the
// matchup's narrowed name universe, the ETB ask's option order).
func (s *State) nameCardOptions(k *prep, d *Decision) {
	d.Sub = ChGenName
	names := k.t.NameCardsAny
	if s.prog(k)[s.res.pc].B.N == 1 {
		names = k.t.NameCards // gorge ValidCards$ Card.nonLand
	}
	for _, c := range names {
		d.add(MakeAction(AkChoose, 0, 0, uint8(c)))
	}
}

// nameCardAnswer lands the answered card name on the resolving source (gorge
// events.Choose Counter "name": a battlefield source's chosen byte, else this
// resolution's answer) and continues the chain.
func (s *State) nameCardAnswer(k *prep, a Action) {
	s.storeChosenName(k, CardID(a.Mode()))
	s.continueRes(k, 0)
}

// embFrozenName is the resolving source's chosen card name, frozen into a new
// emblem for a Card.NamedCard read a later `DB$ Cleanup | ClearNamedCard$`
// chain would otherwise lose (gorge state.ContinuousEffect.ChosenName +
// ChosenNameFrozen): CardID plus one, 0 none.
func (s *State) embFrozenName(k *prep) objWord {
	if sl := s.nameAskSource(k); sl != NoSlot && s.perms[sl].chosen != 255 {
		return objWord(s.perms[sl].chosen) + 1
	}
	if s.res.active {
		return objWord(s.res.nc)
	}
	return 0
}

// namedCardCard is PredNamedCard for the candidate card c off the
// battlefield (CR 201.4): it shares the reading source's chosen card name
// (gorge namePredicate NamedCard). A face-down card has no name (CR 708.2a).
func (s *State) namedCardCard(c CardID, fd bool, ctx *evalCtx) bool {
	if fd {
		return false
	}
	n := s.chosenNameCard(ctx)
	return n != NoCard && n == c
}
