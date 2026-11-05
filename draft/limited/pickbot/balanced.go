package pickbot

import (
	"math/bits"
	"sort"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Balanced is a limited builder that plays a mana curve, a creature band and
// every spell type the pool offers, with card quality deciding within those
// constraints. It chooses the colour pair exactly as Best23 does, then picks
// 23 spells inside that pair and fills the rest with basics through
// build.AutoLands.
type Balanced struct {
	Lookup  CardLookup
	Tuning  Tuning
	Ratings *RatingsOverride
	Shape   DeckShape
}

// tuning returns the builder's knob set, filling the zero value with the
// spec defaults.
func (b *Balanced) tuning() Tuning {
	if b.Tuning == (Tuning{}) {
		return DefaultTuning
	}
	return b.Tuning
}

// shape returns the builder's target, filling the zero value with the spec
// defaults.
func (b *Balanced) shape() DeckShape {
	if b.Shape == (DeckShape{}) {
		return DefaultDeckShape
	}
	return b.Shape
}

// Build turns a finished pool into a deck.
func (b *Balanced) Build(set string, pool []limited.CardRef) build.Deck {
	s := b.shape()
	all := (&Best23{Lookup: b.Lookup, Tuning: b.Tuning, Ratings: b.Ratings}).candidates(set, pool)
	allowed := choosePair(all, s.MainSize, b.tuning().ShortPairPenalty)

	eligible := make([]candidate, 0, len(all))
	for _, c := range all {
		if c.mask&^allowed == 0 {
			eligible = append(eligible, c)
		}
	}

	// A pair short of MainSize splashes a third colour, or failing that
	// plays ThinMainSize spells and an eighteenth land.
	splash := -1
	if len(eligible) < s.MainSize {
		var extra []candidate
		splash, extra = splashCards(all, allowed, s.MainSize-len(eligible), s.SplashMax)
		if splash >= 0 {
			eligible = withSplash(all, allowed, extra)
		}
	}
	if len(eligible) < s.MainSize && s.ThinMainSize > 0 && s.ThinMainSize < s.MainSize {
		s.MainSize = s.ThinMainSize
	}

	sel := &selection{shape: s, taken: make(map[string]bool, s.MainSize)}
	sel.typeQuotas(eligible)
	sel.creatureFloor(eligible)
	sel.curveFloors(eligible)
	sel.fill(eligible)
	sel.thinPool(all)

	deck := build.Deck{Basics: [5]int{}}
	pips := make(map[string][5]int, len(sel.main))
	for _, c := range sel.main {
		deck.Main = append(deck.Main, c.id)
		pips[c.id] = c.pips
	}
	deckColours := allowed
	if splash >= 0 {
		deckColours |= 1 << splash
	}
	duals := b.duals(pool, deckColours, splash, s.DualMax)
	for _, d := range duals {
		deck.Main = append(deck.Main, d.id)
	}
	deck.Basics = build.AutoLands(deck.Main, func(id string) [5]int { return pips[id] })
	if splash >= 0 {
		sources := deck.Basics[splash]
		for _, d := range duals {
			if d.produces&(1<<splash) != 0 {
				sources++
			}
		}
		for sources < s.SplashSources && moveBasic(&deck.Basics, allowed, splash) {
			sources++
		}
	}
	return deck
}

// splashCards picks the splash colour for a pair short by shortfall cards:
// the colour outside the pair with the most one-pip cards inside pair plus
// that colour (capped at max and shortfall), then the larger rating sum,
// then the lower index. It returns -1 and nil when no colour has a card.
func splashCards(all []candidate, allowed uint8, shortfall, limit int) (int, []candidate) {
	best, bestN, bestScore := -1, 0, 0.0
	var bestCards []candidate
	for c := 0; c < 5; c++ {
		bit := uint8(1) << c
		if allowed&bit != 0 {
			continue
		}
		var options []candidate
		for _, cd := range all {
			if cd.mask&bit != 0 && cd.mask&^(allowed|bit) == 0 && cd.pips[c] <= 1 {
				options = append(options, cd)
			}
		}
		n := min(shortfall, limit, len(options))
		score := 0.0
		for _, cd := range options[:n] {
			score += cd.rating
		}
		if n > bestN || n > 0 && n == bestN && score > bestScore {
			best, bestN, bestScore, bestCards = c, n, score, options[:n]
		}
	}
	return best, bestCards
}

// withSplash returns the in-pair candidates plus the splash cards, in
// candidate order.
func withSplash(all []candidate, allowed uint8, splash []candidate) []candidate {
	chosen := make(map[string]bool, len(splash))
	for _, c := range splash {
		chosen[c.id] = true
	}
	out := make([]candidate, 0, len(all))
	for _, c := range all {
		if c.mask&^allowed == 0 || chosen[c.id] {
			out = append(out, c)
		}
	}
	return out
}

// dualLand is a pool nonbasic land that makes only deck colours.
type dualLand struct {
	id, name string
	produces uint8
}

// producedColours is the OR of the colours a land's mana abilities can make.
func producedColours(f *cards.Face) uint8 {
	var mask uint8
	for _, a := range f.ManaAbilities() {
		counts, _ := cards.ProducedCounts(a.Params["Produced"])
		for i := 0; i < 5; i++ {
			if counts[i] > 0 {
				mask |= 1 << i
			}
		}
	}
	return mask
}

// duals returns up to max of the pool's nonbasic lands whose produced
// colours are non-empty and inside deckColours: those making the splash
// colour (when splash >= 0) first, then those making more deck colours,
// then by name and ID.
func (b *Balanced) duals(pool []limited.CardRef, deckColours uint8, splash, max int) []dualLand {
	if b.Lookup == nil || max <= 0 {
		return nil
	}
	var out []dualLand
	for _, ref := range pool {
		c := b.Lookup(ref.Name)
		if c == nil || len(c.Faces) == 0 {
			continue
		}
		f := c.Faces[0]
		if !f.IsLand() || f.IsBasic() {
			continue
		}
		made := producedColours(f)
		if made == 0 || made&^deckColours != 0 {
			continue
		}
		out = append(out, dualLand{id: ref.ID, name: ref.Name, produces: made})
	}
	makesSplash := func(d dualLand) bool { return splash >= 0 && d.produces&(1<<splash) != 0 }
	sort.SliceStable(out, func(i, j int) bool {
		a, c := out[i], out[j]
		if makesSplash(a) != makesSplash(c) {
			return makesSplash(a)
		}
		if na, nc := bits.OnesCount8(a.produces), bits.OnesCount8(c.produces); na != nc {
			return na > nc
		}
		if a.name != c.name {
			return a.name < c.name
		}
		return a.id < c.id
	})
	if len(out) > max {
		out = out[:max]
	}
	return out
}

// moveBasic moves one basic from the pair colour with the most basics (the
// lower index on a tie) to the splash colour, reporting whether one moved.
func moveBasic(basics *[5]int, pair uint8, splash int) bool {
	from := -1
	for c := 0; c < 5; c++ {
		if pair&(1<<c) != 0 && basics[c] > 0 && (from < 0 || basics[c] > basics[from]) {
			from = c
		}
	}
	if from < 0 {
		return false
	}
	basics[from]--
	basics[splash]++
	return true
}

// choosePair returns the colour mask of the pair whose first mainSize
// in-pair candidates rate highest, charging penalty per unfilled slot. The
// earlier pair keeps a tie.
func choosePair(all []candidate, mainSize int, penalty float64) uint8 {
	var bestMask uint8
	bestSum := 0.0
	first := true
	for _, pair := range colourPairs {
		allowed := uint8(1<<pair[0] | 1<<pair[1])
		n := 0
		sum := 0.0
		for _, c := range all {
			if c.mask&^allowed != 0 {
				continue
			}
			if n >= mainSize {
				break
			}
			sum += c.rating
			n++
		}
		sum += penalty * float64(mainSize-n)
		if first || sum > bestSum {
			bestSum, bestMask, first = sum, allowed, false
		}
	}
	return bestMask
}

// selection is the in-progress maindeck with its curve and class counts.
type selection struct {
	shape   DeckShape
	main    []candidate
	taken   map[string]bool
	buckets [6]int
	classes [numClasses]int
}

func (s *selection) full() bool      { return len(s.main) >= s.shape.MainSize }
func (s *selection) creatures() int  { return s.classes[classCreature] }
func (s *selection) count(b int) int { return s.buckets[b] }

// fits reports whether c stays inside the curve maxima and creature cap.
func (s *selection) fits(c candidate) bool {
	if s.buckets[curveBucket(c.mv)] >= s.shape.CurveMax[curveBucket(c.mv)] {
		return false
	}
	if c.class == classCreature && s.creatures() >= s.shape.CreatureMax {
		return false
	}
	return true
}

func (s *selection) take(c candidate) {
	s.main = append(s.main, c)
	s.taken[c.id] = true
	s.buckets[curveBucket(c.mv)]++
	s.classes[c.class]++
}

// best returns the earliest untaken candidate in order that passes keep; a
// nil keep accepts any.
func (s *selection) best(cards []candidate, keep func(candidate) bool) (candidate, bool) {
	for _, c := range cards {
		if s.taken[c.id] {
			continue
		}
		if keep != nil && !keep(c) {
			continue
		}
		return c, true
	}
	return candidate{}, false
}

var quotaClasses = [...]cardClass{
	classInstant, classSorcery, classEnchantment, classArtifact, classPlaneswalker, classBattle,
}

// typeQuotas takes the best card of each quota class until its minimum is
// met. Curve maxima do not block a quota card.
func (s *selection) typeQuotas(eligible []candidate) {
	for _, class := range quotaClasses {
		for !s.full() && s.classes[class] < s.shape.TypeMin[class] {
			c, ok := s.best(eligible, func(c candidate) bool {
				return c.class == class && c.rating >= s.shape.QuotaFloor
			})
			if !ok {
				break
			}
			s.take(c)
		}
	}
}

// creatureFloor takes creatures until CreatureMin, preferring ones below
// their curve minimum.
func (s *selection) creatureFloor(eligible []candidate) {
	for !s.full() && s.creatures() < s.shape.CreatureMin {
		c, ok := s.best(eligible, func(c candidate) bool {
			b := curveBucket(c.mv)
			return c.class == classCreature && s.fits(c) && s.count(b) < s.shape.CurveMin[b]
		})
		if !ok {
			c, ok = s.best(eligible, func(c candidate) bool {
				return c.class == classCreature && s.fits(c)
			})
		}
		if !ok {
			return
		}
		s.take(c)
	}
}

// curveFloors takes the best fitting card in each bucket until its minimum
// is met.
func (s *selection) curveFloors(eligible []candidate) {
	for b := 0; b < 6; b++ {
		for !s.full() && s.count(b) < s.shape.CurveMin[b] {
			bucket := b
			c, ok := s.best(eligible, func(c candidate) bool {
				return curveBucket(c.mv) == bucket && s.fits(c)
			})
			if !ok {
				break
			}
			s.take(c)
		}
	}
}

// fill tops up with the best fitting card, relaxing the maxima only when
// nothing fits.
func (s *selection) fill(eligible []candidate) {
	for !s.full() {
		c, ok := s.best(eligible, s.fits)
		if !ok {
			c, ok = s.best(eligible, nil)
		}
		if !ok {
			return
		}
		s.take(c)
	}
}

// thinPool fills a pool too thin to field MainSize in-pair spells with the
// best remaining cards of any colour.
func (s *selection) thinPool(all []candidate) {
	for !s.full() {
		c, ok := s.best(all, nil)
		if !ok {
			return
		}
		s.take(c)
	}
}
