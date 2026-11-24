package gorgebe

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
	"github.com/adams-shaun/gorge/engine/cards"
)

// --- builders ---

func apStep(v ir.Verb, mod func(*ir.Step)) *ir.Step {
	s := &ir.Step{Verb: v}
	if mod != nil {
		mod(s)
	}
	return s
}

// apRun prints steps as one program with the given targets.
func apRun(tokens map[string]*cards.Card, tgts []*ir.Target, steps ...*ir.Step) (*node, *pr, error) {
	p := &pr{tokens: tokens}
	n, err := p.program(&ir.Program{Steps: steps, Targets: tgts})
	return n, p, err
}

func kvs(s ...string) []kv {
	var out []kv
	for i := 0; i+1 < len(s); i += 2 {
		out = append(out, kv{s[i], s[i+1]})
	}
	return out
}

func apWant(t *testing.T, n *node, api string, params []kv) {
	t.Helper()
	if n == nil {
		t.Fatal("no node")
	}
	if n.API != api || !reflect.DeepEqual(n.Params, params) {
		t.Errorf("got %s %v\nwant %s %v", n.API, n.Params, api, params)
	}
}

func apSVar(p *pr, k string) string {
	for _, s := range p.svars {
		if s.K == k {
			return s.V
		}
	}
	return "<none>"
}

var (
	apYou    = &ir.Ref{Kind: ir.RefYou}
	apSelf   = &ir.Ref{Kind: ir.RefSelf}
	apTgt0   = &ir.Ref{Kind: ir.RefTarget}
	apOppAll = &ir.Ref{Kind: ir.RefAll, Filter: refTestOpp()}
	apHand   = ir.MakeZoneSet(ir.ZoneHand)
)

func apHandFilter() *ir.Filter { return &ir.Filter{Subject: ir.SubjCard, Zones: apHand} }

func apAnyTarget() *ir.Target {
	return &ir.Target{Min: 1, Max: 1, Filter: &ir.Filter{Op: ir.FAny, Args: []*ir.Filter{
		refTestWord(ir.SubjPermanent, ir.PCardType, "Creature"),
		refTestWord(ir.SubjPermanent, ir.PCardType, "Planeswalker"),
		refTestWord(ir.SubjPermanent, ir.PCardType, "Battle"),
		refTestPred(ir.SubjPlayer, ir.PAny, nil),
	}}}
}

func apSpellTarget() *ir.Target {
	return &ir.Target{Min: 1, Max: 1, Filter: refTestPred(ir.SubjSpell, ir.PAny, nil)}
}

// --- tests ---

