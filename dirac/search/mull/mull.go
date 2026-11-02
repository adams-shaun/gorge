// Package mull is gorge's London-mulligan policies
// (bots/botpolicy/mulligan.go, mulligan_constructed.go, mulligan_registry.go)
// ported natively onto kernel card facts, the way dirac/search/hbot ports
// the auto-pay bot: the kernel Decision is not gorge's decision.Decision, so
// the policies cannot be called directly and the logic is ported, not the
// types.
//
// It imports only the kernel (dirac/kop, for the card facts). The facts are
// Card, one hand card's static record; the policies are pure functions over
// a hand of them. Two ask shapes exist in both engines -- the keep/mulligan
// ask and the bottoming ask -- and a bottoming answer gorge gives as one
// multi-pick set is returned here as the whole ascending set; the caller
// serves it one pick at a time as the kernel asks.
//
// Naming and the rng contract follow gorge's registry exactly: the five
// built-ins are coin, lands, never, constructed and forge; coin is the
// zero-value answer the caller decides itself (one 1/3 draw off its own rng
// per keep/mulligan ask that offers a mulligan), the other four consume no
// rng.
package mull

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// HandSize is the FDN Limited hand the round is played at: gorge's census
// (Board.handLands) only answers when the hand it reads is the game's
// HandSize, and the kernel's FDN games are always at 7 (CR 103.4: the hand
// is a full seven while the seat decides).
const HandSize = 7

// Work is the caller-owned scratch one hand's policies write into, so the
// mulligan path allocates nothing in the play loop.
// Every array is HandSize long because the kernel's London hand is always
// HandSize while the seat decides. The answers alias Work's buffers and are
// valid until the next call with the same Work; a worker keeps one per seat.
type Work struct {
	out       [HandSize]int
	idx       [HandSize]int
	spell     [HandSize]int
	land      [HandSize]int
	left      [HandSize]int
	forgeRem  [HandSize]int
	forgePool [HandSize]int
	forgeTuck [HandSize]int
	need      [5]int32
	cover     [5]int32
}

// Card is one hand card's static facts the policies read. Land is gorge
// isLandCard's test (a basic, or a noncreature with no mana cost): the
// kernel's packed cost has no "no cost" string, but a card without a printed
// cost has CMC 0 and no pips, so the CMC-0 noncreature arm is the same set
// on the FDN and vanilla pools (every non-basic land there prints "no
// cost"; no nonland card is costless). Prod is whether the card records any
// mana production at all (a {C}-only producer is Prod without colours).
type Card struct {
	PrintedName string // the printed name (gorge Card.PrintedName; forge's copy-count tiebreak)
	Land        bool
	Basic       bool  // the basic supertype (gorge Card.Basic)
	Creature    bool  // the creature type (gorge Card.Creature)
	Power       int32 // printed power (gorge Card.Power for a card in hand)
	CMC         int32
	Pips        [5]int32 // WUBRG coloured pips of the printed cost (gorge colourPips)
	Produces    [5]bool  // WUBRG colours the card's mana abilities produce
	ProdAny     bool     // a Produced$ Any / Combo / Chosen / special-choice producer (gorge ManaProduction.Any)
	Prod        bool     // any recorded production at all (gorge ManaProduction != zero)
}

// CardOf reads the facts off a kernel card record. The coloured pips are the
// packed cost's WUBRG pips: a hybrid pip lives in Hyb (gorge colourPips does
// not count hybrids either) and {X} counts no pips in either engine. A mana
// ability is "any" when its colours are all of WUBRG (Produced$ Any) or when
// kgen marks it a script-level choice (Produced$ Combo ...), a chosen colour
// or an each-colour-among special: gorge's ManaProduction.Any is set for all
// of those, and producesColour then counts every colour.
func CardOf(def kop.CardDef) Card {
	c := Card{PrintedName: def.Name, CMC: int32(def.Cost.MV())}
	c.Land = def.Types&kop.TBasic != 0 ||
		(def.Types&kop.TCreature == 0 && c.CMC == 0)
	c.Basic = def.Types&kop.TBasic != 0
	c.Creature = def.Types&kop.TCreature != 0
	c.Power = int32(def.Power)
	for i := 0; i < 5; i++ {
		c.Pips[i] = int32(def.Cost.Pips[i])
	}
	for _, ab := range def.Abilities {
		if ab.Kind != kop.AbMana || ab.ManaN == 0 {
			continue
		}
		c.Prod = true
		if ab.ManaColors == kop.ColAny || ab.AFlags&(kop.AfManaChoice|kop.AfManaChosen|kop.AfManaEachColor) != 0 {
			c.ProdAny = true
		}
		for k := 0; k < 5; k++ {
			if ab.ManaColors&(1<<k) != 0 {
				c.Produces[k] = true
			}
		}
	}
	return c
}

