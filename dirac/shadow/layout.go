package shadow

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/view"
)

// Layout is the open-list deal a seat stages its own view with: the game's
// deck lists, one card name per copy, one list per seat.
//
// The lists are open, every seat's list is known (the open-list assumption a
// determinization deal makes). A closed list, where an opponent's deck is
// unknown, belongs to a belief search instead; a Layout cannot represent one.
//
// A Layout also owns the object numbering staging needs and a sticky token
// registry. Both are stateful across Deal calls: a token keeps the index of the
// first view it appeared in for the rest of the game, and once no view shows it
// it is dealt as a ceased token (Hidden.Ceased) at that index.
type Layout struct {
	tbl   *kop.Table
	lists [][]string
	bases []int // bases[s] is seat s's first object index
	total int   // the number of deck cards in all lists

	// tokenRank is a view ID's first-seen rank, an index into tokens. It is a
	// lookup only; the iteration order is tokens, which appends a token the
	// first time a view shows it, in player then battlefield order.
	tokenRank map[int]int
	tokens    []layoutToken
}

// layoutToken is a token the Layout has seen: its view ID and the table token
// name and owner it had when last shown.
type layoutToken struct {
	id    int
	name  string // the table token's script name; "" when the table has no unique match
	owner uint8
}

// NewLayout validates tbl and the open deck lists and returns their Layout.
// Every seat's list must hold 7..dirac.MaxDeck cards and name a non-token
// card of tbl; two lists are the minimum and dirac.MaxSeats the maximum, and
// dirac.MaxObj bounds the cards in all. A refusal is a plain error, not an
// ErrUnstageable: it is a malformed deck list, not a position the shadow
// cannot stage.
func NewLayout(tbl *kop.Table, lists [][]string) (*Layout, error) {
	if tbl == nil {
		return nil, fmt.Errorf("shadow: NewLayout: no table")
	}
	if len(lists) < 2 || len(lists) > dirac.MaxSeats {
		return nil, fmt.Errorf("shadow: NewLayout: %d lists, want 2..%d", len(lists), dirac.MaxSeats)
	}
	l := &Layout{
		tbl:       tbl,
		lists:     make([][]string, len(lists)),
		bases:     make([]int, len(lists)),
		tokenRank: map[int]int{},
	}
	for s := range lists {
		if len(lists[s]) < 7 || len(lists[s]) > dirac.MaxDeck {
			return nil, fmt.Errorf("shadow: NewLayout: seat %d has %d cards, want 7..%d", s, len(lists[s]), dirac.MaxDeck)
		}
		l.bases[s] = l.total
		l.total += len(lists[s])
		l.lists[s] = append([]string(nil), lists[s]...)
		for _, name := range lists[s] {
			if !l.hasCard(name) {
				return nil, fmt.Errorf("shadow: NewLayout: seat %d names %q, which the table does not hold", s, name)
			}
		}
	}
	if l.total > dirac.MaxObj {
		return nil, fmt.Errorf("shadow: NewLayout: %d cards, want at most %d", l.total, dirac.MaxObj)
	}
	return l, nil
}

// hasCard reports whether tbl holds a non-token card named name.
func (l *Layout) hasCard(name string) bool {
	for i := range l.tbl.Cards {
		if l.tbl.Cards[i].Name == name && l.tbl.Cards[i].Types&kop.TToken == 0 {
			return true
		}
	}
	return false
}

// Numbering is the assignment Deal made: every physical card the view shows,
// and every token it shows, to a dirac object index.
//
// ObjOf maps a view's CardView.ID to its index. ViewID is the inverse for the
// view-shown cards; a hidden card the caller deals has an index but no view ID,
// so its ViewID lookup is false. ObjOf's map is a lookup only, and viewID is a
// dense slice by object index with -1 for an index the view did not show.
type Numbering struct {
	objOf  map[int]dirac.Obj
	viewID []int
}

