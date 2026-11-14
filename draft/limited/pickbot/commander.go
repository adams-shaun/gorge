package pickbot

import (
	"fmt"
	"math"
	"math/bits"
	"math/rand/v2"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
)

// The Commander-draft persona (Commander Legends / Mystery Booster Commander
// drafts: pick two per pass, then a 60-card deck, or the event's other
// size, around one or two drafted commanders). The drafter takes a
// commander early, then drafts inside the commanders' colour identity; the
// builder chooses the commander option whose identity holds the strongest
// deck, and fills the land base.

// Commander-draft knobs. The scores are on Heuristic's points scale.
const (
	// cmdrLands is a 60-card deck's land count, nonbasic fixing included;
	// cmdrLandsFor scales it to the deck's size. cmdrSources is a 60-card
	// deck's mana-source floor: its lands plus the mana producers of mana
	// value cmdrSourceMV or less (rocks and dorks); cmdrSourcesFor scales it.
	cmdrLands    = 23
	cmdrSources  = 25
	cmdrSourceMV = 3
	// cmdrExtraLands is how many spells the colour fixing may trade for
	// basics beyond the land count.
	cmdrExtraLands = 6
	// cmdrHeavyCost is the colour search's cost, in sources, per extra pip
	// a card's cost holds beyond one in a colour (at most two).
	cmdrHeavyCost = 0.02
	// cmdrCommitPick is the pool size from which the drafter treats its
	// best commander option's identity as its colours.
	cmdrCommitPick = 10
	// cmdrBonus is a commander's pick bonus while the pool holds none;
	// cmdrPairBonus a commander's while it pairs with the pool's best one.
	cmdrBonus     = 50.0
	cmdrPairBonus = 2.0
	// onIdentity and offIdentity are the committed drafter's colour terms.
	onIdentity  = 1.5
	offIdentity = -4.0
	// fixingBonus is a nonbasic land's value inside the identity, per colour
	// it produces (at most cmdrFixingColours of them; a land that makes
	// fewer than two identity colours fixes nothing). rockBonus is a mana
	// producer's (cmdrCard.rock) added value.
	fixingBonus       = 2.5
	cmdrFixingColours = 3
	rockBonus         = 3.0
	// widthPenalty is an option's cost per identity colour beyond two, when
	// the pool holds none of its fixing; each fixer (a land or producer
	// making two of the option's colours) waives a cmdrFixersPerColour-th
	// of it per extra colour.
	widthPenalty        = 12.0
	cmdrFixersPerColour = 3.0
	// cmdrWidthLoss is the share of its spells' value an option loses per
	// unfixed colour beyond two, at most cmdrWidthLossMax.
	cmdrWidthLoss    = 0.3
	cmdrWidthLossMax = 0.9
)

// hasPlainPartnerCard reports whether c carries plain partner.
func hasPlainPartnerCard(c *cards.Card) bool {
	if c == nil || len(c.Faces) == 0 {
		return false
	}
	for _, k := range c.Faces[0].Keywords {
		if k == "Partner" {
			return true
		}
	}
	return false
}

// cmdrCard is one pool or pack card's facts as the commander persona reads
// them.
type cmdrCard struct {
	ref  limited.CardRef
	card *cards.Card
	mask uint8
	pips [5]int
	mv   int
	// prod is the colours (WUBRG bits) the card's mana abilities make, an
	// any-colour producer all five. src is prod when the card counts as a
	// mana source: a land, or a nonland of mana value <= cmdrSourceMV.
	prod, src uint8
	value     float64
	land      bool
	commander bool
	// rock marks a nonland source (colourless ones too): a mana rock or
	// dork that gets rockBonus.
	rock bool
}

// producedMask is the colours (WUBRG bits) a face's mana abilities make.
func producedMask(f *cards.Face) uint8 {
	mp := f.ManaProduction()
	if mp.Any {
		return 0x1f
	}
	var m uint8
	for i := 0; i < 5; i++ {
		if mp.Colour[i] > 0 {
			m |= 1 << i
		}
	}
	return m
}

// fixes is the number of the colours in mask that the card makes, capped at
// cmdrFixingColours, or 0 when it makes fewer than two of them.
func (c cmdrCard) fixes(mask uint8) int {
	n := bits.OnesCount8(c.src & mask)
	if n < 2 {
		return 0
	}
	return min(n, cmdrFixingColours)
}