// CoinMulls is the coin policy's answer to a drawn 1/3 lot: gorge's default
// arm (policy.go's KMulligan case) mulligans when r.IntN(3) == 0, one draw,
// only when a mulligan option is offered. The caller owns the rng.
func CoinMulls(draw int) bool { return draw == 0 }

// KeepNever is the never policy's keep/mulligan answer: always keep. It
// consumes no rng (a two-player seat that never mulligans is never asked to
// bottom; a bottoming ask under never is BottomCheapest, gorge's
// chooseDiscard).
func KeepNever() bool { return true }

// KeepLands is the lands policy's keep/mulligan rule (gorge keepByLands) for
// a hand of `hand` cards holding `lands` lands that will bottom `bottom`
// cards if kept (0 on the opening seven, 1 after one London mulligan, ...).
// With K = hand - bottom cards to keep:
//
//   - K >= 6 (the opening seven, or one mulligan): keep 2 to 5 lands among
//     the seven drawn; mulligan 0, 1, 6 or 7.
//   - K == 5 (two mulligans): keep unless the seven hold no land or no
//     spell.
//   - K <= 4: always keep.
//
// A hand the census cannot read (not a full opening seven) is kept, gorge
// handLands' ok=false arm.
func KeepLands(lands, hand, bottom int) bool {
	if hand <= 0 || hand != HandSize {
		return true
	}
	switch k := hand - bottom; {
	case k >= 6:
		return lands >= 2 && lands <= 5
	case k == 5:
		return lands >= 1 && lands < hand
	default:
		return true
	}
}

// KeepConstructed is the constructed policy's keep/mulligan rule (gorge
// keepConstructed): the lands policy's shape with the land window derived
// from the seat's own deck's land share (deckTotal cards, deckLands lands;
// gorge reads Board.OwnDeck, the seat's own genesis manifest) and a "has an
// early play" test on the seven. It consumes no rng.
func KeepConstructed(cards []Card, bottom, deckTotal, deckLands int) bool {
	hand := len(cards)
	if hand <= 0 || hand != HandSize {
		return true
	}
	lands := 0
	for _, c := range cards {
		if c.Land {
			lands++
		}
	}
	switch k := hand - bottom; {
	case k >= 6:
		lo, hi := landWindow(deckTotal, deckLands)
		return lands >= lo && lands <= hi && handHasEarlyPlay(cards)
	case k == 5:
		return lands >= 1 && lands < hand
	default:
		return true
	}
}

// forgeMulliganThreshold is Forge's Default.ai MULLIGAN_THRESHOLD.
const forgeMulliganThreshold = 4

// forgeProducesMana is gorge forgeProducesMana: the card records some mana
// production (including a colourless-only source), is an any-coloured
// producer, or is a basic. Unlike produces, no recorded production is NOT
// production: Evolving Wilds is a non-producing land to Forge's bottom rule.
func forgeProducesMana(c Card) bool { return c.Basic || c.ProdAny || c.Prod }

// forgeNameCopies counts copies of name in the land pool, gorge
// forgeNameCopies.
func forgeNameCopies(cards []Card, pool []int, name string) int {
	n := 0
	for _, idx := range pool {
		if cards[idx].PrintedName == name {
			n++
		}
	}
	return n
}

// forgeWorstLand returns the card index of the worst land in pool by Forge's
// getWorstLand order. pool holds card indices and is expected to hold only
// lands. Non-producing lands go first when any; otherwise every land is a
// candidate. A basic scores +1, a creature -4, and the highest score wins;
// on a tie a later land replaces the pick only when its printed name has
// strictly more copies in the pool.
func forgeWorstLand(cards []Card, pool []int) int {
	if len(pool) == 0 {
		return -1
	}
	nonProducing := false
	for _, idx := range pool {
		if !forgeProducesMana(cards[idx]) {
			nonProducing = true
			break
		}
	}
	best, bestScore := -1, 0
	for _, idx := range pool {
		c := cards[idx]
		if nonProducing && forgeProducesMana(c) {
			continue
		}
		score := 0
		if c.Basic {
			score++
		}
		if c.Creature {
			score -= 4
		}
		if best < 0 || score > bestScore {
			best, bestScore = idx, score
			continue
		}
		if score == bestScore &&
			forgeNameCopies(cards, pool, c.PrintedName) > forgeNameCopies(cards, pool, cards[best].PrintedName) {
			best = idx
		}
	}
	return best
}

