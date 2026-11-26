package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The mechanic-bit arena. Per-mechanic masks that almost no pool uses live in
// the State's cold arrays, ms for slot width and mo for object width:
// arenaAssign gives each mechanic a table uses the next free index (a pure
// function of the table, filled into genPrep.ix), and every other mechanic
// reads false through mSl and mObj. arenaNone is past the end of any mask
// array, so the bounds test is also the off-switch: an unused mechanic touches
// no mask.

// mech names one arena mechanic. The constants, numMech and each mech's
// mechWidth and mechName rows are generated (manifest_gen.go) from the
// manifest's entries, dirac/manifest/mech/<name>: a new mechanic is a
// value: next entry there with its state line (slotbit or objbit), plus a
// detector registered from its own file (arenaDetect). Values never change,
// so a pool's index assignment changes only when it uses a new mechanic.
type mech uint8

// key names one bounded State key of the keyed arena: a manifest ID
// (dirac/manifest/key/<name>, generated into manifest_gen.go), fixed for all
// time and costing no State bytes until a pool's cards use it. The keyed
// arena's width classes that give a used key its bytes are later work; until
// then every key entry is reserved and nothing reads one.
type key uint16

// goadMechs names each seat's goad mech in seat order: mechGoad0 and
// mechGoad1 first, mechGoad2 and mechGoad3 appended past mechAtch3, so the
// per-seat goad mech cannot be read by arithmetic off mechGoad0.
var goadMechs = []mech{mechGoad0, mechGoad1, mechGoad2, mechGoad3}

// mech widths: a slot mask indexes the permanent's slot; an object mask
// indexes its Obj.
const (
	wSlot uint8 = iota
	wObj
)

// mechWidth is each mech's width and mechName its name, both filled by
// manifest_gen.go's init functions.
var (
	mechWidth [numMech]uint8
	mechName  [numMech]string
)

// arenaNone is the index of a mechanic the table does not use.
const arenaNone = 0xFF

// arenaDetector reports whether one instruction of t reaches mechanic m.
type arenaDetector struct {
	m      mech
	detect func(t *kop.Table, in *kop.Instr) bool
}

// arenaDetectors are the registered detectors, sorted by mech.
var arenaDetectors []arenaDetector

// arenaDetect registers detect for m, from m's feature file:
//
//	func init() { arenaDetect(mechFoo, func(t *kop.Table, in *kop.Instr) bool { ... }) }
//
// arenaUses marks m used when detect holds for any instruction of any
// ability, after the hand detectors. Registration order never matters: the
// detectors run in mech order.
func arenaDetect(m mech, detect func(t *kop.Table, in *kop.Instr) bool) {
	i := len(arenaDetectors)
	for i > 0 && arenaDetectors[i-1].m > m {
		i--
	}
	arenaDetectors = append(arenaDetectors, arenaDetector{})
	copy(arenaDetectors[i+1:], arenaDetectors[i:])
	arenaDetectors[i] = arenaDetector{m, detect}
}

// triggerEventUses marks the mechanics a triggered ability's event reaches.
func triggerEventUses(a *kop.Ability, u *[numMech]bool) {
	if a.Kind != kop.AbTriggered {
		return
	}
	first := a.Trig.Flags2&kop.Tf2FirstTime != 0
	switch {
	case a.Trig.Event == kop.TrigBecomeMonstrous:
		u[mechMonstrous] = true
	case a.Trig.Event == kop.TrigTaps && first:
		u[mechTappedTurn] = true
	case a.Trig.Event == kop.TrigBecomesTarget && first:
		u[mechBtTurn] = true // a granted line's grantor card carries it too (trig_counterfirst.go)
	}
}

