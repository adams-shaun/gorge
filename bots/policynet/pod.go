package policynet

import (
	"math"
	"math/rand/v2"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// The four-seat Commander pod encoding (FeaturesPod, "entity-pod"): the
// entity set plus what a pod's network needs that the two-seat layout never
// carried. It is a feature set of its own, so every other set encodes byte
// for byte as before and every existing checkpoint keeps loading.
//
// Dense, appended after the entity set's 92 columns: one pod block per
// relative seat (the seat blocks' order: the viewer, then turn order from
// it), then the global block.
//
//	per seat (PodSeatWidth):
//	  0      the monarch (CR 725)
//	  1      has the initiative (CR 726)
//	  2      poison counters /10, clamped
//	  3      energy counters, log1p/4
//	  4      experience counters, log1p/4
//	  5      life gained this turn /10, clamped
//	  6      life lost this turn /10, clamped
//	  7..12  commander damage taken from the next three relative seats'
//	         commanders, two slots each (a partner pair's own tallies, CR
//	         903.10a), /21 clamped: 7 + 2·(k-1) + slot
//	  13,14  commander slot 0 / 1 is in the command zone
//	  15     the seat has a second commander (a partner)
//	global (podGlobalWidth):
//	  0      round bucket min(round, 20)/20, round = 1 + (turn-1)/alive: the
//	         snapshot round both engines can compute (view.roundOf), not
//	         the engine's per-seat turn counter, which a pod runs through
//	         four times as fast
//	  1, 2   day, night (CR 731)
//
// The commander columns stay zero outside a Commander game. Sparse: one
// "<name>|command" id per card in every seat's command zone (seat order).
const (
	PodSeatWidth   = 16
	podGlobalWidth = 3
	// DenseWidthPod is FeaturesPod's state dense width.
	DenseWidthPod = DenseWidth + maxSeatBlocks*PodSeatWidth + podGlobalWidth

	PodMonarch    = 0
	PodInitiative = 1
	PodPoison     = 2
	PodEnergy     = 3
	PodExperience = 4
	PodLifeGained = 5
	PodLifeLost   = 6
	PodCmdDamage0 = 7
	PodCmdInZone0 = 13
	PodPartner    = 15

	podGlobal0     = DenseWidth + maxSeatBlocks*PodSeatWidth
	podRoundBucket = 0
	podDay         = 1
	podNight       = 2
)

// PodLogScale is the pod block's log scale for an open-ended counter
// (energy, experience): log1p(n)/4.
func PodLogScale(n int32) float32 {
	if n <= 0 {
		return 0
	}
	return float32(math.Log1p(float64(n)) / 4)
}

// podRound is the snapshot round: 1 + (turn-1)/alive, 1 before the first
// turn or with no seat alive (view.roundOf's formula, from the view).
func podRound(v *view.View) int32 {
	if v.Turn <= 0 {
		return 1
	}
	alive := int32(0)
	for i := range v.Players {
		if !v.Players[i].Lost {
			alive++
		}
	}
	if alive == 0 {
		return 1
	}
	return 1 + (v.Turn-1)/alive
}

// playerCounter is pv's player counter of kind, its spelling folded to
// upper case (a script writes "ENERGY" or "Energy", "Experience").
func playerCounter(pv *view.PlayerView, kind string) int32 {
	var n int32
	eachCounterSorted(pv.Counters, func(k string, c int32) {
		if upperASCII(k) == kind {
			n += c
		}
	})
	return n
}

// upperASCII is s upper-cased over ASCII letters.
func upperASCII(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] >= 'a' && s[i] <= 'z' {
			b := []byte(s)
			for j := i; j < len(b); j++ {
				if b[j] >= 'a' && b[j] <= 'z' {
					b[j] -= 'a' - 'A'
				}
			}
			return string(b)
		}
	}
	return s
}

