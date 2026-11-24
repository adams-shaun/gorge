package gorgebe

import (
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func vc(n int32) *ir.Value { return &ir.Value{Kind: ir.VConst, N: n} }

func vk(k ir.ValueKind, args ...*ir.Value) *ir.Value { return &ir.Value{Kind: k, Args: args} }

func vref(k ir.ValueKind, r ir.RefKind) *ir.Value { return &ir.Value{Kind: k, Ref: &ir.Ref{Kind: r}} }

var vX = &ir.Value{Kind: ir.VX}

type valueTestCase struct {
	name string
	v    *ir.Value
	ev   ir.EventKind
	body string // "" when the value is refused
	what string // the refusal's What
}

// valueTestPlayerFrame is a frame whose one target slot selects players.
func valueTestPlayerFrame() *frame {
	return &frame{targets: []*ir.Target{{Filter: &ir.Filter{Subject: ir.SubjPlayer}}}, declared: []bool{false}}
}

func valueTestCases() []valueTestCase {
	selfCounters := &ir.Filter{Op: ir.FPred, Pred: ir.PIs, Ref: &ir.Ref{Kind: ir.RefSelf}}
	youAll := &ir.Ref{Kind: ir.RefAll, Filter: &ir.Filter{Subject: ir.SubjPlayer, Op: ir.FPred, Pred: ir.PPlayerIs, Ref: &ir.Ref{Kind: ir.RefYou}}}
	dev := func(c ir.ColorSet) *ir.Value {
		return &ir.Value{Kind: ir.VDevotion, Ref: &ir.Ref{Kind: ir.RefYou}, Colors: c}
	}
	return []valueTestCase{
		{name: "x", v: vX, body: "Count$xPaid"},
		{name: "fold add", v: vk(ir.VAdd, vc(2), vc(3)), body: "5"},
		{name: "fold sub", v: vk(ir.VSub, vc(2), vc(5)), body: "-3"},
		{name: "fold mul", v: vk(ir.VMul, vc(2), vc(3)), body: "6"},
		{name: "add right", v: vk(ir.VAdd, vX, vc(2)), body: "Count$xPaid/Plus.2"},
		{name: "add left", v: vk(ir.VAdd, vc(2), vX), body: "Count$xPaid/Plus.2"},
		{name: "sub", v: vk(ir.VSub, vX, vc(1)), body: "Count$xPaid/Minus.1"},
		{name: "sub from const", v: vk(ir.VSub, vc(1), vX), what: "value:sub"},
		{name: "mul right", v: vk(ir.VMul, vX, vc(2)), body: "Count$xPaid/Times.2"},
		{name: "mul left", v: vk(ir.VMul, vc(3), vX), body: "Count$xPaid/Times.3"},
		{name: "half down", v: vk(ir.VHalfDown, vX), body: "Count$xPaid/HalfDown"},
		{name: "half up", v: vk(ir.VHalfUp, vX), body: "Count$xPaid/HalfUp"},
		{name: "nested", v: vk(ir.VHalfUp, vk(ir.VAdd, vX, vc(1))), what: "value:half-up-nested"},
		{name: "nested mul", v: vk(ir.VMul, vk(ir.VMul, vX, vc(2)), vc(2)), what: "value:mul-nested"},
		{name: "half of literal", v: vk(ir.VHalfDown, vc(4)), what: "value:half-down-nested"},
		{name: "counters on self", v: &ir.Value{Kind: ir.VCountersAmong, Filter: selfCounters, Word: "P1P1"}, body: "Count$CardCounters.P1P1"},
		{name: "counters elsewhere", v: &ir.Value{Kind: ir.VCountersAmong, Filter: &ir.Filter{Op: ir.FPred, Pred: ir.PAny}, Word: "P1P1"}, what: "value:counters-among"},
		{name: "power self", v: vref(ir.VPower, ir.RefSelf), body: "Count$CardPower"},
		{name: "toughness self", v: vref(ir.VToughness, ir.RefSelf), body: "Count$CardToughness"},
		{name: "mana value self", v: vref(ir.VManaValue, ir.RefSelf), body: "Count$CardManaCost"},
		{name: "power target", v: vref(ir.VPower, ir.RefTarget), body: "Targeted$CardPower"},
		{name: "toughness target", v: vref(ir.VToughness, ir.RefTarget), body: "Targeted$CardToughness"},
		{name: "mana value target", v: vref(ir.VManaValue, ir.RefTarget), body: "Targeted$CardManaCost"},
		{name: "power event object", v: vref(ir.VPower, ir.RefEventObject), what: "value:power"},
		{name: "power triggering card", v: vref(ir.VPower, ir.RefEventObject), ev: ir.EvEnters, body: "TriggeredCard$CardPower"},
		{name: "toughness dying card", v: vref(ir.VToughness, ir.RefEventObject), ev: ir.EvDies, body: "TriggeredCard$CardToughness"},
		{name: "distinct mana values", v: &ir.Value{Kind: ir.VDistinctManaValues, Filter: &ir.Filter{Subject: ir.SubjPermanent, Op: ir.FPred, Pred: ir.PCardType, Word: "Creature"}},
			body: "Count$Valid Creature$DifferentCardManaCost"},
		{name: "life you", v: vref(ir.VLife, ir.RefYou), body: "Count$YourLifeTotal"},
		{name: "life all you", v: &ir.Value{Kind: ir.VLife, Ref: youAll}, body: "Count$YourLifeTotal"},
		{name: "life target player", v: vref(ir.VLife, ir.RefTarget), body: "TargetedPlayer$LifeTotal"},
		{name: "life active player", v: vref(ir.VLife, ir.RefActivePlayer), what: "value:life"},
		{name: "starting life", v: vref(ir.VStartingLife, ir.RefYou), body: "Count$YourStartingLife"},
		{name: "life gained", v: vref(ir.VLifeGainedThisTurn, ir.RefYou), body: "Count$LifeYouGainedThisTurn"},
		{name: "devotion white", v: dev(ir.ColorW), body: "Count$Devotion.White"},
		{name: "devotion green", v: dev(ir.ColorG), body: "Count$Devotion.Green"},
		{name: "devotion dual", v: dev(ir.ColorG | ir.ColorU), body: "Count$DevotionDual.Blue.Green"},
		{name: "devotion three", v: dev(ir.ColorW | ir.ColorU | ir.ColorB), what: "value:devotion"},
		{name: "chosen number", v: &ir.Value{Kind: ir.VChosenNumber}, body: "Count$ChosenNumber"},
		{name: "event gains", v: &ir.Value{Kind: ir.VEventAmount}, ev: ir.EvGainsLife, body: "TriggerCount$LifeAmount"},
		{name: "event loses", v: &ir.Value{Kind: ir.VEventAmount}, ev: ir.EvLosesLife, body: "TriggerCount$LifeAmount"},
		{name: "event combat", v: &ir.Value{Kind: ir.VEventAmount}, ev: ir.EvDealsCombatDamage, body: "TriggerCount$DamageAmount"},
		{name: "event combat player", v: &ir.Value{Kind: ir.VEventAmount}, ev: ir.EvDealsCombatDamageToPlayer, body: "TriggerCount$DamageAmount"},
		{name: "event noncombat", v: &ir.Value{Kind: ir.VEventAmount}, ev: ir.EvDealsNoncombatDamageToPlayer, body: "TriggerCount$DamageAmount"},
		{name: "event would damage", v: &ir.Value{Kind: ir.VEventAmount}, ev: ir.EvWouldDealDamage, body: "ReplaceCount$DamageAmount"},
		{name: "event counters", v: &ir.Value{Kind: ir.VEventAmount}, ev: ir.EvCountersPut, body: "TriggerCount$Amount"},
		{name: "event other", v: &ir.Value{Kind: ir.VEventAmount}, ev: ir.EvDies, what: "value:event-amount@dies"},
		{name: "receipt", v: &ir.Value{Kind: ir.VReceiptAmount}, what: "value:receipt-amount"},
		{name: "total power", v: &ir.Value{Kind: ir.VTotalPower}, what: "value:total-power"},
	}
}

func valueTestRun(c valueTestCase) (*pr, string, error) {
	p := &pr{ev: c.ev, frames: []*frame{valueTestPlayerFrame()}}
	b, err := p.body(c.v)
	return p, b, err
}

func TestValueBodies(t *testing.T) {
	for _, c := range valueTestCases() {
		_, b, err := valueTestRun(c)
		if c.what != "" {
			var u *Unprintable
			if !errors.As(err, &u) || u.What != c.what {
				t.Errorf("%s: got %q, %v; want Unprintable %s", c.name, b, err, c.what)
			}
			continue
		}
		if err != nil || b != c.body {
			t.Errorf("%s: got %q, %v; want %q", c.name, b, err, c.body)
		}
	}

	// p.amount: literals stay literal; bodies become SVars, named by first use.
	p := &pr{}
	for i, w := range []struct {
		v    *ir.Value
		want string
	}{
		{vX, "X"},
		{vref(ir.VPower, ir.RefSelf), "N"},
		{vref(ir.VToughness, ir.RefSelf), "N2"},
		{vref(ir.VPower, ir.RefSelf), "N"},
		{vX, "X"},
		{vk(ir.VAdd, vc(1), vc(2)), "3"},
		{vc(4), "4"},
	} {
		got, err := p.amount(w.v)
		if err != nil || got != w.want {
			t.Errorf("amount %d: got %q, %v; want %q", i, got, err, w.want)
		}
	}
	want := []kv{{"X", "Count$xPaid"}, {"N", "Count$CardPower"}, {"N2", "Count$CardToughness"}}
	if len(p.svars) != len(want) {
		t.Fatalf("svars %v, want %v", p.svars, want)
	}
	for i := range want {
		if p.svars[i] != want[i] {
			t.Errorf("svar %d: %v, want %v", i, p.svars[i], want[i])
		}
	}
}

func TestValueHeadsModelled(t *testing.T) {
	bodies := map[string]bool{}
	for _, c := range valueTestCases() {
		if _, b, err := valueTestRun(c); err == nil {
			bodies[b] = true
		}
	}
	for _, c := range condTestCases() {
		p, _, err := condTestRun(c)
		if err != nil {
			continue
		}
		for _, s := range p.svars {
			bodies[s.V] = true
		}
	}
	supported := effects.Supported()
	var missing []string
	n := 0
	for b := range bodies {
		head, ok := cards.ValueHead(b)
		if !ok {
			continue
		}
		n++
		if !supported["count:"+head] {
			missing = append(missing, head+" (body "+b+")")
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("unmodelled value heads:\n%s", strings.Join(missing, "\n"))
	}
	if n == 0 {
		t.Error("no classified bodies checked")
	}
}

// corpusSVarBodies is the set of every SVar value on every face of the corpus.
func corpusSVarBodies(t *testing.T) map[string]bool {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	out := map[string]bool{}
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			for _, v := range f.SVars {
				out[strings.TrimSpace(v)] = true
			}
		}
	}
	return out
}

func TestBodiesAttested(t *testing.T) {
	corpus := corpusSVarBodies(t)
	bodies := map[string]bool{}
	for _, c := range valueTestCases() {
		if _, b, err := valueTestRun(c); err == nil {
			bodies[b] = true
		}
	}
	for _, c := range condTestCases() {
		if p, _, err := condTestRun(c); err == nil {
			for _, s := range p.svars {
				bodies[s.V] = true
			}
		}
	}
	checked := 0
	for b := range bodies {
		if isLiteral(b) {
			continue
		}
		if _, ok := cards.ValueHead(b); ok {
			continue
		}
		checked++
		if !corpus[b] {
			t.Errorf("body %q is classified by no head and appears in no corpus SVar", b)
		}
	}
	t.Logf("%d unclassified bodies attested", checked)
}