// arenaUses reports which mechanics t's programs and triggers reach, by the
// predicates the prep flags use (prepGoad, prepMonstrosity) and the
// registered detectors (arenaDetect).
func arenaUses(t *kop.Table) (u [numMech]bool) {
	permMarkUses(t, &u)    // filter_permmarks.go
	turnTalliesUses(t, &u) // value_turntallies.go
	addPhaseUses(t, &u)    // act_addphase.go
	for i := range t.Conds {
		if t.Conds[i].Kind == kop.CondRemMark {
			u[mechRem] = true // a persistent-remembered gate (exprs.go CondRemMark)
		}
	}
	for i := range t.Cards {
		if cd := &t.Cards[i]; cd.Keywords&kop.KwKicker != 0 || cd.HasMultikicker || cd.Bargain || cd.Offspring || cd.OptEvidence != 0 && cd.Types&(kop.TInstant|kop.TSorcery) == 0 {
			u[mechKicked] = true
		}
		if cd := &t.Cards[i]; cd.Types&kop.TAura != 0 && cd.Enchant.Kind == kop.TgtPlayer {
			for sd := 0; sd < MaxSeats; sd++ {
				u[mechAtch0+mech(sd)] = true // a player-attached Aura's seat (static_attachtimestamp.go)
			}
		}
		once := 0
		if t.Cards[i].Mayhem {
			u[mechMayhem] = true
		}
		if t.Cards[i].HasSneak && !u[mechSneaked] {
			u[mechSneaked] = sneakFilterRead(t) // cold scan, once per table (kw_sneak.go)
		}
		if t.Cards[i].Escape && !u[mechEscaped] {
			u[mechEscaped] = escapeFilterRead(t) // cold scan, once per table (kw_escape.go)
		}
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Cost.Flags&kop.CostOncePerGame != 0 {
				once++
			}
			if a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplExileOnLeave {
				u[mechSBAChain] = true // a state-based batch can park its casualties (repl_exilemark.go)
			}
			if a.Static.A == kop.StackKwOffspring {
				u[mechKicked] = true // a granted offspring's paid mark (enterGen, kw_offspring.go)
			}
			triggerEventUses(a, &u)
			if a.Kind == kop.AbTriggered && a.Trig.Cond != 0 && t.Conds[a.Trig.Cond].Kind == kop.CondEchoOwed {
				u[mechEchoOwed] = true // an echo trigger's gate (trig_echo.go)
			}
			for _, in := range a.Prog {
				switch in.Op {
				case kop.OpGoad:
					for sd := 0; sd < MaxSeats; sd++ {
						u[goadMechs[sd]] = true // goad[sd]: seat sd may goad
					}
				case kop.OpMonstrosity:
					u[mechMonstrous] = true
				case kop.OpRegenerate:
					u[mechRegen] = true
				case kop.OpDealDamageEx:
					if in.C&kop.DmgExileIfDiesThisTurn != 0 {
						u[mechExileEx] = true
					}
					if in.C&(kop.DmgNoRegen|kop.DmgNoRegenTarget) != 0 {
						u[mechNoRegen] = true // repl_noregen.go
					}
				case kop.OpExileEx:
					if kop.CounterKind(in.B.N) == kop.CtrStash {
						u[mechStash] = true // the only stashSet site (interp_gen.go)
					}
				case kop.OpExt:
					mustBlockUses(&in, &u) // combat_mustbeblocked.go
					colProtUses(&in, &u)   // static_colourprot.go
					kwMarkUses(&in, &u)    // kw_kwmark.go
					if in.Ext() == kop.ExtAttacksIfAble {
						u[mechAttacksIfAble] = true // combat_attacksifable.go
						if in.C&kop.AttacksOpponent != 0 {
							u[mechAttacksPlayer] = true
						}
					}
					if in.Ext() == kop.ExtBlocksIfAble {
						u[mechBlocksIfAble] = true // combat_blocksifable.go
					}
					if in.Ext() == kop.ExtExileExMark || in.Ext() == kop.ExtTurnRepl && in.C == kop.TurnReplExileOpp {
						u[mechExileEx] = true
					}
					if in.Ext() == kop.ExtTurnRepl && in.C == kop.TurnReplHexUB {
						u[mechHexUB] = true // repl_turnrepl.go
					}
					if in.Ext() == kop.ExtSuspect && in.C&1 == 0 { // seat-ok: an instruction flag bit
						u[mechSuspected] = true // a suspect; an un-suspect or a read needs no mask
					}
					if in.Ext() == kop.ExtRenown {
						u[mechRenowned] = true // the only writer; a read alone needs no mask (act_renown.go)
					}
					encoreUses(&in, &u) // kw_encore.go: Encore, its grant, Myriad
				case kop.OpGrantKeyword:
					// grantFB is the only writer of the flashback planes, so a pool
					// without such a grant reads 0 grants (cast.go's b.fbGrants too)
					if kop.KwMask(uint16(in.B.N))<<16&kop.KwFlashbackOwnCost != 0 {
						u[mechFBLo], u[mechFBHi] = true, true
					}
				}
			}
		}
		if once >= 2 {
			u[mechOnce2] = true
		}
		if once >= 3 {
			u[mechOnce3] = true
		}
	}
	arenaDetected(t, &u)
	u[mechSBAChain] = u[mechSBAChain] || u[mechExileEx]
	return u
}