// podDense writes the pod blocks into dense (DenseWidthPod wide).
func podDense(dense []float32, v *view.View, seat state.PlayerID) {
	n := len(v.Players)
	meIdx := -1
	commanderGame := false
	for i := range v.Players {
		if v.Players[i].ID == seat {
			meIdx = i
		}
		if len(v.Players[i].Commanders) > 0 {
			commanderGame = true
		}
	}
	g := dense[podGlobal0:]
	g[podRoundBucket] = clamp01(float32(podRound(v)) / 20)
	switch v.DayNight {
	case "day":
		g[podDay] = 1
	case "night":
		g[podNight] = 1
	}
	if meIdx < 0 {
		return
	}
	rel := func(r int) *view.PlayerView { return &v.Players[(meIdx+r)%n] }
	for r := 0; r < n && r < maxSeatBlocks; r++ {
		pv := rel(r)
		b := dense[DenseWidth+r*PodSeatWidth : DenseWidth+(r+1)*PodSeatWidth]
		if pv.IsMonarch {
			b[PodMonarch] = 1
		}
		if pv.HasInitiative {
			b[PodInitiative] = 1
		}
		b[PodPoison] = clamp01(float32(playerCounter(pv, "POISON")) / 10)
		b[PodEnergy] = PodLogScale(playerCounter(pv, "ENERGY"))
		b[PodExperience] = PodLogScale(playerCounter(pv, "EXPERIENCE"))
		b[PodLifeGained] = clamp01(float32(pv.LifeGainedThisTurn) / 10)
		b[PodLifeLost] = clamp01(float32(pv.LifeLostThisTurn) / 10)
		if !commanderGame {
			continue
		}
		for k := 1; k < n && k < maxSeatBlocks; k++ {
			spv := rel(r + k)
			for j := 0; j < 2 && j < len(spv.Commanders); j++ {
				b[PodCmdDamage0+2*(k-1)+j] = clamp01(float32(pv.CmdDamage[spv.Commanders[j].ID]) / 21)
			}
		}
		for j := 0; j < 2 && j < len(pv.Commanders); j++ {
			for i := range pv.Command {
				if pv.Command[i].ID == pv.Commanders[j].ID {
					b[PodCmdInZone0+j] = 1
					break
				}
			}
		}
		if len(pv.Commanders) > 1 {
			b[PodPartner] = 1
		}
	}
}

// podSparse appends one "<name>|command" id per commander in every seat's
// command zone, in seat order (a dungeon or an emblem there is not one: the
// dungeon is PlayerView.Dungeon's).
func podSparse(v *view.View, sk *sink) {
	for i := range v.Players {
		pv := &v.Players[i]
		for j := range pv.Command {
			if isCommander(pv, pv.Command[j].ID) {
				sk.h(fnvAdd(fnvAdd(fnvAdd(fnvOffset, pv.Command[j].Name), "|"), zoneTagCommand), 1)
			}
		}
	}
}

// isCommander reports whether id is one of pv's commanders.
func isCommander(pv *view.PlayerView, id state.ObjID) bool {
	for i := range pv.Commanders {
		if pv.Commanders[i].ID == id {
			return true
		}
	}
	return false
}

// podHashSuffix is the pod contract's part of EncoderHashFor(FeaturesPod):
// the dense layout's widths and its format version.
func podHashSuffix() string {
	return "\x1fpod-format=1\x1fpod-seat=" + formatInt(PodSeatWidth) + "\x1fpod-global=" + formatInt(podGlobalWidth)
}

// NewPodModel builds a FeaturesPod model: NewModel's trunk at the pod dense
// width with a k-wide entity encoder drawn from rng (UpgradeEntity's
// initialisation: the pooled projection and the entity hidden-input columns
// start at zero).
func NewPodModel(rows, h, hidden, k int, rng *rand.Rand) (*Model, error) {
	m := &Model{Rows: rows, H: h, Hidden: hidden, Features: FeaturesMZ, stateDenseW: DenseWidthPod,
		InW: 2*h + OptionSlotWidth + OptionDenseWidth}
	m.initWeights(rng)
	if err := upgradeEntity(m, k, rng, FeaturesPod); err != nil {
		return nil, err
	}
	return m, nil
}