func TestStepNodes(t *testing.T) {
	pl := []*ir.Target{refTestPlayerTarget()}
	type tc struct {
		name  string
		tgts  []*ir.Target
		step  *ir.Step
		api   string
		want  []kv
		bad   string
		svars []kv
	}
	n := func(k int32) func(*ir.Step) { return func(s *ir.Step) { s.N = refTestVal(k) } }
	cases := []tc{
		{name: "draw", step: apStep(ir.VerbDraw, n(2)), api: "Draw", want: kvs("NumCards", "2")},
		{name: "draw who you", step: apStep(ir.VerbDraw, func(s *ir.Step) { s.N = refTestVal(1); s.Who = apYou }),
			api: "Draw", want: kvs("NumCards", "1", "Defined", "You")},
		{name: "draw target player", tgts: pl, step: apStep(ir.VerbDraw, func(s *ir.Step) { s.N = refTestVal(1); s.Who = apTgt0 }),
			api: "Draw", want: kvs("NumCards", "1", "ValidTgts", "Player")},
		{name: "draw optional", step: apStep(ir.VerbDraw, func(s *ir.Step) { s.N = refTestVal(1); s.Optional = true }),
			api: "Draw", want: kvs("NumCards", "1", "OptionalDecider", "You")},
		{name: "draw optional with who", step: apStep(ir.VerbDraw, func(s *ir.Step) { s.N = refTestVal(1); s.Optional = true; s.Who = apYou }),
			bad: "optional:draw"},
		{name: "draw X", step: apStep(ir.VerbDraw, func(s *ir.Step) { s.N = &ir.Value{Kind: ir.VX} }),
			api: "Draw", want: kvs("NumCards", "X"), svars: kvs("X", "Count$xPaid")},
		{name: "draw no amount", step: apStep(ir.VerbDraw, nil), bad: "value:nil"},
		{name: "gain life", step: apStep(ir.VerbGainLife, n(3)), api: "GainLife", want: kvs("LifeAmount", "3")},
		{name: "lose life target", tgts: pl, step: apStep(ir.VerbLoseLife, func(s *ir.Step) { s.N = refTestVal(2); s.Who = apTgt0 }),
			api: "LoseLife", want: kvs("LifeAmount", "2", "ValidTgts", "Player")},
		{name: "mill", step: apStep(ir.VerbMill, func(s *ir.Step) { s.N = refTestVal(4); s.Who = apOppAll }),
			api: "Mill", want: kvs("NumCards", "4", "Defined", "Player.Opponent")},
		{name: "surveil", step: apStep(ir.VerbSurveil, n(2)), api: "Surveil", want: kvs("Amount", "2")},
		{name: "extra turn", step: apStep(ir.VerbExtraTurn, nil), api: "AddTurn", want: kvs("NumTurns", "1")},
		{name: "extra turn who", step: apStep(ir.VerbExtraTurn, func(s *ir.Step) { s.Who = apYou }),
			api: "AddTurn", want: kvs("NumTurns", "1", "Defined", "You")},
		{name: "end turn", step: apStep(ir.VerbEndTurn, nil), api: "EndTurn"},
		{name: "discard choose", step: apStep(ir.VerbDiscard, func(s *ir.Step) {
			s.Obj = &ir.Ref{Kind: ir.RefChoose, Filter: apHandFilter(), N: refTestVal(1)}
		}), api: "Discard", want: kvs("NumCards", "1", "Mode", "TgtChoose")},
		{name: "discard choose target player", tgts: pl, step: apStep(ir.VerbDiscard, func(s *ir.Step) {
			s.Who = apTgt0
			s.Obj = &ir.Ref{Kind: ir.RefChoose, Filter: apHandFilter(), N: refTestVal(2)}
		}), api: "Discard", want: kvs("NumCards", "2", "Mode", "TgtChoose", "ValidTgts", "Player")},
		{name: "discard hand", step: apStep(ir.VerbDiscard, func(s *ir.Step) {
			s.Obj = &ir.Ref{Kind: ir.RefAll, Filter: apHandFilter()}
		}), api: "Discard", want: kvs("Mode", "Hand")},
		{name: "discard chosen by another", step: apStep(ir.VerbDiscard, func(s *ir.Step) {
			s.Obj = &ir.Ref{Kind: ir.RefChoose, Of: apOppAll, Filter: apHandFilter(), N: refTestVal(1)}
		}), bad: "verb:discard"},
		{name: "discard not from hand", step: apStep(ir.VerbDiscard, func(s *ir.Step) {
			s.Obj = &ir.Ref{Kind: ir.RefAll, Filter: &ir.Filter{Subject: ir.SubjCard, Zones: ir.MakeZoneSet(ir.ZoneGraveyard)}}
		}), bad: "verb:discard"},
		{name: "discard restricted hand", step: apStep(ir.VerbDiscard, func(s *ir.Step) {
			f := refTestWord(ir.SubjCard, ir.PCardType, "Land")
			f.Zones = apHand
			s.Obj = &ir.Ref{Kind: ir.RefChoose, Filter: f, N: refTestVal(1)}
		}), bad: "verb:discard"},
		{name: "damage any target", tgts: []*ir.Target{apAnyTarget()}, step: apStep(ir.VerbDealDamage, func(s *ir.Step) {
			s.N = refTestVal(3)
			s.Obj = apTgt0
		}), api: "DealDamage", want: kvs("NumDmg", "3", "ValidTgts", "Any")},
		{name: "damage opponents", step: apStep(ir.VerbDealDamage, func(s *ir.Step) { s.N = refTestVal(2); s.Obj = apOppAll; s.Source = apSelf }),
			api: "DealDamage", want: kvs("NumDmg", "2", "Defined", "Player.Opponent")},
		{name: "damage other source", step: apStep(ir.VerbDealDamage, func(s *ir.Step) {
			s.N = refTestVal(2)
			s.Obj = apOppAll
			s.Source = apYou
		}), api: "DealDamage", want: kvs("NumDmg", "2", "Defined", "Player.Opponent", "DamageSource", "You")},
		{name: "damage who", step: apStep(ir.VerbDealDamage, func(s *ir.Step) { s.N = refTestVal(2); s.Obj = apYou; s.Who = apYou }),
			bad: "verb:deal-damage-who"},
		{name: "destroy target", tgts: []*ir.Target{refTestObjTarget()}, step: apStep(ir.VerbDestroy, func(s *ir.Step) { s.Obj = apTgt0 }),
			api: "Destroy", want: kvs("ValidTgts", "Permanent")},
		{name: "destroy self", step: apStep(ir.VerbDestroy, func(s *ir.Step) { s.Obj = apSelf }), api: "Destroy", want: kvs("Defined", "Self")},
		{name: "destroy all", step: apStep(ir.VerbDestroy, func(s *ir.Step) {
			s.Obj = &ir.Ref{Kind: ir.RefAll, Filter: refTestWord(ir.SubjPermanent, ir.PCardType, "Creature")}
		}), api: "DestroyAll", want: kvs("ValidCards", "Creature")},
		{name: "destroy all players", step: apStep(ir.VerbDestroy, func(s *ir.Step) { s.Obj = apOppAll }), bad: "verb:destroy"},
		{name: "counter", tgts: []*ir.Target{apSpellTarget()}, step: apStep(ir.VerbCounter, func(s *ir.Step) { s.Obj = apTgt0 }),
			api: "Counter", want: kvs("ValidTgts", "Card", "TargetType", "Spell")},
		{name: "put counters", step: apStep(ir.VerbPutCounters, func(s *ir.Step) { s.Word = "P1P1"; s.N = refTestVal(2); s.Obj = apSelf }),
			api: "PutCounter", want: kvs("CounterType", "P1P1", "CounterNum", "2", "Defined", "Self")},
		{name: "put counters no word", step: apStep(ir.VerbPutCounters, func(s *ir.Step) { s.N = refTestVal(2); s.Obj = apSelf }),
			bad: "verb:put-counters"},
		{name: "regenerate self", step: apStep(ir.VerbRegenerate, func(s *ir.Step) { s.Obj = apSelf }), api: "Regenerate"},
		{name: "regenerate target", tgts: []*ir.Target{refTestObjTarget()}, step: apStep(ir.VerbRegenerate, func(s *ir.Step) { s.Obj = apTgt0 }),
			api: "Regenerate", want: kvs("ValidTgts", "Permanent")},
		{name: "unregistered verb", step: apStep(ir.VerbFight, nil), bad: "verb:fight"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			nd, p, err := apRun(nil, c.tgts, c.step)
			if c.bad != "" {
				refTestUnprintable(t, err, c.bad)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			apWant(t, nd, c.api, c.want)
			if !reflect.DeepEqual(p.svars, c.svars) {
				t.Errorf("svars %v, want %v", p.svars, c.svars)
			}
		})
	}
}

