package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Chosen-colour grants (Skrelv, Defector Mite; gorge
// effChooseColor, definedKWGrant and the CantBlockBy Effect's
// freezeChosenColor, cr-pump-definedkw-chosencolor 5026a0910): "Choose a
// colour. Another target creature you control gains toxic 1 and hexproof
// from that colour until end of turn. It can't be blocked by creatures of
// that colour this turn." ExtChooseColorRes asks the colour (ChGenColor)
// into res.chosen (index + 1); each ExtColorGrant then records, until the
// cleanup step, its object (obj, ver), the colour and the grant flags in
// genState.colG (a grant of the same object and colour merges). The
// readers: targetableGen (hexproof from the colour: a source of that colour
// an opponent controls can't target it, CR 702.11d), blockForbiddenGen (a
// blocker of that colour can't block it) and toxicAt (toxic N adds, gorge
// ToxicValue sums every instance). All behind genPrep.anyColGrant.

const raChooseColorRes uint8 = 244

// maxColG bounds a turn's grants. std-tmt reaches 4 at 300 pairs.
const maxColG = 6

type colGrant struct {
	obj Obj
	ver uint8
	col kop.ColorMask
	fl  uint8
}

const (
	cgHex     uint8 = 1 // hexproof from the colour
	cgNoBlock uint8 = 2 // can't be blocked by creatures of the colour
	cgToxic         = 2 // the toxic amount is fl >> cgToxic
)

// prepColGrant sets anyColGrant (and anyInfect for a toxic grant).
func prepColGrant(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Ext() == kop.ExtColorGrant {
					g.anyColGrant = true
					g.anyInfect = g.anyInfect || in.B.N > 0
					prepColProt(&in, g, i) // static_colourprot.go
				}
			}
		}
	}
}

// colGrant is ExtColorGrant.
func (s *State) colGrant(k *prep, in *kop.Instr) {
	if uint8(in.C)&cgProt != 0 {
		s.colProtGrant(k, in) // static_colourprot.go
		return
	}
	c := s.res.chosen
	sl := s.objRef(k, in.Obj)
	if c == 0 || c > 5 || sl == NoSlot {
		return
	}
	o := s.perms[sl].obj
	col := kop.ColorMask(1) << (c - 1)
	fl := uint8(in.C)&(cgHex|cgNoBlock) | uint8(min(s.value(k, in.B), 63))<<cgToxic
	free := -1
	for i := range s.g.colG {
		r := &s.g.colG[i]
		if r.col == col && r.obj == o && r.ver == s.objs[o].ver {
			r.fl = r.fl&(cgHex|cgNoBlock) | fl&(cgHex|cgNoBlock) | min(r.fl>>cgToxic+fl>>cgToxic, 63)<<cgToxic
			s.touchSl(sl)
			return
		}
		if r.col == 0 && free < 0 {
			free = i
		}
	}
	if free < 0 {
		s.capFull(StallColGrant) // loud: never drop a grant
		return
	}
	s.g.colG[free].obj, s.g.colG[free].ver, s.g.colG[free].col, s.g.colG[free].fl = o, s.objs[o].ver, col, fl
	s.touchSl(sl)
}

// cgOf is the permanent at sl's live grants: the colours it has hexproof
// from, the colours that can't block it, and its granted toxic.
func (s *State) cgOf(sl Slot) (hex, noBlock kop.ColorMask, toxic int) {
	o := s.perms[sl].obj
	for i := range s.g.colG {
		r := &s.g.colG[i]
		if r.col == 0 || r.obj != o || r.ver != s.objs[o].ver {
			continue
		}
		if r.fl&cgHex != 0 {
			hex |= r.col
		}
		if r.fl&cgNoBlock != 0 {
			noBlock |= r.col
		}
		toxic += int(r.fl >> cgToxic)
	}
	return hex, noBlock, toxic
}

// cgClear ends the grants (the cleanup step, CR 514.2).
func (s *State) cgClear() {
	for i := range s.g.colG {
		if r := &s.g.colG[i]; r.col != 0 {
			if s.objs[r.obj].zone == kop.ZoneBattlefield && s.objs[r.obj].ver == r.ver {
				s.touch(r.obj)
			}
			r.obj, r.ver, r.col, r.fl = 0, 0, 0, 0
		}
	}
}

// askOptionsColGrant builds the colour ask (the tail of askOptionsWardDisc).
//
//go:noinline
func (s *State) askOptionsColGrant(k *prep, d *Decision) bool {
	if s.res.ask != raChooseColorRes {
		return s.askOptionsEndure(k, d) // act_endure.go
	}
	d.Sub = ChGenColor
	for c := uint8(0); c < 5; c++ {
		d.add(MakeAction(AkChoose, 0, 0, c))
	}
	return true
}

// applyAskColGrant keeps the colour for the resolution.
//
//go:noinline
func (s *State) applyAskColGrant(k *prep, a Action) bool {
	if s.res.ask != raChooseColorRes {
		return s.applyAskEndure(k, a) // act_endure.go
	}
	s.res.chosen = a.Mode() + 1
	s.res.col = s.res.chosen // the ChosenColor filters' copy (static_colourprot.go resColour)
	s.continueRes(k, 0)
	return true
}