// podLifeRows are the pod set's life and lethal rows (t1): the viewer's
// "mz|me|life=" bucket, then the opponents in turn order from the viewer,
// a seat that has left the game skipped, each in its own slot j (the first
// maxSeatBlocks-1): "mz|pod|lifediff<j>=" over (my life - its life)/3+7 and
// "mz|pod|opp<j>|life=" over its life/2. The lethal rows read each opponent
// alone, never every opponent's power summed: "mz|me|lethal-unblocked" when
// my untapped attackers' power reaches some live opponent's life,
// "mz|opp|lethal-unblocked" when some live opponent's own reaches mine.
// others is every other seat in seat order and oppPower[j] others[j]'s
// can-attack power.
func podLifeRows(v *view.View, me *view.PlayerView, others []*view.PlayerView, myPower int32, oppPower []int32, sk *sink) {
	sk.h(fnvAddInt(fnvAdd(fnvOffset, "mz|me|life="), mzClamp(me.Life/2, mzLifeBucketHi)), 1)
	n := len(v.Players)
	meIdx := 0
	for i := range v.Players {
		if v.Players[i].ID == me.ID {
			meIdx = i
		}
	}
	slot := 0
	meLethal, oppLethal := false, false
	for r := 1; r < n; r++ {
		pv := &v.Players[(meIdx+r)%n]
		if pv.Lost {
			continue
		}
		if slot < maxSeatBlocks-1 {
			sk.h(fnvAddInt(fnvAdd(fnvAddInt(fnvAdd(fnvOffset, "mz|pod|lifediff"), int64(slot)), "="), mzClamp((me.Life-pv.Life)/3+7, 14)), 1)
			sk.h(fnvAddInt(fnvAdd(fnvAddInt(fnvAdd(fnvOffset, "mz|pod|opp"), int64(slot)), "|life="), mzClamp(pv.Life/2, mzLifeBucketHi)), 1)
		}
		slot++
		if myPower >= pv.Life {
			meLethal = true
		}
		for j, o := range others {
			if o == pv && oppPower[j] >= me.Life {
				oppLethal = true
			}
		}
	}
	if meLethal {
		sk.h(fnvAdd(fnvOffset, "mz|me|lethal-unblocked"), 1)
	}
	if oppLethal {
		sk.h(fnvAdd(fnvOffset, "mz|opp|lethal-unblocked"), 1)
	}
}

// ---- the pod entity layout (FeaturesPod) ----------------------------------
//
// The entity set's 100 raw columns, then, appended:
//
//	100..103  the controller's relative seat, one-hot (viewer 0, then turn
//	          order from it; a seat past the fourth sets none)
//	104..107  the owner's relative seat, one-hot
//	108..111  an attacker's defending player's relative seat (the player, or
//	          the attacked planeswalker's controller or battle's protector)
//	112       it attacks a planeswalker or a battle (view AttackingPermanent)
//	113..116  each seat that goads it, by relative seat (view GoadedBy)
//	117       it is in the command zone
//	118       it is a commander (some seat's roster)
//	119       reserved
//
// entMine is left zero: the controller one-hot's first column says it.
// The pooled groups are the battlefield of each relative seat (0..3; a seat
// past the fourth pools with the fourth), the viewer's hand, the stack and
// the command zone; the command-zone commanders are listed after the stack,
// every seat's in seat order.
const (
	// EntityRawWidthPod is FeaturesPod's entity raw width.
	EntityRawWidthPod = EntityRawWidth + 20
	// EntityGroupsPod is FeaturesPod's pooled group count.
	EntityGroupsPod = maxSeatBlocks + 3

	PodEntCtrl0       = EntityRawWidth
	PodEntOwner0      = EntityRawWidth + 4
	PodEntAttacked0   = EntityRawWidth + 8
	PodEntAttacksPerm = EntityRawWidth + 12
	PodEntGoadedBy0   = EntityRawWidth + 13
	PodEntCommandZone = EntityRawWidth + 17
	PodEntCommander   = EntityRawWidth + 18

	PodGroupHand    = maxSeatBlocks
	PodGroupStack   = maxSeatBlocks + 1
	PodGroupCommand = maxSeatBlocks + 2

	// podZoneCommand is the entity zone class of a command-zone card (the
	// entZone* classes are the battlefield, hand and stack).
	podZoneCommand = PodEntCommandZone
)

// EntityRawW is fs's entity raw width.
func (fs FeatureSet) EntityRawW() int {
	if fs == FeaturesPod {
		return EntityRawWidthPod
	}
	return EntityRawWidth
}

// EntityGroupsN is fs's pooled entity group count.
func (fs FeatureSet) EntityGroupsN() int {
	if fs == FeaturesPod {
		return EntityGroupsPod
	}
	return EntityGroups
}

// podRel is player p's relative seat from seat (turn order, the view's
// player order), -1 when either is not in the view.
func podRel(v *view.View, seat, p state.PlayerID) int {
	si, pi := -1, -1
	for i := range v.Players {
		if v.Players[i].ID == seat {
			si = i
		}
		if v.Players[i].ID == p {
			pi = i
		}
	}
	if si < 0 || pi < 0 {
		return -1
	}
	n := len(v.Players)
	return (pi - si + n) % n
}

