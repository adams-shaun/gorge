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
	p := s.playerRef(k, in.Obj)
	if p == 255 {
		p = s.res.e.ctrl
	}
	s.faceDownFromTop(k, p, s.value(k, in.A), false, false, false, false)
}

// cloak is OpCloak (gorge effCloak): player Obj cloaks A cards from the top of
// their library (CR 701.58a) -- face down, marked cloaked (ward {2}), with the
// Tapped$/Shuffle$/RememberCloaked$ riders in B.N.
func (s *State) cloak(k *prep, in *kop.Instr) {
	p := s.playerRef(k, in.Obj)
	if p == 255 {
		p = s.res.e.ctrl
	}
	fl := in.B.N
	s.faceDownFromTop(k, p, s.value(k, in.A), true,
		fl&kop.CfTapped != 0, fl&kop.CfShuffle != 0, fl&kop.CfRemember != 0)
}

// faceDownFromTop moves up to n cards from the top of player p's library onto
// the battlefield face down, marked with mark. A full battlefield stalls and
// stops the move (the capacity draw). Tapped fires the taps triggers as the
// MoveZone-then-Tap pair gorge emits; remember joins the resolution's
// remembered set; shuffle shuffles once after any card moved.
func (s *State) faceDownFromTop(k *prep, p uint8, n int, cloaked, tapped, shuffle, remember bool) {
	p &= 1
	if n <= 0 {
		return
	}
	pl := &s.pl[p]
	moved := false
	for ; n > 0 && pl.libLo < pl.libHi; n-- {
		if k.gen != nil && s.occ.full() {
			s.stall(StallBoard)
			break
		}
		o := pl.lib[pl.libLo]
		s.detach(o)
		sl := s.enterFaceDown(k, o, p, kop.MorphMorph, kop.ZoneLibrary)
		s.markFaceDown(sl, cloaked)
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
		moved = true
	}
	if shuffle && moved {
		s.shuffleLib(p)
	}
}

// manifestDread is OpManifestDread (gorge effManifestDread, CR 701.62a):
// repeat A times -- look at the top two cards of the player's library,
// manifest one (the ask) and put the other into its owner's graveyard. One
// card left manifests without an ask. false: an ask is posed.
func (s *State) manifestDread(k *prep, in *kop.Instr) bool {
	p := s.playerRef(k, in.Obj)
	if p == 255 {
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
// in the library; a stale answer falls back to the first card.
func (s *State) dreadMove(k *prep, chosen, other Obj) {
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
			return
		}
		s.detach(chosen)
		sl := s.enterFaceDown(k, chosen, p, kop.MorphMorph, kop.ZoneLibrary)
		s.markFaceDown(sl, false)
		if s.prog(k)[s.res.pc].B.N&kop.CfRemember != 0 {
			s.remember(chosen)
		}
	}
	if ob := &s.objs[other]; ob.zone == kop.ZoneLibrary {
		from := s.detach(other)
		s.placeFrom(other, kop.ZoneGraveyard, false, from, false)
	}
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
