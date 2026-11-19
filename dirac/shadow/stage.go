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
	v := &in.View
	var w dirac.World
	if in.Tbl == nil {
		return w, bad(dirac.ReasonTable, "no table")
	}
	if v.Over {
		return w, bad(ReasonOver, "")
	}
	if len(v.Players) != 2 || int(v.Viewer) >= 2 {
		return w, bad(ReasonSeats, "%d players", len(v.Players))
	}
	if len(v.Stack) != 0 || len(v.Pending) != 0 {
		return w, bad(ReasonStack, "%d on the stack, %d pending", len(v.Stack), len(v.Pending))
	}
	if v.Decision != nil && v.Decision.Kind != "priority" {
		return w, bad(ReasonDecision, "%s", v.Decision.Kind)
	}
	step, ok := stepOf[v.Step]
	if !ok {
		if v.Step == "" {
			return w, bad(ReasonStepName, "empty")
		}
		return w, bad(dirac.ReasonStep, "%s", v.Step)
	}
	byName := map[string]kop.CardID{}
	for i := range in.Tbl.Cards {
		if _, dup := byName[in.Tbl.Cards[i].Name]; !dup {
			byName[in.Tbl.Cards[i].Name] = kop.CardID(i)
		}
	}
	viewer := int(v.Viewer)
	type placed struct {
		obj uint8
		wo  dirac.WorldObj
	}
	var all []placed
	add := func(obj uint8, name string, owner, pos int, z kop.Zone, cv *view.CardView) error {
		id, ok := byName[name]
		if !ok {
			return bad(ReasonUnknown, "%q", name)
		}
		o := dirac.WorldObj{Card: id, Owner: uint8(owner), Zone: z, Pos: uint8(pos), Ctrl: uint8(owner)}
		if cv != nil {
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
			if z == kop.ZoneBattlefield {
				o.Tapped, o.Sick = cv.Tapped, cv.SummonSick
			}
		}
		all = append(all, placed{obj, o})
		return nil
	}
	objOf := func(cv *view.CardView) (uint8, error) {
		o, ok := in.ObjOf(int(cv.ID))
		if !ok {
			return 0, bad(ReasonUnmapped, "%s (%d)", cv.Name, cv.ID)
		}
		return o, nil
	}
	for _, p := range v.Players {
		s := int(p.ID)
		if s >= 2 {
			return w, bad(ReasonSeats, "seat %d", s)
		}
		if len(p.Exile) != 0 || len(p.Command) != 0 {
			return w, bad(ReasonExile, "seat %d", s)
		}
		for _, n := range p.Pool {
			if n != 0 {
				return w, bad(ReasonPool, "seat %d", s)
			}
		}
		if p.Lost {
			return w, bad(ReasonOver, "seat %d lost", s)
		}
		w.Life[s] = p.Life
		if p.LandDropSpent {
			w.LandsPlayed[s] = 1
		}
		for i := range p.Battlefield {
			cv := &p.Battlefield[i]
			o, err := objOf(cv)
			if err == nil {
				err = add(o, cv.Name, s, i, kop.ZoneBattlefield, cv)
			}
			if err != nil {
				return w, err
			}
		}
		for i := range p.Graveyard {
			cv := &p.Graveyard[i]
			o, err := objOf(cv)
			if err == nil {
				err = add(o, cv.Name, s, i, kop.ZoneGraveyard, cv)
			}
			if err != nil {
				return w, err
			}
		}
		if s == viewer {
			for i := range p.Hand {
				cv := &p.Hand[i]
				o, err := objOf(cv)
				if err == nil {
					err = add(o, cv.Name, s, i, kop.ZoneHand, nil)
				}
				if err != nil {
					return w, err
				}
			}
		} else {
			if len(in.Hidden.Hand[s]) != p.HandSize {
				return w, bad(ReasonHidden, "seat %d hand: dealt %d, view shows %d", s, len(in.Hidden.Hand[s]), p.HandSize)
			}
			for i, c := range in.Hidden.Hand[s] {
				if err := add(c.Obj, c.Name, s, i, kop.ZoneHand, nil); err != nil {
					return w, err
				}
			}
		}
		if len(in.Hidden.Library[s]) != p.LibrarySize {
			return w, bad(ReasonHidden, "seat %d library: dealt %d, view shows %d", s, len(in.Hidden.Library[s]), p.LibrarySize)
		}
		for i, c := range in.Hidden.Library[s] {
			if err := add(c.Obj, c.Name, s, i, kop.ZoneLibrary, nil); err != nil {
				return w, err
			}
		}
		if p.GraveyardSize != len(p.Graveyard) {
			return w, bad(ReasonHidden, "seat %d graveyard size", s)
		}
	}
	sort.SliceStable(all, func(a, b int) bool { return all[a].obj < all[b].obj })
	w.Objs = make([]dirac.WorldObj, len(all))
	for i, p := range all {
		if int(p.obj) != i {
			return w, bad(ReasonUnmapped, "object indices are not 0..%d (index %d at %d)", len(all)-1, p.obj, i)
		}
		w.Objs[i] = p.wo
	}
	w.Tbl, w.Cfg, w.Seed = in.Tbl, in.Cfg, in.Seed
	w.NSeat = 2
	w.Turn = uint16(v.Turn)
	w.Step = step
	w.Active = uint8(v.Active)
	w.Prio = uint8(v.Priority)
	return w, nil
}
