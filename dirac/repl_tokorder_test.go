package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// tokIDs are the cards of the token-rewriter scenarios.
type tokIDs struct {
	soldier, illusion, illusionB CardID // the op's token, the rewriters' tokens
	holo, holoB, ds              CardID // Holo-Engineer (-> illusion), a second rewriter (-> illusionB), Doubling Season
	army, human                  CardID // human: the Recruit token, found by its script key
	jace                         int    // the Jace subtype
}

// tokTable is the FDN plains matchup table plus the token-rewriter cards.
func tokTable(t *testing.T) (*kop.Table, tokIDs) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	return tbl, addTokCards(tbl)
}

// addTokCards appends the token-rewriter cards to tbl.
func addTokCards(tbl *kop.Table) tokIDs {
	tbl.Subtypes = append(tbl.Subtypes, "Army", "Jace")
	var id tokIDs
	id.jace = len(tbl.Subtypes) - 1
	n := CardID(len(tbl.Cards))
	id.soldier, id.illusion, id.illusionB, id.holo, id.holoB, id.ds, id.army = n, n+1, n+2, n+3, n+4, n+5, n+6
	rew := func(name string, to CardID) kop.CardDef {
		return kop.CardDef{Name: name, Types: kop.TCreature, Power: 1, Toughness: 1, Abilities: []kop.Ability{{
			Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplRewriteTokens, C: int16(to)}}}}
	}
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Soldier", Types: kop.TCreature, Power: 1, Toughness: 1},
		kop.CardDef{Name: "Test Illusion", Types: kop.TCreature | kop.TArtifact, Power: 3, Toughness: 3},
		kop.CardDef{Name: "Test Illusion B", Types: kop.TCreature, Power: 2, Toughness: 2},
		rew("Test Holo A", n+1), rew("Test Holo B", n+2),
		kop.CardDef{Name: "Test Doubler", Types: kop.TEnchantment, Abilities: []kop.Ability{{
			Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplDoubleTokens}}}},
		kop.CardDef{Name: "Test Army", Types: kop.TCreature, Subtypes: kop.SubMask{}.With(len(tbl.Subtypes) - 2)})
	return id
}

// tokOrderScen is a resolving spell of seat 0 that creates count soldiers,
// with the cards of boards on seat 0's battlefield in that (entry) order.
func tokOrderScen(t *testing.T, count int, board func(id tokIDs) []CardID) (*scen, []kop.Instr, tokIDs) {
	t.Helper()
	var id tokIDs
	sc, prog := czScenWith(t, func(tbl *kop.Table) []kop.Instr {
		id = addTokCards(tbl)
		return []kop.Instr{{Op: kop.OpCreateToken, Obj: kop.RefController, A: kop.Const(count), C: int16(id.soldier)}}
	})
	for _, c := range board(id) {
		sc.perm(c, 0)
	}
	return sc, prog, id
}

// tokAsk is the order ask the resolution is suspended on: its options'
// sources and modes.
func tokAsk(t *testing.T, sc *scen) Decision {
	t.Helper()
	s := sc.s
	if s.res.ask != raGenReplOrder {
		t.Fatalf("res.ask %d, want the order ask %d (stalled %d)", s.res.ask, raGenReplOrder, s.g.stalled)
	}
	var d Decision
	if !s.resolveOptionsGen(sc.k, &d) || d.Kind != DkChoose || d.Sub != ChGenReplOrder {
		t.Fatalf("decision kind %d sub %d, want DkChoose/ChGenReplOrder", d.Kind, d.Sub)
	}
	return d
}

// tokOf counts the battlefield permanents of card c.
func tokOf(sc *scen, c CardID) int { return len(sc.copiesOf(c)) }

// answer applies option i of the posed ask.
func (sc *scen) tokAnswer(t *testing.T, i int) {
	t.Helper()
	d := tokAsk(t, sc)
	if i >= int(d.N) {
		t.Fatalf("option %d of %d", i, d.N)
	}
	sc.s.applyResolveGen(sc.k, d.Options[i])
}

