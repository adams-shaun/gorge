package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// manifest, cloak and manifest dread (CR 701.40, 701.58,
// 701.62), and the turn-face-up procedures a manifested or cloaked permanent
// offers (CR 701.40b-d, 701.58b-d). Cold: reached from execOblige and the
// priority walk only when the table has one of the ops (genPrep.anyFaceDown).

// Provenance marks. perm.flags has no free bit (13-14 are pfRegenX, cleared
// every cleanup; 15 is pfSkipUntap), and object.of's eight bits are taken
// (ofHeld is bit 7, not free). A face-down permanent's perm.gty holds only
// its derived face-down types (TCreature) plus gtyFaceDown, so the two
// highest type bits double as provenance there: bit 13 (TToken) means
// manifested, bit 14 (TVehicle) cloaked. typesAt/loseStatOn mask both when
// reading gty as types, refreshTy preserves them, and clearFaceDown zeroes
// gty. No generated table derives TToken/TVehicle in an override, and a
// face-up permanent never reads these marks. A face-down permanent with
// neither mark was cast with Morph/Megamorph/Disguise.
const (
	gtyManifested kop.TypeMask = 1 << 13 // TToken while face down
	gtyCloaked    kop.TypeMask = 1 << 14 // TVehicle while face down
	gtyProvMask                = gtyManifested | gtyCloaked
)

// markFaceDown records the provenance of the face-down permanent at sl.
func (s *State) markFaceDown(sl Slot, cloaked bool) {
	if cloaked {
		s.perms[sl].gty |= gtyCloaked
		return
	}
	s.perms[sl].gty |= gtyManifested
}

// manifested and cloaked report the provenance marks.
func (s *State) manifested(sl Slot) bool { return s.perms[sl].gty&gtyManifested != 0 }
func (s *State) cloaked(sl Slot) bool    { return s.perms[sl].gty&gtyCloaked != 0 }

// execDread runs the manifest/cloak ops (execOblige's default branch).
// done=false means the op is not one of these; false ok means a dread ask is
// posed.
//
//go:noinline
func (s *State) execDread(k *prep, in *kop.Instr) (ok, done bool) {
	switch in.Op {
	case kop.OpManifest:
		s.manifest(k, in)
		return true, true
	case kop.OpCloak:
		s.cloak(k, in)
		return true, true
	case kop.OpManifestDread:
		return s.manifestDread(k, in), true
	}
	return s.execDefined(k, in) // filter_defined.go
}

// manifest is OpManifest (gorge effManifest): player Obj manifests A cards
// from the top of their library (CR 701.40a) -- the real card moves face down
// and is marked manifested. No event when A <= 0 or the library is empty.
func (s *State) manifest(k *prep, in *kop.Instr) {
	if in.B.N&kop.CfManifestRemembered != 0 {
		s.faceDownRemembered(k, 0, false)
		return
	}
	p := s.playerRef(k, in.Obj)
	if p == 255 {
		p = s.res.e.ctrl
	}
	s.faceDownFromTop(k, p, p, s.value(k, in.A), false, false, false)
}

// cloak is OpCloak (gorge effCloak): player Obj cloaks A cards from the top of
// their library (CR 701.58a) -- face down, marked cloaked (ward {2}), with the
// Tapped$/Shuffle$/RememberCloaked$ riders in B.N. CfChosen instead cloaks the
// resolution's chosen card from where it sits (gorge effCloak's Choices$ hand
// pick); CfCloakCtrl keeps Obj as the library's owner but the cards enter
// under the resolving controller (gorge's cloak(t.Obj, c.Controller), CR
// 110.2a).
func (s *State) cloak(k *prep, in *kop.Instr) {
	fl := int(in.B.N)
	if fl&kop.CfCloakRemembered != 0 {
		s.cloakRemembered(k, fl)
		return
	}
	if fl&kop.CfChosen != 0 {
		s.cloakChosen(k, fl)
		return
	}
	p := s.playerRef(k, in.Obj)
	if p == 255 {
		p = s.res.e.ctrl
	}
	ctrl := p
	if fl&kop.CfCloakCtrl != 0 {
		// The cloaking player puts the card onto the battlefield, so it
		// enters under THAT player's control (CR 110.2a), even though the
		// card leaves the library's owner's library.
		ctrl = s.res.e.ctrl
	}
	s.faceDownFromTop(k, p, ctrl, s.value(k, in.A), true,
		fl&kop.CfTapped != 0, fl&kop.CfRemember != 0)
}

// chooseFaceDownPick is OpChooseCard C: the cloak-from-hand pick (gorge
// effCloak's chooseFromZoneForFaceDown, its option kind "face_down_choice",
// CR 701.58a): the ask is the mandatory single pick OpChooseCard already
// poses, and an empty pool is no ask (gorge's n <= 0 returns no ids).
const chooseFaceDownPick = 7

