package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Miss Highwater's loss copy: "for as long as they have a contract counter,
// when they lose the game, for each artifact and creature they controlled,
// create a token that's a copy of it", the Effect its damage trigger makes for
// the player it contracts. The Effect is per seat here: owed[p] is the seat
// whose Effect owes p's loss copy (that seat plus one, 0 for none), set when
// the trigger's put gives p a contract counter with the ext op's C set
// (putContract; kgen's contractLossEffect arms the put when the same chain
// carries the LosesGame Effect).
//
// Gorge's LosesGame trigger goes on the stack at the loss and resolves after
// the CR 800.4a sweep, so the kernel stages the copy the same way: leaveGame
// snapshots p's artifact/creature cards and enqueues an abDelayed trigger
// controlled by the Effect's controller, and resolveDelayed mints the copies
// from that snapshot when it resolves. Copying directly in leaveGame put the
// tokens on the board before gorge's trigger resolved and diverged at the
// first priority of the pod lockstep.
//
// The Effect's Static$ Always self-exile once p has no contract counter is
// not modelled: a contract counter is never removed in the corpus
// (/usr/bin/grep -rl Contract .cards/cardsfolder), so the record is never
// cleared by a count that falls to zero. leaveGame clears it as it fires.
//
// The records are cold arrays of one element per seat, kept only when
// genPrep.anyContract (the contract counts' own condition).

// contractLossTrigAmt is the abDelayed stack entry's trigAmt marking Miss
// Highwater's staged loss copy. Negative markers are the source-less delayed
// triggers' own (speed -1, venture -2, monarch -3/-4, rad -5, ring -6..-8,
// extra turn -9), so -10 is the next free one.
const contractLossTrigAmt int16 = -10

type contractOwed struct {
	ctrl uint8
	src  Obj // the Effect's source (the LosesGame trigger's source)
}

var coldContractOwed = newCold[contractOwed]("contractOwed", func(g *genPrep) int {
	if g.anyContract {
		return MaxSeats
	}
	return 0
})

// contractMod is one staged modified permanent's copiable values (CR 707.2):
// the card it is, its copy basis (ovr.copyOf/ovCopy, CR 707.3) and its
// layer-1 copy exceptions and as-enters copy grants (CR 707.9b). The
// immediate copy read these from the live template slot; after the CR 800.4a
// sweep the slot is gone, so the sweep keeps them here. Nothing else a
// continuous effect changed is staged: gorge mints the LKI copy from the
// departed object's printed card and folded CopyFace alone
// (engine/events/apply_copy.go copyTokenBasis), and the continuous effects on
// it ended when it left the battlefield (CR 611.2), so the record carries no
// P/T, type, keyword, colour or lose-abilities override -- the token's
// continuous state is what applies to it on the board. A face-down permanent's
// face-down copiable values (CR 708.2a) are not staged either: the kernel
// derives them from the fd mark exactly as copyTokenEnterFrom's face-down
// branch derives a live template's. The mint basis is cardAt, the same card
// the live mint's newToken takes (the hidden card for a plain face-down
// permanent); gorge mints its copy on the empty faceDownCopyCard over
// CopiableFace's face-down set (copyTokenBasis reads CopiesFaceDown first),
// so its token never carries the hidden card, and the kernel's face-down mark
// is what hides it here. Pointer-free, so the cold record stays forkable and
// the transition loop allocates nothing.
type contractMod struct {
	card CardID   // perms[sl].card, the card it is (not the face it shows)
	ovr  override // its copy basis (flags ovCopy, copyOf only)
	// fd is 0 when the permanent was face up, else 1 + its provenance
	// (gtyProvMask >> 13): 1 a plain face-down permanent (a Morph-family
	// cast), 2 manifested, 3 cloaked, 4 both (a permanent copy of one).
	// Gorge mints the copy on its empty faceDownCopyCard over
	// CopiableFace's face-down set (copyTokenBasis); the kernel mints on
	// the snapshot basis with copyTokenEnterFrom's face-down branch, which
	// derives the same values from gtyFaceDown and the walk and keeps the
	// provenance for faceDownWard (a cloak's ward {2}, CR 701.58a; a
	// manifested card's none, CR 701.40a).
	fd   uint8
	nexc uint8 // copy exception records (Table.Overrides indexes)
	exc  [maxOvr]uint8
	ng   uint8 // as-enters copy grant records: (override, maker card) pairs
	gr   [maxOvr]cgrantRef
}

