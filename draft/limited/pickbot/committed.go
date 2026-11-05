package pickbot

import (
	"math/rand/v2"

	"github.com/adams-shaun/gorge/draft/limited"
)

// CommittedTuning is DefaultTuning with a stronger colour term: ColourFitMax
// 24 is the knee of the measured sweep (8 -> 16 -> 24 each move the count of
// seats short of in-pair playables; 40 adds nothing). Every other field is
// DefaultTuning's, so the ramp still reaches full weight by pack 1 pick 10.
var CommittedTuning = func() Tuning {
	t := DefaultTuning
	t.ColourFitMax = 24
	return t
}()

// DefaultNeedWeight is Committed's needs weight: a full class deficit is
// worth 8 centred points, the smallest swept value that ends off-colour
// padding and under the heuristic's 10-point common-to-rare gap, so need
// never takes a much weaker card.
const DefaultNeedWeight = 8.0

// needsFromPick is the draft pick (1-based, counted over the whole draft)
// from which the needs term applies, once the colour ramp is full.
const needsFromPick = 10

// curveNeedShare is the weight of a missing mana value relative to a
// missing card type: a missing type outranks a missing curve slot.
const curveNeedShare = 0.75

// Committed is RatingColour with a stronger colour commitment and, from the
// 10th pick, a needs term: among on-colour cards it favours the card type
// and mana value the pool is short of against the deck shape. The zero value
// is ready to use; set Lookup to give it card facts (without one it is
// RatingColour with the committed tuning).
type Committed struct {
	Lookup CardLookup
	// Tuning is the RatingColour knob set; the zero value is CommittedTuning.
	Tuning Tuning
	// Ratings is passed to the underlying RatingColour.
	Ratings *RatingsOverride
	// Shape is the deck shape the needs are measured against; the zero
	// value is DefaultDeckShape.
	Shape DeckShape
	// NeedWeight scales the needs term; zero is DefaultNeedWeight.
	NeedWeight float64
}

func (d *Committed) tuning() Tuning {
	if d.Tuning == (Tuning{}) {
		return CommittedTuning
	}
	return d.Tuning
}

func (d *Committed) shape() DeckShape {
	if d.Shape == (DeckShape{}) {
		return DefaultDeckShape
	}
	return d.Shape
}

func (d *Committed) weight() float64 {
	if d.NeedWeight == 0 {
		return DefaultNeedWeight
	}
	return d.NeedWeight
}

// pair is the two colours with the most picks, as a WUBRG mask; the lower
// colour index wins a tie.
func (d *Committed) pair(picks []limited.CardRef) uint8 {
	var count [5]int
	for _, p := range picks {
		m := factsOf(d.Lookup, p.Name).mask
		for c := 0; c < 5; c++ {
			if m&(1<<c) != 0 {
				count[c]++
			}
		}
	}
	first, second := -1, -1
	for c := 0; c < 5; c++ {
		switch {
		case first < 0 || count[c] > count[first]:
			first, second = c, first
		case second < 0 || count[c] > count[second]:
			second = c
		}
	}
	return uint8(1<<first | 1<<second)
}

// Scores is RatingColour's scores under the committed tuning plus, from the
// 10th pick, the needs term for each on-pair nonland card.
func (d *Committed) Scores(v PickView, dst []float64) []float64 {
	base := &RatingColour{Lookup: d.Lookup, Tuning: d.tuning(), Ratings: d.Ratings}
	dst = base.Scores(v, dst)
	if d.Lookup == nil || len(v.Picks) < needsFromPick-1 {
		return dst
	}
	pair := d.pair(v.Picks)
	shape := d.shape()
	w := d.weight()

	var haveClass [numClasses]int
	var haveBucket [6]int
	for _, p := range v.Picks {
		c := d.Lookup(p.Name)
		if c == nil || len(c.Faces) == 0 || c.Faces[0].IsLand() {
			continue
		}
		if c.ColourIdentity()&^pair != 0 {
			continue
		}
		front := c.Faces[0]
		haveClass[classOf(front)]++
		haveBucket[curveBucket(int(front.Cmc()))]++
	}

	for i, ref := range v.Pack {
		c := d.Lookup(ref.Name)
		if c == nil || len(c.Faces) == 0 || c.Faces[0].IsLand() {
			continue
		}
		if c.ColourIdentity()&^pair != 0 {
			continue
		}
		front := c.Faces[0]
		cls := classOf(front)
		tc := shape.TypeMin[cls]
		if cls == classCreature {
			tc = shape.CreatureMin + 2
		}
		if tc > 0 && haveClass[cls] < tc {
			dst[i] += w * float64(tc-haveClass[cls]) / float64(tc)
		}
		b := curveBucket(int(front.Cmc()))
		tb := 0
		if shape.CurveMin[b] > 0 {
			tb = shape.CurveMin[b] + 1
		}
		if tb > 0 && haveBucket[b] < tb {
			dst[i] += curveNeedShare * w * float64(tb-haveBucket[b]) / float64(tb)
		}
	}
	return dst
}

// Pick scores the pack (Scores) and returns the best card's ID, ties broken
// with r; "" for an empty pack.
func (d *Committed) Pick(v PickView, r *rand.Rand) string {
	if len(v.Pack) == 0 {
		return ""
	}
	return v.Pack[BestScore(d.Scores(v, nil), r)].ID
}
