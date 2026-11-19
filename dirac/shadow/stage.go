package shadow

import (
	"fmt"
	"sort"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/view"
)

// Reasons WorldFromView adds to dirac's (dirac.UnstageableReasons): what only
// an observation shows. Every ErrUnstageable this package returns carries one
// of these or one of dirac's.
const (
	ReasonOver     = "game-over"       // the game has ended
	ReasonSeats    = "seats"           // not a two-seat view
	ReasonStack    = "stack"           // the stack or the trigger queue is not empty
	ReasonDecision = "decision"        // the view's decision is not a priority decision
	ReasonUnknown  = "unknown-card"    // a card name the table does not hold
	ReasonUnmapped = "unmapped-object" // an object the caller's index cannot place
	ReasonHidden   = "hidden"          // the dealt hidden zones disagree with the counts the view shows
	ReasonExile    = "exile"           // exile or the command zone holds a card
	ReasonToken    = "token"           // a token or a copy
	ReasonMark     = "mark"            // counters, damage, an attachment, a combat mark, face-down, a flag
	ReasonPool     = "pool"            // floating mana
	ReasonStepName = "step-name"       // a step name this package does not know
)

// Reasons lists every reason WorldFromView returns that dirac does not.
var Reasons = []string{
	ReasonOver, ReasonSeats, ReasonStack, ReasonDecision, ReasonUnknown, ReasonUnmapped,
	ReasonHidden, ReasonExile, ReasonToken, ReasonMark, ReasonPool, ReasonStepName,
}

var stepOf = map[string]kop.Step{
	"upkeep": kop.StepUpkeep, "draw": kop.StepDraw, "main1": kop.StepMain1,
	"main2": kop.StepMain2, "end": kop.StepEnd,
}

// Card is one hidden card the caller dealt: its object index and its name.
type Card struct {
	Obj  uint8
	Name string
}

// Hidden is the caller's assignment of the zones the view hides. Hand[p] is
// seat p's hand in order (the viewer's own hand is read from the view
// instead, and Hand[viewer] is ignored); Library[p] is seat p's library from
// the top. Sizes must equal the counts the view shows.
type Hidden struct {
	Hand    [2][]Card
	Library [2][]Card
}

// Input is everything WorldFromView reads.
type Input struct {
	View   view.View
	Hidden Hidden
	// ObjOf maps a card the view shows (its CardView.ID) to its object
	// index in the staged game. The caller owns the numbering: seat 0's
	// cards first, then seat 1's, as dirac.World's Objs.
	ObjOf func(id int) (uint8, bool)
	Tbl   *kop.Table
	Cfg   dirac.Config
	Seed  uint64
}

func bad(reason, format string, a ...any) error {
	return dirac.ErrUnstageable{Reason: reason, Detail: fmt.Sprintf(format, a...)}
}

// StageFromView is dirac.NewFromWorld over WorldFromView.
func StageFromView(in *Input) (*dirac.State, error) {
	w, err := WorldFromView(in)
	if err != nil {
		return nil, err
	}
	return dirac.NewFromWorld(&w)
}

// WorldFromView is the position in.View shows, its hidden zones as
// in.Hidden deals them. It is a dirac.ErrUnstageable when the position is
// outside the minimal staging scope.
func WorldFromView(in *Input) (dirac.World, error) {
	var w dirac.World
	step, err := checkScope(in)
	if err != nil {
		return w, err
	}
	b := &builder{in: in, byName: map[string]kop.CardID{}}
	for i := range in.Tbl.Cards {
		if _, dup := b.byName[in.Tbl.Cards[i].Name]; !dup {
			b.byName[in.Tbl.Cards[i].Name] = kop.CardID(i)
		}
	}
	v := &in.View
	for _, p := range v.Players {
		if err := b.seat(&w, &p, int(p.ID) == int(v.Viewer)); err != nil {
			return w, err
		}
	}
	if err := b.assemble(&w); err != nil {
		return w, err
	}
	w.Tbl, w.Cfg, w.Seed = in.Tbl, in.Cfg, in.Seed
	w.NSeat = 2
	w.Turn = uint16(v.Turn)
	w.Step = step
	w.Active = uint8(v.Active)
	w.Prio = uint8(v.Priority)
	return w, nil
}

// checkScope refuses a view outside the minimal scope and returns its step.
func checkScope(in *Input) (kop.Step, error) {
	v := &in.View
	if in.Tbl == nil {
		return 0, bad(dirac.ReasonTable, "no table")
	}
	if v.Over {
		return 0, bad(ReasonOver, "")
	}
	if len(v.Players) != 2 || int(v.Viewer) >= 2 {
		return 0, bad(ReasonSeats, "%d players", len(v.Players))
	}
	if len(v.Stack) != 0 || len(v.Pending) != 0 {
		return 0, bad(ReasonStack, "%d on the stack, %d pending", len(v.Stack), len(v.Pending))
	}
	if v.Decision != nil && v.Decision.Kind != "priority" {
		return 0, bad(ReasonDecision, "%s", v.Decision.Kind)
	}
	step, ok := stepOf[v.Step]
	if !ok {
		if v.Step == "" {
			return 0, bad(ReasonStepName, "empty")
		}
		return 0, bad(dirac.ReasonStep, "%s", v.Step)
	}
	return step, nil
}

