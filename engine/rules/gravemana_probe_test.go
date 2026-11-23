package rules

// Probe of gorge's graveyard-zone mana ability (Jack-o'-Lantern: "{1}, Exile
// Jack-o'-Lantern from your graveyard: Add one mana of any color",
// CR 605.1a, 605.2a) for the dirac scoping ticket. The three questions:
//
//  1. is the graveyard source ever a raw priority "activate" option, or only
//     a payment-window option;
//  2. does the prefix search (scriptSearch.activations) fund through it, its
//     {1} fee paid by mana an earlier activation floated;
//  3. where does it sit in a payment window (after the battlefield and the
//     hand), and is it withheld when its colour is not absorbed.
//
// Each scenario builds its own board, records the real option lists with
// t.Logf (go test -v prints them) and asserts the facts the report states.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const gravePlains = "Name:Plains\nTypes:Basic Land Plains\nOracle:x\n"

// graveScarecrow is a fee-only filter ({2}: Add one mana of any color), Signpost
// Scarecrow's mana ability.
const graveScarecrow = "Name:Grave Scarecrow\nTypes:Artifact Creature Scarecrow\nPT:2/4\nA:AB$ Mana | Cost$ 2 | Produced$ Any\nOracle:x\n"

const graveProbeSpell = "Name:Grave Probe Spell\nManaCost:%s\nTypes:Instant\nA:SP$ Draw | Num$ 1\nOracle:x\n"

// graveProbeBoard is one scenario: seat 0 holds the spell (cost) in hand,
// Jack-o'-Lantern in the graveyard, the named hand guides, the lands on the
// battlefield and the pool floating.
type graveProbeBoard struct {
	e     *Engine
	jack  state.ObjID
	spell state.ObjID
}

func newGraveProbe(t *testing.T, cost string, lands []string, guides int, pool string) *graveProbeBoard {
	t.Helper()
	corpus := []string{"Jack-o'-Lantern"}
	for range guides {
		corpus = append(corpus, "Simian Spirit Guide")
	}
	e, _, _ := altCostEngine(t, 4417, corpus, []string{fmt.Sprintf(graveProbeSpell, cost)}, nil)
	b := &graveProbeBoard{e: e}
	b.jack = findCardObj(t, e, 0, "Jack-o'-Lantern", state.ZGraveyard)
	b.spell = findCardObj(t, e, 0, "Grave Probe Spell", state.ZHand)
	for range guides {
		findAndMoveToHand(t, e, 0, "Simian Spirit Guide")
	}
	for _, l := range lands {
		id := onBoard(t, e, 0, l)
		e.G.Obj(id).SummonSick = false
	}
	if pool != "" {
		addMana(t, e, 0, pool)
	} else {
		e.pending = nil
		e.askPriority(0)
	}
	if o := e.G.Obj(b.jack); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: Jack-o'-Lantern not in the graveyard: %+v", o)
	}
	if o := e.G.Obj(b.spell); o == nil || o.Zone != state.ZHand {
		t.Fatalf("precondition: the probe spell not in hand: %+v", o)
	}
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority {
		t.Fatalf("precondition: no priority decision: %+v", d)
	}
	return b
}

func (b *graveProbeBoard) name(o decision.Option) string {
	if o.Obj == 0 {
		return ""
	}
	if ob := b.e.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
		return ob.Face().Name + "@" + ob.Zone.String()
	}
	return "?"
}

// list renders options as "kind:object" in order.
func (b *graveProbeBoard) list(opts []decision.Option) string {
	var parts []string
	for _, o := range opts {
		parts = append(parts, o.Kind+":"+b.name(o))
	}
	return "[" + strings.Join(parts, " ") + "]"
}

// rawActivates is the priority decision's activate options.
func (b *graveProbeBoard) rawActivates() []decision.Option {
	var out []decision.Option
	for _, o := range b.e.Pending().Options {
		if o.Kind == "activate" {
			out = append(out, o)
		}
	}
	return out
}

func (b *graveProbeBoard) offers(opts []decision.Option, kind string, obj state.ObjID) bool {
	for _, o := range opts {
		if o.Kind == kind && o.Obj == obj {
			return true
		}
	}
	return false
}

// potentialCasts is the potential walk's cast options for the spell.
func (b *graveProbeBoard) potentialCast() bool {
	_, opts := b.e.potentialWalkOf(0, true)
	return b.offers(opts, "cast", b.spell)
}