// TestTokRewriteOrderAskResume: a rewriter beside a multiplier is gorge's
// CR 616.1 order ask (replacement_token.go driveTokenReplacements): the
// creator is asked, each pick applies at its scan point, the ask comes back
// while two or more candidates that do not all commute remain, and the token
// is made when the walk ends. A creator who has left the game makes no
// choice: the candidates apply in scan order.
func TestTokRewriteOrderAskResume(t *testing.T) {
	t.Run("rewriter and doubler", func(t *testing.T) {
		for pick := 0; pick < 2; pick++ {
			sc, prog, id := tokOrderScen(t, 1, func(id tokIDs) []CardID { return []CardID{id.holo, id.ds} })
			s := sc.s
			if s.exec(sc.k, &prog[0]) {
				t.Fatal("the op ran on: no ask for a rewriter beside a doubler")
			}
			d := tokAsk(t, sc)
			if d.Player != 0 || d.N != 2 {
				t.Fatalf("player %d options %d, want the creator asked over both replacements", d.Player, d.N)
			}
			for i := 0; i < 2; i++ {
				if int(d.Options[i].Mode()) != i || s.cardAt(s.objs[Obj(d.Options[i].Source())].slot) != []CardID{id.holo, id.ds}[i] {
					t.Fatalf("option %d: source %d mode %d, want the replacements in scan order", i, d.Options[i].Source(), d.Options[i].Mode())
				}
			}
			if tokOf(sc, id.soldier)+tokOf(sc, id.illusion) != 0 {
				t.Fatal("a token was made before the answer")
			}
			s.applyResolveGen(sc.k, d.Options[pick])
			if s.res.ask == raGenReplOrder {
				t.Fatalf("pick %d: asked again with one replacement left", pick)
			}
			if got := tokOf(sc, id.illusion); got != 2 || tokOf(sc, id.soldier) != 0 {
				t.Fatalf("pick %d: %d Illusions and %d Soldiers, want 2 and 0", pick, got, tokOf(sc, id.soldier))
			}
			if s.g.tok.on || s.g.ro.n != 0 {
				t.Fatalf("pick %d: walk left behind (tok.on %v ro.n %d)", pick, s.g.tok.on, s.g.ro.n)
			}
		}
	})

	t.Run("suffix re-check", func(t *testing.T) {
		// Holo, Doubler, Doubler: picking a Doubler first leaves a rewriter
		// beside a Doubler (they do not commute: ask again); picking the
		// rewriter first leaves two Doublers (they commute: no ask)
		sc, prog, id := tokOrderScen(t, 1, func(id tokIDs) []CardID { return []CardID{id.holo, id.ds, id.ds} })
		s := sc.s
		s.exec(sc.k, &prog[0])
		d := tokAsk(t, sc)
		if d.N != 3 {
			t.Fatalf("%d options, want 3", d.N)
		}
		sc.tokAnswer(t, 1) // the first Doubler
		d = tokAsk(t, sc)  // asked again exactly once more
		if d.N != 2 || d.Options[0].Mode() != 0 || d.Options[1].Mode() != 2 {
			t.Fatalf("second ask: %d options, modes %d %d, want the rewriter and the second Doubler", d.N, d.Options[0].Mode(), d.Options[1].Mode())
		}
		sc.tokAnswer(t, 1)
		if s.res.ask == raGenReplOrder {
			t.Fatal("asked a third time")
		}
		if got := tokOf(sc, id.illusion); got != 4 {
			t.Fatalf("%d Illusions, want 4", got)
		}

		sc, prog, id = tokOrderScen(t, 1, func(id tokIDs) []CardID { return []CardID{id.holo, id.ds, id.ds} })
		sc.s.exec(sc.k, &prog[0])
		sc.tokAnswer(t, 0) // the rewriter first
		if sc.s.res.ask == raGenReplOrder {
			t.Fatal("asked again with only commuting Doublers left")
		}
		if got := tokOf(sc, id.illusion); got != 4 {
			t.Fatalf("%d Illusions, want 4", got)
		}
	})

	t.Run("order is observable", func(t *testing.T) {
		// two rewriters: the last applied wins
		for pick, want := range [2]func(tokIDs) CardID{func(id tokIDs) CardID { return id.illusionB }, func(id tokIDs) CardID { return id.illusion }} {
			sc, prog, id := tokOrderScen(t, 1, func(id tokIDs) []CardID { return []CardID{id.holo, id.holoB} })
			sc.s.exec(sc.k, &prog[0])
			sc.tokAnswer(t, pick)
			if tokOf(sc, want(id)) != 1 || tokOf(sc, id.soldier) != 0 {
				t.Fatalf("pick %d: want one %s", pick, sc.s.k().t.Cards[want(id)].Name)
			}
		}
	})

	t.Run("each token asks", func(t *testing.T) {
		sc, prog, id := tokOrderScen(t, 2, func(id tokIDs) []CardID { return []CardID{id.holo, id.ds} })
		sc.s.exec(sc.k, &prog[0])
		sc.tokAnswer(t, 0)
		if got := tokOf(sc, id.illusion); got != 2 {
			t.Fatalf("after the first answer %d Illusions, want the first token's 2", got)
		}
		sc.tokAnswer(t, 1) // the second token asks on its own
		if got := tokOf(sc, id.illusion); got != 4 || sc.s.res.ask == raGenReplOrder {
			t.Fatalf("%d Illusions, ask %d, want 4 and done", got, sc.s.res.ask)
		}
	})

	t.Run("creator lost", func(t *testing.T) {
		sc, prog, id := tokOrderScen(t, 1, func(id tokIDs) []CardID { return []CardID{id.holo, id.holoB} })
		sc.s.pl[0].lost = true
		sc.s.exec(sc.k, &prog[0])
		if sc.s.res.ask == raGenReplOrder {
			t.Fatal("asked a player who has left the game")
		}
		if tokOf(sc, id.illusionB) != 1 || tokOf(sc, id.illusion) != 0 {
			t.Fatal("want scan order: the second rewriter's token")
		}
	})

	t.Run("a pick is applied once", func(t *testing.T) {
		sc, prog, id := tokOrderScen(t, 1, func(id tokIDs) []CardID { return []CardID{id.holo, id.ds, id.ds} })
		sc.s.exec(sc.k, &prog[0])
		sc.tokAnswer(t, 1)
		d := tokAsk(t, sc)
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Mode() == 1 {
				t.Fatalf("the applied replacement is offered again: %v", d.Options[:d.N])
			}
		}
		_ = id
	})
}

