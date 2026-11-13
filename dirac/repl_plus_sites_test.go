package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// replPlusSiteTable is vanilla plus a "source you control" filter, a
// ReplDamagePlus card adding 1 to a recipient in C, a creature filter (the
// divided-damage spell's target spec), and the caller's cards. It returns
// the table, the plus card's id and the creature filter's id.
func replPlusSiteTable(c int16, cards ...kop.CardDef) (*kop.Table, CardID, kop.FilterID) {
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Exprs = []kop.Expr{{}} // prepareGen only runs a table with generated side tables
	fid := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Ctrl: kop.CtrlYou})
	cfid := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
	plus := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Site Plus", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{
			Kind: kop.ReplDamagePlus, A: 1, C: c, Filter: fid}}}})
	tbl.Cards = append(tbl.Cards, cards...)
	return &tbl, plus, cfid
}

// The direct replDamage sites (repl_plus.go): a resolving op's damage
// pass whose exactly one event is matched by two or more pure plus-N
// replacements poses gorge's no-op CR 616.1 order ask before any dealing,
// the answer re-executing the op with the summed amount. Each test keeps its
// precondition assertions: the two plus cards are on the battlefield under
// the source's controller (that is where replPlus counts its candidates) and
// the source is where the site reads it.

// trigProg is the one-instruction ability the site tests resolve.
func trigProg(card string, in kop.Instr) kop.CardDef {
	return kop.CardDef{Name: card, Types: kop.TCreature,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered, Prog: []kop.Instr{in}}}}
}

// runTrig pushes the trigger's entry and runs its program to the pause.
func runTrig(t *testing.T, sc *scen, src Obj, card CardID) {
	t.Helper()
	e := stackEntry{kind: seTrigger, obj: src, srcVer: sc.s.objs[src].ver, ctrl: 0, card: card, trigObj: NoObj}
	sc.s.push(e)
	sc.s.res.e = e
	sc.s.res.pc = 0
	sc.s.runProg(sc.k)
}

// answerReplOrder poses the parked order ask and answers it (both orders
// give the same sum; one pick applies the pair).
func answerReplOrder(t *testing.T, sc *scen, wantPlayer uint8) {
	t.Helper()
	if sc.s.res.ask != raGenReplOrder {
		t.Fatalf("ask %d, want the parked replacement-order ask", sc.s.res.ask)
	}
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenReplOrder || d.Player != wantPlayer || d.N != 2 {
		t.Fatalf("order ask kind %d sub %d player %d options %d, want DkChoose ChGenReplOrder/%d/2",
			d.Kind, d.Sub, d.Player, d.N, wantPlayer)
	}
	sc.s.Apply(d.Options[0])
}

// TestReplPlusSiteDmgSrcPlAsks: two plus-N replacements matching an
// OpDealDamageFrom player-mode event park the no-op order ask before the
// pass deals; the answer re-executes the op, which deals the summed amount
// (3 + 1 + 1) and clears the exemption.
func TestReplPlusSiteDmgSrcPlAsks(t *testing.T) {
	tbl, plus, _ := replPlusSiteTable(int16(kop.ReplDmgOppPlayer|kop.ReplDmgOppPerm),
		trigProg("Test Trigger", kop.Instr{Op: kop.OpDealDamageFrom, Obj: kop.RefSelf,
			A: kop.Const(3), C: int16(kop.RefEachOpponent), B: kop.Const(1)}))
	sc := newScenTable(t, tbl, Config{NoElide: true})
	trig := plus + 1
	src := sc.perm(trig, 0)
	sc.perm(plus, 0)
	sc.perm(plus, 0)
	life := sc.s.pl[1].life
	runTrig(t, sc, src, trig)
	if sc.s.g.plusBatchAsked != true || sc.s.g.nc.n != 5 || sc.s.g.ro.n != 2 {
		t.Fatalf("parked pass: flag %v nc.n %d candidates %d, want true/5/2 (stall %s)",
			sc.s.g.plusBatchAsked, sc.s.g.nc.n, sc.s.g.ro.n, StallName(sc.s.StallReason()))
	}
	if sc.s.pl[1].life != life {
		t.Fatalf("parked pass already dealt: opponent life %d, want %d", sc.s.pl[1].life, life)
	}
	answerReplOrder(t, sc, 1)
	if sc.s.pl[1].life != life-5 {
		t.Fatalf("after the answer the opponent lost %d, want 5 (3 + 1 + 1)", life-sc.s.pl[1].life)
	}
	if sc.s.g.plusBatchAsked || sc.s.g.nc.n != 0 || sc.s.StallReason() != 0 {
		t.Fatalf("after the answer: flag %v nc.n %d stall %s, want none",
			sc.s.g.plusBatchAsked, sc.s.g.nc.n, StallName(sc.s.StallReason()))
	}
}

