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
	ReasonOver     = "game-over"        // the game has ended
	ReasonSeats    = "seats"            // not a view of 2..dirac.MaxSeats players
	ReasonSeatLeft = "seat-left"        // a seat has left the game while it goes on
	ReasonStack    = "stack"            // a stack entry that is not a plain token spell or a self-counter ability: a trigger, a target, a mode, an X, a copy
	ReasonPending  = "pending"          // a trigger waiting to be put on the stack
	ReasonDecision = "decision"         // the view's decision is not a priority decision
	ReasonUnknown  = "unknown-card"     // a card name the table does not hold
	ReasonUnmapped = "unmapped-object"  // an object the caller's index cannot place
	ReasonHidden   = "hidden"           // the dealt hidden zones disagree with the counts the view shows
	ReasonExile    = "exile"            // exile or the command zone holds a card
	ReasonToken    = dirac.ReasonToken  // a copy, or a token the table has no plain creature token for
	ReasonCounter  = "counter"          // a counter other than a nonzero +1/+1 count on a battlefield card
	ReasonAttach   = dirac.ReasonAttach // an attachment dirac cannot rebuild, or one attached to a player
	ReasonMark     = "mark"             // counters, damage, an attachment, a combat mark, face-down, a flag
	ReasonPool     = "pool"             // floating mana
	ReasonStepName = "step-name"        // a step name this package does not know
)

// Reasons lists every reason WorldFromView returns that dirac does not.
var Reasons = []string{
	ReasonOver, ReasonSeats, ReasonSeatLeft, ReasonStack, ReasonPending, ReasonDecision, ReasonUnknown, ReasonUnmapped,
	ReasonHidden, ReasonExile, ReasonCounter, ReasonMark, ReasonPool, ReasonStepName,
}

var stepOf = map[string]kop.Step{
	"upkeep": kop.StepUpkeep, "draw": kop.StepDraw, "main1": kop.StepMain1,
	"main2": kop.StepMain2, "end": kop.StepEnd,
	"begin-combat": kop.StepBeginCombat, "declare-attackers": kop.StepAttackers, "declare-blockers": kop.StepBlockers,
}

// Card is one hidden card the caller dealt: its object index and its name.
type Card struct {
	Obj  dirac.Obj
	Name string
}

// Hidden is the caller's assignment of the zones the view hides. Hand[p] is
// seat p's hand in order (the viewer's own hand is read from the view
// instead, and Hand[viewer] is ignored); Library[p] is seat p's library from
// the top. Sizes must equal the counts the view shows.
type Hidden struct {
	Hand    [dirac.MaxSeats][]Card
	Library [dirac.MaxSeats][]Card
}

// Input is everything WorldFromView reads.
type Input struct {
	View   view.View
	Hidden Hidden
	// ObjOf maps a card the view shows (its CardView.ID) to its object
	// index in the staged game. The caller owns the numbering: seat 0's
	// cards first, then seat 1's, as dirac.World's Objs.
	ObjOf func(id int) (dirac.Obj, bool)
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
	step, pose, err := checkScope(in)
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
	if err := b.stack(&w); err != nil {
		return w, err
	}
	if err := b.combat(); err != nil {
		return w, err
	}
	if err := b.assemble(&w); err != nil {
		return w, err
	}
	w.Tbl, w.Cfg, w.Seed = in.Tbl, in.Cfg, in.Seed
	w.NSeat = uint8(len(v.Players))
	w.Turn = uint16(v.Turn)
	w.Step = step
	w.Active = uint8(v.Active)
	w.Prio = uint8(v.Priority)
	w.Pose = pose
	if pose == dirac.PoseBlockers {
		w.BlkDef = uint8(v.Decision.Player) // the defender asked (read only at more than two seats)
	}
	return w, nil
}

// checkScope refuses a view outside the minimal scope and returns its step and
// the decision it poses.
func checkScope(in *Input) (kop.Step, dirac.Pose, error) {
	v := &in.View
	if in.Tbl == nil {
		return 0, 0, bad(dirac.ReasonTable, "no table")
	}
	if v.Over {
		return 0, 0, bad(ReasonOver, "")
	}
	if len(v.Players) < 2 || len(v.Players) > dirac.MaxSeats || int(v.Viewer) >= len(v.Players) {
		return 0, 0, bad(ReasonSeats, "%d players", len(v.Players))
	}
	if len(v.Pending) != 0 {
		return 0, 0, bad(ReasonPending, "%d pending", len(v.Pending))
	}
	step, ok := stepOf[v.Step]
	if !ok {
		if v.Step == "" {
			return 0, 0, bad(ReasonStepName, "empty")
		}
		return 0, 0, bad(dirac.ReasonStep, "%s", v.Step)
	}
	pose, err := poseOf(v, step)
	return step, pose, err
}

