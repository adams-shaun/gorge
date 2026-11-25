package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The dies-to-exile rider: a ReplExileOnLeave ability that carries a Prog (the
// sub-ability chain of "instead exile that card and create a 2/2 black Zombie
// creature token", Kalitas, Traitor of Ghet). gorge resolves the replacement's
// ChangeZone body and its linked SubAbility$ chain in one walk with the
// replacing permanent as the source (rules runReplaceWith), so the exile move
// happens first and the chain after it, "You" being the source's controller
// and ReplacedCard the exiled card.
//
// The kernel splits that over two points. dieZone decides the exile and, when
// exactly one source applies and it has a rider, records the source's identity
// on the dying slot (exileRiderMark; the cold region coldExRider, no State
// byte), while the batch's pre-move scan still sees it: a source may leave
// before a later member of the same batch moves (two opposing Kalitas dying
// together), and its slot may be reused by a token since. removePerm, which
// every death ends in, reads the record (exileRiderTake) and runs the source's
// prog once the card is in exile (exileRiderRun), in a synthetic resolution
// that names the source, so a mid-resolution caller's own resolution is
// restored after.
//
// Nothing here guesses: a CR 616.1 order (park) may hold one rider source
// beside replacements without a rider — the answer decides whether that
// replacement applies first, so only the chosen source's rider runs — but two
// or more competing rider sources stall, since one dying slot records one
// rider. A prog that asks or parks stalls (StallExileRider).

// useExileRider: some card's ReplExileOnLeave carries a rider prog; the
// table gets the cold mark region.
var useExileRider = newGenUse(tableHasExileRider)

func tableHasExileRider(t *kop.Table) bool {
	for i := range t.Cards {
		if exileRiderAb(&t.Cards[i]) >= 0 {
			return true
		}
	}
	return false
}

func init() { stallNames[StallExileRider] = "exilerider" }

// exRiderMark is one dying slot's rider record: the source slot + 1 (0 none)
// and the source's identity as the pre-move scan saw it — object, controller
// ("You" of the prog), object version and card. The source may be gone, or its
// slot reused, by the time the rider runs.
type exRiderMark struct {
	src  uint8  // the source slot + 1; 0 none
	obj  Obj    // the source's object
	ctrl uint8  // the source's controller
	ver  uint8  // the source object's ver at the mark
	card CardID // the source's card (last known information)
	pow  int16  // the replaced card's derived power before the move (ReplacedCardLKI$CardPower)
}

// coldExRider is the rider record per dying slot. It binds no view (a view is
// a slice header in State): the cold paths read it by of.
var coldExRider = newCold[exRiderMark]("exRider", func(g *genPrep) int {
	if g.uses.has(useExileRider) {
		return MaxPerm
	}
	return 0
}).perSlot()

// coldExRiderPow is the replaced card's power the running rider prog reads
// (kop.ExReplLKIPower). One scalar, saved and restored by exileRiderRun so a
// nested resolution cannot leak a capture. It adds no State bytes.
var coldExRiderPow = newCold[int16]("exRiderPow", func(g *genPrep) int {
	if g.uses.has(useExileRider) {
		return 1
	}
	return 0
})

// exileRiderAb is the index of cd's ReplExileOnLeave ability that carries a
// rider prog, or -1.
func exileRiderAb(cd *kop.CardDef) int {
	for i := range cd.Abilities {
		if a := &cd.Abilities[i]; a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplExileOnLeave && len(a.Prog) != 0 {
			return i
		}
	}
	return -1
}

// exileRiderOn reports whether the table has riders (the mark region exists).
func exileRiderOn(k *prep) bool {
	return k.gen != nil && k.gen.uses.has(useExileRider) && len(k.gen.exileOnLeave) != 0
}

// exileRiderReset clears sl's record: a death decision starts afresh, and a
// move that did not happen leaves none behind.
func (s *State) exileRiderReset(k *prep, sl Slot) {
	if !exileRiderOn(k) {
		return
	}
	if m := coldExRider.of(s); int(sl) < len(m) {
		m[sl] = exRiderMark{}
	}
}

// exileRiderMark records the rider source src of the death of sl that dieZone
// just replaced with exile; only a sole replacement (n 1) carries a rider (two
// or more stall before). The source is on the battlefield here (dieZone's
// scan runs before any member of the batch moves), so the record holds what
// the rider needs even after it leaves.
func (s *State) exileRiderMark(k *prep, sl Slot, n int, src Slot) {
	if n != 1 || src == NoSlot || !exileRiderOn(k) {
		return
	}
	m := coldExRider.of(s)
	if int(sl) >= len(m) {
		return
	}
	c := s.cardAt(src)
	if exileRiderAb(&k.t.Cards[c]) < 0 {
		return
	}
	sp := &s.perms[src]
	pw, _ := s.ptGen(k, sl)
	m[sl] = exRiderMark{src: uint8(src) + 1, obj: sp.obj, ctrl: sp.ctrl, ver: s.objs[sp.obj].ver, card: c, pow: int16(pw)}
}

// exileRiderTake is the rider record of the departure of sl to z (src 0 when
// none), clearing it. Only a move to exile runs it.
func (s *State) exileRiderTake(k *prep, sl Slot, z kop.Zone) exRiderMark {
	if !exileRiderOn(k) {
		return exRiderMark{}
	}
	marks := coldExRider.of(s)
	if int(sl) >= len(marks) || marks[sl].src == 0 {
		return exRiderMark{}
	}
	m := marks[sl]
	marks[sl] = exRiderMark{}
	if z != kop.ZoneExile {
		return exRiderMark{}
	}
	return m
}

// exileRiderAmong stalls (StallExileRider) and reports true when the objects
// srcs, the competing dies-to-exile sources of a parked move, carry more than
// one rider, or one that is not a battlefield permanent (it cannot be
// recorded on the dying slot). One battlefield rider beside plain
// replacements parks.
func (s *State) exileRiderAmong(k *prep, srcs []Obj) bool {
	if !exileRiderOn(k) {
		return false
	}
	rider, n := NoObj, 0
	for _, o := range srcs {
		if exileRiderAb(&k.t.Cards[s.objs[o].card]) >= 0 {
			rider, n = o, n+1
		}
	}
	if n > 1 || n == 1 && s.objs[rider].zone != kop.ZoneBattlefield {
		s.capFull(StallExileRider)
		return true
	}
	return false
}

// exileRiderParkMark records the parked move's single rider source on the
// dying slot, using the source's identity as the pre-move scan sees it, or
// clears the record when the move has none. The answer then keeps it only
// when that source is the one chosen (settleExileRider), and removePerm runs
// the rider after the move (exileRiderTake/exileRiderRun).
func (s *State) exileRiderParkMark(k *prep, o Obj, srcs []Obj) {
	if !exileRiderOn(k) {
		return
	}
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield {
		return
	}
	m := coldExRider.of(s)
	if int(ob.slot) >= len(m) {
		return
	}
	rider, n := NoObj, 0
	for _, so := range srcs {
		if s.objs[so].zone != kop.ZoneBattlefield || exileRiderAb(&k.t.Cards[s.objs[so].card]) < 0 {
			continue
		}
		rider, n = so, n+1
	}
	if n != 1 {
		m[ob.slot] = exRiderMark{}
		return
	}
	sp := &s.perms[s.objs[rider].slot]
	pw, _ := s.ptGen(k, ob.slot)
	m[ob.slot] = exRiderMark{src: uint8(s.objs[rider].slot) + 1, obj: sp.obj, ctrl: sp.ctrl, ver: s.objs[sp.obj].ver, card: s.objs[rider].card, pow: int16(pw)}
}

// settleExileRider keeps the parked move's recorded rider only when the CR
// 616.1 answer (mode) chose that source: a mark answer, or any other source,
// applies its effect first and the rider's replacement never does.
func (s *State) settleExileRider(k *prep, r parkedMove, mode int) {
	if !exileRiderOn(k) {
		return
	}
	ob := &s.objs[r.obj]
	if ob.zone != kop.ZoneBattlefield {
		return
	}
	m := coldExRider.of(s)
	if int(ob.slot) >= len(m) || m[ob.slot].src == 0 {
		return
	}
	i := mode - maxExMarks
	if i < 0 || i >= r.nsrc() || r.src[i] != m[ob.slot].obj {
		m[ob.slot] = exRiderMark{}
	}
}

// exileRiderRun runs the rider prog recorded for the dying slot after o, the
// replaced card, moved to exile. The record holds the source's identity as
// the pre-move scan saw it, so the prog runs even when the source left in the
// same batch (two opposing Kalitas dying together; gorge holds the
// replacement's source the same way) or its slot was reused since. The prog
// runs in a synthetic resolution of the source's replacement ability (the
// source's controller is "You", o is ReplacedCard); the enclosing resolution,
// if any, is restored.
func (s *State) exileRiderRun(k *prep, m exRiderMark, o Obj) {
	c := m.card
	i := exileRiderAb(&k.t.Cards[c])
	if i < 0 {
		s.capFull(StallExileRider)
		return
	}
	saved, savedRep := s.res, s.g.replaced
	s.g.replaced = o
	var pow []int16
	var savedPow int16
	if pow = coldExRiderPow.of(s); len(pow) != 0 {
		savedPow, pow[0] = pow[0], m.pow
	}
	s.res = resCtx{
		e:      stackEntry{kind: seAbility, obj: m.obj, srcVer: m.ver, card: c, ab: uint8(i), ctrl: m.ctrl, trigObj: NoObj},
		active: true, lastTok: NoObj, eachObj: NoObj,
	}
	s.res.e.tgt = noTargets
	ok := s.runPreCast(k, k.t.Cards[c].Abilities[i].Prog)
	s.res, s.g.replaced = saved, savedRep
	if len(pow) != 0 {
		pow[0] = savedPow
	}
	if !ok {
		s.capFull(StallExileRider)
	}
}

// replLKIPower is the replaced card's power the running dies-to-exile rider
// captured before the move (kop.ExReplLKIPower, ReplacedCardLKI$CardPower).
// 0 outside a rider run, where the scalar is not set.
func (s *State) replLKIPower() int {
	if p := coldExRiderPow.of(s); len(p) != 0 && s.g.replaced != NoObj {
		return int(p[0])
	}
	return 0
}