func TestAddMana(t *testing.T) {
	sym := func(k ir.ManaKind, c ir.ColorSet) ir.ManaSym { return ir.ManaSym{Kind: k, Colors: c} }
	col := func(c ir.ColorSet) ir.ManaSym { return sym(ir.ManaColored, c) }
	mana := func(mod func(*ir.Step), ms ...ir.ManaSym) *ir.Step {
		return apStep(ir.VerbAddMana, func(s *ir.Step) {
			s.Mana = ms
			if mod != nil {
				mod(s)
			}
		})
	}
	w, u, r, g := ir.ColorW, ir.ColorU, ir.ColorR, ir.ColorG
	b := ir.ColorB
	cases := []struct {
		name string
		step *ir.Step
		want []kv
		bad  string
	}{
		{name: "one", step: mana(nil, col(r)), want: kvs("Produced", "R")},
		{name: "same colour twice", step: mana(nil, col(r), col(r)), want: kvs("Produced", "R", "Amount", "2")},
		{name: "colourless", step: mana(nil, sym(ir.ManaColorless, 0), sym(ir.ManaColorless, 0)), want: kvs("Produced", "C", "Amount", "2")},
		{name: "one with N", step: mana(func(s *ir.Step) { s.N = refTestVal(4) }, col(b)), want: kvs("Produced", "B", "Amount", "4")},
		{name: "N counted by a filter", step: mana(func(s *ir.Step) {
			s.N = &ir.Value{Kind: ir.VCount, Filter: refTestWord(ir.SubjPermanent, ir.PSubtype, "Elf")}
		}, col(g)), want: kvs("Produced", "G", "Amount", "N")},
		{name: "any colour", step: mana(nil, col(w|u|b|r|g)), want: kvs("Produced", "Any")},
		{name: "any colour N", step: mana(func(s *ir.Step) { s.N = refTestVal(2) }, col(w|u|b|r|g)), want: kvs("Produced", "Any", "Amount", "2")},
		{name: "combo", step: mana(nil, col(r|w)), want: kvs("Produced", "Combo W R")},
		{name: "combo of four", step: mana(nil, col(g|u|b|r)), want: kvs("Produced", "Combo U B R G")},
		{name: "distinct colours", step: mana(nil, col(u), col(b), col(r)), want: kvs("Produced", "U B R")},
		{name: "distinct with colourless", step: mana(nil, sym(ir.ManaColorless, 0), col(g)), want: kvs("Produced", "C G")},
		{name: "repeated and distinct", step: mana(nil, col(r), col(r), col(g)), bad: "verb:add-mana"},
		{name: "distinct with N", step: mana(func(s *ir.Step) { s.N = refTestVal(2) }, col(u), col(b)), bad: "verb:add-mana"},
		{name: "optional", step: mana(func(s *ir.Step) { s.Optional = true }, col(r)), bad: "verb:add-mana"},
		{name: "generic", step: mana(nil, sym(ir.ManaGeneric, 0)), bad: "verb:add-mana"},
		{name: "single combo with colourless", step: mana(nil, sym(ir.ManaColorless, 0), col(g), col(g)), bad: "verb:add-mana"},
		{name: "none", step: mana(nil), bad: "verb:add-mana"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			nd, _, err := apRun(nil, nil, c.step)
			if c.bad != "" {
				refTestUnprintable(t, err, c.bad)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			apWant(t, nd, "Mana", c.want)
		})
	}
	// Consecutive add-mana steps stay separate nodes: a single merged node
	// would not have the two-node shape of the same ritual in Forge.
	t.Run("consecutive steps chain", func(t *testing.T) {
		two := mana(nil, col(r), col(r))
		per := mana(func(s *ir.Step) {
			s.N = &ir.Value{Kind: ir.VCount, Filter: refTestWord(ir.SubjCard, ir.PName, "Rite")}
		}, col(r))
		nd, _, err := apRun(nil, nil, two, per)
		if err != nil {
			t.Fatal(err)
		}
		apWant(t, nd, "Mana", kvs("Produced", "R", "Amount", "2"))
		if nd.Sub == nil {
			t.Fatal("no second node")
		}
		apWant(t, nd.Sub, "Mana", kvs("Produced", "R", "Amount", "N"))
	})
}

