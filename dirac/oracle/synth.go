package oracle

import (
	"fmt"
	"math/bits"
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
)

const (
	synthDeckSize  = 40
	synthSpells    = 23 // spell slots, filled from the queue
	synthMaxCopies = 2  // of one spell in a deck
	synthLands     = 4  // nonbasic land slots
	// synthQueueStream is the PCG stream of the spell queue's shuffles. A deck's own
	// draws use stream = deck index, so this one is out of their reach.
	synthQueueStream = ^uint64(0)
)

// synthColours is WUBRG, the order of the masks and of cardset.SynthBasics.
const synthColours = "WUBRG"

// PlaySet is the set dplay plays: a deck pool by name, else (a name with the prefix
// "std-", "dr-" or "tail-": cardset.IsSynthName) the synthetic pool of that name with unverified cards playable
// and refused cards still refused, as kfuzz -synth plays it.
func PlaySet(name string) (cardset.Set, error) {
	if cardset.IsSynthName(name) {
		set, err := cardset.SynthByName(name)
		if err != nil {
			return nil, err
		}
		return cardset.AllowUnverified(set), nil
	}
	return cardset.ByName(name)
}

// colourMask is the colours whose pips cost names. Hybrid and Phyrexian pips count
// all their colours.
func colourMask(cost string) uint8 {
	var m uint8
	for i := range synthColours {
		if strings.IndexByte(cost, synthColours[i]) >= 0 {
			m |= 1 << i
		}
	}
	return m
}

// pipMask is the colours a card's faces need. It is every face, because a Room's
// second door or a split card's second half is cast too; a transform face's back
// has no pips.
func pipMask(c *cards.Card) uint8 {
	var m uint8
	for _, f := range c.Faces {
		if f != nil {
			m |= colourMask(f.ManaCost)
		}
	}
	return m
}

// synthKind sorts a card for dealing. A card with no main-deck card type (a scheme, a
// plane, a vanguard) cannot be dealt, and the reach report counts it as not dealt.
type synthKind uint8

const (
	synthUndealable synthKind = iota
	synthSpell
	synthLand
)

func kindOf(c *cards.Card) synthKind {
	if len(c.Faces) == 0 || c.Faces[0] == nil {
		return synthUndealable
	}
	kind := synthUndealable
	for _, t := range c.Faces[0].Types {
		switch t {
		case "Land":
			return synthLand
		case "Creature", "Artifact", "Enchantment", "Instant", "Sorcery", "Planeswalker", "Battle":
			kind = synthSpell
		}
	}
	return kind
}