// cmdrFacts resolves one card's facts; an unscripted card is valued at
// unscriptedScore so neither half of the persona plays it.
func cmdrFacts(lookup CardLookup, ref limited.CardRef, value func(*cards.Card, limited.CardRef) float64) cmdrCard {
	c := cmdrCard{ref: ref}
	if lookup != nil {
		c.card = lookup(ref.Name)
	}
	if c.card == nil || len(c.card.Faces) == 0 {
		c.value = unscriptedScore
		return c
	}
	f := factsOf(lookup, ref.Name)
	c.mask, c.pips = f.mask, f.pips
	c.value = Heuristic(c.card, ref.Rarity)
	if value != nil {
		c.value = value(c.card, ref)
	}
	face := c.card.Faces[0]
	c.land = face.IsLand()
	c.commander = deck.IsCommanderEligible(c.card)
	c.mv = int(face.ManaValue())
	c.prod = producedMask(face)
	if c.land || c.mv <= cmdrSourceMV {
		c.src = c.prod
	}
	if c.land {
		c.value = fixingBonus * float64(c.fixes(0x1f))
	} else if c.mv <= cmdrSourceMV && !face.ManaProduction().IsZero() {
		c.rock = true
		c.value += rockBonus
	}
	return c
}

// cmdrOption is one commander choice: a single commander or a pair, with
// its colour identity.
type cmdrOption struct {
	idx  []int // indices into the candidate slice
	mask uint8
}

// commanderOptions lists every single commander and every
// deck.IsDraftCommanderPair pair among cs, in index order.
func commanderOptions(cs []cmdrCard) []cmdrOption {
	var out []cmdrOption
	for i, c := range cs {
		if !c.commander {
			continue
		}
		out = append(out, cmdrOption{idx: []int{i}, mask: c.mask})
		for j := i + 1; j < len(cs); j++ {
			if cs[j].commander && deck.IsDraftCommanderPair(c.card, cs[j].card) {
				out = append(out, cmdrOption{idx: []int{i, j}, mask: c.mask | cs[j].mask})
			}
		}
	}
	return out
}

// cmdrDeckSize resolves a persona's DeckSize: 0 means
// build.CommanderDeckSize.
func cmdrDeckSize(size int) int {
	if size == 0 {
		return build.CommanderDeckSize
	}
	return size
}

// cmdrLandsFor is a size-card deck's land count: cmdrLands scaled from 60
// cards, rounded half up (50 cards: 19 lands).
func cmdrLandsFor(size int) int {
	return (2*cmdrLands*size + build.CommanderDeckSize) / (2 * build.CommanderDeckSize)
}

// cmdrSourcesFor is a size-card deck's mana-source floor: cmdrSources scaled
// from 60 cards, rounded half up.
func cmdrSourcesFor(size int) int {
	return (2*cmdrSources*size + build.CommanderDeckSize) / (2 * build.CommanderDeckSize)
}

// widthCost is the cost of a commander identity: widthPenalty per colour
// beyond two, a third colour and beyond straining a mostly basic mana base,
// less so for each of the fixers (cmdrCard.fixes) the pool holds for it.
func widthCost(mask uint8, fixers int) float64 {
	return widthPenalty * widthWaste(mask, fixers)
}

// widthWaste is the number of colours beyond two that the pool's fixers
// leave unfixed.
func widthWaste(mask uint8, fixers int) float64 {
	n := bits.OnesCount8(mask)
	if n <= 2 {
		return 0
	}
	waived := math.Min(1, float64(fixers)/(cmdrFixersPerColour*float64(n-2)))
	return float64(n-2) * (1 - waived)
}

// poolFixers counts the pool cards inside mask that fix it.
func poolFixers(cs []cmdrCard, mask uint8) int {
	n := 0
	for _, c := range cs {
		if c.mask&^mask == 0 && c.value > unscriptedScore && c.fixes(mask) > 0 {
			n++
		}
	}
	return n
}