// --- tokens ---

// apTokens is a synthetic registry of three token scripts, built through the
// parser. Their text is made up here, not copied from the corpus.
func apTokens(t *testing.T) map[string]*cards.Card {
	t.Helper()
	scripts := map[string]string{
		"w_1_1_cat":          "Name:Cat Token\nManaCost:no cost\nColors:white\nTypes:Creature Cat\nPT:1/1\n",
		"w_1_1_cat_lifelink": "Name:Cat Token\nManaCost:no cost\nColors:white\nTypes:Creature Cat\nPT:1/1\nK:Lifelink\n",
		"c_a_food":           "Name:Food Token\nManaCost:no cost\nTypes:Artifact Food\nA:AB$ GainLife | Cost$ T | LifeAmount$ 7\n",
	}
	out := map[string]*cards.Card{}
	for k, s := range scripts {
		c, diags := ParseScript(k+".txt", []byte(s))
		if len(diags) > 0 {
			t.Fatalf("%s: %v", k, diags)
		}
		out[k] = c
	}
	return out
}

func apToken(mod func(*ir.Face)) *ir.Face {
	f := &ir.Face{ColorIndicator: ir.ColorW, Types: []string{"Creature"}, Subtypes: []string{"Cat"},
		Power: &ir.PT{N: 1}, Toughness: &ir.PT{N: 1}}
	if mod != nil {
		mod(f)
	}
	return f
}