// SynthPool deals the cards set plays into nDecks 40-card decks, a pure function of
// set's list, which entries it plays, the corpus and seed. Rule: a deck takes two colours from PCG(seed, deck);
// 23 spells from a shuffled queue of the lowered nonlands whose pips are within its
// colours, at most 2 copies, the oldest in the queue first; up to 4 of the set's
// nonbasic lands, shuffled by the same generator; basics alternating over its colours
// to 40.
//
// A card of three or more colours fits no pair, so when one reaches the head of the
// queue the deck takes that card's colours instead. Without this a three-colour card
// is never dealt, and a set of them (TDM's clans) has a blind spot nothing reports.
//
// A deal is from the cards the set plays and nothing else, so no deck is turned away
// for holding a card the run would refuse, and a pool plays its whole 1500 pairs. The
// run that evidence is judged by plays everything lowered but the cards refused by name
// (unverified ones included, they are what needs the games); the run that settles the
// refused list, and a test pinning a pair, plays the refused cards too and so deals every
// lowered one. A refused entry's repro names a pair of that second deal, which does not
// move when the list does. Which cards a deck holds is chosen by the queue, never by how
// a card plays: a card that is played is as likely as any other. Cards are classified
// from the compiled corpus, never by re-parsing scripts. The returned map is keyed by deck name and is for lookup:
// the order is the slice's.
func SynthPool(reg *cards.Registry, set cardset.Set, seed uint64, nDecks int) ([]PoolDeck, map[string]deck.File, error) {
	var spells, lands []string
	var masks []uint8 // by index in spells
	plays := set.Supported()
	for i, name := range set.Names()[:set.NumCards()] {
		if set.Status(i) == cardset.StatusNotLowered || !plays[i] || cardset.IsSynthBasic(name) {
			continue
		}
		c, ok := reg.Lookup(name)
		if !ok {
			return nil, nil, fmt.Errorf("synth %s: card %q is not in the corpus", set.Name(), name)
		}
		switch kindOf(c) {
		case synthSpell:
			spells = append(spells, name)
			masks = append(masks, pipMask(c))
		case synthLand:
			lands = append(lands, name)
		}
	}
	for _, b := range cardset.SynthBasics {
		if _, ok := set.Lookup(b); !ok {
			return nil, nil, fmt.Errorf("synth %s: basic %q is not in the pool", set.Name(), b)
		}
	}
	queueRand := rand.New(rand.NewPCG(seed, synthQueueStream))
	var queue []int // indexes into spells, oldest first
	refill := func() {
		for _, i := range queueRand.Perm(len(spells)) {
			queue = append(queue, i)
		}
	}
	take := func(allowed uint8, have []uint8) int {
		for q, i := range queue {
			if masks[i]&^allowed == 0 && have[i] < synthMaxCopies {
				queue = slices.Delete(queue, q, q+1)
				return i
			}
		}
		return -1
	}
	pool := make([]PoolDeck, 0, nDecks)
	files := make(map[string]deck.File, nDecks)
	have := make([]uint8, len(spells))
	for d := 0; d < nDecks; d++ {
		r := rand.New(rand.NewPCG(seed, uint64(d)))
		a := r.IntN(len(synthColours))
		b := r.IntN(len(synthColours) - 1)
		if b >= a {
			b++
		}
		allowed := uint8(1)<<a | uint8(1)<<b
		if len(queue) == 0 {
			refill()
		}
		if len(queue) > 0 && bits.OnesCount8(masks[queue[0]]) > 2 {
			allowed = masks[queue[0]]
		}
		clear(have)
		entries := make([]deck.Entry, 0, synthSpells+synthLands+len(synthColours))
		count := 0
		for count < synthSpells {
			i := take(allowed, have)
			if i < 0 {
				refill()
				if i = take(allowed, have); i < 0 {
					break
				}
			}
			have[i]++
			count++
		}
		for i, n := range have {
			if n > 0 {
				entries = append(entries, deck.Entry{Name: spells[i], Count: int(n)})
			}
		}
		for _, i := range r.Perm(len(lands))[:min(synthLands, len(lands))] {
			entries = append(entries, deck.Entry{Name: lands[i], Count: 1})
			count++
		}
		entries = appendBasics(entries, allowed, synthDeckSize-count)
		slices.SortFunc(entries, func(x, y deck.Entry) int { return strings.Compare(x.Name, y.Name) })
		colours := maskString(allowed)
		name := fmt.Sprintf("%s_%05d_%s", set.Name(), d, colours)
		files[name] = deck.File{Name: name, Format: "limited", Archetype: "synthetic", Cards: entries}
		pool = append(pool, PoolDeck{Name: name, Split: "train", Colors: colours})
	}
	return pool, files, nil
}

// appendBasics adds n basics, alternating over allowed's colours in WUBRG order.
func appendBasics(entries []deck.Entry, allowed uint8, n int) []deck.Entry {
	var colours []int
	for i := range synthColours {
		if allowed>>i&1 != 0 {
			colours = append(colours, i)
		}
	}
	counts := make([]int, len(colours))
	for i := 0; i < n; i++ {
		counts[i%len(colours)]++
	}
	for i, c := range colours {
		if counts[i] > 0 {
			entries = append(entries, deck.Entry{Name: cardset.SynthBasics[c], Count: counts[i]})
		}
	}
	return entries
}

func maskString(m uint8) string {
	var b strings.Builder
	for i := range synthColours {
		if m>>i&1 != 0 {
			b.WriteByte(synthColours[i])
		}
	}
	return b.String()
}

// Where a card of a synthetic pool's list got to, furthest first. Evidence is a claim
// about games actually played, so a pool that does not reach a card says so here.
const (
	ReachEvidence   = "evidence"    // met the floor: EvidenceMinPlays and EvidenceAllKeys, in clean games
	ReachKeys       = "keys-unused" // enough plays in clean games, an ability never used
	ReachFewPlays   = "few-plays"   // in a game, under EvidenceMinPlays plays in clean games
	ReachNoGame     = "no-game"     // in a deck, in no game that started
	ReachNotDealt   = "not-dealt"   // lowered, in no deck
	ReachNotLowered = "not-lowered" // kgen refuses it
	ReachRefused    = "refused"     // refused by name, so its pairs are turned away
)