// optionScore rates one commander option for a size-card deck: its
// commanders' values plus the best spells inside its identity that fit the
// deck's spell slots, minus a point per empty slot and widthPenalty per
// colour beyond two.
func optionScore(cs []cmdrCard, o cmdrOption, size int) float64 {
	score := 0.0
	in := map[int]bool{}
	for _, i := range o.idx {
		score += cs[i].value
		in[i] = true
	}
	var vals []float64
	fixers := 0
	for i, c := range cs {
		if in[i] || c.mask&^o.mask != 0 || c.value <= unscriptedScore {
			continue
		}
		if c.fixes(o.mask) > 0 {
			fixers++
		}
		if c.land {
			continue
		}
		vals = append(vals, c.value)
	}
	score -= widthCost(o.mask, fixers)
	// Unfixed colours beyond two also cost the spells a share of their value:
	// a deck that cannot cast them does not play them.
	keep := 1 - math.Min(cmdrWidthLossMax, cmdrWidthLoss*widthWaste(o.mask, fixers))
	sort.Sort(sort.Reverse(sort.Float64Slice(vals)))
	slots := size - cmdrLandsFor(size) - len(o.idx)
	for i := 0; i < slots; i++ {
		if i < len(vals) {
			score += vals[i] * keep
		} else {
			score--
		}
	}
	return score
}

// bestOption is the highest-scoring commander option for a size-card deck,
// ties to the earlier option; ok is false when cs holds no commander.
func bestOption(cs []cmdrCard, size int) (cmdrOption, bool) {
	var best cmdrOption
	bestScore, ok := 0.0, false
	for _, o := range commanderOptions(cs) {
		if s := optionScore(cs, o, size); !ok || s > bestScore {
			best, bestScore, ok = o, s, true
		}
	}
	return best, ok
}

// CommanderDrafter is the Commander-draft drafter: it rates each pack card
// by Heuristic (or Value, when set), takes a commander while its pool has
// none, and from cmdrCommitPick picks on drafts inside its best commander
// option's colour identity, valuing a commander that pairs with that option.
// DeckSize is the event's deck size the options are scored for; 0 means
// build.CommanderDeckSize.
type CommanderDrafter struct {
	Lookup   CardLookup
	DeckSize int
	// Value, when non-nil, replaces Heuristic as a nonland card's value
	// (a learned or simulated value table); nil keeps Heuristic.
	Value func(c *cards.Card, ref limited.CardRef) float64
}

// PickAdvice is one pack card's rating by the Commander drafter, split into
// the terms a player can read: Value (the card's own strength), Commander
// (the commander bonus: none in the pool yet, or a pair for the plan) and
// Identity (inside, outside or open to the plan's colour identity). Score is
// their sum, the number Pick maximises; Reason spells the terms.
type PickAdvice struct {
	ID        string
	Score     float64
	Value     float64
	Commander float64
	Identity  float64
	Reason    string
}

// facts is cmdrFacts with the drafter's Value override.
func (d *CommanderDrafter) facts(ref limited.CardRef) cmdrCard {
	return cmdrFacts(d.Lookup, ref, d.Value)
}

// Rank rates every pack card in pack order, and names the plan's colour
// identity ("" before the pool holds a commander).
func (d *CommanderDrafter) Rank(v PickView) ([]PickAdvice, uint8) {
	pool := make([]cmdrCard, len(v.Picks))
	for i, ref := range v.Picks {
		pool[i] = d.facts(ref)
	}
	plan, committed := bestOption(pool, cmdrDeckSize(d.DeckSize))
	hasCommander := committed
	committed = committed && len(pool) >= cmdrCommitPick
	var lead *cards.Card
	var planMask uint8
	if hasCommander {
		lead = pool[plan.idx[0]].card
		planMask = plan.mask
	}
	// open: the plan is one commander that may still take a partner, so a
	// card one colour outside its identity may yet be played.
	open := hasCommander && len(plan.idx) == 1 && (hasPlainPartnerCard(lead) || deck.HasImpliedDraftPartner(lead))

	out := make([]PickAdvice, len(v.Pack))
	for i, ref := range v.Pack {
		c := d.facts(ref)
		a := PickAdvice{ID: ref.ID, Value: c.value}
		var why []string
		why = append(why, fmt.Sprintf("value %.1f", c.value))
		if c.commander {
			switch {
			case !hasCommander:
				a.Commander = cmdrBonus - widthCost(c.mask, 0)
				why = append(why, "a commander, and you have none yet")
			case len(plan.idx) == 1 && deck.IsDraftCommanderPair(lead, c.card):
				a.Commander = cmdrPairBonus
				why = append(why, "pairs with "+lead.Faces[0].Name)
			case widthCost(planMask, poolFixers(pool, planMask)) > widthCost(c.mask, poolFixers(pool, c.mask)):
				// a narrower commander than a plan with too little fixing
				a.Commander = widthCost(planMask, poolFixers(pool, planMask)) - widthCost(c.mask, poolFixers(pool, c.mask))
				why = append(why, "a narrower commander than your plan")
			}
		}
		if committed && !c.land {
			switch {
			case c.mask&^plan.mask == 0:
				a.Identity = onIdentity
				why = append(why, "inside your commander's colours")
			case c.commander && len(plan.idx) == 1 && deck.IsDraftCommanderPair(lead, c.card):
				// a pairing commander widens the identity: no penalty
			case open && bits.OnesCount8(c.mask&^plan.mask) <= 1:
				// playable once a partner joins: neither bonus nor penalty
				why = append(why, "playable if a partner joins")
			default:
				a.Identity = offIdentity
				why = append(why, "outside your commander's colours")
			}
		}
		a.Score = c.value + a.Commander + a.Identity
		if committed && c.land {
			if c.mask&^plan.mask != 0 {
				a.Score = 0
				why = append(why, "land outside your colours")
			} else {
				a.Value = fixingBonus * float64(c.fixes(plan.mask))
				a.Score = a.Value + a.Commander
				if a.Value > 0 {
					why = append(why, "fixing for your colours")
				}
			}
		}
		a.Reason = strings.Join(why, "; ")
		out[i] = a
	}
	return out, planMask
}

