package neutral_test

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	_ "github.com/adams-shaun/gorge/bots/all"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/agent"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/neutral"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// A manual-mana host (mtg-kernel's shape): the seat taps each source with an
// activate_mana_ability candidate, and a cast_spell candidate appears once the
// pool pays. payBoard describes one such position.
type payBoard struct {
	lands  []string // seat 0's untapped permanents with a mana ability, by card name
	hand   []string
	tapped map[string]bool // the lands tapped so far
}

var landMana = map[string]string{"Mountain": "R", "Island": "U", "Plains": "W", "Forest": "G", "Swamp": "B"}

func oid(zone, name string) string { return "o-" + zone + "-" + strings.ReplaceAll(name, " ", "") }

func ref0(zone, name string) string {
	return `{"object_id":"` + oid(zone, name) + `","card_name":"` + name + `","owner_seat":"p0","controller_seat":"p0","zone":"` + zone + `"}`
}

func (b payBoard) pool() map[string]int {
	pool := map[string]int{}
	for l := range b.tapped {
		if col, ok := landMana[l]; ok {
			pool[col]++
		}
	}
	return pool
}

func (b payBoard) observation() string {
	record := func(zone, name string, perm string) string {
		return `{"object_id":"` + oid(zone, name) + `","card_name":"` + name + `","owner_seat":"p0","controller_seat":"p0","zone":"` + zone +
			`","full_name":null,"face_down":false,"token":false,"copy":false,"characteristics":null,"permanent":` + perm + `,"exiled_by":null}`
	}
	var bf, hand []string
	for _, l := range b.lands {
		bf = append(bf, record("battlefield", l, fmt.Sprintf(`{"tapped":%v,"summoning_sick":false,"damage":0,"counters":{},"attached_to":null,`+
			`"attacking":false,"attack_target":null,"blocking":false,"blocked_attackers":[],"phased_out":false,"statuses":[],"class_level":null,"chosen":[]}`, b.tapped[l])))
	}
	for _, h := range b.hand {
		hand = append(hand, record("hand", h, "null"))
	}
	pool := b.pool()
	player := func(seat string, hand, bf []string, pool map[string]int) string {
		hs := "null"
		if hand != nil {
			hs = "[" + strings.Join(hand, ",") + "]"
		}
		return fmt.Sprintf(`{"seat":"%s","life":20,"poison":null,"counters":null,"mana_pool":{"W":%d,"U":%d,"B":%d,"R":%d,"G":%d,"C":0},`+
			`"lands_played_this_turn":0,"mulligans_taken":0,"designations":[],"progress":null,"hand_count":%d,"library_count":30,"hand":%s,`+
			`"battlefield":[%s],"graveyard":[],"exile":[],"command":[]}`, seat, pool["W"], pool["U"], pool["B"], pool["R"], pool["G"], len(hand), hs, strings.Join(bf, ","))
	}
	return `{"viewer":"p0","turn":5,"phase_step":"precombat_main","active_seat":"p0","priority_seat":"p0","passed_seats":[],"day_night":null,` +
		`"stack":[],"pending_triggers":null,"known":[],"players":[` + player("p0", hand, bf, pool) + `,` + player("p1", nil, nil, map[string]int{}) + `]}`
}

// candidates is the host's offer: pass, a tap of every untapped source, and a
// cast of each spell the pool pays.
func (b payBoard) candidates(afford func(spell string, pool map[string]int) bool) []string {
	out := []string{`{"kind":"pass"}`}
	for _, l := range b.lands {
		if !b.tapped[l] {
			out = append(out, `{"kind":"activate_mana_ability","source":`+ref0("battlefield", l)+`,"ability_index":0,"mana_choice":null,"cost_target":null}`)
		}
	}
	for _, h := range b.hand {
		if afford(h, b.pool()) {
			out = append(out, `{"kind":"cast_spell","source":`+ref0("hand", h)+`,"method":null}`)
		}
	}
	return out
}

func chooseLine(step int, b payBoard, cands []string) string {
	body := `{"acting_seat":"p0","seat_step":` + fmt.Sprint(step) + `,"group":{"group_id":` + fmt.Sprint(step) + `,"substep_index":0,"substep_count":1},` +
		`"context":{"kind":"priority","source":null,"purpose":null,"text":null,"rewind":false},"observation":` + b.observation() + `,"candidates":[`
	for i, c := range cands {
		if i > 0 {
			body += ","
		}
		body += fmt.Sprintf(`{"candidate_id":%d,"semantic":%s,"display_text":null}`, i, c)
	}
	return `{"request_type":"choose","protocol":"spellbench/v2","request_id":"r-` + fmt.Sprint(step) + `","game_id":"g","decision":` + body + `],"extensions":{}}}`
}