// ReachCard is one card of a list.
type ReachCard struct {
	Name   string
	Status string // a Reach constant
	Decks  int    // decks of the pool holding it
	Games  int    // games that started with it in a deck (counting a deck twice in a mirror)
	Plays  int    // casts and land plays in clean games
}

// Reach is how much of a synthetic pool's list a run reached. Basics are not counted:
// every pool holds them whatever its list says.
type Reach struct {
	Pool                                              string
	Games                                             int // games that started
	List, Lowered, Dealt, Appeared, MinPlays, Evident int
	Cards                                             []ReachCard // sorted by name
}

// SynthReach reports a run. started is the deck-name pairs of the games that started,
// whatever their outcome, and cov the coverage of the clean ones (MergeClean; nil
// when there were none).
func SynthReach(set cardset.Set, pool []PoolDeck, files map[string]deck.File, started [][2]string, cov *Coverage) *Reach {
	r := &Reach{Pool: set.Name(), Games: len(started)}
	names := set.Names()[:set.NumCards()]
	index := make(map[string]int, len(names))
	for i, n := range names {
		if !cardset.IsSynthBasic(n) {
			index[n] = len(r.Cards)
			st := ReachNotDealt
			if set.Status(i) == cardset.StatusNotLowered {
				st = ReachNotLowered
			} else {
				r.Lowered++
				if set.Status(i) == cardset.StatusRefused {
					st = ReachRefused
				}
			}
			r.Cards = append(r.Cards, ReachCard{Name: n, Status: st})
		}
	}
	r.List = len(r.Cards)
	inDeck := func(name string, f func(*ReachCard)) {
		for _, e := range files[name].Cards {
			if i, ok := index[e.Name]; ok {
				f(&r.Cards[i])
			}
		}
	}
	for _, d := range pool {
		inDeck(d.Name, func(c *ReachCard) { c.Decks++ })
	}
	for _, p := range started {
		inDeck(p[0], func(c *ReachCard) { c.Games++ })
		inDeck(p[1], func(c *ReachCard) { c.Games++ })
	}
	var evidence []string
	if cov != nil {
		evidence = cov.Evidence()
	}
	for i := range r.Cards {
		c := &r.Cards[i]
		if c.Status == ReachNotLowered || c.Status == ReachRefused {
			continue
		}
		if c.Decks > 0 {
			r.Dealt++
			c.Status = ReachNoGame
		}
		if c.Games == 0 {
			continue
		}
		r.Appeared++
		c.Status = ReachFewPlays
		if cov == nil || cov.Cards[c.Name] == nil {
			continue
		}
		cc := cov.Cards[c.Name]
		c.Plays = cc.Cast + cc.Land
		if c.Plays < EvidenceMinPlays {
			continue
		}
		r.MinPlays++
		c.Status = ReachKeys
		if _, ok := slices.BinarySearch(evidence, c.Name); ok {
			r.Evident++
			c.Status = ReachEvidence
		}
	}
	return r
}

// Line is the one-line report, in the words the evidence floor uses.
func (r *Reach) Line() string {
	pct := 0.0
	if r.List > 0 {
		pct = 100 * float64(r.Evident) / float64(r.List)
	}
	return fmt.Sprintf("reach %s: %d cards in the list, %d lowered, %d dealt, %d appeared in a game, %d met %d plays in clean games, %d evidence (%.1f%% of the list, over %d games)",
		r.Pool, r.List, r.Lowered, r.Dealt, r.Appeared, r.MinPlays, EvidenceMinPlays, r.Evident, pct, r.Games)
}

// TSV is one row per card, in name order: name, status, decks, games, plays.
func (r *Reach) TSV() []byte {
	var b strings.Builder
	b.WriteString("card\tstatus\tdecks\tgames\tplays\n")
	for _, c := range r.Cards {
		fmt.Fprintf(&b, "%s\t%s\t%d\t%d\t%d\n", c.Name, c.Status, c.Decks, c.Games, c.Plays)
	}
	return []byte(b.String())
}