// poseOf is the decision v poses. A view shows its decision only to the seat
// that makes it, so an observer sees none at a combat step and cannot tell a
// pose from the priority round after a declaration that made no pick: that is
// ReasonDecision unless the combat marks settle it (attackers on the board at
// the declare attackers step, blocks at the declare blockers step).
func poseOf(v *view.View, step kop.Step) (dirac.Pose, error) {
	if d := v.Decision; d != nil {
		switch {
		case d.Kind == "priority":
			return dirac.PosePriority, nil
		case d.Kind == "attackers" && step == kop.StepAttackers:
			return dirac.PoseAttackers, nil
		case d.Kind == "blockers" && step == kop.StepBlockers:
			return dirac.PoseBlockers, nil
		}
		return 0, bad(ReasonDecision, "%s at %s", d.Kind, v.Step)
	}
	if step != kop.StepAttackers && step != kop.StepBlockers {
		return dirac.PosePriority, nil
	}
	for _, p := range v.Players {
		for i := range p.Battlefield {
			if c := &p.Battlefield[i]; step == kop.StepAttackers && c.Attacking || step == kop.StepBlockers && c.Blocked {
				return dirac.PosePriority, nil
			}
		}
	}
	return 0, bad(ReasonDecision, "no decision shown at %s: priority or a pose", v.Step)
}

// placed is one object of the staged game, keyed by its object index.
type placed struct {
	obj dirac.Obj
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
func (b *builder) add(obj dirac.Obj, name string, owner, pos int, z kop.Zone, cv *view.CardView) error {
	id, ok := b.byName[name]
	if cv != nil && cv.IsToken {
		id, ok = b.tokenCard(cv)
		if !ok {
			return bad(ReasonToken, "%s", name)
		}
	}
	if !ok {
		return bad(ReasonUnknown, "%q", name)
	}
	o := dirac.WorldObj{Card: id, Owner: uint8(owner), Zone: z, Pos: uint8(pos), Ctrl: uint8(owner)}
	if cv != nil {
		if err := checkCard(cv, name, owner, z); err != nil {
			return err
		}
		if err := b.markTokenAttach(&o, cv, name); err != nil {
			return err
		}
		if z == kop.ZoneBattlefield {
			if err := b.markAttack(&o, cv, name); err != nil {
				return err
			}
			o.Tapped, o.Sick = cv.Tapped, cv.SummonSick
			o.Damage, o.P1P1 = uint8(cv.Damage), uint16(cv.Counters["P1P1"])
		}
	}
	b.all = append(b.all, placed{obj, o})
	return nil
}

// checkMarks refuses counters and damage the staging cannot rebuild. On the
// battlefield it carries damage within a byte and a nonzero +1/+1 count; a
// card off the battlefield carries neither (dirac keeps none there).
func checkMarks(cv *view.CardView, name string, z kop.Zone) error {
	for kind, n := range cv.Counters {
		if kind != "P1P1" || n <= 0 || n > 0xffff || z != kop.ZoneBattlefield {
			return bad(ReasonCounter, "%s: %s %d", name, kind, n)
		}
	}
	if cv.Damage < 0 || cv.Damage > 255 || (cv.Damage != 0 && z != kop.ZoneBattlefield) {
		return bad(ReasonMark, "%s: damage %d", name, cv.Damage)
	}
	return nil
}

// checkCard refuses a shown card the minimal scope cannot stage.
func checkCard(cv *view.CardView, name string, owner int, z kop.Zone) error {
	if cv.IsCopy || cv.IsToken && z != kop.ZoneBattlefield {
		return bad(ReasonToken, "%s", name)
	}
	if cv.AttachedToPlayer || cv.AttachedTo != 0 && z != kop.ZoneBattlefield {
		return bad(ReasonAttach, "%s", name)
	}
	if err := checkMarks(cv, name, z); err != nil {
		return err
	}
	if cv.FaceDown || cv.Flags != 0 || cv.AttackingPermanent ||
		z != kop.ZoneBattlefield && (cv.Attacking || cv.Blocked || len(cv.BlockedBy) != 0) ||
		len(cv.GoadedBy) != 0 || len(cv.Imprinted) != 0 || len(cv.ExiledCards) != 0 {
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
	if s >= len(b.in.View.Players) {
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
		return bad(ReasonSeatLeft, "seat %d", s)
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

// assemble orders the gathered objects by index. The cards of the decks take
// 0..m-1, as dirac.World lays them out, and the tokens the view shows follow;
// a gap before a token is a token the view no longer shows. (A token that left
// after the last shown one leaves no gap: the staged game just holds fewer
// objects than the one that made it.)
func (b *builder) assemble(w *dirac.World) error {
	all := b.all
	sort.SliceStable(all, func(a, c int) bool { return all[a].obj < all[c].obj })
	w.Objs = make([]dirac.WorldObj, len(all))
	for i, p := range all {
		if int(p.obj) != i {
			if p.wo.Token {
				return bad(ReasonToken, "a token left the view: object index %d at %d", p.obj, i)
			}
			return bad(ReasonUnmapped, "object indices are not 0..%d (index %d at %d)", len(all)-1, p.obj, i)
		}
		w.Objs[i] = p.wo
	}
	return nil
}