// maxContractMods caps the modified permanents a departing seat stages. The
// budget is the State bar (TestEveryPoolFitsTheStateBar): only a permanent a
// plain card copy cannot describe takes a record, and a seat over the cap
// stalls (a declared capacity limit) rather than dropping copiable values.
const maxContractMods = 32

// contractPending is p's staged loss copy: the Effect's controller and, in the
// copy body's selector order, the cards of the artifacts and creatures p
// controlled when it left. cards holds each permanent's copiable face
// (cardAt); mod[i] is 1 + its contractMod index when a plain copy of that card
// is not enough (a copy basis or a copy exception).
// The CR 800.4a sweep takes those permanents before the trigger resolves, so
// the snapshot is the copy's last-known basis.
type contractPending struct {
	ctrl  uint8
	n     uint8
	nm    uint8
	cards [MaxPerm]CardID
	mod   [MaxPerm]uint8
	mods  [maxContractMods]contractMod
}

var coldContractPending = newCold[contractPending]("contractPending", func(g *genPrep) int {
	if g.anyContract {
		return MaxSeats
	}
	return 0
})

// contractOwedBy is the seat whose Effect owes seat p's loss copy and the
// Effect's source, ok false when none does.
func (s *State) contractOwedBy(p uint8) (c uint8, src Obj, ok bool) {
	if o := coldContractOwed.of(s); o != nil && p < MaxSeats && o[SeatIx(p)].ctrl != 0 {
		return o[SeatIx(p)].ctrl - 1, o[SeatIx(p)].src, true
	}
	return 0, NoObj, false
}

// setContractOwed records that controller c's Effect, sourced at src, owes
// seat p's loss copy.
func (s *State) setContractOwed(p, c uint8, src Obj) {
	if o := coldContractOwed.of(s); o != nil && p < MaxSeats && c < MaxSeats {
		o[SeatIx(p)] = contractOwed{ctrl: c + 1, src: src}
	}
}

// contractSourceLeft drops every loss-copy debt whose Effect source just left
// the battlefield. The Effect's Duration$ Permanent lifetime is bounded by its
// source's battlefield incarnation (gorge delayedRegistrationLive's
// SourceBattlefield case), so after Miss Highwater leaves, a player's lingering
// contract counter owes no copy; a later loss must not enqueue a trigger gorge
// no longer has. A staged trigger already on the stack is unaffected (CR 603.7:
// the registration is independent once it fired).
func (s *State) contractSourceLeft(src Obj) {
	owed := coldContractOwed.of(s)
	if owed == nil {
		return
	}
	for i := range owed {
		if owed[i].ctrl != 0 && owed[i].src == src {
			owed[i] = contractOwed{}
		}
	}
}