// arenaDetected marks the mechanics a registered detector finds in t.
func arenaDetected(t *kop.Table, u *[numMech]bool) {
	for _, d := range arenaDetectors {
		if u[d.m] {
			continue
		}
	cards:
		for i := range t.Cards {
			for j := range t.Cards[i].Abilities {
				for k := range t.Cards[i].Abilities[j].Prog {
					if d.detect(t, &t.Cards[i].Abilities[j].Prog[k]) {
						u[d.m] = true
						break cards
					}
				}
			}
		}
	}
}

// arenaAssign gives each used mechanic the next free arena index of its
// width in enum order (arenaNone for the unused). nms and nmo are the slot and
// object masks in use. mechFBHi directly follows mechFBLo and they are used
// together, so its index is mechFBLo's + 1, which the flashback helpers rely on.
func arenaAssign(t *kop.Table) (ix [numMech]uint8, nms, nmo uint8) {
	u := arenaUses(t)
	for m := mech(0); m < numMech; m++ {
		ix[m] = arenaNone
		switch {
		case !u[m]:
		case mechWidth[m] == wSlot:
			ix[m] = nms
			nms++
		default:
			ix[m] = nmo
			nmo++
		}
	}
	return ix, nms, nmo
}

// prepArena fills g.ix; coldMS and coldMO size the masks from it.
func prepArena(t *kop.Table, g *genPrep) {
	g.ix, _, _ = arenaAssign(t)
}

// coldMS and coldMO are the slot- and object-width masks a table's arena
// assigned.
var (
	coldMS = newCold[slotMask]("ms", func(g *genPrep) int { return arenaCount(g, wSlot) }).view(func(c *coldArrays) *[]slotMask { return &c.ms })
	coldMO = newCold[objBits]("mo", func(g *genPrep) int { return arenaCount(g, wObj) }).view(func(c *coldArrays) *[]objBits { return &c.mo })
)

// arenaCount is how many masks of width w g.ix assigned.
func arenaCount(g *genPrep, w uint8) int {
	n := 0
	for m, i := range g.ix {
		if i != arenaNone && mechWidth[m] == w {
			n++
		}
	}
	return n
}

// mSl reports whether mechanic m's mask holds sl; false when the table does
// not use m (and for a hand-written table, which has no genPrep).
func (s *State) mSl(k *prep, m mech, sl Slot) bool {
	if k.gen == nil {
		return false
	}
	i := k.gen.ix[m]
	return int(i) < len(s.cold.ms) && s.cold.ms[i].has(sl)
}

// mSlSet sets sl in mechanic m's mask (a no-op when the table does not use m).
func (s *State) mSlSet(k *prep, m mech, sl Slot) {
	if k.gen == nil {
		return
	}
	if i := k.gen.ix[m]; int(i) < len(s.cold.ms) {
		s.cold.ms[i].set(sl)
	}
}

// mSlClr clears sl in mechanic m's mask.
func (s *State) mSlClr(k *prep, m mech, sl Slot) {
	if k.gen == nil {
		return
	}
	if i := k.gen.ix[m]; int(i) < len(s.cold.ms) {
		s.cold.ms[i].clr(sl)
	}
}

// mSlZero empties mechanic m's whole mask.
func (s *State) mSlZero(k *prep, m mech) {
	if k.gen == nil {
		return
	}
	if i := k.gen.ix[m]; int(i) < len(s.cold.ms) {
		s.cold.ms[i] = slotMask{}
	}
}

// arenaClrSlot clears slot sl in every arena mask: sl is taking a new
// permanent, so no mechanic's mark for the previous one survives (the one
// loop that replaces the per-mechanic clears at enter).
func (s *State) arenaClrSlot(sl Slot) {
	ms := s.cold.ms
	for i := range ms {
		ms[i] = ms[i].without(sl)
	}
	s.coldClrSlot(sl) // every perSlot cold region's record (statecopy.go)
	s.xblkDrop(sl)    // the last blocker's second block (combat_extrablock.go)
	s.mbaDrop(sl)     // the last permanent's block duties (combat_mustblockatk.go)
	for i := range s.cold.exMark {
		if r := &s.cold.exMark[i]; r.sl == sl {
			r.live = false // the last permanent's exile-instead records end with it (repl_exilemark.go)
		}
	}
}

// mSlMask returns mechanic m's mask (empty when the table does not use m).
func (s *State) mSlMask(k *prep, m mech) slotMask {
	if k.gen == nil {
		return slotMask{}
	}
	if i := k.gen.ix[m]; int(i) < len(s.cold.ms) {
		return s.cold.ms[i]
	}
	return slotMask{}
}
