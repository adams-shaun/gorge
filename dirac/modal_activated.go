package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A modal activated ability announces its mode as it is activated (CR 602.2b): the
// bsModes stage of buildProgram's seAbility row, before X, the targets and the costs.
// One choosing more than one mode has no stage there and asks at resolution:
// resolveTop calls modalActAsk before the program runs, effCharm's order in gorge.
// Cold and out of line: only a modal ability ever gets here.

const (
	raGenModalMode   = raGenManifestDread + 1 // the resolving modal ability's modes (DkModes, AkMode)
	raGenModalTarget = raGenManifestDread + 2 // a chosen mode's target slot (res.left = the slot; ChGenResTarget)
)

// modalActAsk starts the mode ask of the resolving modal activated ability
// a; false: nothing to ask.
//
//go:noinline
func (s *State) modalActAsk(k *prep, a *kop.Ability) bool {
	if a.NModes == 0 || s.res.e.mode != 0 {
		return false
	}
	if k.gen != nil && k.gen.anyCharmPick && s.charmPickAllBarred(a, s.resSrcSlot(k)) {
		return false // every mode was chosen before: nothing happens (gorge CharmEligibleModes; modal_charmpick.go)
	}
	s.res.ask, s.res.askP, s.res.chosen = raGenModalMode, s.res.e.ctrl, 0
	s.ctl = ctlResolve
	return true
}

// modalActAtResolution: the ability chooses more than one mode.
func (s *State) modalActAtResolution(a *kop.Ability, pick uint8) bool {
	return pick > 1
}

func (s *State) modalAb(k *prep) *kop.Ability {
	return &k.t.Cards[s.res.e.card].Abilities[s.res.e.ab]
}

// modalBounds is the pick bounds of target slot t (multiTargetStep's, minus
// the kicked / gift / same-controller ranges no modal ability carries).
func modalBounds(t *kop.TargetSpec) (lo, hi uint8) {
	lo, hi = 1, 1
	if t.Min|t.Max != 0 {
		lo, hi = t.Min, t.Max
	}
	if t.UpTo {
		lo = 0
	}
	return
}

// modalCands lists slot r.left's candidates not picked yet.
func (s *State) modalCands(k *prep, out *[MaxOptions]uint8) int {
	r := &s.res
	var all [MaxOptions]uint8
	n := s.targetCands(k, &s.modalAb(k).Targets[r.left], r.e.ctrl, s.resSrcSlot(k), r.e.obj, &all)
	m := 0
	x := s.xtOf(&r.e)
	for i := 0; i < n; i++ {
		c := all[i]
		if r.peekI > 0 && r.e.tgt[r.left] == c {
			continue
		}
		dup := false
		for j := 0; j < int(x.n); j++ {
			dup = dup || x.slot[j] == r.left && x.code[j] == c
		}
		if !dup {
			out[m] = c
			m++
		}
	}
	return m
}

// modalNextTarget poses the next pick of the chosen modes' target slots
// from res.left on (res.peekI picks of the slot so far); gorge asks no empty
// target, and a multi-object slot closes with no done-only ask once nothing
// is left (unlike the cast-time slot). false: no slot is left, the program runs.
func (s *State) modalNextTarget(k *prep) bool {
	r := &s.res
	a := s.modalAb(k)
	for ; int(r.left) < int(a.NTargets); r.left, r.peekI = r.left+1, 0 {
		t := &a.Targets[r.left]
		if tm := a.TgtModes[r.left]; tm == 0 || tm&r.e.mode == 0 || t.Kind == kop.TgtNone || t.TFlags&kop.TgfAtResolution != 0 {
			continue
		}
		_, hi := modalBounds(t)
		var buf [MaxOptions]uint8
		n := s.modalCands(k, &buf)
		if r.peekI >= hi || n == 0 {
			continue
		}
		r.ask, r.askP = raGenModalTarget, r.e.ctrl
		return true
	}
	return false
}