// cloakChosen is OpCloak with CfChosen: the resolution's chosen card (the
// preceding OpChooseCard's pick, gorge chooseFromZoneForFaceDown) cloaks from
// where it sits under the resolving controller (CR 110.2a: the cloaking
// player puts the card onto the battlefield). Nothing moves when no card was
// picked (an empty pool) or the object is already on the battlefield (gorge's
// cloak closure skips a battlefield object, and every corpus shape sources
// from hand/library/exile). A full battlefield stalls and stops the move.
func (s *State) cloakChosen(k *prep, fl int) {
	o := s.objRefObj(kop.RefChosen)
	if o == NoObj {
		return
	}
	if ob := &s.objs[o]; ob.zone == kop.ZoneBattlefield {
		return
	}
	if k.gen != nil && s.occ.full() {
		s.stall(StallBoard)
		return
	}
	from := s.detach(o)
	sl := s.enterFaceDown(k, o, s.res.e.ctrl, kop.MorphMorph, from)
	s.markFaceDown(sl, true)
	s.faceDownRiders(k, o, sl, fl&kop.CfTapped != 0, fl&kop.CfRemember != 0)
}

// cloakRemembered is OpCloak with CfCloakRemembered (gorge effCloak's
// Defined$ Remembered arm): each card of the remembered set cloaks from where
// it sits under the resolving controller, in set order. A battlefield object
// is skipped (gorge's cloak closure), a full battlefield stalls and stops.
// CfShuffle is gorge's pile shuffle, a no-op: no shuffle, no RNG draw. The
// count is fixed up front because CfRemember can append to the set.
func (s *State) cloakRemembered(k *prep, fl int) {
	s.faceDownRemembered(k, fl, true)
}

// faceDownRemembered is cloakRemembered's walk for either mark: a manifest of
// the remembered set (gorge effManifest's Defined$ Remembered arm,
// manifestObjects: each remembered card under the resolving controller, a
// battlefield card skipped) differs only in the provenance mark.
func (s *State) faceDownRemembered(k *prep, fl int, cloaked bool) {
	n := int(s.g.nrem)
	for i := 0; i < n; i++ {
		o := s.g.rem[i]
		if o == NoObj || s.objs[o].zone == kop.ZoneBattlefield {
			continue
		}
		if k.gen != nil && s.occ.full() {
			s.stall(StallBoard)
			return
		}
		from := s.detach(o)
		sl := s.enterFaceDown(k, o, s.res.e.ctrl, kop.MorphMorph, from)
		s.markFaceDown(sl, cloaked)
		s.faceDownRiders(k, o, sl, fl&kop.CfTapped != 0, fl&kop.CfRemember != 0)
	}
}

// faceDownRiders applies the cloak/manifest entry riders to the face-down
// permanent at sl: tapped enters tapped and fires the taps triggers as the
// MoveZone-then-Tap pair gorge emits; remember joins the resolution's
// remembered set.
func (s *State) faceDownRiders(k *prep, o Obj, sl Slot, tapped, remember bool) {
	if tapped {
		s.perms[sl].flags |= pfTapped
		s.touchSl(sl)
		if k.gen != nil && s.subs[kop.TrigTaps].any() {
			s.fireTaps(k, sl)
		}
	}
	if remember {
		s.remember(o)
	}
}

// faceDownFromTop moves up to n cards from the top of player p's library onto
// the battlefield face down for ctrl, marked with mark. A full battlefield
// stalls and stops the move (the capacity draw). Tapped fires the taps
// triggers as the MoveZone-then-Tap pair gorge emits; remember joins the
// resolution's remembered set. Cloak's Shuffle$ is gorge's pile shuffle: it
// shuffles no library and draws no RNG.
func (s *State) faceDownFromTop(k *prep, p, ctrl uint8, n int, cloaked, tapped, remember bool) {
	p &= 1
	if n <= 0 {
		return
	}
	pl := &s.pl[p]
	for ; n > 0 && pl.libLo < pl.libHi; n-- {
		if k.gen != nil && s.occ.full() {
			s.stall(StallBoard)
			break
		}
		o := pl.lib[pl.libLo]
		s.detach(o)
		sl := s.enterFaceDown(k, o, ctrl, kop.MorphMorph, kop.ZoneLibrary)
		s.markFaceDown(sl, cloaked)
		s.faceDownRiders(k, o, sl, tapped, remember)
	}
}