// ObjOf returns the object index the view-shown card id was assigned.
func (n *Numbering) ObjOf(id int) (dirac.Obj, bool) {
	if n == nil {
		return dirac.NoObj, false
	}
	o, ok := n.objOf[id]
	return o, ok
}

// ViewID returns the view-shown card at object index o.
func (n *Numbering) ViewID(o dirac.Obj) (int, bool) {
	if n == nil || int(o) < 0 {
		return 0, false
	}
	i := int(o)
	if i >= len(n.viewID) || n.viewID[i] < 0 {
		return 0, false
	}
	return n.viewID[i], true
}

func (n *Numbering) add(id int, o dirac.Obj) {
	n.objOf[id] = o
	for len(n.viewID) <= int(o) {
		n.viewID = append(n.viewID, -1)
	}
	n.viewID[int(o)] = id
}

// Deal assigns an Input for the seat v.Viewer sees. The deck ranges are fixed
// by the lists: seat s's range is base_s..base_s+len(lists[s])-1, the sum of
// the earlier lists. Within that range the physical cards the view shows take
// the next indices in this order: every player's battlefield (player order,
// list order) for the cards the seat owns, its graveyard, exile and command
// zone, the stack's spells it owns (stack order), and the viewer's own hand.
// The unseen cards fill the rest of the range in list order.
//
// The order of the hidden cards is deliberately arbitrary: a search re-deals
// every zone the seat cannot see at each simulation (dirac/search's redeal), so
// the initial order carries no information and no guess. The counts are not
// arbitrary: the unseen multiset must be exactly the hidden count the view
// shows (HandSize+LibrarySize for another seat, LibrarySize for the viewer), or
// Deal returns ReasonHidden. An unseen card the list no longer holds is a
// ReasonHidden too ("seat %d shows %q beyond its list").
//
// Tokens take the indices after every deck, in the order this Layout first saw
// each one, and keep them: a token that left the view becomes a CeasedToken at
// its index, with the name and owner it had when last shown. (A token made and
// gone between two views is never seen; the gap it leaves is refused as a
// token when a later one follows it.)
//
// Deal reads only the view and the lists. Its only writes are the token
// registry's first-seen ranks and last-seen names and owners.
func (l *Layout) Deal(v *view.View, cfg dirac.Config, seed uint64) (*Input, *Numbering, error) {
	if l == nil || l.tbl == nil {
		return nil, nil, bad(ReasonHidden, "no layout")
	}
	if v == nil {
		return nil, nil, bad(ReasonHidden, "no view")
	}
	viewer := int(v.Viewer)
	if viewer < 0 || viewer >= len(l.lists) {
		return nil, nil, bad(ReasonHidden, "viewer seat %d is outside %d lists", viewer, len(l.lists))
	}
	players := make([]*view.PlayerView, len(l.lists))
	for s := range l.lists {
		p := playerView(v, s)
		if p == nil {
			return nil, nil, bad(ReasonHidden, "seat %d is not in the view", s)
		}
		players[s] = p
	}

	// shown collects each seat's physical cards in the deal order. A token is
	// not a physical card: it takes an index after every deck, in the token
	// registry below.
	shown := make([][]dealtCard, len(l.lists))
	addShown := func(cv *view.CardView) {
		if cv.IsToken {
			return
		}
		s := int(cv.Owner)
		if s < 0 || s >= len(l.lists) {
			return
		}
		shown[s] = append(shown[s], dealtCard{id: int(cv.ID), name: cv.Name})
	}
	for pi := range v.Players {
		p := &v.Players[pi]
		for ci := range p.Battlefield {
			addShown(&p.Battlefield[ci])
		}
	}
	for s := range l.lists {
		p := players[s]
		for _, zone := range [][]view.CardView{p.Graveyard, p.Exile, p.Command} {
			for ci := range zone {
				cv := &zone[ci]
				if int(cv.Owner) != s {
					continue
				}
				addShown(cv)
			}
		}
	}
	for i := range v.Stack {
		sv := &v.Stack[i]
		if sv.Kind != "spell" || sv.Card == nil || sv.Card.IsCopy || sv.Card.IsToken {
			continue
		}
		addShown(sv.Card)
	}
	for ci := range players[viewer].Hand {
		cv := &players[viewer].Hand[ci]
		if int(cv.Owner) != viewer {
			continue
		}
		addShown(cv)
	}

	nums := &Numbering{objOf: map[int]dirac.Obj{}}
	hidden := Hidden{}
	next := make([]int, len(l.lists))
	copy(next, l.bases)
	for s := range l.lists {
		counts := map[string]int{}
		for _, name := range l.lists[s] {
			counts[name]++
		}
		for _, c := range shown[s] {
			if counts[c.name] == 0 {
				return nil, nil, bad(ReasonHidden, "seat %d shows %q beyond its list", s, c.name)
			}
			counts[c.name]--
			nums.add(c.id, dirac.Obj(next[s]))
			next[s]++
		}
		// The unseen cards are the remaining multiset in list order: the
		// first occurrence of each name is the one still unseen.
		unseen := make([]string, 0, len(l.lists[s]))
		for _, name := range l.lists[s] {
			if counts[name] != 0 {
				counts[name]--
				unseen = append(unseen, name)
			}
		}
		hand, lib := int(players[s].HandSize), int(players[s].LibrarySize)
		if hand < 0 || lib < 0 {
			return nil, nil, bad(ReasonHidden, "seat %d shows a negative hidden count (hand %d + library %d)", s, hand, lib)
		}
		want := lib
		if s != viewer {
			want = hand + lib
		}
		if len(unseen) != want {
			return nil, nil, bad(ReasonHidden, "seat %d has %d unseen cards, view shows %d (hand %d + library %d)", s, len(unseen), want, hand, lib)
		}
		cards := make([]Card, len(unseen))
		for i, name := range unseen {
			cards[i] = Card{Obj: dirac.Obj(next[s]), Name: name}
			next[s]++
		}
		if s == viewer {
			hidden.Library[s] = cards
			continue
		}
		hidden.Hand[s] = cards[:hand]
		hidden.Library[s] = cards[hand:]
	}

	// Tokens come after every deck, each at its first-seen rank for the rest of
	// the game: the ones the view shows are numbered, the rest are ceased.
	shownTok := map[int]bool{}
	for pi := range v.Players {
		p := &v.Players[pi]
		for ci := range p.Battlefield {
			cv := &p.Battlefield[ci]
			if !cv.IsToken {
				continue
			}
			id := int(cv.ID)
			r, ok := l.tokenRank[id]
			if !ok {
				r = len(l.tokens)
				l.tokenRank[id] = r
				l.tokens = append(l.tokens, layoutToken{id: id})
			}
			t := &l.tokens[r]
			t.owner = uint8(cv.Owner)
			t.name = ""
			if cid, ok := tableToken(l.tbl, cv); ok {
				t.name = l.tbl.Cards[cid].Name
			}
			shownTok[id] = true
		}
	}
	for r := range l.tokens {
		t := &l.tokens[r]
		o := dirac.Obj(l.total + r)
		if shownTok[t.id] {
			nums.add(t.id, o)
		} else {
			hidden.Ceased = append(hidden.Ceased, CeasedToken{Obj: o, Name: t.name, Owner: t.owner})
		}
	}

	in := &Input{
		View: *v, Hidden: hidden, ObjOf: nums.ObjOf,
		Tbl: l.tbl, Cfg: cfg, Seed: seed,
	}
	return in, nums, nil
}

// dealtCard is one physical card the view shows: its CardView.ID and name.
type dealtCard struct {
	id   int
	name string
}

// playerView is the PlayerView of seat s, nil when the view does not carry it.
func playerView(v *view.View, s int) *view.PlayerView {
	for i := range v.Players {
		if int(v.Players[i].ID) == s {
			return &v.Players[i]
		}
	}
	return nil
}