// window announces the spell and returns the payment window's options, or
// nil and what happened instead when no window opened (the pool paid it, or
// the cast was not announced).
func (b *graveProbeBoard) window(t *testing.T) ([]decision.Option, string) {
	t.Helper()
	d := b.e.Pending()
	var act *decision.PaymentAction
	for _, a := range b.e.EnsurePaymentActions() {
		if a.Cast.Object == b.spell {
			a := a
			act = &a
		}
	}
	if act == nil {
		return nil, "no payment action for the spell"
	}
	if err := b.e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Announce: &decision.AnnounceSelection{ActionID: act.ID}}); err != nil {
		t.Fatalf("Submit announce: %v", err)
	}
	w := b.e.Pending()
	if w == nil || w.Kind != decision.KChoose || w.ManaPayment == nil {
		if o := b.e.G.Obj(b.spell); o != nil && o.Zone == state.ZStack {
			return nil, "spell on the stack (paid with no window)"
		}
		return nil, "no window; pending kind " + string(w.Kind)
	}
	return w.Options, ""
}

func indexOf(opts []decision.Option, obj state.ObjID) int {
	for _, o := range opts {
		if o.Kind == "mana" && o.Obj == obj {
			return o.Index
		}
	}
	return -1
}

// TestGraveManaGorgeProbe records and pins gorge's behaviour for a graveyard
// mana ability whose {1} fee is paid from the floating pool.
func TestGraveManaGorgeProbe(t *testing.T) {
	t.Parallel()

	// Q1: raw priority "activate" options.
	t.Run("Q1 priority offer", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name  string
			lands []string
			pool  string
			want  bool
		}{
			{"nothing floating, no land", nil, "", false},
			{"one {R} floating", nil, "R", true},
			{"untapped Mountain, nothing floating", []string{apMountain}, "", false},
		} {
			b := newGraveProbe(t, "W", tc.lands, 0, tc.pool)
			act := b.rawActivates()
			got := b.offers(act, "activate", b.jack)
			t.Logf("Q1 %-28s raw activates %s -> Jack offered=%v", tc.name, b.list(act), got)
			if got != tc.want {
				t.Errorf("Q1 %s: Jack activate offered = %v, want %v", tc.name, got, tc.want)
			}
		}
	})

	// Q1b: the raw activate order of a hand guide and the graveyard source (the
	// kernel's funding passes follow it: hand guides, then the graveyard).
	t.Run("Q1b guide and Jack order", func(t *testing.T) {
		t.Parallel()
		b := newGraveProbe(t, "W", nil, 1, "R")
		act := b.rawActivates()
		t.Logf("Q1b hand Simian + graveyard Jack, {R} floating: raw activates %s", b.list(act))
		if len(act) != 2 || b.name(act[0]) != "Simian Spirit Guide@hand" || act[1].Obj != b.jack {
			t.Errorf("Q1b: raw activates %s, want the hand guide then Jack", b.list(act))
		}
	})

	// Q2b: the scripts beside a fee-only filter (Signpost Scarecrow, {2}: one mana of
	// any color). Measured: Jack is in the script only where the filter is no
	// live census gap (its {2} fee out of the potential pool's reach, so the
	// full search runs); a live gap makes the relaxed planner offer the cast
	// early, and the script ends before Jack (the payment window takes it,
	// Q3). Jack's answer is the colour the cost needs.
	t.Run("Q2b filter gap", func(t *testing.T) {
		t.Parallel()
		m, sc := apMountain, graveScarecrow
		const (
			mtn  = "priority(activate:Mountain@battlefield:)"
			jack = "priority(activate:Jack-o'-Lantern@graveyard:)"
			ask  = "choose(mana:Jack-o'-Lantern@graveyard:%s)"
		)
		for _, tc := range []struct {
			name  string
			cost  string
			lands []string
			pool  string
			want  []string
		}{
			// the Scarecrow's {2} fee is out of the one Mountain's reach: no live gap, Jack funds
			{"Mountain + Scarecrow, {W}", "W", []string{m, sc}, "", []string{mtn, jack, fmt.Sprintf(ask, "W")}},
			// the colour answered is the one the cost needs, not the first of W U B R G
			{"Mountain + Scarecrow, {U}", "U", []string{m, sc}, "", []string{mtn, jack, fmt.Sprintf(ask, "U")}},
			{"Mountain + Scarecrow, {G}", "G", []string{m, sc}, "", []string{mtn, jack, fmt.Sprintf(ask, "G")}},
			// {R} already floating: the first step drops (Jack is never the first step of an empty pool)
			{"Scarecrow, {R} floating, {W}", "W", []string{sc}, "R", []string{jack, fmt.Sprintf(ask, "W")}},
			// a live gap: the planner offers the cast after one tap and the script ends there
			{"3 Mountain + Scarecrow, {W}{W}", "WW", []string{m, m, m, sc}, "", []string{mtn}},
		} {
			b := newGraveProbe(t, tc.cost, tc.lands, 0, tc.pool)
			if live := len(tc.want) == 1; b.potentialCast() != live {
				t.Errorf("Q2b %s: the planner's offer of the cast is %v, want %v", tc.name, !live, live)
			}
			script, why := b.e.PotentialPlayScript(0, decision.PotentialAction{Kind: "cast", Obj: b.spell}, potentialScriptNodes)
			var steps []string
			for _, st := range script {
				var picks []string
				for _, p := range st.Picks {
					picks = append(picks, fmt.Sprintf("%s:%s:%s", p.Kind, b.name(decision.Option{Obj: p.Obj}), p.ManaSymbol))
				}
				steps = append(steps, string(st.Kind)+"("+strings.Join(picks, ",")+")")
			}
			t.Logf("Q2b %-44s steps=%v reason=%q", tc.name, steps, why)
			if why != "" || strings.Join(steps, " ") != strings.Join(tc.want, " ") {
				t.Errorf("Q2b %s: script %v (reason %q), want %v", tc.name, steps, why, tc.want)
			}
		}
	})

	// Q2: the prefix search funds through the graveyard source.
	t.Run("Q2 prefix search", func(t *testing.T) {
		t.Parallel()
		// {W} is payable only as Mountain (R floats) -> Jack ({1} from the R, adds W).
		b := newGraveProbe(t, "W", []string{apMountain}, 0, "")
		got := b.potentialCast()
		t.Logf("Q2 Mountain + graveyard Jack, spell {W}: potential walk offers the cast = %v", got)
		if got {
			t.Errorf("Q2: the planner priced a graveyard source (the cast is offered); the kernel's census assumption fails")
		}
		ctl := newGraveProbe(t, "R", []string{apMountain}, 0, "")
		t.Logf("Q2 control Mountain, spell {R}: potential walk offers the cast = %v", ctl.potentialCast())
		if !ctl.potentialCast() {
			t.Errorf("Q2 control: the {R} spell is not offered with a Mountain")
		}
		none := newGraveProbe(t, "W", nil, 0, "")
		t.Logf("Q2 Jack alone (no land), spell {W}: potential walk offers the cast = %v", none.potentialCast())
		fl := newGraveProbe(t, "W", nil, 0, "R")
		t.Logf("Q2 Jack with {R} floating, spell {W}: potential walk offers the cast = %v", fl.potentialCast())
		_, opts := fl.e.potentialWalkOf(0, true)
		t.Logf("Q2 floating-R potential walk options: %s", fl.list(opts))
		for _, sc := range []struct {
			name string
			b    *graveProbeBoard
		}{
			{"Mountain + Jack, spell {W}", b},
			{"Jack alone, spell {W}", none},
			{"Jack with {R} floating, spell {W}", fl},
		} {
			script, why := sc.b.e.PotentialPlayScript(0, decision.PotentialAction{Kind: "cast", Obj: sc.b.spell}, potentialScriptNodes)
			var steps []string
			for _, st := range script {
				steps = append(steps, fmt.Sprintf("%s%+v", st.Kind, st.Picks))
			}
			t.Logf("Q2 prefix script %-36s steps=%v reason=%q", sc.name, steps, why)
			wantFound := sc.b != none
			if (why == "") != wantFound {
				t.Errorf("Q2 %s: script reason %q, want found=%v", sc.name, why, wantFound)
			}
			if wantFound {
				n := len(script)
				if n < 2 || script[n-2].Kind != decision.KPriority || script[n-1].Kind != decision.KChoose {
					t.Errorf("Q2 %s: script %v does not end with Jack's raw activate then its colour ask", sc.name, steps)
					continue
				}
				if p := script[n-2].Picks; len(p) != 1 || p[0].Kind != "activate" || p[0].Obj != sc.b.jack {
					t.Errorf("Q2 %s: the second-last step is %+v, want Jack's activate", sc.name, p)
				}
				if p := script[n-1].Picks; len(p) != 1 || p[0].Kind != "mana" || p[0].ManaSymbol != "W" {
					t.Errorf("Q2 %s: the last step is %+v, want the colour ask answered W", sc.name, p)
				}
			}
		}
	})

	// Q3: the payment window.
	t.Run("Q3 window", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name   string
			cost   string
			lands  []string
			guides int
			pool   string
		}{
			{"{W}: Plains + Mountain + Simian + Jack, empty pool", "W", []string{gravePlains, apMountain}, 1, ""},
			{"{W}: Plains + Mountain + Simian + Jack, R floating", "W", []string{gravePlains, apMountain}, 1, "R"},
			{"{1}{W}: Plains + Mountain + Jack, R floating", "1W", []string{gravePlains, apMountain}, 0, "R"},
			{"{1}{W}: Plains + Mountain + Jack, empty pool", "1W", []string{gravePlains, apMountain}, 0, ""},
			{"{R}: Mountain + Simian + Jack, empty pool", "R", []string{apMountain}, 1, ""},
			{"{R}{R}: Mountain + Simian + Jack, empty pool", "RR", []string{apMountain}, 1, ""},
		} {
			b := newGraveProbe(t, tc.cost, tc.lands, tc.guides, tc.pool)
			w, why := b.window(t)
			if w == nil {
				t.Logf("Q3 %-48s %s", tc.name, why)
				continue
			}
			listed := b.offers(w, "mana", b.jack)
			t.Logf("Q3 %-48s window %s jackListed=%v", tc.name, b.list(w), listed)
			if listed != (tc.pool != "" && tc.cost == "W") {
				t.Errorf("Q3 %s: Jack listed = %v, want it listed exactly when an unabsorbed mana floats", tc.name, listed)
			}
			if listed {
				// the window's answer is the first option listed: Jack follows every battlefield and hand source
				for _, o := range w {
					if o.Kind == "mana" && o.Obj != b.jack && o.Index > indexOf(w, b.jack) {
						t.Errorf("Q3 %s: %s listed after Jack", tc.name, b.name(o))
					}
				}
			}
		}
	})

	// Q4: the asks one activation of Jack opens at priority, by what floats.
	t.Run("Q4 activation asks", func(t *testing.T) {
		t.Parallel()
		for _, pool := range []string{"R", "RG", "RR", "GGU"} {
			b := newGraveProbe(t, "W", nil, 0, pool)
			before := b.e.G.Players[0].Pool
			d := b.e.Pending()
			idx := -1
			for _, o := range d.Options {
				if o.Kind == "activate" && o.Obj == b.jack {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("Q4 pool %s: Jack not offered at priority: %s", pool, b.list(d.Options))
			}
			var asks []string
			submitChoices(t, b.e, idx)
			for n := 0; n < 8; n++ {
				d := b.e.Pending()
				if d == nil || d.Kind == decision.KPriority {
					break
				}
				var labels []string
				for _, o := range d.Options {
					labels = append(labels, o.Kind+"/"+o.Label)
				}
				asks = append(asks, fmt.Sprintf("%s %q %v", d.Kind, d.Prompt, labels))
				submitChoices(t, b.e, 0)
			}
			after := b.e.G.Players[0].Pool
			t.Logf("Q4 pool %-4s before=%v asks=%v after pool=%v jack=%s", pool, before, asks, after, b.e.G.Obj(b.jack).Zone)
			if z := b.e.G.Obj(b.jack).Zone; z != state.ZExile {
				t.Errorf("Q4 pool %s: Jack is in %s, want exile (the CARDNAME exile settles unasked)", pool, z)
			}
			if len(asks) != 1 || !strings.HasPrefix(asks[0], "choose \"Choose a colour of mana\"") {
				t.Errorf("Q4 pool %s: asks %v, want exactly the colour ask (no fee pick, no exile pick)", pool, asks)
			}
			// the fee takes one mana from the pool in W U B R G order; the colour ask's first answer (W) is added
			want := before
			for i := range want {
				if want[i] > 0 {
					want[i]--
					break
				}
			}
			want[0]++
			if after != want {
				t.Errorf("Q4 pool %s: pool after = %v, want %v (fee from the first non-empty colour in WUBRGC order, then W)", pool, after, want)
			}
		}
	})
}