func neutralAgent(t *testing.T, reg *cards.Registry, deck string) *agent.Server {
	t.Helper()
	a, err := agent.New("bot")
	if err != nil {
		t.Fatal(err)
	}
	a.SetRegistry(reg)
	if err := a.EnableNeutral(); err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		`{"request_type":"hello","protocol":"spellbench/v2","request_id":"r-0"}`,
		`{"request_type":"game_start","protocol":"spellbench/v2","request_id":"r-1","game_id":"g","seat":"p0","agent_seed":7,"own_deck":` + deck + `,"opponent_deck":` + deck + `}`,
	} {
		if out := a.Handle([]byte(line)); strings.Contains(string(out), `"error"`) {
			t.Fatalf("%s", out)
		}
	}
	return a
}

func pick(t *testing.T, a *agent.Server, line string) int {
	t.Helper()
	var choice struct {
		Selection struct {
			CandidateID int `json:"candidate_id"`
		} `json:"selection"`
	}
	if err := json.Unmarshal(a.Handle([]byte(line)), &choice); err != nil {
		t.Fatal(err)
	}
	return choice.Selection.CandidateID
}

// nativePlans builds the same position on gorge's own engine and returns the
// sources, by name and in order, of the first plan the planner offers for each
// spell in hand: the plan a native bot would submit.
func nativePlans(t *testing.T, reg *cards.Registry, b payBoard) map[string][]string {
	t.Helper()
	get := func(n string) *cards.Card {
		c, ok := reg.Lookup(n)
		if !ok {
			t.Fatalf("no card %q", n)
		}
		return c
	}
	deck0 := []*cards.Card{}
	for _, n := range append(slices.Clone(b.lands), b.hand...) {
		deck0 = append(deck0, get(n))
	}
	for len(deck0) < 40 {
		deck0 = append(deck0, get("Wastes"))
	}
	deck1 := make([]*cards.Card, 40)
	for i := range deck1 {
		deck1[i] = get("Wastes")
	}
	e := rules.New(rules.Config{Seed: 7, Names: []string{"p0", "p1"}, Decks: [][]*cards.Card{deck0, deck1}, Tokens: reg.Tokens})
	used := map[state.ObjID]bool{}
	take := func(name string) state.ObjID {
		for i := range e.G.Objs {
			if o := &e.G.Objs[i]; o.Card != nil && o.Owner == 0 && !used[o.ID] && o.Face().Name == name {
				used[o.ID] = true
				return o.ID
			}
		}
		t.Fatalf("%q not in seat 0's genesis", name)
		return 0
	}
	for _, n := range b.lands {
		id := take(n)
		e.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: e.G.Obj(id).Zone, To: state.ZBattlefield})
	}
	for _, n := range b.hand {
		id := take(n)
		if from := e.G.Obj(id).Zone; from != state.ZHand {
			e.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: from, To: state.ZHand})
		}
	}
	e.Advance()
	for i := 0; i < 3000; i++ {
		d := e.Pending()
		if d == nil || e.G.Over {
			t.Fatal("the native fixture ended before seat 0's main phase")
		}
		if d.Kind == decision.KPriority && d.Player == 0 && e.G.Active == 0 && e.G.Step == state.StepMain1 && len(e.G.Stack) == 0 {
			break
		}
		in := decision.Intent{Seq: d.Seq, Player: d.Player}
		if d.Kind == decision.KPriority {
			for _, o := range d.Options {
				if o.Kind == "pass" {
					in.Choices = []int{o.Index}
				}
			}
		} else {
			for j := 0; j < d.Min && j < len(d.Options); j++ {
				in.Choices = append(in.Choices, d.Options[j].Index)
			}
		}
		if err := e.Submit(in); err != nil {
			t.Fatalf("driving the native fixture: %v", err)
		}
	}
	plans := map[string][]string{}
	for _, a := range e.EnsurePaymentActions() {
		var srcs []string
		for _, act := range a.Plans[0].Activations {
			srcs = append(srcs, e.G.Obj(act.Source).Face().Name)
		}
		plans[e.G.Obj(a.Cast.Object).Face().Name] = srcs
	}
	return plans
}