// askOptionsModalAct builds the modal-activation asks' options; false: not one.
//
//go:noinline
func (s *State) askOptionsModalAct(k *prep, d *Decision) bool {
	r := &s.res
	switch r.ask {
	case raGenModalMode:
		a := s.modalAb(k)
		d.Kind = DkModes
		for m := 0; m < int(a.NModes); m++ {
			if r.e.mode&(1<<m) == 0 && !(k.gen != nil && k.gen.anyCharmPick && s.charmPickBarred(a, s.resSrcSlot(k), m)) {
				d.add(MakeAction(AkMode, 0, 0, uint8(m)))
			}
		}
	case raGenModalTarget:
		lo, _ := modalBounds(&s.modalAb(k).Targets[r.left])
		var buf [MaxOptions]uint8
		n := s.modalCands(k, &buf)
		d.Sub = ChGenResTarget
		for i := 0; i < n; i++ {
			d.add(MakeAction(AkChoose, TCode(buf[i]), 0, 0))
		}
		if r.peekI >= lo {
			d.add(MakeAction(AkDone, 0, 0, 0))
		}
	default:
		return s.askOptionsUnread(k, d) || s.askOptionsCzParams(k, d) || s.askOptionsCzAttach(k, d) || s.askOptionsBearer(k, d)
	}
	return true
}

// applyAskModalAct applies an answer to a modal-activation ask; false: not one.
//
//go:noinline
func (s *State) applyAskModalAct(k *prep, a Action) bool {
	r := &s.res
	switch r.ask {
	case raGenModalMode:
		r.e.mode |= 1 << a.Mode()
		r.chosen++
		ab := s.modalAb(k)
		if k.gen != nil && k.gen.anyCharmPick {
			s.charmPickLog(ab, s.resSrcSlot(k), a.Mode()) // modal_charmpick.go
		}
		if r.chosen < max8(ab.ModesPick, 1) && int(r.chosen) < int(ab.NModes) {
			return true // the next mode
		}
		if k.gen != nil && k.gen.anyDilemma {
			s.faceDilemma(k, r.e.ctrl) // trig_dilemma.go
		}
		r.left, r.peekI = 0, 0
	case raGenModalTarget:
		i := int(r.left)
		if a.Kind() == AkChoose {
			c := a.Source()
			var ver uint8
			if isObjTarget(c) {
				ver = s.objs[c].ver
			}
			// gorge records the pick as the ability's TargetsChosen:
			// BecomesTarget / ward first, then the crime (checkWard's order)
			s.targetedOne(k, &r.e, uint8(c))
			if k.gen != nil && k.gen.crimeOn {
				s.crimeModalPick(k, &r.e, i, r.peekI, uint8(c))
			}
			if r.peekI == 0 {
				r.e.tgt[i], r.e.tver[i] = uint8(c), ver
				if r.e.nt <= uint8(i) {
					r.e.nt = uint8(i) + 1
				}
			} else if x := s.xtMut(&r.e); x == nil {
				return true
			} else if x.n < maxXTgt {
				x.slot[x.n], x.code[x.n], x.ver[x.n] = uint8(i), uint8(c), ver
				x.n++
			} else {
				s.capFull(StallXTgt)
			}
			r.bad &^= 1 << i
			r.peekI++
			if _, hi := modalBounds(&s.modalAb(k).Targets[i]); r.peekI < hi {
				r.ask = raNone
				if s.modalNextTarget(k) {
					return true
				}
				s.runProg(k)
				return true
			}
		} else if r.peekI == 0 && r.e.nt <= uint8(i) {
			r.e.nt = uint8(i) + 1
		}
		r.left, r.peekI = r.left+1, 0
	default:
		return s.applyAskUnread(k, a) || s.applyAskCzParams(k, a) || s.applyAskCzAttach(k, a) || s.applyAskBearer(k, a)
	}
	r.ask = raNone
	if !s.modalNextTarget(k) {
		s.runProg(k)
	}
	return true
}