// contractLossCopy stages seat p's owed loss copy as p leaves the game: it
// snapshots the artifacts and creatures p controls in the copy body's selector
// order and enqueues the abDelayed trigger that will mint the copies. The
// effect's controller (a seat that left too gets none) is the trigger's
// controller.
func (s *State) contractLossCopy(k *prep, p uint8) {
	c, src, ok := s.contractOwedBy(p)
	if !ok {
		return
	}
	coldContractOwed.of(s)[SeatIx(p)] = contractOwed{}
	if s.out(c) {
		return
	}
	pending := coldContractPending.of(s)
	if pending == nil {
		return
	}
	rec := &pending[SeatIx(p)]
	rec.ctrl, rec.n = c, 0
	// Gorge resolves Miss Highwater's
	// `Defined$ TriggeredLastStateBattlefieldLKICopy.Creature+...,Artifact+...`
	// group by group, each in battlefield-zone order (effects/lost_board.go:
	// the union is built in script order, and a permanent matching two groups
	// is listed once). The kernel's battlefield order is perm.ts (zones.go
	// joined: entry order, a control change moves it to the end), not
	// slot-mask or object-index order. Sort the snapshot with the selector's
	// order: creatures first, then artifacts that are not creatures, each in
	// ts order, so the copies bind the same object indices as gorge's.
	var ts [MaxPerm]uint16
	var creature [MaxPerm]bool
	n := 0
	for m := s.ctrlM[SeatIx(p)].and(s.occ); m.any(); m = m.rest() {
		sl := m.first()
		ty := s.typesAt(k, sl)
		if ty&(kop.TCreature|kop.TArtifact) == 0 {
			continue
		}
		if n >= len(rec.cards) {
			s.stall(StallBoard)
			return
		}
		crt := ty&kop.TCreature != 0
		t := s.perms[sl].ts
		basis := s.cardAt(sl)
		mi := uint8(0)
		if m, ok := s.contractModAt(sl); ok {
			if int(rec.nm) >= len(rec.mods) {
				s.stall(StallBoard)
				return
			}
			rec.mods[rec.nm] = m
			rec.nm++
			mi = rec.nm // 1-based, 0 = a plain card copy
		}
		j := n
		for j > 0 {
			prevCrt := creature[j-1]
			if !crt && prevCrt {
				break // a non-creature sorts after every creature
			}
			if prevCrt == crt && ts[j-1] <= t {
				break
			}
			ts[j], creature[j], rec.cards[j], rec.mod[j] = ts[j-1], creature[j-1], rec.cards[j-1], rec.mod[j-1]
			j--
		}
		ts[j], creature[j], rec.cards[j], rec.mod[j] = t, crt, basis, mi
		n++
	}
	rec.n = uint8(n)
	// A contracted loser who controls no artifact or creature still puts the
	// LosesGame trigger on the stack (it resolves doing nothing), so enqueue
	// even with an empty snapshot.
	ver, card := uint8(0), CardID(0)
	if src != NoObj && int(src) < int(s.nobj) {
		ver, card = s.objs[src].ver, s.objs[src].card
	}
	s.enqueue(src, ver, card, abDelayed, c, NoObj, p, contractLossTrigAmt)
}

// contractModAt snapshots the copiable values of the permanent at sl that a
// plain copy of its printed card cannot show, ok false when there are none.
// The copy basis is ovr.copyOf under ovCopy (cardAt's own read, CR 707.3); the
// copy exceptions are the layer-1 fxCopyExc records the live template mint
// (copyExcAdd) would have read (CR 707.9b); the as-enters copy grants are the
// fxOverride ovCopyGrant records the live mint's copyGrantsAdd carries from a
// copied copy (Mercurial Pretender's AddAbilities$, CR 707.9b, 707.3: gorge
// folds the grant into the copied face, engine/effects/clone_etb_sub.go
// cloneGrantSVars, so the LKI copy carries it too); a face-down permanent's
// face-down copiable values are the fd mark (CR 708.2a), which the mint
// derives as copyTokenEnterFrom's face-down branch does. No override flag,
// keyword, generated type or walk mark is staged: the continuous effects that
// wrote them ended when the permanent left the battlefield (CR 611.2), so
// gorge's LKI mint reads none of them either, and an until-end-of-turn one is
// no copiable value even before that (CR 707.2).
func (s *State) contractModAt(sl Slot) (contractMod, bool) {
	p := &s.perms[sl]
	if p.ovr.flags&ovCopy == 0 && !s.fxM.has(sl) && p.gty&gtyFaceDown == 0 {
		return contractMod{}, false
	}
	c := contractMod{card: p.card}
	if p.ovr.flags&ovCopy != 0 {
		c.ovr = override{flags: ovCopy, copyOf: p.ovr.copyOf}
	}
	if p.gty&gtyFaceDown != 0 {
		c.fd = 1 + uint8(p.gty&gtyProvMask>>13)
	}
	if s.fxM.has(sl) {
		for i := 0; i < int(s.g.nfx) && int(c.nexc) < maxOvr; i++ {
			if r := &s.g.fx[i]; r.sl == sl && r.kind == fxCopyExc {
				c.exc[c.nexc] = r.r.idx
				c.nexc++
			}
		}
		c.ng = uint8(s.copyGrantRecs(sl, &c.gr))
	}
	if c.ovr.flags&ovCopy == 0 && c.nexc == 0 && c.ng == 0 && c.fd == 0 {
		return contractMod{}, false
	}
	return c, true
}