// podOneHot sets raw[off+r] for a relative seat r in 0..3.
func podOneHot(raw []float32, off, r int) {
	if r >= 0 && r < maxSeatBlocks {
		raw[off+r] = 1
	}
}

// walkPodEntityOrder is the pod entity order: my battlefield, every other
// battlefield in seat order (each grouped by its controller's relative
// seat), my hand, the stack, then every seat's command-zone commanders.
func walkPodEntityOrder(v *view.View, seat state.PlayerID, f func(cv *view.CardView, group uint8, zone int, sv *view.StackView)) {
	bfGroup := func(p state.PlayerID) uint8 {
		r := podRel(v, seat, p)
		if r < 0 || r >= maxSeatBlocks {
			r = maxSeatBlocks - 1
		}
		return uint8(r)
	}
	var me *view.PlayerView
	for i := range v.Players {
		if v.Players[i].ID == seat {
			me = &v.Players[i]
		}
	}
	if me != nil {
		for i := range me.Battlefield {
			f(&me.Battlefield[i], 0, entZoneBf, nil)
		}
	}
	for i := range v.Players {
		if v.Players[i].ID == seat {
			continue
		}
		g := bfGroup(v.Players[i].ID)
		for j := range v.Players[i].Battlefield {
			f(&v.Players[i].Battlefield[j], g, entZoneBf, nil)
		}
	}
	if me != nil {
		for i := range me.Hand {
			f(&me.Hand[i], PodGroupHand, entZoneHand, nil)
		}
	}
	for i := range v.Stack {
		if sv := &v.Stack[i]; sv.Card != nil {
			f(sv.Card, PodGroupStack, entZoneStack, sv)
		}
	}
	for i := range v.Players {
		pv := &v.Players[i]
		for j := range pv.Command {
			if isCommander(pv, pv.Command[j].ID) {
				f(&pv.Command[j], PodGroupCommand, podZoneCommand, nil)
			}
		}
	}
}

// podEntity writes the pod raw columns of cv (listed in zone).
func podEntity(raw []float32, cv *view.CardView, zone int, v *view.View, seat state.PlayerID) {
	raw[entMine] = 0
	podOneHot(raw, PodEntCtrl0, podRel(v, seat, cv.Controller))
	podOneHot(raw, PodEntOwner0, podRel(v, seat, cv.Owner))
	if zone == entZoneBf && cv.Attacking && cv.AttackingPlayer != nil {
		podOneHot(raw, PodEntAttacked0, podRel(v, seat, *cv.AttackingPlayer))
		if cv.AttackingPermanent {
			raw[PodEntAttacksPerm] = 1
		}
	}
	if zone == entZoneBf {
		for _, p := range cv.GoadedBy {
			podOneHot(raw, PodEntGoadedBy0, podRel(v, seat, p))
		}
	}
	if zone == podZoneCommand {
		raw[PodEntCommandZone] = 1
	}
	for i := range v.Players {
		if isCommander(&v.Players[i], cv.ID) {
			raw[PodEntCommander] = 1
			break
		}
	}
}

// PodSeatLeafWidth is the width of one relative seat's dense facts
// (PodSeatVector): its seat block, Commander block and pod block.
const PodSeatLeafWidth = seatBlockWidth + commanderBlockWidth + PodSeatWidth

// PodSeatVector copies relative seat r's dense facts out of a FeaturesPod
// dense vector into dst (PodSeatLeafWidth): the seat block, the Commander
// block, the pod block (a graph network's player-node leaf). Columns a
// shorter vector lacks read as zero.
func PodSeatVector(dense []float32, r int, dst []float32) {
	clear(dst[:PodSeatLeafWidth])
	part := func(off, base, w int) {
		for i := 0; i < w; i++ {
			if base+i < len(dense) {
				dst[off+i] = dense[base+i]
			}
		}
	}
	part(0, denseSeat0+r*seatBlockWidth, seatBlockWidth)
	part(seatBlockWidth, denseCmd0+r*commanderBlockWidth, commanderBlockWidth)
	part(seatBlockWidth+commanderBlockWidth, DenseWidth+r*PodSeatWidth, PodSeatWidth)
}

// PodCardOwner is a FeaturesPod entity card's owner relative seat (its
// owner one-hot), -1 when none is set.
func PodCardOwner(c *EntityCard) int {
	for r := 0; r < maxSeatBlocks; r++ {
		if PodEntOwner0+r < len(c.Raw) && c.Raw[PodEntOwner0+r] != 0 {
			return r
		}
	}
	return -1
}