// TestReplPlusSiteDmgSrcPlTwoEventsStall: RefEachPlayer's two player events
// both matched by the two plus-N replacements are one ask per event, which
// the one-ask-per-op model does not pose: the loud stall stays and nothing
// is dealt.
func TestReplPlusSiteDmgSrcPlTwoEventsStall(t *testing.T) {
	tbl, plus, _ := replPlusSiteTable(int16(kop.ReplDmgPlayer|kop.ReplDmgOppPlayer),
		trigProg("Test Trigger", kop.Instr{Op: kop.OpDealDamageFrom, Obj: kop.RefSelf,
			A: kop.Const(3), C: int16(kop.RefEachPlayer), B: kop.Const(1)}))
	sc := newScenTable(t, tbl, Config{NoElide: true})
	trig := plus + 1
	src := sc.perm(trig, 0)
	sc.perm(plus, 0)
	sc.perm(plus, 0)
	l0, l1 := sc.s.pl[0].life, sc.s.pl[1].life
	runTrig(t, sc, src, trig)
	if got := sc.s.StallReason(); got != StallReplPlus {
		t.Fatalf("two competing player events: stall %s, want replplus", StallName(got))
	}
	if sc.s.g.plusBatchAsked || sc.s.g.nc.n != 0 {
		t.Fatalf("two competing player events: flag %v nc.n %d, want no park",
			sc.s.g.plusBatchAsked, sc.s.g.nc.n)
	}
	if sc.s.pl[0].life != l0 || sc.s.pl[1].life != l1 {
		t.Fatalf("two competing player events dealt: life %d/%d, want %d/%d",
			sc.s.pl[0].life, sc.s.pl[1].life, l0, l1)
	}
}

// dmgMapScen is a site-B scenario: the trigger source is a battlefield creature
// of seat 0 whose ability resolves the collected damage map.
func dmgMapScen(t *testing.T, plus CardID, tbl *kop.Table) (*scen, Obj, CardID) {
	t.Helper()
	sc := newScenTable(t, tbl, Config{NoElide: true})
	srcCard := plus + 1
	src := sc.perm(srcCard, 0)
	return sc, src, plus + 2
}

// runMap collects the announced map and resolves it.
func runMap(t *testing.T, sc *scen, src Obj, card CardID, ents ...struct {
	code uint8
	amt  uint8
}) {
	t.Helper()
	for i, e := range ents {
		sc.s.g.dmg[i].code, sc.s.g.dmg[i].amt = e.code, e.amt
	}
	sc.s.g.ndmg = uint8(len(ents))
	runTrig(t, sc, src, card)
}