func TestTokenMatch(t *testing.T) {
	toks := apTokens(t)
	mk := func(f *ir.Face, mod func(*ir.Step)) *ir.Step {
		return apStep(ir.VerbCreateToken, func(s *ir.Step) {
			s.N, s.Token = refTestVal(2), f
			if mod != nil {
				mod(s)
			}
		})
	}
	keyword := func(k *ir.Keyword) func(*ir.Face) {
		return func(f *ir.Face) { f.Abilities = []*ir.Ability{{Kind: ir.AbKeyword, Keyword: k}} }
	}
	cases := []struct {
		name   string
		tokens map[string]*cards.Card
		step   *ir.Step
		want   []kv
		bad    string
	}{
		{name: "plain cat", tokens: toks, step: mk(apToken(nil), nil),
			want: kvs("TokenAmount", "2", "TokenScript", "w_1_1_cat", "TokenOwner", "You")},
		{name: "lifelink cat", tokens: toks, step: mk(apToken(keyword(&ir.Keyword{Name: "Lifelink"})), nil),
			want: kvs("TokenAmount", "2", "TokenScript", "w_1_1_cat_lifelink", "TokenOwner", "You")},
		{name: "food", tokens: toks, step: mk(&ir.Face{Name: "Food", Types: []string{"Artifact"}, Subtypes: []string{"Food"}}, nil),
			want: kvs("TokenAmount", "2", "TokenScript", "c_a_food", "TokenOwner", "You")},
		{name: "tapped for the opponents", tokens: toks, step: mk(apToken(nil), func(s *ir.Step) {
			s.Who = apOppAll
			s.Enter = &ir.Enter{Tapped: true}
			s.Optional = true
		}), want: kvs("TokenAmount", "2", "TokenScript", "w_1_1_cat", "TokenOwner", "Player.Opponent", "TokenTapped", "True", "OptionalDecider", "You")},
		{name: "unmatched colour", tokens: toks, step: mk(apToken(func(f *ir.Face) { f.ColorIndicator = ir.ColorR }), nil), bad: "token:unmatched"},
		{name: "unmatched power", tokens: toks, step: mk(apToken(func(f *ir.Face) { f.Power = &ir.PT{N: 2} }), nil), bad: "token:unmatched"},
		{name: "unmatched keyword", tokens: toks, step: mk(apToken(keyword(&ir.Keyword{Name: "Flying"})), nil), bad: "token:unmatched"},
		{name: "nameless face with abilities in the script", tokens: toks,
			step: mk(&ir.Face{Types: []string{"Artifact"}, Subtypes: []string{"Food"}}, nil), bad: "token:unmatched"},
		{name: "keyword args", tokens: toks, step: mk(apToken(keyword(&ir.Keyword{Name: "Ward", N: refTestVal(1)})), nil), bad: "token:keyword-args"},
		{name: "abilities", tokens: toks, step: mk(apToken(func(f *ir.Face) {
			f.Abilities = []*ir.Ability{{Kind: ir.AbSpell, Body: &ir.Program{Steps: []*ir.Step{{Verb: ir.VerbNone}}}}}
		}), nil), bad: "verb:none"},
		{name: "no registry", step: mk(apToken(nil), nil), bad: "token:no-registry"},
		{name: "enter counters", tokens: toks, step: mk(apToken(nil), func(s *ir.Step) { s.Enter = &ir.Enter{Counter: "P1P1", N: refTestVal(1)} }),
			bad: "token:enter"},
		{name: "enter under owner", tokens: toks, step: mk(apToken(nil), func(s *ir.Step) { s.Enter = &ir.Enter{UnderOwner: true} }), bad: "token:enter"},
		{name: "no token", tokens: toks, step: mk(nil, nil), bad: "verb:create-token"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			nd, _, err := apRun(c.tokens, nil, c.step)
			if c.bad != "" {
				refTestUnprintable(t, err, c.bad)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			apWant(t, nd, "Token", c.want)
		})
	}
}

