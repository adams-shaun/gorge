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
