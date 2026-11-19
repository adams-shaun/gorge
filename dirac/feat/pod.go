package feat

import (
	"math"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// pod.go is policynet's FeaturesPod (bots/policynet/pod.go), the four-seat
// Commander pod encoding, off unless SetPod turns it on: the dense pod
// blocks after the 92 entity columns and the "<name>|command" sparse ids.
// With it off the encoder writes FeaturesEntity byte for byte and leaves
// every column past DenseWidth zero.

// The pod dense layout (policynet's pinned constants; pod.go's doc).
const (
	podSeatWidth   = 16
	podGlobalWidth = 3
	// DenseWidthPod is the pod set's state dense width; State.Dense holds
	// it, and only its first DenseWidth columns are written with pod off.
	DenseWidthPod = DenseWidth + maxSeatBlocks*podSeatWidth + podGlobalWidth

	podMonarch     = 0
	podInitiative  = 1
	podPoison      = 2
	podEnergy      = 3
	podExperience  = 4
	podLifeGained  = 5
	podLifeLost    = 6
	podCmdDamage0  = 7
	podCmdInZone0  = 13
	podPartner     = 15
	podGlobal0     = DenseWidth + maxSeatBlocks*podSeatWidth
	podRoundBucket = 0
	podDay         = 1
	podNight       = 2
)

// SetPod switches the pod encoding (policynet FeaturesPod) on or off.
func (e *Encoder) SetPod(on bool) { e.pod = on }

// Pod reports whether the pod encoding is on.
func (e *Encoder) Pod() bool { return e.pod }

// DenseW is the width of the dense vector this encoder writes: DenseWidthPod
// with pod on, else DenseWidth.
func (e *Encoder) DenseW() int {
	if e.pod {
		return DenseWidthPod
	}
	return DenseWidth
}

// podLogScale is policynet.PodLogScale: log1p(n)/4.
func podLogScale(n int) float32 {
	if n <= 0 {
		return 0
	}
	return float32(math.Log1p(float64(n)) / 4)
}

func clampUnit(x float32) float32 {
	if x > 1 {
		return 1
	}
	if x < 0 {
		return 0
	}
	return x
}

// podDense is policynet's podDense over the kernel position.
func (e *Encoder) podDense(s *dirac.State, dn *[DenseWidthPod]float32) {
	v := &e.v
	n := int(v.n)
	alive := int32(0)
	for p := 0; p < n; p++ {
		if !s.FeatLost(uint8(p)) {
			alive++
		}
	}
	round := int32(1)
	if v.turn > 0 && alive > 0 {
		round = 1 + (v.turn-1)/alive
	}
	g := dn[podGlobal0:]
	g[podRoundBucket] = clampUnit(float32(round) / 20)
	switch s.DayNight() {
	case 1:
		g[podDay] = 1
	case 2:
		g[podNight] = 1
	}
	cmdGame := s.FeatCommanderGame()
	mon, ini := s.Monarch(), s.Initiative()
	for r := 0; r < n && r < maxSeatBlocks; r++ {
		p := v.relPlayer(r)
		b := dn[DenseWidth+r*podSeatWidth : DenseWidth+(r+1)*podSeatWidth]
		if int(mon) == int(p) {
			b[podMonarch] = 1
		}
		if int(ini) == int(p) {
			b[podInitiative] = 1
		}
		b[podPoison] = clampUnit(float32(s.Poison(p)) / 10)
		b[podEnergy] = podLogScale(s.Energy(p))
		b[podExperience] = podLogScale(s.Experience(p))
		gained, lost := s.FeatLifeTurn(p)
		b[podLifeGained] = clampUnit(float32(gained) / 10)
		b[podLifeLost] = clampUnit(float32(lost) / 10)
		if !cmdGame {
			continue
		}
		for k := 1; k < n && k < maxSeatBlocks; k++ {
			q := v.relPlayer(r + k)
			for j := uint8(0); j < 2; j++ {
				if s.FeatCommanderN(q, j) == noObj {
					continue
				}
				b[podCmdDamage0+2*(k-1)+int(j)] = clampUnit(float32(s.FeatCommanderDamageN(p, q, j)) / 21)
			}
		}
		for j := uint8(0); j < 2; j++ {
			if o := s.FeatCommanderN(p, j); o != noObj && e.po(o).Zone == kop.ZoneCommand {
				b[podCmdInZone0+int(j)] = 1
			}
		}
		if s.FeatCommanderN(p, 1) != noObj {
			b[podPartner] = 1
		}
	}
}

// podSparse is policynet's podSparse: one "<name>|command" id per card in
// every seat's command zone, in seat order.
func (e *Encoder) podSparse() {
	v := &e.v
	for p := 0; p < int(v.n); p++ {
		for _, ci := range v.players[p].cmd {
			e.push(v.cards[ci].c.nameTag[ztCommand], 1)
		}
	}
}

// podLifeRowTables are policynet's podLifeRows bucket rows per slot:
// "mz|pod|lifediff<j>=" and "mz|pod|opp<j>|life=".
var podRowsLifediff, podRowsOppLife = func() (d, o [maxSeatBlocks - 1][]uint16) {
	for j := range d {
		d[j] = bucketRows("mz|pod|lifediff"+string(rune('0'+j))+"=", 14)
		o[j] = bucketRows("mz|pod|opp"+string(rune('0'+j))+"|life=", lifeBucketHi)
	}
	return
}()

// podLifeRows is policynet's podLifeRows: the opponents in turn order from
// the viewer, a departed seat skipped, one life slot each, and the lethal
// rows read one opponent at a time.
func (e *Encoder) podLifeRows(me *vplayer, mine *agg) {
	v := &e.v
	e.push(rowsMeLife[clampI(me.life/2, lifeBucketHi)], 1)
	slot := 0
	meLethal, oppLethal := false, false
	for r := 1; r < int(v.n); r++ {
		p := v.relPlayer(r)
		if e.st0.FeatLost(p) {
			continue
		}
		pl := &v.players[p]
		if slot < maxSeatBlocks-1 {
			e.push(podRowsLifediff[slot][clampI((me.life-pl.life)/3+7, 14)], 1)
			e.push(podRowsOppLife[slot][clampI(pl.life/2, lifeBucketHi)], 1)
		}
		slot++
		if mine.canAttackPower >= pl.life {
			meLethal = true
		}
		var theirs agg
		for _, ci := range pl.bf {
			theirs.add(&v.cards[ci])
		}
		if theirs.canAttackPower >= me.life {
			oppLethal = true
		}
	}
	if meLethal {
		e.push(rowMeLethal, 1)
	}
	if oppLethal {
		e.push(rowOppLethal, 1)
	}
}

// The pod entity layout (policynet pod.go's): the entity set's raw columns,
// then the controller, owner, attacked-player and goader relative-seat
// one-hots, the attacks-a-permanent, command-zone and commander bits.
const (
	EntityRawWidthPod = EntityRawWidth + 20
	EntityGroupsPod   = maxSeatBlocks + 3

	podEntCtrl0       = EntityRawWidth
	podEntOwner0      = EntityRawWidth + 4
	podEntAttacked0   = EntityRawWidth + 8
	podEntAttacksPerm = EntityRawWidth + 12
	podEntGoadedBy0   = EntityRawWidth + 13
	podEntCommandZone = EntityRawWidth + 17
	podEntCommander   = EntityRawWidth + 18

	podGroupHand    = maxSeatBlocks
	podGroupStack   = maxSeatBlocks + 1
	podGroupCommand = maxSeatBlocks + 2

	// entZoneCommand is the entity zone class of a command-zone card
	// (policynet podZoneCommand).
	entZoneCommand = podEntCommandZone
)

func podOneHot(raw *[EntityRawWidthPod]float32, off, r int) {
	if r >= 0 && r < maxSeatBlocks {
		raw[off+r] = 1
	}
}

// podBfGroup is the pooled group of a battlefield controlled by seat p.
func (v *vview) podBfGroup(p uint8) uint8 {
	return uint8(min(v.relSeat(p), maxSeatBlocks-1))
}

// podEntities is policynet's pod entity order: my battlefield, every other
// battlefield in seat order (each grouped by its controller's relative
// seat), my hand, the stack, then every seat's command-zone commanders.
func (e *Encoder) podEntities(seat uint8, me *vplayer, one func(ci int32, group uint8, zone int)) {
	v := &e.v
	for _, ci := range me.bf {
		one(ci, 0, entZoneBf)
	}
	for p := uint8(0); p < v.n; p++ {
		if p == seat {
			continue
		}
		g := v.podBfGroup(p)
		for _, ci := range v.players[p].bf {
			one(ci, g, entZoneBf)
		}
	}
	for _, ci := range me.hand {
		one(ci, podGroupHand, entZoneHand)
	}
	for i := range v.stack {
		if ci := v.stack[i].cv; ci >= 0 {
			one(ci, podGroupStack, entZoneStack)
		}
	}
	for p := uint8(0); p < v.n; p++ {
		for _, ci := range v.players[p].cmd {
			one(ci, podGroupCommand, entZoneCommand)
		}
	}
}

// podEntity is policynet's podEntity: the pod raw columns of c.
func (e *Encoder) podEntity(raw *[EntityRawWidthPod]float32, c *vcard, zone int) {
	v := &e.v
	s := e.st0
	raw[entMine] = 0
	po := e.po(c.obj)
	podOneHot(raw, podEntCtrl0, v.relSeat(c.ctrl))
	podOneHot(raw, podEntOwner0, v.relSeat(po.Owner))
	if zone == entZoneBf && c.attacking && po.Attacks != dirac.NoTarget {
		podOneHot(raw, podEntAttacked0, v.relSeat(s.FeatAttackDefender(po.Attacks)))
		if !dirac.FeatAttacksPlayer(po.Attacks) {
			raw[podEntAttacksPerm] = 1
		}
	}
	if zone == entZoneBf {
		for p := uint8(0); p < v.n; p++ {
			if s.FeatGoadedBy(c.obj, p) {
				podOneHot(raw, podEntGoadedBy0, v.relSeat(p))
			}
		}
	}
	if zone == entZoneCommand {
		raw[podEntCommandZone] = 1
	}
	for p := uint8(0); p < v.n; p++ {
		if s.FeatCommanderN(p, 0) == c.obj || s.FeatCommanderN(p, 1) == c.obj {
			raw[podEntCommander] = 1
			break
		}
	}
}