// TestReplPlusSiteDamageBatchAsks: a two-recipient damage map with exactly
// one competing event parks the ask, the map surviving the pause; the answer
// re-executes the op, which deals both recipients, the competing one summed.
// A map with two competing events keeps the loud stall, nothing dealt.
func TestReplPlusSiteDamageBatchAsks(t *testing.T) {
	tbl, plus, _ := replPlusSiteTable(int16(kop.ReplDmgOppPlayer|kop.ReplDmgOppPerm),
		trigProg("Test Map Source", kop.Instr{}),
		trigProg("Test Map", kop.Instr{Op: kop.OpResolveDamageMap}))
	sc, src, mapCard := dmgMapScen(t, plus, tbl)
	sc.perm(plus, 0)
	sc.perm(plus, 0)
	own := sc.perm(v.GrizzlyBears, 0)
	life := sc.s.pl[1].life
	runMap(t, sc, src, mapCard,
		struct{ code, amt uint8 }{uint8(PlayerTarget1), 3},
		struct{ code, amt uint8 }{uint8(own), 2})
	if sc.s.g.plusBatchAsked != true || sc.s.g.nc.n != 5 || sc.s.g.ro.n != 2 {
		t.Fatalf("parked map: flag %v nc.n %d candidates %d, want true/5/2 (stall %s)",
			sc.s.g.plusBatchAsked, sc.s.g.nc.n, sc.s.g.ro.n, StallName(sc.s.StallReason()))
	}
	if sc.s.g.ndmg != 2 || TCode(sc.s.g.dmg[0].code) != PlayerTarget1 || sc.s.g.dmg[0].amt != 3 || sc.s.g.dmg[1].amt != 2 {
		t.Fatalf("the collected map did not survive the pause: ndmg %d %v", sc.s.g.ndmg, sc.s.g.dmg[:2])
	}
	if sc.s.pl[1].life != life || sc.s.perms[sc.slot(own)].damage != 0 {
		t.Fatalf("parked map already dealt: opponent life %d, own bear damage %d", sc.s.pl[1].life, sc.s.perms[sc.slot(own)].damage)
	}
	answerReplOrder(t, sc, 1)
	if sc.s.pl[1].life != life-5 || sc.s.perms[sc.slot(own)].damage != 2 {
		t.Fatalf("after the answer: opponent lost %d (want 5), own bear damage %d (want 2)",
			life-sc.s.pl[1].life, sc.s.perms[sc.slot(own)].damage)
	}
	if sc.s.g.ndmg != 0 || sc.s.g.plusBatchAsked || sc.s.g.nc.n != 0 || sc.s.StallReason() != 0 {
		t.Fatalf("after the answer: ndmg %d flag %v nc.n %d stall %s, want all clear",
			sc.s.g.ndmg, sc.s.g.plusBatchAsked, sc.s.g.nc.n, StallName(sc.s.StallReason()))
	}

	// two competing events: the loud stall, nothing dealt
	sc, src, mapCard = dmgMapScen(t, plus, tbl)
	sc.perm(plus, 0)
	sc.perm(plus, 0)
	opp := sc.perm(v.GrizzlyBears, 1)
	life = sc.s.pl[1].life
	runMap(t, sc, src, mapCard,
		struct{ code, amt uint8 }{uint8(PlayerTarget1), 3},
		struct{ code, amt uint8 }{uint8(opp), 2})
	if got := sc.s.StallReason(); got != StallReplPlus {
		t.Fatalf("two competing map events: stall %s, want replplus", StallName(got))
	}
	if sc.s.g.plusBatchAsked || sc.s.g.nc.n != 0 {
		t.Fatalf("two competing map events: flag %v nc.n %d, want no park", sc.s.g.plusBatchAsked, sc.s.g.nc.n)
	}
	if sc.s.pl[1].life != life || sc.s.perms[sc.slot(opp)].damage != 0 {
		t.Fatalf("two competing map events dealt: opponent life %d, its bear damage %d",
			sc.s.pl[1].life, sc.s.perms[sc.slot(opp)].damage)
	}
}