// Pick returns the pack card the drafter values most; r breaks ties.
func (d *CommanderDrafter) Pick(v PickView, r *rand.Rand) string {
	ranked, _ := d.Rank(v)
	bestID, bestScore := "", 0.0
	for i, a := range ranked {
		score := a.Score + r.Float64()*1e-6
		if i == 0 || score > bestScore {
			bestID, bestScore = a.ID, score
		}
	}
	return bestID
}

// CommanderBuilder is the Commander-draft builder: it picks the commander
// option (bestOption) whose identity holds the strongest spells, plays its
// best on-identity spells and on-identity nonbasic lands, and fills the
// deck to exactly DeckSize cards, commanders included (0 means
// build.CommanderDeckSize), with basics of the identity's colours split by
// pip share. The land count scales with the size (cmdrLandsFor: 23 at 60,
// 19 at 50). A pool without a commander yields a deck without one, which
// ValidateCommanderSize refuses.
type CommanderBuilder struct {
	Lookup   CardLookup
	DeckSize int
	// Value, when non-nil, replaces Heuristic as a nonland card's value,
	// as CommanderDrafter.Value does; nil keeps Heuristic. Lands keep
	// their fixing value.
	Value func(c *cards.Card, ref limited.CardRef) float64
}

// Build returns the seat's Commander-draft deck.
func (b *CommanderBuilder) Build(set string, pool []limited.CardRef) build.Deck {
	cs := make([]cmdrCard, len(pool))
	for i, ref := range pool {
		cs[i] = cmdrFacts(b.Lookup, ref, b.Value)
	}
	size := cmdrDeckSize(b.DeckSize)
	nLands := cmdrLandsFor(size)
	plan, ok := bestOption(cs, size)
	var d build.Deck
	in := map[int]bool{}
	if ok {
		for _, i := range plan.idx {
			d.Main = append(d.Main, cs[i].ref.ID)
			d.Commanders = append(d.Commanders, cs[i].ref.ID)
			in[i] = true
		}
	}
	// On-identity nonbasic lands that fix (make two of the identity's
	// colours) count toward the land total, best fixing first.
	var lands, spells []int
	for i, c := range cs {
		if in[i] || c.value <= unscriptedScore || c.mask&^plan.mask != 0 {
			continue
		}
		if c.land {
			if c.fixes(plan.mask) > 0 {
				lands = append(lands, i)
			}
			continue
		}
		spells = append(spells, i)
	}
	byValue := func(idx []int) {
		sort.SliceStable(idx, func(a, b int) bool { return cs[idx[a]].value > cs[idx[b]].value })
	}
	byValue(lands)
	byValue(spells)
	if len(lands) > nLands/2 {
		lands = lands[:nLands/2]
	}
	slots := max(0, size-nLands-len(d.Commanders))
	sel, rest := spells, []int(nil)
	if len(spells) > slots {
		sel, rest = append([]int(nil), spells[:slots]...), spells[slots:]
	}
	sel = fillSources(cs, plan, lands, sel, rest, size, nLands)
	for _, i := range lands {
		d.Main = append(d.Main, cs[i].ref.ID)
	}
	for _, i := range sel {
		d.Main = append(d.Main, cs[i].ref.ID)
	}
	var pips [5]int
	for _, i := range append(append([]int(nil), plan.idx...), sel...) {
		for c, p := range cs[i].pips {
			pips[c] += p
		}
	}
	d.Basics = splitBasicsFor(cs, plan, lands, sel, size-len(d.Main), pips)
	return d
}