// --- control flow ---

func apThreshold() *ir.Cond {
	return ctRef(ctYou(), ctCmpPred(ir.PCardsInGraveyard, ir.CmpGE, 7))
}

func apManaB(n int) *ir.Step {
	ms := make([]ir.ManaSym, n)
	for i := range ms {
		ms[i] = ir.ManaSym{Kind: ir.ManaColored, Colors: ir.ColorB}
	}
	return apStep(ir.VerbAddMana, func(s *ir.Step) { s.Mana = ms })
}

func apProg(steps ...*ir.Step) *ir.Program { return &ir.Program{Steps: steps} }

func apDamage(n int32) *ir.Step {
	return apStep(ir.VerbDealDamage, func(s *ir.Step) { s.N = refTestVal(n); s.Obj = apTgt0 })
}

func TestIfAndForEach(t *testing.T) {
	gate := kvs("ConditionCheckSVar", "N", "ConditionSVarCompare", "GE7")
	notGate := kvs("ConditionCheckSVar", "N", "ConditionSVarCompare", "LT7")
	draw := apStep(ir.VerbDraw, func(s *ir.Step) { s.N = refTestVal(1) })
	gain := apStep(ir.VerbGainLife, func(s *ir.Step) { s.N = refTestVal(2) })
	ifStep := func(then, els *ir.Program) *ir.Step {
		return apStep(ir.VerbIf, func(s *ir.Step) { s.Cond, s.Then, s.Else = apThreshold(), then, els })
	}
	iter := &ir.Ref{Kind: ir.RefIterPlayer}
	loop := func(mod func(*ir.Step), inner ...*ir.Step) *ir.Step {
		return apStep(ir.VerbForEachPlayer, func(s *ir.Step) {
			s.Obj, s.Then = refTestAllPlayers(refTestOpp()), apProg(inner...)
			if mod != nil {
				mod(s)
			}
		})
	}
	each := func(v ir.Verb, mod func(*ir.Step)) *ir.Step {
		return apStep(v, func(s *ir.Step) {
			s.Who = iter
			if mod != nil {
				mod(s)
			}
		})
	}
	oneCard := func(s *ir.Step) { s.N = refTestVal(1) }

	t.Run("amount merge, mana", func(t *testing.T) {
		nd, p, err := apRun(nil, nil, ifStep(apProg(apManaB(5)), apProg(apManaB(3))))
		if err != nil {
			t.Fatal(err)
		}
		apWant(t, nd, "Mana", kvs("Produced", "B", "Amount", "N2"))
		if got := apSVar(p, "N"); got != "Count$ValidGraveyard Card.YouOwn" {
			t.Errorf("N is %q", got)
		}
		if got := apSVar(p, "N2"); got != "Count$Compare N GE7.5.3" {
			t.Errorf("N2 is %q", got)
		}
	})
	t.Run("amount merge, damage keeps the target", func(t *testing.T) {
		nd, p, err := apRun(nil, []*ir.Target{apAnyTarget()}, ifStep(apProg(apDamage(4)), apProg(apDamage(2))))
		if err != nil {
			t.Fatal(err)
		}
		apWant(t, nd, "DealDamage", kvs("NumDmg", "N2", "ValidTgts", "Any"))
		if got := apSVar(p, "N2"); got != "Count$Compare N GE7.4.2" {
			t.Errorf("N2 is %q", got)
		}
	})
	t.Run("no else conditions every node", func(t *testing.T) {
		nd, _, err := apRun(nil, nil, ifStep(apProg(draw, gain), nil))
		if err != nil {
			t.Fatal(err)
		}
		apWant(t, nd, "Draw", append(kvs("NumCards", "1"), gate...))
		apWant(t, nd.Sub, "GainLife", append(kvs("LifeAmount", "2"), gate...))
	})
	t.Run("else is the negated chain", func(t *testing.T) {
		nd, _, err := apRun(nil, nil, ifStep(apProg(draw), apProg(gain)))
		if err != nil {
			t.Fatal(err)
		}
		apWant(t, nd, "Draw", append(kvs("NumCards", "1"), gate...))
		apWant(t, nd.Sub, "GainLife", append(kvs("LifeAmount", "2"), notGate...))
	})
	t.Run("else differing in more than the amount", func(t *testing.T) {
		two := apStep(ir.VerbDraw, func(s *ir.Step) { s.N = refTestVal(2); s.Who = apYou })
		nd, _, err := apRun(nil, nil, ifStep(apProg(draw), apProg(two)))
		if err != nil {
			t.Fatal(err)
		}
		if nd.Sub == nil {
			t.Fatalf("not two conditioned nodes: %+v", nd)
		}
		apWant(t, nd.Sub, "Draw", append(kvs("NumCards", "2", "Defined", "You"), notGate...))
	})
	t.Run("branches name different targets", func(t *testing.T) {
		nd, _, err := apRun(nil, []*ir.Target{apAnyTarget()}, ifStep(apProg(apDamage(3)), apProg(gain)))
		if err != nil {
			t.Fatal(err)
		}
		apWant(t, nd, "DealDamage", append(kvs("NumDmg", "3", "ValidTgts", "Any"), gate...))
	})
	t.Run("shared target", func(t *testing.T) {
		destroy := apStep(ir.VerbDestroy, func(s *ir.Step) { s.Obj = apTgt0 })
		_, _, err := apRun(nil, []*ir.Target{apAnyTarget()}, ifStep(apProg(apDamage(3)), apProg(destroy)))
		refTestUnprintable(t, err, "if:shared-target")
	})
	t.Run("if refusals", func(t *testing.T) {
		_, _, err := apRun(nil, nil, apStep(ir.VerbIf, func(s *ir.Step) { s.Then = apProg(draw) }))
		refTestUnprintable(t, err, "cond:nil")
		opt := ifStep(apProg(draw), nil)
		opt.Optional = true
		_, _, err = apRun(nil, nil, opt)
		refTestUnprintable(t, err, "verb:if")
		_, _, err = apRun(nil, nil, ifStep(nil, nil))
		refTestUnprintable(t, err, "program:empty")
	})
	t.Run("for each opponent", func(t *testing.T) {
		nd, _, err := apRun(nil, nil, loop(nil, each(ir.VerbLoseLife, func(s *ir.Step) { s.N = refTestVal(1) })))
		if err != nil {
			t.Fatal(err)
		}
		apWant(t, nd, "LoseLife", kvs("LifeAmount", "1", "Defined", "Player.Opponent"))
	})
	t.Run("for each refusals", func(t *testing.T) {
		lose := each(ir.VerbLoseLife, oneCard)
		cases := []struct {
			name string
			step *ir.Step
			bad  string
		}{
			{"two steps", loop(nil, lose, each(ir.VerbDraw, oneCard)), "verb:for-each-player"},
			{"inner who is you", loop(nil, apStep(ir.VerbDraw, func(s *ir.Step) { s.N = refTestVal(1); s.Who = apYou })), "verb:for-each-player"},
			{"inner without who", loop(nil, apStep(ir.VerbDraw, oneCard)), "verb:for-each-player"},
			{"iter player used twice", loop(nil, each(ir.VerbLoseLife, func(s *ir.Step) {
				s.N = &ir.Value{Kind: ir.VLife, Ref: iter}
			})), "verb:for-each-player"},
			{"not players", loop(func(s *ir.Step) {
				s.Obj = &ir.Ref{Kind: ir.RefAll, Filter: refTestWord(ir.SubjPermanent, ir.PCardType, "Creature")}
			}, lose),
				"verb:for-each-player"},
			{"optional", loop(func(s *ir.Step) { s.Optional = true }, lose), "verb:for-each-player"},
			{"inner unregistered", loop(nil, each(ir.VerbFight, nil)), "verb:fight"},
		}
		for _, c := range cases {
			_, _, err := apRun(nil, nil, c.step)
			if err == nil {
				t.Errorf("%s: printed", c.name)
				continue
			}
			refTestUnprintable(t, err, c.bad)
		}
	})
}