// forgeWorstCard returns the position in rem of Forge's "worst card" among
// the cards still in hand (rem may mix lands and nonlands). rem holds card
// indices.
func forgeWorstCard(cards []Card, rem []int) int {
	if len(rem) == 0 {
		return -1
	}
	allLands := true
	for _, idx := range rem {
		if !cards[idx].Land {
			allLands = false
			break
		}
	}
	if allLands {
		pick := forgeWorstLand(cards, rem)
		for i, idx := range rem {
			if idx == pick {
				return i
			}
		}
		return -1
	}
	best := -1
	for i, idx := range rem {
		c := cards[idx]
		if !c.Creature {
			continue
		}
		if best < 0 || c.Power < cards[rem[best]].Power ||
			(c.Power == cards[rem[best]].Power && c.CMC < cards[rem[best]].CMC) {
			best = i
		}
	}
	if best >= 0 {
		return best
	}
	best = 0
	for i := 1; i < len(rem); i++ {
		if cards[rem[i]].CMC <= cards[rem[best]].CMC {
			best = i
		}
	}
	return best
}

// forgeTuck returns the card indices to bottom, in pick order, per Forge's
// tuckCardsViaMulligan loop (gorge forgeTuck). It stops early when the hand
// runs out. The work buffers hold the remaining-hand order, the land pool
// and the answer; no allocation.
func forgeTuck(cards []Card, k int, w *Work) []int {
	n := len(cards)
	if k <= 0 || n == 0 {
		return nil
	}
	if k > n {
		k = n
	}
	d := (n - k) / 2
	producingLands := 0
	for _, c := range cards {
		if c.Land && forgeProducesMana(c) {
			producingLands++
		}
	}
	rem := w.forgeRem[:n]
	for i := range rem {
		rem[i] = i
	}
	remN := n
	bottomedLands := 0
	outN := 0
	for outN < k && remN > 0 {
		lands := 0
		for i := 0; i < remN; i++ {
			if cards[rem[i]].Land {
				lands++
			}
		}
		if lands-bottomedLands > d {
			pn := 0
			for i := 0; i < remN; i++ {
				if cards[rem[i]].Land {
					w.forgePool[pn] = rem[i]
					pn++
				}
			}
			pick := forgeWorstLand(cards, w.forgePool[:pn])
			w.forgeTuck[outN] = pick
			outN++
			for i := 0; i < remN; i++ {
				if rem[i] == pick {
					copy(rem[i:remN-1], rem[i+1:remN])
					remN--
					break
				}
			}
			bottomedLands++
			continue
		}
		uncastable := -1
		for i := 0; i < remN; i++ {
			c := cards[rem[i]]
			if c.Land {
				continue
			}
			if producingLands == 0 || int(c.CMC) > producingLands {
				if uncastable < 0 || c.CMC > cards[rem[uncastable]].CMC {
					uncastable = i
				}
			}
		}
		if uncastable >= 0 {
			pick := rem[uncastable]
			w.forgeTuck[outN] = pick
			outN++
			copy(rem[uncastable:remN-1], rem[uncastable+1:remN])
			remN--
			continue
		}
		worst := forgeWorstCard(cards, rem[:remN])
		pick := rem[worst]
		w.forgeTuck[outN] = pick
		outN++
		copy(rem[worst:remN-1], rem[worst+1:remN])
		remN--
		if cards[pick].Land {
			bottomedLands++
		}
	}
	return w.forgeTuck[:outN]
}

// KeepForge is the forge policy's keep/mulligan rule (gorge keepForge), the
// clean-room port of Forge's London mulligan. gorge asks keep/mulligan with
// seven in hand and the penalty in the prompt, so it simulates Forge's
// deterministic bottoming on the seven and judges the remainder; this does
// the same. deckTotal/deckLands are the seat's own deck census (gorge
// Board.OwnDeck; zero total is the neutral no-manifest ratio).
func KeepForge(cards []Card, bottom, deckTotal, deckLands int, w *Work) bool {
	hand := len(cards)
	if hand <= 0 || hand != HandSize {
		return true
	}
	h := hand - bottom
	if h-bottom < forgeMulliganThreshold {
		return true
	}
	tucked := forgeTuck(cards, bottom, w)
	var out [HandSize]bool
	for _, idx := range tucked {
		if idx >= 0 && idx < hand {
			out[idx] = true
		}
	}
	lands := 0
	for i, c := range cards {
		if !out[i] && c.Land {
			lands++
		}
	}
	ratio := 3 // neutral without a manifest: neither deck override fires
	if deckTotal > 0 {
		librarySize := deckTotal - h
		libraryLands := deckLands - lands
		if libraryLands <= 0 {
			return true
		}
		ratio = librarySize / libraryLands
	}
	switch {
	case lands < 2:
		return ratio > 6
	case lands == h:
		return ratio < 2
	case h >= 7 && lands >= h-1:
		return ratio < 2
	default:
		return true
	}
}

