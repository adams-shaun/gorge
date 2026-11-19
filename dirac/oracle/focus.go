package oracle

import (
	"bufio"
	"bytes"
	"fmt"
	"math/bits"
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
)

// A focus pool is a synthetic pool dealt around a list of focus cards: the cards a
// plain deal reaches too rarely, or with too few of their abilities used, to earn
// evidence. Every deck holds FocusPerDeck of them, two copies each, with the enablers
// the list names beside them; the rest of the deck is dealt as SynthPool deals it.
// It is still a pure function of the set's list, the focus list, the corpus and the
// seed, and a card is still chosen by the list, never by how it plays, so its games
// are evidence on the same terms as a plain pool's.

// FocusPerDeck is how many focus cards a focus deck holds by default.
const FocusPerDeck = 4

// FocusCard is one line of a focus list: a card, then the cards of the same list
// that its abilities need beside it (a Room payoff wants Rooms, a disguise payoff
// face-down creatures).
type FocusCard struct {
	Name string
	With []string
}

// ParseFocus reads a focus list: one card a line, "#" comments and blank lines
// skipped, enablers after a tab separated by ";".
func ParseFocus(b []byte) ([]FocusCard, error) {
	var out []FocusCard
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, with, _ := strings.Cut(line, "\t")
		fc := FocusCard{Name: strings.TrimSpace(name)}
		for _, w := range strings.Split(with, ";") {
			if w = strings.TrimSpace(w); w != "" {
				fc.With = append(fc.With, w)
			}
		}
		out = append(out, fc)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("focus list is empty")
	}
	return out, nil
}

// focusInfo is what FocusPoolWith keeps of a playable card: its pip colours and its kind.
type focusInfo struct {
	mask uint8
	kind synthKind
}

// focusPlayable lists the cards of set a focus deck can hold: every supported, lowered
// nonbasic spell or land, with its colours, and the spell and land names apart.
func focusPlayable(reg *cards.Registry, set cardset.Set, pool string) (map[string]focusInfo, []string, []string, error) {
	plays := set.Supported()
	playable := map[string]focusInfo{}
	var spells, lands []string
	for i, name := range set.Names()[:set.NumCards()] {
		if set.Status(i) == cardset.StatusNotLowered || !plays[i] || cardset.IsSynthBasic(name) {
			continue
		}
		c, ok := reg.Lookup(name)
		if !ok {
			return nil, nil, nil, fmt.Errorf("focus %s: card %q is not in the corpus", pool, name)
		}
		k := kindOf(c)
		switch k {
		case synthSpell:
			spells = append(spells, name)
		case synthLand:
			lands = append(lands, name)
		default:
			continue
		}
		playable[name] = focusInfo{mask: pipMask(c), kind: k}
	}
	return playable, spells, lands, nil
}

// focusWithoutBasics is focus with its basic-land lines left out, as a new slice.
func focusWithoutBasics(focus []FocusCard) []FocusCard {
	return slices.DeleteFunc(slices.Clone(focus), func(fc FocusCard) bool { return cardset.IsSynthBasic(fc.Name) })
}

// FocusPool is FocusPoolWith over a set with no cards from other pools.
func FocusPool(reg *cards.Registry, set cardset.Set, pool string, focus []FocusCard, perDeck int, seed uint64, nDecks int) ([]PoolDeck, map[string]deck.File, error) {
	return FocusPoolWith(reg, set, nil, pool, focus, perDeck, seed, nDecks)
}