// deckSources counts, per colour, the deck's mana sources: the lands and
// the commanders and spells that are sources of it.
func deckSources(cs []cmdrCard, groups ...[]int) (n [5]int) {
	for _, g := range groups {
		for _, i := range g {
			for c := 0; c < 5; c++ {
				if cs[i].src&(1<<c) != 0 {
					n[c]++
				}
			}
		}
	}
	return n
}

// colourNeeds is the sources each colour of the deck's identity needs in a
// deck of nLands lands, per the mana-base rule: a colour with under 15% of
// the pips needs max(3, 0.18·lands); otherwise 0.35, 0.45 or 0.55 of the
// lands as its heaviest cost has 1, 2 or 3+ pips of it. A colour with no
// pips needs none.
func colourNeeds(cs []cmdrCard, mask uint8, nLands int, deckIdx ...[]int) (need [5]float64) {
	var pips, heaviest [5]int
	total := 0
	for _, g := range deckIdx {
		for _, i := range g {
			for c := 0; c < 5; c++ {
				pips[c] += cs[i].pips[c]
				total += cs[i].pips[c]
				heaviest[c] = max(heaviest[c], cs[i].pips[c])
			}
		}
	}
	l := float64(nLands)
	for c := 0; c < 5; c++ {
		switch {
		case mask&(1<<c) == 0 || pips[c] == 0:
		case float64(pips[c]) < 0.15*float64(total):
			need[c] = math.Max(3, 0.18*l)
		case heaviest[c] >= 3:
			need[c] = 0.55 * l
		case heaviest[c] == 2:
			need[c] = 0.45 * l
		default:
			need[c] = 0.35 * l
		}
	}
	return need
}

// basicsFor splits n basics over the colours that have a need, each going
// to the colour furthest short of its need (relative), given the sources
// already held; ties to the lower colour. ok is false when no colour has a
// need.
func basicsFor(n int, have [5]int, need [5]float64) (out [5]int, ok bool) {
	for k := 0; k < n; k++ {
		best, bestShort := -1, 0.0
		for c := 0; c < 5; c++ {
			if need[c] == 0 {
				continue
			}
			if short := (need[c] - float64(have[c]+out[c])) / need[c]; best < 0 || short > bestShort {
				best, bestShort = c, short
			}
		}
		if best < 0 {
			return out, false
		}
		out[best]++
	}
	return out, n == 0 || out != [5]int{}
}

// shortfall is the sum over colours of the sources a deck lacks.
func shortfall(have, basics [5]int, need [5]float64) float64 {
	s := 0.0
	for c := 0; c < 5; c++ {
		s += math.Max(0, need[c]-float64(have[c]+basics[c]))
	}
	return s
}

