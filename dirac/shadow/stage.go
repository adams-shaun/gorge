package shadow

import (
	"fmt"
	"slices"
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
	ReasonExile    = "exile"            // exile holds a card, or the command zone a card that is not its commander
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

// CeasedToken is a token that has ceased to exist (CR 111.7, 704.5d): it keeps
// the object index the game gave it and no view shows it. Name is the table
// token's script name as tokenCard resolves it ("r_1_1_goblin").
type CeasedToken struct {
	Obj   dirac.Obj
	Name  string
	Owner uint8
}

// Hidden is the caller's assignment of the zones the view hides. Hand[p] is
// seat p's hand in order (the viewer's own hand is read from the view
// instead, and Hand[viewer] is ignored); Library[p] is seat p's library from
// the top. Sizes must equal the counts the view shows. Ceased lists the tokens
// that have ceased to exist, which fill the gaps among the token indices.
type Hidden struct {
	Hand    [dirac.MaxSeats][]Card
	Library [dirac.MaxSeats][]Card
	// Ceased is every token that has ceased to exist, at the index the game gave it.
	Ceased []CeasedToken
	// Gone[p] is every card of seat p's deck, in object order, for a player the
	// view shows as Lost: those cards ceased to exist when the seat left (CR
	// 800.4a). Read only for a lost player.
	Gone [dirac.MaxSeats][]Card
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
	if err := b.ceased(); err != nil {
		return w, err
	}
	if err := b.commanderDamage(&w); err != nil {
		return w, err
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
// the declare attackers step, blocks at the declare blockers step; at more
// than two seats blocks settle nothing, as another defender may still be to
// declare).
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
			if c := &p.Battlefield[i]; step == kop.StepAttackers && c.Attacking || step == kop.StepBlockers && len(v.Players) == 2 && c.Blocked {
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
	if err := b.commander(w, p); err != nil {
		return err
	}
	for _, n := range p.Pool {
		if n != 0 {
			return bad(ReasonPool, "seat %d", s)
		}
	}
	if p.Lost {
		return b.departed(w, p, s)
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
	if err := b.shown(p.Command, s, kop.ZoneCommand, true); err != nil {
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

// departed adds a seat that has left the game: the view shows it with nothing
// in any zone, and the caller's Gone cards are its ceased objects.
func (b *builder) departed(w *dirac.World, p *view.PlayerView, s int) error {
	if len(p.Hand) != 0 || len(p.Library) != 0 || len(p.Battlefield) != 0 || len(p.Graveyard) != 0 ||
		p.HandSize != 0 || p.LibrarySize != 0 || p.GraveyardSize != 0 {
		return bad(ReasonSeatLeft, "a departed seat shows cards (seat %d)", s)
	}
	if len(p.Commanders) != 0 || len(p.Command) != 0 {
		return bad(dirac.ReasonCommander, "a departed seat keeps a commander (seat %d)", s)
	}
	w.Lost[s] = true
	w.Life[s] = p.Life
	for _, c := range b.in.Hidden.Gone[s] {
		if err := b.add(c.Obj, c.Name, s, 0, kop.ZoneNone, nil); err != nil {
			return err
		}
	}
	return nil
}

// commander refuses an exile or command zone seat p's position cannot stage
// (exile holds nothing; the command zone, its commander only) and reads the
// commander: one a seat, its cast count, and each player's damage from it
// (the damage is read once every seat's commander is known: commanderDamage).
func (b *builder) commander(w *dirac.World, p *view.PlayerView) error {
	s := int(p.ID)
	if len(p.Exile) != 0 {
		return bad(ReasonExile, "seat %d", s)
	}
	for i := range p.Command {
		if !slices.ContainsFunc(p.Commanders, func(c view.CardView) bool { return c.ID == p.Command[i].ID }) {
			return bad(ReasonExile, "seat %d command zone holds %s, not a commander", s, p.Command[i].Name)
		}
	}
	if len(p.Commanders) > 1 {
		return bad(dirac.ReasonCommander, "seat %d has %d commanders", s, len(p.Commanders))
	}
	if len(p.Commanders) == 0 {
		w.Cmd[s] = dirac.NoObj
		return nil
	}
	o, ok := b.in.ObjOf(int(p.Commanders[0].ID))
	if !ok {
		return bad(ReasonUnmapped, "seat %d commander %s (%d)", s, p.Commanders[0].Name, p.Commanders[0].ID)
	}
	w.Commander, w.Cmd[s] = true, o
	if len(p.CommanderCasts) != 1 || p.CommanderCasts[0] < 0 || p.CommanderCasts[0] > 255 {
		return bad(dirac.ReasonCommander, "seat %d commander casts %v", s, p.CommanderCasts)
	}
	w.CmdCasts[s] = uint8(p.CommanderCasts[0])
	return nil
}

// commanderDamage reads each player's commander damage (CR 903.10a) into w:
// for player q and seat p, what q took from p's commander. The map is read by
// key, never ranged; a tally past a byte or on a card that is no seat's
// commander is refused.
func (b *builder) commanderDamage(w *dirac.World) error {
	if !w.Commander {
		return nil
	}
	v := &b.in.View
	for q := range v.Players {
		pq := &v.Players[q]
		known := 0
		for p := range v.Players {
			if len(v.Players[p].Commanders) == 0 {
				continue
			}
			id := v.Players[p].Commanders[0].ID
			d, ok := pq.CmdDamage[id]
			if ok {
				known++
			}
			if d < 0 || d > 255 {
				return bad(dirac.ReasonCommander, "seat %d commander damage %d", q, d)
			}
			w.CmdDamage[int(pq.ID)][int(v.Players[p].ID)] = uint8(d)
		}
		if known != len(pq.CmdDamage) {
			return bad(dirac.ReasonCommander, "seat %d took damage from a card that is no seat's commander", q)
		}
	}
	return nil
}

// assemble orders the gathered objects by index. The cards of the decks take
// 0..m-1, as dirac.World lays them out, and the tokens follow, the ceased ones
// (Hidden.Ceased) among them; a gap before a token that no ceased token fills
// is refused. (A token that ceased after the last one is named leaves no gap:
// the staged game just holds fewer objects than the one that made it.)
func (b *builder) assemble(w *dirac.World) error {
	all := b.all
	sort.SliceStable(all, func(a, c int) bool { return all[a].obj < all[c].obj })
	w.Objs = make([]dirac.WorldObj, len(all))
	for i, p := range all {
		if int(p.obj) != i {
			if p.wo.Token {
				return bad(ReasonToken, "a token left the view and is not ceased: object index %d at %d", p.obj, i)
			}
			return bad(ReasonUnmapped, "object indices are not 0..%d (index %d at %d)", len(all)-1, p.obj, i)
		}
		w.Objs[i] = p.wo
	}
	return nil
}
