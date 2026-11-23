package main

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
	"github.com/adams-shaun/gorge/engine/cards"
)

// deckSalt is the second PCG word of every deck draw (the shadow seat's
// harness uses the same, so a seed names the same decks there).
const deckSalt = 0x5748

// maxRedeals bounds the decks a set pool redraws when a drawn matchup is
// refused by the set's guards.
const maxRedeals = 40

// errUsage marks a flag the command refuses (exit 2).
var errUsage = errors.New("usage")

// pool is one card pool the bench plays: the corpus registry its games run
// over, the set its table came from (nil for vanilla) and how a game's two
// decks are drawn.
type pool struct {
	reg *cards.Registry
	set cardset.Set
	// deal draws game seed's table and the two decks.
	deal func(seed uint64) (*kop.Table, [2][]kop.CardID, error)
	// redeals counts the matchups deal redrew.
	redeals int
}

// openPool builds the pool named name.
func openPool(name string) (*pool, error) {
	switch {
	case name == "vanilla":
		return vanillaPool()
	case name == "mbc-stage":
		return mbcStagePool()
	case strings.HasPrefix(name, "set:") && len(name) > len("set:"):
		return setPool(strings.TrimPrefix(name, "set:"))
	}
	return nil, fmt.Errorf("%w: unknown pool %q (vanilla, mbc-stage or set:<name>)", errUsage, name)
}

// openRegistry opens the whole corpus registry: a gorge game
// (oracle.Matchup.Config) needs the full name universe, not a subset.
func openRegistry() (*cards.Registry, error) {
	dir, err := cardset.CorpusDir()
	if err != nil {
		return nil, err
	}
	return cards.SharedCorpus(dir)
}

// vanillaPool is the stage pool: 17 basic lands, 20 vanilla or keyword
// creatures and 3 Hungry Ghoul, from dirac/vanilla.
func vanillaPool() (*pool, error) {
	var lands, creatures, names []kop.CardID
	for id := vanilla.Plains; id <= vanilla.Forest; id++ {
		lands = append(lands, id)
	}
	for id := vanilla.SavannahLions; id <= vanilla.TundraWall; id++ {
		creatures = append(creatures, id)
	}
	names = append(append(names, lands...), creatures...)
	names = append(names, vanilla.HungryGhoul)
	var ns []string
	for _, id := range names {
		ns = append(ns, vanilla.Table.Cards[id].Name)
	}
	reg, err := openRegistry()
	if err != nil {
		return nil, err
	}
	deal := func(seed uint64) (*kop.Table, [2][]kop.CardID, error) {
		r := rand.New(rand.NewPCG(seed, deckSalt))
		var decks [2][]kop.CardID
		for s := range decks {
			for i := 0; i < 17; i++ {
				decks[s] = append(decks[s], lands[r.IntN(len(lands))])
			}
			for i := 0; i < 20; i++ {
				decks[s] = append(decks[s], creatures[r.IntN(len(creatures))])
			}
			decks[s] = append(decks[s], vanilla.HungryGhoul, vanilla.HungryGhoul, vanilla.HungryGhoul)
		}
		return vanilla.Table, decks, nil
	}
	return &pool{reg: reg, deal: deal}, nil
}

// mbcStagePool is the corpus pool of dirac/shadow's stage_cards_test.go:
// Pacifism, Swiftfoot Boots, Dragon Fodder, Raise the Alarm, two basic lands
// and creatures with combat keywords.
func mbcStagePool() (*pool, error) {
	set, err := cardset.ByName("mbc")
	if err != nil {
		return nil, err
	}
	lands := []string{"Plains", "Mountain"}
	creatures := []string{
		"Fencing Ace", "Exultant Skymarcher", "Tormented Angel", "Charging Monstrosaur",
		"Wayward Giant", "Nimble Birdsticker", "Starlit Angel", "Shrike Force", "Faithbearer Paladin",
		"Knight of the Tusk",
	}
	spells := []string{"Pacifism", "Swiftfoot Boots", "Dragon Fodder", "Raise the Alarm"}
	var names []string
	names = append(append(append(names, spells...), lands...), creatures...)
	var list []cardset.Entry
	for _, n := range names {
		list = append(list, cardset.Entry{Name: n, Count: 1})
	}
	tbl, _, _, err := set.Matchup(list, list)
	if err != nil {
		return nil, err
	}
	id := map[string]kop.CardID{}
	for i := range tbl.Cards {
		if tbl.Cards[i].Types&kop.TToken == 0 {
			id[tbl.Cards[i].Name] = kop.CardID(i)
		}
	}
	reg, err := openRegistry()
	if err != nil {
		return nil, err
	}
	deal := func(seed uint64) (*kop.Table, [2][]kop.CardID, error) {
		r := rand.New(rand.NewPCG(seed, deckSalt))
		var decks [2][]kop.CardID
		for s := range decks {
			add := func(n int, from ...string) {
				for i := 0; i < n; i++ {
					decks[s] = append(decks[s], id[from[r.IntN(len(from))]])
				}
			}
			add(16, lands...)
			add(3, "Pacifism")
			add(3, "Swiftfoot Boots")
			add(4, "Dragon Fodder")
			add(3, "Raise the Alarm")
			add(14, creatures...)
		}
		return tbl, decks, nil
	}
	return &pool{reg: reg, set: set, deal: deal}, nil
}