// fillSources trades the weakest chosen spells for mana producers (rest
// holds the unchosen spells, best first): first until the deck's lands and
// producers reach cmdrSourcesFor(size), then for producers of the colour the
// deck is furthest short of, while that shortfall shrinks.
func fillSources(cs []cmdrCard, plan cmdrOption, lands, sel, rest []int, size, nLands int) []int {
	sel = append([]int(nil), sel...)
	rest = append([]int(nil), rest...)
	// swap trades the weakest non-producer in sel for the best unchosen
	// card want accepts.
	swap := func(want func(i int) bool) bool {
		in := -1
		for k := len(sel) - 1; k >= 0; k-- {
			if !cs[sel[k]].rock {
				in = k
				break
			}
		}
		if in < 0 {
			return false
		}
		for k, i := range rest {
			if want(i) {
				sel[in], rest[k] = i, sel[in]
				byVal := func(idx []int) {
					sort.SliceStable(idx, func(a, b int) bool { return cs[idx[a]].value > cs[idx[b]].value })
				}
				byVal(sel)
				byVal(rest)
				return true
			}
		}
		return false
	}
	producers := func() int {
		n := 0
		for _, g := range [][]int{plan.idx, sel} {
			for _, i := range g {
				if cs[i].rock {
					n++
				}
			}
		}
		return n
	}
	for producers() < cmdrSourcesFor(size)-nLands {
		if !swap(func(i int) bool { return cs[i].rock }) {
			break
		}
	}
	// Short of producers, the floor is met with lands: the weakest spells
	// make way for basics.
	for len(sel) > 0 && producers() < cmdrSourcesFor(size)-(size-len(plan.idx)-len(sel)) {
		k := len(sel) - 1
		for k >= 0 && cs[sel[k]].rock {
			k--
		}
		if k < 0 {
			break
		}
		sel = append(sel[:k], sel[k+1:]...)
	}
	// Then, while the colour rule is unmet, take the swap of a chosen
	// non-producer for an unchosen card that most shrinks the shortfall,
	// the cheapest in value among equals.
	// heavy adds a small cost per heavy pip commitment, so a swap that
	// lightens one of several heavy cards counts as progress before the
	// last of them leaves.
	heavy := func(sel []int) float64 {
		h := 0.0
		for _, g := range [][]int{plan.idx, sel} {
			for _, i := range g {
				for c := 0; c < 5; c++ {
					if p := cs[i].pips[c]; p >= 2 && plan.mask&(1<<c) != 0 {
						h += float64(min(p, 3) - 1)
					}
				}
			}
		}
		return h * cmdrHeavyCost
	}
	short := func(sel []int) float64 {
		basics := size - len(plan.idx) - len(lands) - len(sel)
		need := colourNeeds(cs, plan.mask, size-len(plan.idx)-len(sel), plan.idx, sel)
		have := deckSources(cs, plan.idx, lands, sel)
		alloc, _ := basicsFor(basics, have, need)
		return shortfall(have, alloc, need)
	}
	floor := len(sel) - cmdrExtraLands
	for cur := short(sel); cur > 0; {
		curObj := cur + heavy(sel)
		bestK, bestR, bestObj, bestLoss := -1, -2, curObj, 0.0
		try := func(k, r int, o, loss float64) {
			if o < bestObj-1e-9 || (bestK >= 0 && o < bestObj+1e-9 && loss < bestLoss) {
				bestK, bestR, bestObj, bestLoss = k, r, o, loss
			}
		}
		for k, out := range sel {
			if cs[out].rock {
				continue
			}
			for r, in := range rest {
				sel[k], rest[r] = in, out
				o := short(sel) + heavy(sel)
				sel[k], rest[r] = out, in
				try(k, r, o, cs[out].value-cs[in].value)
			}
			// or the spell makes way for a basic
			if len(sel) > floor {
				without := append(append([]int(nil), sel[:k]...), sel[k+1:]...)
				try(k, -1, short(without)+heavy(without), cs[out].value)
			}
		}
		if bestK < 0 {
			break
		}
		if bestR == -1 {
			sel = append(sel[:bestK], sel[bestK+1:]...)
		} else {
			sel[bestK], rest[bestR] = rest[bestR], sel[bestK]
		}
		cur = short(sel)
	}
	return sel
}

// splitBasicsFor is the deck's basics: n of them, split by basicsFor against
// the colour needs, or by pip share (splitBasics) when no colour has a need.
func splitBasicsFor(cs []cmdrCard, plan cmdrOption, lands, sel []int, n int, pips [5]int) [5]int {
	need := colourNeeds(cs, plan.mask, n+len(lands), plan.idx, sel)
	if out, ok := basicsFor(n, deckSources(cs, plan.idx, lands, sel), need); ok {
		return out
	}
	return splitBasics(n, plan.mask, pips)
}

// splitBasics divides n basics over the identity's colours by pip share
// (largest remainder, ties to the lower colour), evenly when the spells
// carry no pips. A colourless identity spreads them over all five as a
// stand-in: the limited basics are the five coloured ones, no Wastes.
func splitBasics(n int, mask uint8, pips [5]int) [5]int {
	var out [5]int
	if n <= 0 {
		return out
	}
	var w [5]int
	total := 0
	for c := 0; c < 5; c++ {
		if mask&(1<<c) != 0 {
			w[c] = pips[c]
			total += w[c]
		}
	}
	if total == 0 {
		for c := 0; c < 5; c++ {
			if mask&(1<<c) != 0 {
				w[c] = 1
				total++
			}
		}
	}
	if total == 0 {
		w = [5]int{1, 1, 1, 1, 1}
		total = 5
	}
	var rem [5]int
	left := n
	for c := range out {
		out[c] = n * w[c] / total
		rem[c] = n * w[c] % total
		left -= out[c]
	}
	order := []int{0, 1, 2, 3, 4}
	sort.SliceStable(order, func(a, b int) bool { return rem[order[a]] > rem[order[b]] })
	for _, c := range order {
		if left == 0 {
			break
		}
		if w[c] > 0 {
			out[c]++
			left--
		}
	}
	return out
}