// FocusPoolWith deals nDecks 40-card decks named after pool from set's playable cards
// around focus. Deck d starts from focus card d mod len(focus): its colours are that
// card's (topped up to two from PCG(seed, d) when it has fewer), then it takes that
// card and up to perDeck-1 more focus cards fitting those colours, in list order from
// there, two copies each, and every enabler they name that fits. The other spell
// slots, the nonbasic lands and the basics are SynthPool's. A focus card or enabler
// the set does not play is an error, so a stale list fails loudly. Only a basic land a
// list names is skipped, not an error: every deck holds basics whatever the list says,
// and the evidence lists the loop writes always name them beside the cards they credit.
//
// extra names the cards of set that came from other pools (kfuzz -synth-with): they
// resolve as enablers only. A focus card is never one, and the filler never deals one,
// so a deck holds an extra card only when a focus line names it. An enabler a line
// names k times is dealt k copies when it is a land, and those land copies go past
// synthLands (the filler keeps its slots); a spell keeps synthMaxCopies whatever the
// repeats.
func FocusPoolWith(reg *cards.Registry, set cardset.Set, extra map[string]bool, pool string, focus []FocusCard, perDeck int, seed uint64, nDecks int) ([]PoolDeck, map[string]deck.File, error) {
	if perDeck < 1 {
		perDeck = 1
	}
	if focus = focusWithoutBasics(focus); len(focus) == 0 {
		return nil, nil, fmt.Errorf("focus %s: no card but basic lands", pool)
	}
	playable, spells, lands, err := focusPlayable(reg, set, pool)
	if err != nil {
		return nil, nil, err
	}
	// the filler lists are the main pool's: drop the cards another pool lent
	spells = slices.DeleteFunc(spells, func(n string) bool { return extra[n] })
	lands = slices.DeleteFunc(lands, func(n string) bool { return extra[n] })
	// A list names a card as the set lists it; an evidence row names its first face.
	byFace := map[string]string{}
	for n := range playable {
		f, _, _ := strings.Cut(n, " // ")
		if old, ok := byFace[f]; ok && !extra[old] && extra[n] {
			continue // a main pool's card wins its face
		}
		byFace[f] = n
	}
	resolve := func(n string, enabler bool) (string, bool) {
		if _, ok := playable[n]; ok && (enabler || !extra[n]) {
			return n, true
		}
		f, _, _ := strings.Cut(n, " // ")
		full, ok := byFace[f]
		return full, ok && (enabler || !extra[full])
	}
	for i := range focus {
		n, ok := resolve(focus[i].Name, false)
		if !ok {
			return nil, nil, fmt.Errorf("focus %s: %q is not a card %s plays", pool, focus[i].Name, set.Name())
		}
		focus[i].Name = n
		for j, w := range focus[i].With {
			if focus[i].With[j], ok = resolve(w, true); !ok {
				return nil, nil, fmt.Errorf("focus %s: enabler %q of %q is not a card %s plays", pool, w, n, set.Name())
			}
		}
	}
	for _, b := range cardset.SynthBasics {
		if _, ok := set.Lookup(b); !ok {
			return nil, nil, fmt.Errorf("focus %s: basic %q is not in the pool", pool, b)
		}
	}
	slices.Sort(spells)
	slices.Sort(lands)
	queueRand := rand.New(rand.NewPCG(seed, synthQueueStream))
	var queue []string
	out := make([]PoolDeck, 0, nDecks)
	files := make(map[string]deck.File, nDecks)
	for d := 0; d < nDecks; d++ {
		r := rand.New(rand.NewPCG(seed, uint64(d)))
		head := focus[d%len(focus)]
		allowed := playable[head.Name].mask
		for bits.OnesCount8(allowed) < 2 {
			allowed |= 1 << r.IntN(len(synthColours))
		}
		fits := func(n string) bool { return playable[n].mask&^allowed == 0 }
		have := map[string]int{}
		nSpells, nLands := 0, 0
		// add deals n. A land enabler a line names want times is topped up to want
		// copies, past synthLands but never into the spell slots; a focus card that
		// is a land keeps one nonbasic slot.
		add := func(n string, want int, enabler bool) {
			if !fits(n) {
				return
			}
			if playable[n].kind == synthLand {
				if !enabler {
					if have[n] == 0 && nLands < synthLands {
						have[n] = 1
						nLands++
					}
					return
				}
				for have[n] < want && nLands+synthSpells < synthDeckSize {
					have[n]++
					nLands++
				}
				return
			}
			if have[n] > 0 {
				return
			}
			if nSpells+synthMaxCopies > synthSpells {
				return
			}
			have[n] = synthMaxCopies
			nSpells += synthMaxCopies
		}
		taken := 0
		for k := 0; k < len(focus) && taken < perDeck; k++ {
			fc := focus[(d+k)%len(focus)]
			if have[fc.Name] > 0 || !fits(fc.Name) {
				continue
			}
			add(fc.Name, 1, false)
			taken++
			for _, w := range fc.With {
				add(w, countOf(fc.With, w), true)
			}
		}
		for nSpells < synthSpells {
			if len(queue) == 0 {
				for _, i := range queueRand.Perm(len(spells)) {
					queue = append(queue, spells[i])
				}
			}
			q := slices.IndexFunc(queue, func(n string) bool { return fits(n) && have[n] < synthMaxCopies })
			if q < 0 {
				// nothing left in this pass fits: start a fresh pass, or give up
				// when no spell of the list fits at all
				if !slices.ContainsFunc(spells, func(n string) bool { return fits(n) && have[n] < synthMaxCopies }) {
					break
				}
				queue = queue[:0]
				continue
			}
			have[queue[q]]++
			nSpells++
			queue = slices.Delete(queue, q, q+1)
		}
		for _, i := range r.Perm(len(lands)) {
			if nLands >= synthLands {
				break
			}
			if have[lands[i]] == 0 {
				have[lands[i]] = 1
				nLands++
			}
		}
		entries := make([]deck.Entry, 0, len(have)+len(synthColours))
		for n, c := range have {
			entries = append(entries, deck.Entry{Name: n, Count: c})
		}
		entries = appendBasics(entries, allowed, synthDeckSize-nSpells-nLands)
		slices.SortFunc(entries, func(x, y deck.Entry) int { return strings.Compare(x.Name, y.Name) })
		colours := maskString(allowed)
		name := fmt.Sprintf("%s_%05d_%s", pool, d, colours)
		files[name] = deck.File{Name: name, Format: "limited", Archetype: "synthetic", Cards: entries}
		out = append(out, PoolDeck{Name: name, Split: "train", Colors: colours})
	}
	return out, files, nil
}

// countOf is how many times name is in names.
func countOf(names []string, name string) int {
	n := 0
	for _, x := range names {
		if x == name {
			n++
		}
	}
	return n
}