// The five colours, in basic-land order, and the realistic deck's shape.
var (
	basicLands = [5]string{"Plains", "Island", "Swamp", "Mountain", "Forest"}
	colourBits = [5]cards.ColourMask{
		cards.ColourMaskWhite, cards.ColourMaskBlue, cards.ColourMaskBlack, cards.ColourMaskRed, cards.ColourMaskGreen,
	}
)

const (
	deckLands   = 17
	deckSpells  = 23
	maxCopies   = 4
	colourPairs = 10
)

// setDecks draws realistic decks from one generated set: per seat two
// colours, deckLands basics of them (9 and 8) and deckSpells cards drawn
// uniformly with replacement, at most maxCopies of a name, from the set's
// supported non-land cards whose front face's colours fall within the pair.
type setDecks struct {
	set   cardset.Set
	names []string
	// eligible[a][b] (a < b) is the pool indexes a deck of colours a and b
	// may draw, ascending.
	eligible [5][5][]int
}

// setPool opens the pool of the set named name.
func setPool(name string) (*pool, error) {
	set, err := cardset.ByName(name)
	if err != nil {
		return nil, err
	}
	reg, err := openRegistry()
	if err != nil {
		return nil, err
	}
	sd := &setDecks{set: set, names: set.Names()}
	for _, b := range basicLands {
		if i, ok := set.Lookup(b); !ok || !set.Supported()[i] {
			return nil, fmt.Errorf("set %s does not play %s", name, b)
		}
	}
	supported := set.Supported()
	for i := 0; i < set.NumCards(); i++ {
		if !supported[i] {
			continue
		}
		c, ok := reg.Lookup(sd.names[i])
		if !ok || len(c.Faces) == 0 {
			continue
		}
		face := c.Faces[0]
		if isLand(face) {
			continue
		}
		col := faceColours(face)
		for a := 0; a < 5; a++ {
			for b := a + 1; b < 5; b++ {
				if col&^(colourBits[a]|colourBits[b]) == 0 {
					sd.eligible[a][b] = append(sd.eligible[a][b], i)
				}
			}
		}
	}
	p := &pool{reg: reg, set: set}
	p.deal = func(seed uint64) (*kop.Table, [2][]kop.CardID, error) { return sd.deal(p, seed) }
	return p, nil
}

// isLand reports whether a face's type line holds Land.
func isLand(f *cards.Face) bool {
	for _, t := range f.Types {
		if t == "Land" {
			return true
		}
	}
	return false
}

// faceColours is a face's colours: its cost's pips, else its colour
// indicator. Colourless is 0.
func faceColours(f *cards.Face) cards.ColourMask {
	pips, _ := cards.ManaCostPips(f.ManaCost)
	pips &^= cards.ColourMaskColourless
	if pips != 0 {
		return pips
	}
	var m cards.ColourMask
	for _, c := range strings.Split(f.Colors, ",") {
		for i, n := range [5]string{"white", "blue", "black", "red", "green"} {
			if strings.EqualFold(strings.TrimSpace(c), n) {
				m |= colourBits[i]
			}
		}
	}
	return m
}

// deal draws game seed's two decks and builds the matchup table, redrawing a
// matchup the set's guards refuse.
func (sd *setDecks) deal(p *pool, seed uint64) (*kop.Table, [2][]kop.CardID, error) {
	r := rand.New(rand.NewPCG(seed, deckSalt))
	var lastErr error
	for try := 0; try < maxRedeals; try++ {
		var es [2][]cardset.Entry
		ok := true
		for s := range es {
			var e []cardset.Entry
			if e, ok = sd.draw(r); !ok {
				break
			}
			es[s] = e
		}
		if !ok {
			lastErr = errors.New("a colour pair has too few cards to fill a deck")
			p.redeals++
			continue
		}
		tbl, a, b, err := sd.set.Matchup(es[0], es[1])
		if err == nil {
			return tbl, [2][]kop.CardID{a, b}, nil
		}
		lastErr = err
		p.redeals++
	}
	return nil, [2][]kop.CardID{}, fmt.Errorf("seed %d: no matchup after %d draws: %w", seed, maxRedeals, lastErr)
}

// draw is one deck: two colours, lands and spells. ok is false when the pair
// holds too few cards to fill deckSpells under maxCopies.
func (sd *setDecks) draw(r *rand.Rand) (deck []cardset.Entry, ok bool) {
	c1 := r.IntN(5)
	c2 := (c1 + 1 + r.IntN(4)) % 5
	lo, hi := min(c1, c2), max(c1, c2)
	pick := sd.eligible[lo][hi]
	if len(pick)*maxCopies < deckSpells {
		return nil, false
	}
	deck = []cardset.Entry{
		{Name: basicLands[c1], Count: 9},
		{Name: basicLands[c2], Count: deckLands - 9},
	}
	idx := map[string]int{} // name -> position in deck; read by key only
	for n := 0; n < deckSpells; {
		name := sd.names[pick[r.IntN(len(pick))]]
		j, seen := idx[name]
		if !seen {
			idx[name] = len(deck)
			deck = append(deck, cardset.Entry{Name: name, Count: 1})
		} else if deck[j].Count < maxCopies {
			deck[j].Count++
		} else {
			continue
		}
		n++
	}
	return deck, true
}