// manifestDread is OpManifestDread (gorge effManifestDread, CR 701.62a):
// repeat A times -- look at the top two cards of the player's library,
// manifest one (the ask) and put the other into its owner's graveyard. One
// card left manifests without an ask. false: an ask is posed.
func (s *State) manifestDread(k *prep, in *kop.Instr) bool {
	p := s.playerRef(k, in.Obj)
	if p == 255 {
		if in.Obj == kop.RefRememberedOwner {
			// gorge definedSpecRememberedOwner over an empty remembered
			// set (the ETB exiled nothing): nobody manifests dread.
			return true
		}
		p = s.res.e.ctrl
	}
	n := s.value(k, in.A)
	if n <= 0 {
		return true
	}
	s.res.askP = p & 1
	s.res.left = uint8(min(n, 255))
	return s.dreadAsk(k)
}

// dreadAsk drives the single-card shortcut and poses the two-card ask.
func (s *State) dreadAsk(k *prep) bool {
	for s.res.left > 0 {
		pl := &s.pl[s.res.askP]
		if pl.libLo >= pl.libHi {
			s.res.ask = raNone
			return true
		}
		if pl.libHi-pl.libLo == 1 {
			if k.gen != nil && s.occ.full() {
				s.stall(StallBoard)
				s.res.ask = raNone
				return true
			}
			o := pl.lib[pl.libLo]
			s.detach(o)
			sl := s.enterFaceDown(k, o, s.res.askP, kop.MorphMorph, kop.ZoneLibrary)
			s.markFaceDown(sl, false)
			if s.prog(k)[s.res.pc].B.N&kop.CfRemember != 0 {
				s.remember(o)
			}
			s.res.left--
			// the one-card window's marker (gorge manifestDreadMove): the
			// action's ID list is empty, so TrigManifestDread fires with no
			// trigger object
			s.fireKwActObj(k, kop.TrigManifestDread, s.res.askP, 0, NoObj)
			continue
		}
		s.res.peek[0] = pl.lib[pl.libLo]
		s.res.peek[1] = pl.lib[pl.libLo+1]
		s.res.peekN = 2
		s.res.ask = raGenManifestDread
		return false
	}
	s.res.ask = raNone
	return true
}

// dreadMove applies one answered manifest-dread window: chosen manifests, the
// other card goes to its owner's graveyard (CR 701.62a). Both cards are still
// in the library; a stale answer falls back to the first card. True: the other
// card's move is parked for its CR 616.1 exile-order ask (repl_exilemark.go),
// its marker already emitted; the answer moves it and goes on with the window.
func (s *State) dreadMove(k *prep, chosen, other Obj) bool {
	p := s.res.askP
	if chosen != s.res.peek[0] && chosen != s.res.peek[1] {
		chosen, other = s.res.peek[0], s.res.peek[1]
	}
	if other == chosen {
		if chosen == s.res.peek[0] {
			other = s.res.peek[1]
		} else {
			other = s.res.peek[0]
		}
	}
	if ob := &s.objs[chosen]; ob.zone == kop.ZoneLibrary {
		if k.gen != nil && s.occ.full() {
			s.stall(StallBoard)
			return false
		}
		s.detach(chosen)
		sl := s.enterFaceDown(k, chosen, p, kop.MorphMorph, kop.ZoneLibrary)
		s.markFaceDown(sl, false)
		if s.prog(k)[s.res.pc].B.N&kop.CfRemember != 0 {
			s.remember(chosen)
		}
	}
	mill := NoObj
	if ob := &s.objs[other]; ob.zone == kop.ZoneLibrary {
		if s.park(k, other, parkDread, 0) {
			// The window's marker still fires now: gorge emits it after the
			// MoveZone that parked, before the answer (face_manifest.go's
			// mirror of manifestDreadMove). The card stays on the library
			// list until the answer moves it.
			s.fireKwActObj(k, kop.TrigManifestDread, p, 0, other)
			return true
		}
		from := s.detach(other)
		s.placeFrom(other, kop.ZoneGraveyard, false, from, false)
		mill = other
	}
	// the window's marker (gorge manifestDreadMove: emitted after the moves,
	// its ID list the card the window put into its owner's graveyard, empty
	// when neither card moved): the Mode$ ManifestDread trigger's firing event
	s.fireKwActObj(k, kop.TrigManifestDread, p, 0, mill)
	return false
}

// cardMintsFaceDown reports whether a card's programs contain an op that puts
// a face-down permanent onto the battlefield (Manifest, Cloak,
// ManifestDread), so genPrep.anyFaceDown turns on the face-down walk for a
// table with no Morph-family card.
func cardMintsFaceDown(c *kop.CardDef) bool {
	for i := range c.Abilities {
		p := c.Abilities[i].Prog
		for j := range p {
			switch p[j].Op {
			case kop.OpManifest, kop.OpCloak, kop.OpManifestDread:
				return true
			}
		}
	}
	return false
}