// TestNeutralPaymentLowersChosenPlan: three lands of different colours, and
// a two-mana artifact beside a one-red-mana instant in hand. gorge's planner
// pays the artifact with the lands the instant does not need, and the neutral
// seat, whichever spell its bot casts, taps exactly the sources of that spell's
// native plan, in the plan's order, before it casts it.
func TestNeutralPaymentLowersChosenPlan(t *testing.T) {
	reg := testcorpus.Registry(t)
	b := payBoard{lands: []string{"Mountain", "Island", "Plains"}, hand: []string{"Mind Stone", "Lightning Bolt"}, tapped: map[string]bool{}}
	native := nativePlans(t, reg, b)
	if got := native["Mind Stone"]; len(got) != 2 || slices.Contains(got, "Mountain") {
		t.Fatalf("precondition: gorge's plan for Mind Stone should leave the Mountain for Lightning Bolt, got %v", got)
	}
	if got := native["Lightning Bolt"]; !slices.Equal(got, []string{"Mountain"}) {
		t.Fatalf("precondition: the plan for Lightning Bolt is %v", got)
	}
	afford := func(spell string, pool map[string]int) bool {
		if spell == "Lightning Bolt" {
			return pool["R"] >= 1
		}
		return pool["W"]+pool["U"]+pool["B"]+pool["R"]+pool["G"] >= 2
	}
	deck := `{"name":"Mix","decklist":[{"name":"Mind Stone","count":4},{"name":"Lightning Bolt","count":4},{"name":"Mountain","count":4},{"name":"Island","count":4},{"name":"Plains","count":4}]}`
	a := neutralAgent(t, reg, deck)
	var taps []string
	for step := 2; ; step++ {
		cands := b.candidates(afford)
		i := pick(t, a, chooseLine(step, b, cands))
		var sem protocol.Semantic
		if err := json.Unmarshal([]byte(cands[i]), &sem); err != nil {
			t.Fatal(err)
		}
		switch sem.Kind {
		case "activate_mana_ability":
			var src struct {
				Source struct {
					CardName string `json:"card_name"`
				} `json:"source"`
			}
			json.Unmarshal([]byte(cands[i]), &src)
			taps = append(taps, src.Source.CardName)
			b.tapped[src.Source.CardName] = true
		case "cast_spell":
			var src struct {
				Source struct {
					CardName string `json:"card_name"`
				} `json:"source"`
			}
			json.Unmarshal([]byte(cands[i]), &src)
			if want := native[src.Source.CardName]; !slices.Equal(taps, want) {
				t.Fatalf("cast %s after tapping %v, but gorge's plan taps %v", src.Source.CardName, taps, want)
			}
			if a.Fallbacks() != 0 {
				t.Fatalf("%d fallbacks while paying", a.Fallbacks())
			}
			t.Logf("cast %s after tapping %v", src.Source.CardName, taps)
			return
		default:
			t.Fatalf("the seat answered %s after tapping %v: it never cast", sem.Kind, taps)
		}
		if step > 6 {
			t.Fatalf("no cast after %d taps", len(taps))
		}
	}
}

// TestNeutralPaymentFallbackCounted: the only source of red mana is a creature
// sacrificed for it, a plan the lowering does not run. The seat takes the first
// mana candidate, the fallback is counted once, and nothing errors.
func TestNeutralPaymentFallbackCounted(t *testing.T) {
	reg := testcorpus.Registry(t)
	b := payBoard{lands: []string{"Wild Cantor"}, hand: []string{"Lightning Bolt"}, tapped: map[string]bool{}}
	var cands []string
	cands = append(cands, `{"kind":"pass"}`)
	for _, col := range []string{"W", "U", "B", "R", "G"} {
		cands = append(cands, `{"kind":"activate_mana_ability","source":`+ref0("battlefield", "Wild Cantor")+`,"ability_index":0,"mana_choice":"`+col+`","cost_target":null}`)
	}
	var q struct {
		Decision protocol.SeatDecision `json:"decision"`
	}
	if err := json.Unmarshal([]byte(chooseLine(2, b, cands)), &q); err != nil {
		t.Fatal(err)
	}
	s := neutral.NewSession(reg)
	p, err := s.Payload(&q.Decision)
	if err != nil {
		t.Fatal(err)
	}
	var sel *decision.PaymentSelection
	for _, a := range p.Decision.PaymentActions {
		if a.Plans[0].Activations[0].Consequence == nil || !a.Plans[0].Activations[0].Consequence.Sacrifice {
			t.Fatalf("precondition: the plan should sacrifice the Cantor: %+v", a.Plans[0])
		}
		sel = &decision.PaymentSelection{ActionID: a.ID, Plan: decision.ClonePaymentPlan(a.Plans[0])}
	}
	if sel == nil {
		t.Fatalf("no payment action offered: %+v", s.Counts)
	}
	first := -1
	for i, op := range p.Ops {
		if len(op.Pays) > 0 {
			first = i
		}
	}
	if first != 1 {
		t.Fatalf("the fallback should be the first mana candidate (1), got %d (ops %+v)", first, p.Ops)
	}
	// The agent took it and told the session which plan the bot meant.
	s.Picked(first)
	s.PickedPayment(sel)
	if s.Counts["pay_fallback"] != 1 || s.Counts["pay_abort"] != 0 {
		t.Fatalf("counts %+v", s.Counts)
	}
}