// BottomForge is the forge policy's bottoming answer (gorge forgeBottoming):
// the whole ascending set of card indices to put on the bottom, computed
// from the hand in the caller's option order so ties break exactly as they
// do in KeepForge's keep-time simulation.
func BottomForge(cards []Card, m int, w *Work) []int {
	n := len(cards)
	if n == 0 || m <= 0 {
		return nil
	}
	if m > n {
		m = n
	}
	picks := forgeTuck(cards, m, w)
	if len(picks) == 0 {
		return nil
	}
	out := w.out[:len(picks)]
	copy(out, picks)
	for i := 1; i < len(out); i++ { // ascending, gorge chooseLowest's order
		x := out[i]
		j := i
		for j > 0 && out[j-1] > x {
			out[j] = out[j-1]
			j--
		}
		out[j] = x
	}
	return out
}

// landWindow is the land-count window of a 7-card keep (gorge
// Board.landWindow): lo is 2 and hi is ceil(7*share)+2 clamped to [4, 5],
// share being the own deck's land fraction (24 of 60 gives 5, 16 of 60 gives
// 4). Without a deck the window is 2..5, the fixed Limited window.
func landWindow(total, lands int) (lo, hi int) {
	lo, hi = 2, 5
	if total <= 0 {
		return lo, hi
	}
	// ceil(7*lands/total) in integers.
	exp := (7*lands + total - 1) / total
	hi = min(max(exp+2, 4), 5)
	return lo, hi
}

// produces reports whether card c can make colour (an index of the five
// colours): an "any" producer and a card with no recorded production count
// for every colour, the same reading gorge's landBottoming and
// producesColour use.
func produces(c Card, colour int) bool {
	if c.ProdAny {
		return true
	}
	if !c.Prod {
		return true
	}
	return c.Produces[colour]
}

// handHasEarlyPlay is gorge handHasEarlyPlay: the hand holds a spell
// castable by turn 3 with its lands AND its lands produce a colour some
// spell needs. A hand of only colourless spells has no needed colour, so
// that half is vacuous.
func handHasEarlyPlay(cards []Card) bool {
	lands := 0
	for _, c := range cards {
		if c.Land {
			lands++
		}
	}
	avail := min(lands, 3)
	if avail == 0 {
		return false
	}
	var src [5]int
	for _, l := range cards {
		if !l.Land {
			continue
		}
		for col := range src {
			if produces(l, col) {
				src[col]++
			}
		}
	}
	var needed [5]bool
	castable := false
	for _, s := range cards {
		if s.Land {
			continue
		}
		pips := s.Pips
		coloured := 0
		ok := s.CMC <= int32(avail)
		for col := range pips {
			if pips[col] > 0 {
				needed[col] = true
			}
			coloured += int(pips[col])
			if int(pips[col]) > min(src[col], avail) {
				ok = false
			}
		}
		if coloured > avail {
			ok = false
		}
		castable = castable || ok
	}
	if !castable {
		return false
	}
	anyNeeded := false
	for col := range needed {
		if needed[col] {
			anyNeeded = true
			if src[col] > 0 {
				return true
			}
		}
	}
	return !anyNeeded
}

