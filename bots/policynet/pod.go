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