// TestTokRewriteMintSites: every mint site makes the card a lone rewriter
// turns its token into, in the number the rewriter keeps; beside a lone
// Doubler the original card is doubled and no other card appears.
func TestTokRewriteMintSites(t *testing.T) {
	type site struct {
		name string
		// run makes the site's tokens; the scenario has seat 0 as the resolving controller
		run func(sc *scen, id tokIDs)
		// the original card the site makes, and how many a Doubler makes of it
		orig    func(id tokIDs) CardID
		doubled int
		// stall: the declared stall beside a Doubler instead
		stall uint8
	}
	sites := []site{
		{name: "amass", orig: func(id tokIDs) CardID { return id.army }, doubled: 2, run: func(sc *scen, id tokIDs) {
			in := kop.Instr{Op: kop.OpAmass, Obj: kop.RefController, A: kop.Const(1), C: int16(id.army), B: kop.Const(0)}
			sc.s.exec(sc.k, &in)
		}},
		{name: "incubate", orig: func(id tokIDs) CardID { return id.soldier }, doubled: 2, run: func(sc *scen, id tokIDs) {
			in := kop.Instr{Op: kop.OpIncubate, Obj: kop.RefController, A: kop.Const(1), B: kop.Const(1), C: int16(id.soldier)}
			sc.s.incubate(sc.k, &in)
		}},
		{name: "empower", orig: func(id tokIDs) CardID { return id.soldier }, doubled: 2, run: func(sc *scen, id tokIDs) {
			in := kop.Instr{Op: kop.OpEmpower, Obj: kop.RefController, A: kop.Const(0), B: kop.Const(1), C: int16(id.soldier)}
			sc.s.empower(sc.k, &in)
		}},
		{name: "endure", orig: func(id tokIDs) CardID { return id.soldier }, stall: StallEndureN, run: func(sc *scen, id tokIDs) {
			in := kop.Instr{Op: kop.OpExt, C: 0}
			in.B.N = int16(id.soldier)
			sc.s.endureSpirit(sc.k, &in, 0, 2)
		}},
		{name: "recruit", orig: func(id tokIDs) CardID { return id.human }, doubled: 2, run: func(sc *scen, id tokIDs) {
			sc.s.recruitToken(sc.k, 0)
		}},
		{name: "copy with counters", orig: func(id tokIDs) CardID { return id.soldier }, doubled: 2, run: func(sc *scen, id tokIDs) {
			src := sc.perm(id.soldier, 0)
			ext := kop.ExtInstr(kop.ExtCopyTokenCtr)
			ext.Obj, ext.B, ext.C = kop.RefSelf, kop.Const(1), int16(kop.CtrP1P1)
			sc.s.res.e.obj = src
			sc.s.copyTokenCtr(sc.k, &ext)
		}},
		{name: "copy of each", orig: func(id tokIDs) CardID { return id.soldier }, doubled: 2, run: func(sc *scen, id tokIDs) {
			sc.perm(id.soldier, 0)
			all := kop.FilterID(len(sc.k.t.Filters) - 1)
			in := kop.Instr{Op: kop.OpExt, A: kop.Const(int(kop.ExtCopyAllEach)), B: kop.Const(0), F: all}
			sc.s.copyAllEach(sc.k, &in)
		}},
		{name: "copy token", orig: func(id tokIDs) CardID { return id.soldier }, doubled: 2, run: func(sc *scen, id tokIDs) {
			src := sc.perm(id.soldier, 0)
			in := kop.Instr{Op: kop.OpCopyAsTokenEx, Obj: kop.RefSelf, A: kop.Const(1)}
			sc.s.res.e.obj = src
			sc.s.execCopyToken(sc.k, &in)
		}},
		{name: "create token", orig: func(id tokIDs) CardID { return id.soldier }, doubled: 2, run: func(sc *scen, id tokIDs) {
			in := kop.Instr{Op: kop.OpCreateToken, Obj: kop.RefController, A: kop.Const(1), C: int16(id.soldier)}
			sc.s.exec(sc.k, &in)
		}},
	}
	scen := func(t *testing.T, board ...func(tokIDs) CardID) (*scen, tokIDs) {
		tbl, id := tokTable(t)
		tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou, Zone: kop.ZoneBattlefield})
		id.human = CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "w_1_1_human_soldier", Types: kop.TCreature, Power: 1, Toughness: 1})
		tbl.Overrides = append(tbl.Overrides, kop.Override{AddSub: kop.SubMask{}.With(id.jace)})
		sc := newScenTable(t, tbl, Config{NoElide: true})
		for _, b := range board {
			sc.perm(b(id), 0)
		}
		sc.s.res.e.ctrl = 0
		return sc, id
	}
	for _, st := range sites {
		t.Run(st.name+" rewritten", func(t *testing.T) {
			sc, id := scen(t, func(id tokIDs) CardID { return id.holo })
			if !sc.s.k().gen.anyTokRew {
				t.Fatal("the table has no rewriter")
			}
			st.run(sc, id)
			if sc.s.g.stalled != 0 {
				t.Fatalf("stalled %d (%s), want the rewritten mint", sc.s.g.stalled, stallNames[sc.s.g.stalled])
			}
			if got := tokOf(sc, id.illusion); got < 1 {
				t.Fatalf("no Illusion made; the original card has %d on the battlefield", tokOf(sc, st.orig(id)))
			}
			// the rewriter keeps the number: no token of the original card is made
			// (a copy site's source is the one permanent of it)
			want := 0
			if st.name == "copy with counters" || st.name == "copy of each" || st.name == "copy token" {
				want = 1
			}
			if got := tokOf(sc, st.orig(id)); got != want {
				t.Fatalf("%d of the original card on the battlefield, want %d", got, want)
			}
		})
		t.Run(st.name+" doubled", func(t *testing.T) {
			sc, id := scen(t, func(id tokIDs) CardID { return id.ds })
			st.run(sc, id)
			if st.stall != 0 {
				if sc.s.g.stalled != st.stall {
					t.Fatalf("stalled %d, want the declared %d", sc.s.g.stalled, st.stall)
				}
				return
			}
			if sc.s.g.stalled != 0 {
				t.Fatalf("stalled %d (%s)", sc.s.g.stalled, stallNames[sc.s.g.stalled])
			}
			src := 0
			if st.name == "copy with counters" || st.name == "copy of each" || st.name == "copy token" {
				src = 1
			}
			if got := tokOf(sc, st.orig(id)) - src; got != st.doubled {
				t.Fatalf("%d tokens of the original card, want %d", got, st.doubled)
			}
			if tokOf(sc, id.illusion) != 0 {
				t.Fatal("an Illusion appeared without a rewriter")
			}
		})
	}
}
