package view_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

func damageCard(t *testing.T, src string) *cards.Card {
	t.Helper()
	c, d := cards.ParseBytes("t.txt", []byte(src))
	if len(d) != 0 {
		t.Fatalf("diags: %v", d)
	}
	c.Link()
	for _, f := range c.Faces {
		f.ApplyIntrinsics()
	}
	return c
}

// combatGame is a two-seat game parked at seat 0's declare attackers step with
// the named creature of each seat on the battlefield (ready to attack).
func combatGame(t *testing.T, atkSrc, blkSrc string) (e *rules.Engine, atk, blk state.ObjID) {
	t.Helper()
	land := damageCard(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n")
	a, b := damageCard(t, atkSrc), damageCard(t, blkSrc)
	deck := func(c *cards.Card) []*cards.Card {
		d := []*cards.Card{c}
		for len(d) < 40 {
			d = append(d, land)
		}
		return d
	}
	e = rules.New(rules.Config{Seed: 5, Names: []string{"a", "b"}, Decks: [][]*cards.Card{deck(a), deck(b)}})
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		switch o.Card {
		case a:
			atk = o.ID
		case b:
			blk = o.ID
		default:
			continue
		}
		e.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: o.Zone, To: state.ZBattlefield})
	}
	if atk == 0 || blk == 0 {
		t.Fatalf("creatures not found: %d %d", atk, blk)
	}
	e.G.Obj(atk).SummonSick = false
	e.Emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.Emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	e.Advance()
	return e, atk, blk
}

func answer(t *testing.T, e *rules.Engine, kind decision.Kind, objs ...state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != kind {
		t.Fatalf("want a %s decision, got %+v", kind, d)
	}
	var choices []int
	for _, o := range d.Options {
		for _, id := range objs {
			if o.Obj == id {
				choices = append(choices, o.Index)
			}
		}
	}
	if len(choices) != len(objs) {
		t.Fatalf("options %+v do not offer %v", d.Options, objs)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
		t.Fatal(err)
	}
}

func passPriority(t *testing.T, e *rules.Engine) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("want priority, got %+v", d)
	}
	for _, o := range d.Options {
		if o.Kind == "pass" {
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}}); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no pass option: %+v", d.Options)
}

func cardOf(t *testing.T, v view.View, id state.ObjID) view.CardView {
	t.Helper()
	for _, p := range v.Players {
		for _, c := range p.Battlefield {
			if c.ID == id {
				return c
			}
		}
	}
	t.Fatalf("object %d is not on a battlefield", id)
	return view.CardView{}
}

// roundsOf passes priority through the declare blockers step and the combat
// damage step and records, at each priority decision, the damage step view and
// the views of the creatures.
type round struct {
	step, damage string
	first        bool
	atk, blk     view.CardView
}

func playCombat(t *testing.T, atkSrc, blkSrc string) []round {
	t.Helper()
	e, atk, blk := combatGame(t, atkSrc, blkSrc)
	answer(t, e, decision.KAttackers, atk)
	for e.G.Step == state.StepDeclareAttackers {
		passPriority(t, e)
	}
	answer(t, e, decision.KBlockers, blk)
	var out []round
	for i := 0; i < 40 && !e.G.Over && e.G.Step != state.StepMain2; i++ {
		d := e.Pending()
		if d == nil {
			break
		}
		v := view.Project(e.G, e, d.Player, d)
		r := round{step: v.Step, damage: v.DamageStep, first: v.FirstStrikeStep}
		for _, p := range v.Players {
			for _, c := range p.Battlefield {
				if c.ID == atk {
					r.atk = c
				}
				if c.ID == blk {
					r.blk = c
				}
			}
		}
		out = append(out, r)
		passPriority(t, e)
	}
	return out
}

// TestProjectBlockingAndDamageStep: a blocker shows Blocking, also once its
// attacker has left combat; the combat damage step's two priority rounds are
// told apart even after the first striker's victim has died; and a view off
// the step, or of a game without combat, carries none of the new fields.
func TestProjectBlockingAndDamageStep(t *testing.T) {
	fs := "Name:Striker\nManaCost:1 R\nTypes:Creature Bear\nPT:2/2\nK:First Strike\nOracle:x\n"
	vanilla := "Name:Wall\nManaCost:1 W\nTypes:Creature Wall\nPT:1/1\nOracle:x\n"

	// A first striker blocked by a vanilla 1/1 that it kills in the first pass.
	rs := playCombat(t, fs, vanilla)
	var seen []string
	for _, r := range rs {
		if r.step == "declare-blockers" {
			if !r.blk.Blocking || len(r.atk.BlockedBy) != 1 || r.atk.Blocking {
				t.Fatalf("declare blockers: blocker Blocking=%v attacker BlockedBy=%v Blocking=%v", r.blk.Blocking, r.atk.BlockedBy, r.atk.Blocking)
			}
			if r.damage != "" || r.first {
				t.Fatalf("declare blockers carries damage step %q first %v", r.damage, r.first)
			}
		}
		if r.step == "combat-damage" {
			seen = append(seen, r.damage)
			if !r.first {
				t.Errorf("combat damage round %q: FirstStrikeStep false in a first-strike combat", r.damage)
			}
			if !r.atk.Blocked || len(r.atk.BlockedBy) != 0 {
				t.Errorf("combat damage round %q: attacker Blocked=%v BlockedBy=%v, want blocked by no one left", r.damage, r.atk.Blocked, r.atk.BlockedBy)
			}
		}
		if r.step == "end-combat" && (r.damage != "" || r.first) {
			t.Errorf("end of combat carries damage step %q first %v", r.damage, r.first)
		}
	}
	if strings.Join(seen, ",") != "first-strike,first-strike,regular,regular" {
		t.Fatalf("damage steps of the combat damage priority rounds = %v, want two first-strike then two regular", seen)
	}

	// A vanilla attacker blocked by a first striker dies in the first pass; its
	// blocker still blocks (CR 506.4) with no attacker to name it.
	rs = playCombat(t, vanilla, fs)
	orphan := 0
	for _, r := range rs {
		if r.step == "combat-damage" && r.blk.Blocking {
			orphan++
			if r.atk.ID != 0 {
				t.Fatalf("the attacker is still on the battlefield: %+v", r.atk)
			}
		}
	}
	if orphan == 0 {
		t.Fatalf("no combat damage view shows the blocker of a departed attacker as Blocking: %+v", rs)
	}

	// No first striker: one round, "regular", no first-strike pass.
	rs = playCombat(t, vanilla, vanilla)
	n := 0
	for _, r := range rs {
		if r.step == "combat-damage" {
			n++
			if r.damage != "regular" || r.first {
				t.Fatalf("no first strike: damage step %q first %v", r.damage, r.first)
			}
		}
	}
	if n == 0 {
		t.Fatal("no combat damage priority round reached")
	}

	// No combat: the fields are absent from the JSON, byte for byte.
	e := rules.New(rules.Config{Seed: 1, Names: []string{"a", "b"}, Decks: [][]*cards.Card{
		{damageCard(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n")}, {damageCard(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n")}}})
	e.Advance()
	v := view.Project(e.G, e, 0, e.Pending())
	got, _ := json.Marshal(v)
	z := v
	z.DamageStep, z.FirstStrikeStep = "", false
	want, _ := json.Marshal(z)
	if string(got) != string(want) {
		t.Fatalf("a no-combat view marshals differently once the fields are zeroed")
	}
	for _, k := range []string{"damage_step", "first_strike_step", `"blocking"`} {
		if strings.Contains(string(got), k) {
			t.Fatalf("a no-combat view carries %s: %s", k, got)
		}
	}
}
