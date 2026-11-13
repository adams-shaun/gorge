package pickbot

import (
	"math/bits"
	"math/rand/v2"
	"sort"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
)

// The Commander-draft persona (Commander Legends / Mystery Booster Commander
// drafts: pick two per pass, then a 60-card deck around one or two drafted
// commanders). The drafter takes a commander early, then drafts inside the
// commanders' colour identity; the builder chooses the commander option
// whose identity holds the strongest 60, and fills the land base.

// Commander-draft knobs. The scores are on Heuristic's points scale.
const (
	// cmdrLands is the deck's land count, nonbasic fixing included.
	cmdrLands = 17
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
	// it produces.
	fixingBonus = 0.75
	// widthPenalty is an option's cost per identity colour beyond two.
	widthPenalty = 4.0
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
	ref       limited.CardRef
	card      *cards.Card
	mask      uint8
	pips      [5]int
	value     float64
	land      bool
	commander bool
}

// cmdrFacts resolves one card's facts; an unscripted card is valued at
// unscriptedScore so neither half of the persona plays it.
func cmdrFacts(lookup CardLookup, ref limited.CardRef) cmdrCard {
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
	c.land = c.card.Faces[0].IsLand()
	c.commander = deck.IsCommanderEligible(c.card)
	if c.land {
		c.value = 0
		if n := bits.OnesCount8(c.mask); n > 0 {
			c.value = fixingBonus * float64(n)
		}
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

// optionScore rates one commander option: its commanders' values plus the
// best spells inside its identity that fit the deck's spell slots, minus a
// point per empty slot and widthPenalty per colour beyond two.
func optionScore(cs []cmdrCard, o cmdrOption) float64 {
	score := 0.0
	in := map[int]bool{}
	for _, i := range o.idx {
		score += cs[i].value
		in[i] = true
	}
	var vals []float64
	for i, c := range cs {
		if in[i] || c.land || c.mask&^o.mask != 0 || c.value <= unscriptedScore {
			continue
		}
		vals = append(vals, c.value)
	}
	// A third colour and beyond strains a 17-land, mostly basic mana base.
	if n := bits.OnesCount8(o.mask); n > 2 {
		score -= widthPenalty * float64(n-2)
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(vals)))
	slots := build.CommanderDeckSize - cmdrLands - len(o.idx)
	for i := 0; i < slots; i++ {
		if i < len(vals) {
			score += vals[i]
		} else {
			score--
		}
	}
	return score
}

// bestOption is the highest-scoring commander option, ties to the earlier
// option; ok is false when cs holds no commander.
func bestOption(cs []cmdrCard) (cmdrOption, bool) {
	var best cmdrOption
	bestScore, ok := 0.0, false
	for _, o := range commanderOptions(cs) {
		if s := optionScore(cs, o); !ok || s > bestScore {
			best, bestScore, ok = o, s, true
		}
	}
	return best, ok
}

// CommanderDrafter is the Commander-draft drafter: it rates each pack card
// by Heuristic, takes a commander while its pool has none, and from
// cmdrCommitPick picks on drafts inside its best commander option's colour
// identity, valuing a commander that pairs with that option.
type CommanderDrafter struct {
	Lookup CardLookup
}

// Pick returns the pack card the drafter values most; r breaks ties.
func (d *CommanderDrafter) Pick(v PickView, r *rand.Rand) string {
	pool := make([]cmdrCard, len(v.Picks))
	for i, ref := range v.Picks {
		pool[i] = cmdrFacts(d.Lookup, ref)
	}
	plan, committed := bestOption(pool)
	hasCommander := committed
	committed = committed && len(pool) >= cmdrCommitPick
	var lead *cards.Card
	if hasCommander {
		lead = pool[plan.idx[0]].card
	}
	// open: the plan is one commander that may still take a partner, so a
	// card one colour outside its identity may yet be played.
	open := hasCommander && len(plan.idx) == 1 && (hasPlainPartnerCard(lead) || deck.HasImpliedDraftPartner(lead))

	bestID, bestScore := "", 0.0
	for i, ref := range v.Pack {
		c := cmdrFacts(d.Lookup, ref)
		score := c.value
		if c.commander {
			switch {
			case !hasCommander:
				score += cmdrBonus
			case len(plan.idx) == 1 && deck.IsDraftCommanderPair(lead, c.card):
				score += cmdrPairBonus
			}
		}
		if committed && !c.land {
			switch {
			case c.mask&^plan.mask == 0:
				score += onIdentity
			case c.commander && len(plan.idx) == 1 && deck.IsDraftCommanderPair(lead, c.card):
				// a pairing commander widens the identity: no penalty
			case open && bits.OnesCount8(c.mask&^plan.mask) <= 1:
				// playable once a partner joins: neither bonus nor penalty
			default:
				score += offIdentity
			}
		}
		if committed && c.land && c.mask&^plan.mask != 0 {
			score = 0
		}
		score += r.Float64() * 1e-6
		if i == 0 || score > bestScore {
			bestID, bestScore = ref.ID, score
		}
	}
	return bestID
}

// CommanderBuilder is the Commander-draft builder: it picks the commander
// option (bestOption) whose identity holds the strongest spells, plays its
// best on-identity spells and on-identity nonbasic lands, and fills the
// deck to build.CommanderDeckSize with basics of the identity's colours
// split by pip share. A pool without a commander yields a deck without
// one, which ValidateCommander refuses.
type CommanderBuilder struct {
	Lookup CardLookup
}

// Build returns the seat's Commander-draft deck.
func (b *CommanderBuilder) Build(set string, pool []limited.CardRef) build.Deck {
	cs := make([]cmdrCard, len(pool))
	for i, ref := range pool {
		cs[i] = cmdrFacts(b.Lookup, ref)
	}
	plan, ok := bestOption(cs)
	var d build.Deck
	in := map[int]bool{}
	if ok {
		for _, i := range plan.idx {
			d.Main = append(d.Main, cs[i].ref.ID)
			d.Commanders = append(d.Commanders, cs[i].ref.ID)
			in[i] = true
		}
	}
	// On-identity nonbasic lands that make coloured mana count toward the
	// land total, best fixing first.
	var lands, spells []int
	for i, c := range cs {
		if in[i] || c.value <= unscriptedScore || c.mask&^plan.mask != 0 {
			continue
		}
		if c.land {
			if c.mask != 0 {
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
	if len(lands) > cmdrLands/2 {
		lands = lands[:cmdrLands/2]
	}
	for _, i := range lands {
		d.Main = append(d.Main, cs[i].ref.ID)
	}
	slots := build.CommanderDeckSize - cmdrLands - len(d.Commanders)
	if len(spells) > slots {
		spells = spells[:slots]
	}
	var pips [5]int
	for _, i := range append(append([]int(nil), plan.idx...), spells...) {
		for c, p := range cs[i].pips {
			pips[c] += p
		}
	}
	for _, i := range spells {
		d.Main = append(d.Main, cs[i].ref.ID)
	}
	d.Basics = splitBasics(build.CommanderDeckSize-len(d.Main), plan.mask, pips)
	return d
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