// BottomToLands is the lands and constructed policies' bottoming answer
// (gorge landBottoming): m cards go to the bottom so the kept hand (K = n-m
// cards) holds at most maxLands = max(ceil(K/2), min(2, K)) lands. Lands go
// first while the kept hand would hold more than that; every other pick is
// a spell, the highest mana value first (ties on the lower index, gorge
// chooseDiscard's order for spells). Since maxLands is at least 2 for
// K >= 2, the rule never bottoms a hand below two lands while it has them.
// Among lands, the one whose colours the hand's spells need least is
// bottomed first: each land scores, over the colours it produces, the
// spells' pips of that colour divided by how many remaining lands produce
// it (a land with no recorded production, such as Evolving Wilds, or an
// "Any" producer counts for every colour); the lowest score goes, ties on
// the lower index. The answer is in ascending index order, like gorge
// chooseLowest's.
func BottomToLands(cards []Card, m int, w *Work) []int {
	n := len(cards)
	if m <= 0 {
		return nil
	}
	if m > n {
		m = n
	}
	nl, ns := 0, 0
	for i, c := range cards {
		if c.Land {
			w.land[nl] = i
			nl++
		} else {
			w.spell[ns] = i
			ns++
		}
	}
	k := n - m
	maxLands := max((k+1)/2, min(2, k))
	landsOut := min(max(nl-maxLands, 0), m)
	spellsOut := m - landsOut
	if spellsOut > ns { // unreachable for maxLands <= k; kept for safety
		spellsOut = ns
		landsOut = m - spellsOut
	}

	// Spells: highest mana value first, ties on the lower index. Insertion
	// sort over the at-most-HandSize indices: no allocation.
	for i := 1; i < ns; i++ {
		x := w.spell[i]
		j := i
		for j > 0 {
			prev := w.spell[j-1]
			if cards[prev].CMC > cards[x].CMC ||
				(cards[prev].CMC == cards[x].CMC && prev < x) {
				break
			}
			w.spell[j] = prev
			j--
		}
		w.spell[j] = x
	}
	outN := 0
	for i := 0; i < spellsOut; i++ {
		w.out[outN] = w.spell[i]
		outN++
	}

	// Lands: greedily drop the least needed one, recounting cover each time.
	for col := range w.need {
		w.need[col] = 0
	}
	for i := spellsOut; i < ns; i++ {
		p := cards[w.spell[i]].Pips
		for col := range w.need {
			w.need[col] += p[col]
		}
	}
	copy(w.left[:], w.land[:nl])
	left := nl
	for drop := 0; drop < landsOut; drop++ {
		for col := range w.cover {
			w.cover[col] = 0
		}
		for i := 0; i < left; i++ {
			c := cards[w.left[i]]
			for col := range w.cover {
				if produces(c, col) {
					w.cover[col]++
				}
			}
		}
		best, bestScore := 0, 0.0
		for i := 0; i < left; i++ {
			c := cards[w.left[i]]
			score := 0.0
			for col := range w.need {
				if w.need[col] > 0 && produces(c, col) {
					score += float64(w.need[col]) / float64(w.cover[col])
				}
			}
			if i == 0 || score < bestScore {
				best, bestScore = i, score
			}
		}
		w.out[outN] = w.left[best]
		outN++
		copy(w.left[best:left-1], w.left[best+1:left])
		left--
	}
	out := w.out[:outN]
	for i := 1; i < len(out); i++ { // ascending, gorge chooseLowest's order
		x := out[i]
		j := i
		for j > 0 && out[j-1] > x {
			out[j] = out[j-1]
			j--
		}
		out[j] = x
	}
	return out
}

// BottomCheapest is the coin and never policies' bottoming answer (gorge
// chooseDiscard): keep cheap plays over expensive ones, without the cast
// rule's blanket creature premium. A basic land or a free noncreature is
// conservatively land-like (the facts' Land arm); everything else is worth
// minus its mana value. The m least valuable cards go, ties on the lower
// index, in ascending index order (gorge chooseLowest's).
func BottomCheapest(cards []Card, m int, w *Work) []int {
	n := len(cards)
	if n == 0 || m <= 0 {
		return nil
	}
	if m > n {
		m = n
	}
	idx := w.idx[:n]
	for i := range idx {
		idx[i] = i
	}
	// Least valuable first, ties on the lower index: insertion sort over
	// the at-most-HandSize indices, no allocation.
	for i := 1; i < n; i++ {
		x := idx[i]
		wx := worth(cards[x])
		j := i
		for j > 0 {
			prev := idx[j-1]
			wp := worth(cards[prev])
			if wp < wx || (wp == wx && prev < x) {
				break
			}
			idx[j] = prev
			j--
		}
		idx[j] = x
	}
	out := w.out[:m]
	copy(out, idx[:m])
	for i := 1; i < len(out); i++ { // ascending, gorge chooseLowest's order
		x := out[i]
		j := i
		for j > 0 && out[j-1] > x {
			out[j] = out[j-1]
			j--
		}
		out[j] = x
	}
	return out
}

// worth is chooseDiscard's worth function.
func worth(c Card) int32 {
	if c.Land {
		return 1
	}
	return -c.CMC
}