// placed is one object of the staged game, keyed by its object index.
type placed struct {
	obj uint8
	wo  dirac.WorldObj
}

// builder gathers the objects of a view's zones.
type builder struct {
	in     *Input
	byName map[string]kop.CardID
	all    []placed
}

// add records one card. A card the view shows (cv non-nil) must carry no
// mark the staging cannot rebuild.
func (b *builder) add(obj uint8, name string, owner, pos int, z kop.Zone, cv *view.CardView) error {
	id, ok := b.byName[name]
	if !ok {
		return bad(ReasonUnknown, "%q", name)
	}
	o := dirac.WorldObj{Card: id, Owner: uint8(owner), Zone: z, Pos: uint8(pos), Ctrl: uint8(owner)}
	if cv != nil {
		if err := checkCard(cv, name, owner); err != nil {
			return err
		}
		if z == kop.ZoneBattlefield {
			o.Tapped, o.Sick = cv.Tapped, cv.SummonSick
		}
	}
	b.all = append(b.all, placed{obj, o})
	return nil
}

// checkCard refuses a shown card the minimal scope cannot stage.
func checkCard(cv *view.CardView, name string, owner int) error {
	if cv.IsToken || cv.IsCopy {
		return bad(ReasonToken, "%s", name)
	}
	if cv.FaceDown || cv.Flags != 0 || len(cv.Counters) != 0 || cv.Damage != 0 || cv.Attacking ||
		cv.AttackingPermanent || cv.Blocked || len(cv.BlockedBy) != 0 || cv.AttachedTo != 0 ||
		cv.AttachedToPlayer || len(cv.GoadedBy) != 0 || len(cv.Imprinted) != 0 || len(cv.ExiledCards) != 0 {
		return bad(ReasonMark, "%s", name)
	}
	if int(cv.Owner) != owner || int(cv.Controller) != owner {
		return bad(dirac.ReasonControl, "%s owner %d controller %d", name, cv.Owner, cv.Controller)
	}
	return nil
}

// shown adds the cards of one visible zone of seat s.
func (b *builder) shown(cards []view.CardView, s int, z kop.Zone, withView bool) error {
	for i := range cards {
		cv := &cards[i]
		o, ok := b.in.ObjOf(int(cv.ID))
		if !ok {
			return bad(ReasonUnmapped, "%s (%d)", cv.Name, cv.ID)
		}
		var mark *view.CardView
		if withView {
			mark = cv
		}
		if err := b.add(o, cv.Name, s, i, z, mark); err != nil {
			return err
		}
	}
	return nil
}

// dealt adds the cards the caller dealt into one hidden zone of seat s, after
// checking the count against what the view shows.
func (b *builder) dealt(cards []Card, s int, z kop.Zone, shown int, what string) error {
	if len(cards) != shown {
		return bad(ReasonHidden, "seat %d %s: dealt %d, view shows %d", s, what, len(cards), shown)
	}
	for i, c := range cards {
		if err := b.add(c.Obj, c.Name, s, i, z, nil); err != nil {
			return err
		}
	}
	return nil
}

// seat adds one player's zones and sets the player's life and land drop.
func (b *builder) seat(w *dirac.World, p *view.PlayerView, viewer bool) error {
	s := int(p.ID)
	if s >= 2 {
		return bad(ReasonSeats, "seat %d", s)
	}
	if len(p.Exile) != 0 || len(p.Command) != 0 {
		return bad(ReasonExile, "seat %d", s)
	}
	for _, n := range p.Pool {
		if n != 0 {
			return bad(ReasonPool, "seat %d", s)
		}
	}
	if p.Lost {
		return bad(ReasonOver, "seat %d lost", s)
	}
	w.Life[s] = p.Life
	if p.LandDropSpent {
		w.LandsPlayed[s] = 1
	}
	if err := b.shown(p.Battlefield, s, kop.ZoneBattlefield, true); err != nil {
		return err
	}
	if err := b.shown(p.Graveyard, s, kop.ZoneGraveyard, true); err != nil {
		return err
	}
	var err error
	if viewer {
		err = b.shown(p.Hand, s, kop.ZoneHand, false)
	} else {
		err = b.dealt(b.in.Hidden.Hand[s], s, kop.ZoneHand, p.HandSize, "hand")
	}
	if err != nil {
		return err
	}
	if err := b.dealt(b.in.Hidden.Library[s], s, kop.ZoneLibrary, p.LibrarySize, "library"); err != nil {
		return err
	}
	if p.GraveyardSize != len(p.Graveyard) {
		return bad(ReasonHidden, "seat %d graveyard size", s)
	}
	return nil
}

// assemble orders the gathered objects by index; the indices must be 0..n-1.
func (b *builder) assemble(w *dirac.World) error {
	all := b.all
	sort.SliceStable(all, func(a, c int) bool { return all[a].obj < all[c].obj })
	w.Objs = make([]dirac.WorldObj, len(all))
	for i, p := range all {
		if int(p.obj) != i {
			return bad(ReasonUnmapped, "object indices are not 0..%d (index %d at %d)", len(all)-1, p.obj, i)
		}
		w.Objs[i] = p.wo
	}
	return nil
}