// TestReplPlusSiteDamageMapSurvivesPark: the map op reads s.g.ndmg and
// s.g.dmg only when the batch deals, so a parked pass leaves the map intact
// for the answer's re-execution, which deals exactly the announced amounts
// (no lost or doubled event).
func TestReplPlusSiteDamageMapSurvivesPark(t *testing.T) {
	tbl, plus, _ := replPlusSiteTable(int16(kop.ReplDmgOppPlayer|kop.ReplDmgOppPerm),
		trigProg("Test Map Source", kop.Instr{}),
		trigProg("Test Map", kop.Instr{Op: kop.OpResolveDamageMap}))
	sc, src, mapCard := dmgMapScen(t, plus, tbl)
	sc.perm(plus, 0)
	sc.perm(plus, 0)
	l0, l1 := sc.s.pl[0].life, sc.s.pl[1].life
	runMap(t, sc, src, mapCard,
		struct{ code, amt uint8 }{uint8(PlayerTarget1), 3},
		struct{ code, amt uint8 }{uint8(PlayerTarget0), 2})
	if sc.s.g.ndmg != 2 || sc.s.g.dmg[0].amt != 3 || sc.s.g.dmg[1].amt != 2 {
		t.Fatalf("the collected map did not survive the pause: ndmg %d amts %d/%d",
			sc.s.g.ndmg, sc.s.g.dmg[0].amt, sc.s.g.dmg[1].amt)
	}
	answerReplOrder(t, sc, 1)
	if sc.s.pl[1].life != l1-5 || sc.s.pl[0].life != l0-2 {
		t.Fatalf("after the answer the map dealt %d/%d, want 5 to the opponent and 2 to its controller",
			l1-sc.s.pl[1].life, l0-sc.s.pl[0].life)
	}
}