// contractLossCopyResolve mints the copies staged for seat p when the
// LosesGame trigger resolves. A different controller (a stale staged record)
// or an Effect controller that has left the game makes none.
func (s *State) contractLossCopyResolve(k *prep, ctrl, p uint8) {
	pending := coldContractPending.of(s)
	if pending == nil || p >= MaxSeats || ctrl >= MaxSeats {
		return
	}
	rec := &pending[SeatIx(p)]
	if rec.ctrl != ctrl || rec.n == 0 {
		return
	}
	n := rec.n
	rec.n, rec.nm = 0, 0
	if s.out(ctrl) {
		return
	}
	for i := uint8(0); i < n; i++ {
		mi := rec.mod[i]
		if mi == 0 {
			if s.mintTokenCopyCard(k, ctrl, rec.cards[i]) {
				return // the board is full (StallBoard)
			}
			continue
		}
		if s.mintTokenCopyStaged(k, ctrl, rec.cards[i], &rec.mods[mi-1]) {
			return
		}
	}
}

// mintTokenCopyCard mints a plain token that is a copy of card c, controlled
// by p (a permanent whose copiable values are exactly the card's).
func (s *State) mintTokenCopyCard(k *prep, p uint8, c CardID) bool {
	if s.nearCapacity(k) {
		s.stall(StallBoard)
		return true
	}
	t := s.newToken(c, p)
	s.objs[t].of |= ofCopy
	var in kop.Instr
	s.copyTokenEnterFrom(k, &in, t, NoSlot, NoSlot, nil)
	return false
}

// mintTokenCopyStaged mints a token that is a copy of the modified permanent c
// snapshotted at the CR 800.4a sweep, controlled by p. The host permanent has
// already left the battlefield, so the token is entered from c's copiable face
// and then given c's card, copy basis, copy exceptions and copy grants. The enters check is
// held until those values are in place, as the live template mint applies them
// before its own ETB triggers. A face-down c (fd != 0) enters through the
// face-down branch of copyTokenEnterFd, given the departed template's mark:
// the face-down characteristics ride the enter's face and the walk,
// the provenance bits and the spareCopyFaceDown copy mark carry over (CR
// 707.2, 708.2a, 701.58a), and the token is face up (projFaceDown).
func (s *State) mintTokenCopyStaged(k *prep, p uint8, basis CardID, c *contractMod) bool {
	if s.nearCapacity(k) {
		s.stall(StallBoard)
		return true
	}
	t := s.newToken(basis, p)
	s.objs[t].of |= ofCopy
	var in kop.Instr
	var eb entBatch
	s.copyTokenEnterFd(k, &in, t, NoSlot, NoSlot, &eb, c.fd)
	if sl := s.objs[t].slot; s.objs[t].zone == kop.ZoneBattlefield && s.occ.has(sl) {
		pm := &s.perms[sl]
		pm.card, pm.ovr = c.card, c.ovr
		if c.fd != 0 {
			// the face-down derivation enterNF's face branch put on the
			// token: a face-down permanent's copiable types ride ovGenTy
			// and the walk (face_facedown.go), and the record's copy basis
			// is only the ovr.copyOf read cardAt makes
			pm.ovr.flags |= ovGenTy
			pm.ovr.walk = walkOn
		}
		s.objs[t].card = c.card
		for j := uint8(0); j < c.nexc; j++ {
			s.copyExcPush(sl, c.exc[j])
		}
		if c.nexc != 0 {
			pm.ovr.kw |= s.copyExcKw(k, sl)
		}
		// the as-enters copy grants, appended as the live mint's copyGrantsAdd
		// applies them, each stamped anew (CR 707.9b)
		for j := uint8(0); j < c.ng; j++ {
			s.copyGrantAppend(k, sl, c.gr[j])
		}
		s.touchSl(sl)
		s.refreshTy(k) // the exceptions' added types ride gty (ovGenTy), derived, not carried
	}
	s.fireOrHoldEntries(k, &eb)
	return false
}