// TestPrintersCompose checks the cross-family cases the value, condition and
// filter printers left for the step printers.
func TestPrintersCompose(t *testing.T) {
	t.Run("count of a named card in graveyards", func(t *testing.T) {
		f := refTestWord(ir.SubjCard, ir.PName, "Rite of Flame")
		f.Zones = ir.MakeZoneSet(ir.ZoneGraveyard)
		p := &pr{}
		a, err := p.amount(&ir.Value{Kind: ir.VCount, Filter: f})
		if err != nil || a != "N" || apSVar(p, "N") != "Count$ValidGraveyard Card.namedRite of Flame" {
			t.Errorf("amount %q, %v; svars %v", a, err, p.svars)
		}
	})
	t.Run("source is untapped, both renderings", func(t *testing.T) {
		f := &ir.Filter{Subject: ir.SubjPermanent, Op: ir.FNot, Args: []*ir.Filter{refTestPred(ir.SubjPermanent, ir.PTapped, nil)}}
		cf, err := (&pr{}).cond(&ir.Cond{Kind: ir.CRefMatches, Ref: apSelf, Filter: f})
		if err != nil {
			t.Fatal(err)
		}
		abil, _ := cf.onAbility()
		trig, _ := cf.onTrigger()
		if !reflect.DeepEqual(abil, kvs("ConditionDefined", "Self", "ConditionPresent", "Permanent.untapped", "ConditionCompare", "GE1")) {
			t.Errorf("ability %v", abil)
		}
		if !reflect.DeepEqual(trig, kvs("IsPresent", "Permanent.untapped+Self", "PresentCompare", "GE1")) {
			t.Errorf("trigger %v", trig)
		}
	})
	t.Run("count compare", func(t *testing.T) {
		p := &pr{}
		cf, err := p.cond(&ir.Cond{Kind: ir.CCountCompare, Filter: refTestWord(ir.SubjPermanent, ir.PCardType, "Creature"), Cmp: ir.CmpGE, R: refTestVal(3)})
		if err != nil || cf.SVar != "N" || cf.Cmp != "GE3" || apSVar(p, "N") != "Count$Valid Creature" {
			t.Errorf("%+v, %v, svars %v", cf, err, p.svars)
		}
	})
	t.Run("opponents operand", func(t *testing.T) {
		n := &node{API: "X"}
		if err := (&pr{}).operand(n, "Defined", apOppAll); err != nil || !reflect.DeepEqual(n.Params, kvs("Defined", "Player.Opponent")) {
			t.Errorf("%v, %v", n.Params, err)
		}
	})
	t.Run("mana value at most X", func(t *testing.T) {
		p := &pr{}
		v, err := p.valid(refTestPerm(ir.PManaValue, func(f *ir.Filter) { f.Cmp, f.Value = ir.CmpLE, &ir.Value{Kind: ir.VX} }))
		if err != nil || v != "Permanent.cmcLEX" || apSVar(p, "X") != "Count$xPaid" {
			t.Errorf("%q, %v, svars %v", v, err, p.svars)
		}
	})
}