// TestReplPlusSiteAnswerDivideStallsLoud: the raGenDivide answer is not an
// op-instruction pause (re-executing OpDealDamageDivided would re-pose the
// divide ask gorge does not repeat), so its damageBatch skips the probe and
// a pure plus-N competition keeps the loud stall: no park, no exemption.
// Today's cap deals the summed amount before the game ends at the next step;
// the pin records that.
func TestReplPlusSiteAnswerDivideStallsLoud(t *testing.T) {
	tbl, plus, cfid := replPlusSiteTable(int16(kop.ReplDmgOppPlayer | kop.ReplDmgOppPerm))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Divide", Types: kop.TInstant,
		Abilities: []kop.Ability{{Kind: kop.AbSpell,
			Targets:  [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtObject, Filter: cfid}},
			NTargets: 1,
			Prog: []kop.Instr{{Op: kop.OpDealDamageDivided, Obj: kop.RefTarget0,
				A: kop.Const(4)}}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	div := plus + 1
	b1 := sc.perm(v.GrizzlyBears, 1)
	b2 := sc.perm(v.GrizzlyBears, 1)
	sc.perm(plus, 0)
	sc.perm(plus, 0)
	sp := sc.obj(div, 0)
	sc.s.objs[sp].zone = kop.ZoneStack
	e := stackEntry{kind: seSpell, obj: sp, ctrl: 0, card: div, trigObj: NoObj}
	sc.s.push(e)
	sc.s.res.e = e
	sc.s.res.pc = 0
	sc.s.res.e.tgt[0] = uint8(b1)
	x := sc.s.xtMut(&sc.s.res.e)
	x.n, x.slot[0], x.code[0], x.ver[0] = 1, 0, uint8(b2), sc.s.objs[b2].ver
	sc.s.runProg(sc.k)
	if sc.s.res.ask != raGenDivide {
		t.Fatalf("ask %d, want the divide ask over the two bears", sc.s.res.ask)
	}
	for i := 0; i < 4; i++ {
		d := sc.s.Step()
		if d.Sub != ChGenDivide || d.N != 2 {
			t.Fatalf("divide pick %d: kind sub %d options %d, want ChGenDivide/2", i, d.Sub, d.N)
		}
		sc.s.Apply(d.Options[i/2]) // 2 + 2
	}
	if got := sc.s.StallReason(); got != StallReplPlus {
		t.Fatalf("the divide's damageBatch stalled %s, want replplus", StallName(got))
	}
	if sc.s.g.plusBatchAsked || sc.s.g.nc.n != 0 {
		t.Fatalf("the divide's damageBatch parked: flag %v nc.n %d, want no park",
			sc.s.g.plusBatchAsked, sc.s.g.nc.n)
	}
	if d1, d2 := sc.s.perms[sc.slot(b1)].damage, sc.s.perms[sc.slot(b2)].damage; d1 != 4 || d2 != 4 {
		t.Fatalf("divide damage %d/%d, want today's summed 4/4 (2 + 1 + 1 each)", d1, d2)
	}
}

// TestReplPlusSitePainStallsLoud: the painland rider has no program step to
// park (mana_pain.go), so two plus-N replacements matching its self-damage
// keep the loud stall: no park, no exemption. Today's cap deals the summed
// amount before the game ends at the next step; the pin records that.
func TestReplPlusSitePainStallsLoud(t *testing.T) {
	tbl, plus, _ := replPlusSiteTable(int16(kop.ReplDmgPlayer),
		kop.CardDef{Name: "Test Pain Land", Types: kop.TLand})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	land := plus + 1
	src := sc.perm(land, 0)
	sc.perm(plus, 0)
	sc.perm(plus, 0)
	life := sc.s.pl[0].life
	sc.s.painDamage(sc.k, sc.slot(src), 0, 1)
	if got := sc.s.StallReason(); got != StallReplPlus {
		t.Fatalf("the pain rider's competition stalled %s, want replplus", StallName(got))
	}
	if sc.s.g.plusBatchAsked || sc.s.g.nc.n != 0 {
		t.Fatalf("the pain rider parked: flag %v nc.n %d, want no park", sc.s.g.plusBatchAsked, sc.s.g.nc.n)
	}
	if got := sc.s.pl[0].life; got != life-3 {
		t.Fatalf("pain life %d, want today's summed loss to %d (1 + 1 + 1)", got, life-3)
	}
}

// The competing event is not always the pass's first: gorge deals the
// earlier events before it collects the competing one's replacements and
// asks (each event's replacements are collected as the event happens, CR
// 616.1e), so a park there would show gorge's earlier damage at an ask whose
// re-execution deals it again. The kernel stalls loudly instead, before any
// dealing. Each test keeps its precondition assertions: the compared values
// differ (the stall is non-zero, the recipients are untouched only because
// nothing was dealt) and the plus cards sit where the candidate walk reads
// them.

// TestReplPlusSiteDamageMapCompetingSecondStalls: the map's own-bear event
// comes first, the opponent-matched event second; the pass stalls loudly
// with nothing dealt.
func TestReplPlusSiteDamageMapCompetingSecondStalls(t *testing.T) {
	tbl, plus, _ := replPlusSiteTable(int16(kop.ReplDmgOppPlayer|kop.ReplDmgOppPerm),
		trigProg("Test Map Source", kop.Instr{}),
		trigProg("Test Map", kop.Instr{Op: kop.OpResolveDamageMap}))
	sc, src, mapCard := dmgMapScen(t, plus, tbl)
	sc.perm(plus, 0)
	sc.perm(plus, 0)
	own := sc.perm(v.GrizzlyBears, 0)
	life := sc.s.pl[1].life
	if sc.s.perms[sc.slot(own)].damage != 0 || sc.s.pl[1].life != life {
		t.Fatalf("precondition: own bear damage %d, opponent life %d, want untouched %d",
			sc.s.perms[sc.slot(own)].damage, sc.s.pl[1].life, life)
	}
	runMap(t, sc, src, mapCard,
		struct{ code, amt uint8 }{uint8(own), 2},
		struct{ code, amt uint8 }{uint8(PlayerTarget1), 3})
	if got := sc.s.StallReason(); got != StallReplPlus {
		t.Fatalf("competing second map event: stall %s, want replplus", StallName(got))
	}
	if sc.s.g.plusBatchAsked || sc.s.g.nc.n != 0 {
		t.Fatalf("competing second map event parked: flag %v nc.n %d, want no park",
			sc.s.g.plusBatchAsked, sc.s.g.nc.n)
	}
	if sc.s.perms[sc.slot(own)].damage != 0 || sc.s.pl[1].life != life {
		t.Fatalf("the stalled pass dealt: own bear damage %d, opponent life %d, want 0/%d",
			sc.s.perms[sc.slot(own)].damage, sc.s.pl[1].life, life)
	}
}

// TestReplPlusSiteDmgSrcPlCompetingSecondStalls: RefEachPlayer's opponent
// event is the pass's second; the pass stalls loudly with both players'
// life untouched.
func TestReplPlusSiteDmgSrcPlCompetingSecondStalls(t *testing.T) {
	tbl, plus, _ := replPlusSiteTable(int16(kop.ReplDmgOppPlayer),
		trigProg("Test Trigger", kop.Instr{Op: kop.OpDealDamageFrom, Obj: kop.RefSelf,
			A: kop.Const(3), C: int16(kop.RefEachPlayer), B: kop.Const(1)}))
	sc := newScenTable(t, tbl, Config{NoElide: true})
	trig := plus + 1
	src := sc.perm(trig, 0)
	sc.perm(plus, 0)
	sc.perm(plus, 0)
	l0, l1 := sc.s.pl[0].life, sc.s.pl[1].life
	runTrig(t, sc, src, trig)
	if got := sc.s.StallReason(); got != StallReplPlus {
		t.Fatalf("competing second player event: stall %s, want replplus", StallName(got))
	}
	if sc.s.g.plusBatchAsked || sc.s.g.nc.n != 0 {
		t.Fatalf("competing second player event parked: flag %v nc.n %d, want no park",
			sc.s.g.plusBatchAsked, sc.s.g.nc.n)
	}
	if sc.s.pl[0].life != l0 || sc.s.pl[1].life != l1 {
		t.Fatalf("the stalled pass dealt: life %d/%d, want %d/%d",
			sc.s.pl[0].life, sc.s.pl[1].life, l0, l1)
	}
}

// TestReplPlusBatchCompetingSecondStalls: a fight-shaped hit batch whose
// second hit is the opponent-matched one stalls loudly (StallReplPlusBatch,
// trig_batches.go) with both bears untouched; the source is a seat-0
// creature so the candidate walk finds the seat-0 plus cards.
func TestReplPlusBatchCompetingSecondStalls(t *testing.T) {
	tbl, plus, _ := replPlusSiteTable(int16(kop.ReplDmgOppPlayer | kop.ReplDmgOppPerm))
	sc := newScenTable(t, tbl, Config{NoElide: true})
	fighter := sc.perm(v.GrizzlyBears, 0)
	own := sc.perm(v.GrizzlyBears, 0)
	opp := sc.perm(v.GrizzlyBears, 1)
	sc.perm(plus, 0)
	sc.perm(plus, 0)
	var hb hitBatch
	hb.add(sc.s, sc.slot(fighter), sc.slot(own), 2)
	hb.add(sc.s, sc.slot(fighter), sc.slot(opp), 3)
	if hb.n != 2 {
		t.Fatalf("batch holds %d hits, want 2", hb.n)
	}
	if !sc.s.dealHits(sc.k, hb.h[:hb.n]) {
		t.Fatal("dealHits returned false, want the batch consumed")
	}
	if got := sc.s.StallReason(); got != StallReplPlusBatch {
		t.Fatalf("competing second batch hit: stall %s, want replplusbatch", StallName(got))
	}
	if sc.s.g.plusBatchAsked || sc.s.g.nc.n != 0 {
		t.Fatalf("competing second batch hit parked: flag %v nc.n %d, want no park",
			sc.s.g.plusBatchAsked, sc.s.g.nc.n)
	}
	if d1, d2 := sc.s.perms[sc.slot(own)].damage, sc.s.perms[sc.slot(opp)].damage; d1 != 0 || d2 != 0 {
		t.Fatalf("the stalled batch dealt: bear damage %d/%d, want 0/0", d1, d2)
	}
}
