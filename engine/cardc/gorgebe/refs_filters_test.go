package gorgebe

import (
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// --- builders ---

func refTestRef(k ir.RefKind) *ir.Ref { return &ir.Ref{Kind: k} }

func refTestOf(k ir.RefKind, of *ir.Ref) *ir.Ref { return &ir.Ref{Kind: k, Of: of} }

func refTestTarget(slot uint8) *ir.Ref { return &ir.Ref{Kind: ir.RefTarget, Slot: slot} }

func refTestPred(subj ir.Subject, pred ir.Pred, mod func(*ir.Filter)) *ir.Filter {
	f := &ir.Filter{Subject: subj, Op: ir.FPred, Pred: pred}
	if mod != nil {
		mod(f)
	}
	return f
}

func refTestWord(subj ir.Subject, pred ir.Pred, w string) *ir.Filter {
	return refTestPred(subj, pred, func(f *ir.Filter) { f.Word = w })
}

func refTestPerm(pred ir.Pred, mod func(*ir.Filter)) *ir.Filter {
	return refTestPred(ir.SubjPermanent, pred, mod)
}

func refTestAll(subj ir.Subject, args ...*ir.Filter) *ir.Filter {
	return &ir.Filter{Subject: subj, Op: ir.FAll, Args: args}
}

func refTestAny(args ...*ir.Filter) *ir.Filter { return &ir.Filter{Op: ir.FAny, Args: args} }

func refTestNot(f *ir.Filter) *ir.Filter { return &ir.Filter{Op: ir.FNot, Args: []*ir.Filter{f}} }

func refTestRefPred(subj ir.Subject, pred ir.Pred, r *ir.Ref) *ir.Filter {
	return refTestPred(subj, pred, func(f *ir.Filter) { f.Ref = r })
}

// refTestAllPlayers is a player operand as CC-12 lowers it: every player matching f.
func refTestAllPlayers(f *ir.Filter) *ir.Ref { return &ir.Ref{Kind: ir.RefAll, Filter: f} }

func refTestYou() *ir.Filter {
	return refTestRefPred(ir.SubjPlayer, ir.PPlayerIs, refTestRef(ir.RefYou))
}

func refTestOpp() *ir.Filter {
	return refTestRefPred(ir.SubjPlayer, ir.POpponentOf, refTestRef(ir.RefYou))
}

func refTestVal(n int32) *ir.Value { return &ir.Value{Kind: ir.VConst, N: n} }

func refTestNum(subj ir.Subject, pred ir.Pred, c ir.CmpOp, n int32) *ir.Filter {
	return refTestPred(subj, pred, func(f *ir.Filter) { f.Cmp, f.Value = c, refTestVal(n) })
}

func refTestPr(ev ir.EventKind, evSelf bool, targets ...*ir.Target) *pr {
	p := &pr{ev: ev, evSelf: evSelf}
	if targets != nil {
		p.frames = append(p.frames, &frame{targets: targets, declared: make([]bool, len(targets))})
	}
	return p
}

func refTestPlayerTarget() *ir.Target {
	return &ir.Target{Min: 1, Max: 1, Filter: refTestPred(ir.SubjPlayer, ir.PAny, nil)}
}

func refTestObjTarget() *ir.Target {
	return &ir.Target{Min: 1, Max: 1, Filter: refTestPred(ir.SubjPermanent, ir.PAny, nil)}
}

func refTestUnprintable(t *testing.T, err error, what string) {
	t.Helper()
	var u *Unprintable
	if !errors.As(err, &u) || u.What != what {
		t.Errorf("err %v, want unprintable %q", err, what)
	}
}

// --- Defined$ ---

type refTestCase struct {
	name   string
	ev     ir.EventKind
	evSelf bool
	tgts   []*ir.Target
	ref    *ir.Ref
	want   string // "" with bad set: an Unprintable
	bad    string
}

func refTestDefinedCases() []refTestCase {
	tgt := refTestTarget(0)
	ev := func(k ir.RefKind) *ir.Ref { return refTestRef(k) }
	of := refTestOf
	pl := []*ir.Target{refTestPlayerTarget()}
	ob := []*ir.Target{refTestObjTarget()}
	c := []refTestCase{
		{name: "self", ref: ev(ir.RefSelf), want: "Self"},
		{name: "you", ref: ev(ir.RefYou), want: "You"},
		{name: "target player", tgts: pl, ref: tgt, want: "TargetedPlayer"},
		{name: "target object", tgts: ob, ref: tgt, want: "Targeted"},
		{name: "target unbound", ref: tgt, bad: "ref:target-unbound"},
		{name: "target slot out of range", tgts: ob, ref: refTestTarget(3), bad: "ref:target-unbound"},
		{name: "event object self", ev: ir.EvEnters, evSelf: true, ref: ev(ir.RefEventObject), want: "Self"},
		{name: "event object no event", ref: ev(ir.RefEventObject), bad: "ref:event-object"},
		{name: "event object upkeep", ev: ir.EvUpkeep, ref: ev(ir.RefEventObject), bad: "ref:event-object@upkeep"},
		{name: "event player no event", ref: ev(ir.RefEventPlayer), bad: "ref:event-player"},
		{name: "event player dies", ev: ir.EvDies, ref: ev(ir.RefEventPlayer), bad: "ref:event-player@dies"},
		{name: "controller of target", tgts: ob, ref: of(ir.RefControllerOf, tgt), want: "TargetedController"},
		{name: "controller of self", ref: of(ir.RefControllerOf, ev(ir.RefSelf)), want: "You"},
		{name: "controller of event object, upkeep", ev: ir.EvUpkeep, ref: of(ir.RefControllerOf, ev(ir.RefEventObject)),
			bad: "ref:controller-of"},
		{name: "controller of you", ref: of(ir.RefControllerOf, ev(ir.RefYou)), bad: "ref:controller-of"},
		{name: "owner of target", tgts: ob, ref: of(ir.RefOwnerOf, tgt), want: "TargetedOwner"},
		{name: "owner of self", ref: of(ir.RefOwnerOf, ev(ir.RefSelf)), bad: "ref:owner-of"},
		{name: "player or controller of target", tgts: ob, ref: of(ir.RefPlayerOrControllerOf, tgt), want: "TargetedOrController"},
		{name: "player or controller of self", ref: of(ir.RefPlayerOrControllerOf, ev(ir.RefSelf)), bad: "ref:player-or-controller-of"},
		{name: "all you", ref: refTestAllPlayers(refTestYou()), want: "You"},
		{name: "all opponents", ref: refTestAllPlayers(refTestOpp()), want: "Player.Opponent"},
		{name: "all players", ref: refTestAllPlayers(refTestPred(ir.SubjPlayer, ir.PAny, nil)), want: "Player"},
		{name: "all permanents", ref: &ir.Ref{Kind: ir.RefAll, Filter: refTestPerm(ir.PAny, nil)}, bad: "ref:all"},
		{name: "all, no filter", ref: ev(ir.RefAll), bad: "ref:all"},
	}
	evObj := map[ir.EventKind]string{
		ir.EvEnters: "TriggeredCard", ir.EvDies: "TriggeredNewCardLKICopy", ir.EvPutIntoGraveyard: "TriggeredNewCardLKICopy",
		ir.EvLeaves: "TriggeredNewCardLKICopy", ir.EvAttacks: "TriggeredAttacker",
		ir.EvDealsCombatDamage: "TriggeredSourceLKICopy", ir.EvDealsCombatDamageToPlayer: "TriggeredSourceLKICopy",
		ir.EvDealsNoncombatDamageToPlayer: "TriggeredSourceLKICopy", ir.EvCast: "TriggeredSpellAbility",
		ir.EvBecomesTarget: "TriggeredSourceSA", ir.EvCountersPut: "TriggeredCard",
		ir.EvWouldEnter: "ReplacedCard", ir.EvWouldPutCounters: "ReplacedCard", ir.EvWouldDealDamage: "ReplacedCard",
		ir.EvWouldCreateTokens: "ReplacedCard", ir.EvWouldDie: "ReplacedCard",
	}
	for _, e := range sortedEvents(evObj) {
		c = append(c, refTestCase{name: "event object " + e.String(), ev: e, ref: ev(ir.RefEventObject), want: evObj[e]})
	}
	evPl := map[ir.EventKind]string{
		ir.EvGainsLife: "TriggeredPlayer", ir.EvLosesLife: "TriggeredPlayer", ir.EvUpkeep: "TriggeredPlayer",
		ir.EvEndStep: "TriggeredPlayer", ir.EvDraws: "TriggeredCardController",
		ir.EvDealsCombatDamageToPlayer: "TriggeredTarget", ir.EvDealsNoncombatDamageToPlayer: "TriggeredTarget",
		ir.EvCast: "TriggeredActivator",
	}
	for _, e := range sortedEvents(evPl) {
		c = append(c, refTestCase{name: "event player " + e.String(), ev: e, ref: ev(ir.RefEventPlayer), want: evPl[e]})
	}
	ctl := map[ir.EventKind]string{
		ir.EvBecomesTarget: "TriggeredSourceSAController", ir.EvCast: "TriggeredActivator",
		ir.EvEnters: "TriggeredCardController", ir.EvDies: "TriggeredCardController",
		ir.EvPutIntoGraveyard: "TriggeredCardController",
	}
	for _, e := range sortedEvents(ctl) {
		c = append(c, refTestCase{name: "controller of event object " + e.String(), ev: e,
			ref: of(ir.RefControllerOf, ev(ir.RefEventObject)), want: ctl[e]})
	}
	for _, k := range []ir.RefKind{ir.RefChoose, ir.RefTopOfLibrary, ir.RefReceipt, ir.RefIterPlayer, ir.RefIterObject,
		ir.RefUnion, ir.RefAttachedTo, ir.RefActivePlayer, ir.RefDefendingPlayer} {
		c = append(c, refTestCase{name: "unspelled " + k.String(), ref: ev(k), bad: "ref:" + k.String()})
	}
	return c
}

func sortedEvents[V any](m map[ir.EventKind]V) []ir.EventKind {
	var out []ir.EventKind
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func TestDefinedTable(t *testing.T) {
	for _, tc := range refTestDefinedCases() {
		p := refTestPr(tc.ev, tc.evSelf, tc.tgts...)
		got, err := p.defined(tc.ref)
		if tc.bad != "" {
			refTestUnprintable(t, err, tc.bad)
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("%s: got %q, %v, want %q", tc.name, got, err, tc.want)
		}
	}
	if _, err := (&pr{}).defined(nil); err == nil {
		t.Error("nil ref printed")
	}
}

// --- Valid lists ---

type refTestValid struct {
	name string
	f    *ir.Filter
	want string
	bad  string
}

func refTestValidCases() []refTestValid {
	P, C, O, S := ir.SubjPermanent, ir.SubjCard, ir.SubjObject, ir.SubjPlayer
	_ = O
	ctrl := func(pred ir.Pred, r *ir.Ref) *ir.Filter { return refTestRefPred(P, pred, r) }
	active := refTestRef(ir.RefActivePlayer)
	tgtPl := refTestTarget(0)
	notP := func(f *ir.Filter) *ir.Filter { return refTestNot(f) }
	w := func(subj ir.Subject, pred ir.Pred, word string) *ir.Filter { return refTestWord(subj, pred, word) }
	colour := func(c ir.ColorSet) *ir.Filter {
		return refTestPred(P, ir.PColor, func(f *ir.Filter) { f.Colors = c })
	}
	any4 := func(order ...int) *ir.Filter {
		parts := []*ir.Filter{
			w(P, ir.PCardType, "Creature"), w(P, ir.PCardType, "Planeswalker"), w(P, ir.PCardType, "Battle"),
			refTestPred(S, ir.PAny, nil),
		}
		var args []*ir.Filter
		for _, i := range order {
			args = append(args, parts[i])
		}
		return refTestAny(args...)
	}
	cases := []refTestValid{
		{name: "any perm", f: refTestPerm(ir.PAny, nil), want: "Permanent"},
		{name: "any card", f: refTestPred(C, ir.PAny, nil), want: "Card"},
		{name: "any spell", f: refTestPred(ir.SubjSpell, ir.PAny, nil), want: "Card"},
		{name: "any object", f: refTestPred(O, ir.PAny, nil), want: "Card"},
		{name: "any player", f: refTestPred(S, ir.PAny, nil), want: "Player"},
		{name: "no subject", f: refTestPred(0, ir.PAny, nil), bad: "filter:no-subject"},
		{name: "not any", f: notP(refTestPerm(ir.PAny, nil)), bad: "pred:any-not"},
		{name: "card type", f: w(P, ir.PCardType, "Creature"), want: "Creature"},
		{name: "not card type", f: notP(w(P, ir.PCardType, "Creature")), want: "Permanent.nonCreature"},
		{name: "subtype", f: w(P, ir.PSubtype, "Elf"), want: "Elf"},
		{name: "subtype respelled", f: w(P, ir.PSubtype, "TimeLord"), want: "Time Lord"},
		{name: "not subtype", f: notP(w(P, ir.PSubtype, "Elf")), want: "Permanent.nonElf"},
		{name: "supertype", f: w(P, ir.PSupertype, "Legendary"), want: "Permanent.Legendary"},
		{name: "not supertype", f: notP(w(P, ir.PSupertype, "Basic")), want: "Permanent.nonBasic"},
		{name: "permanent card", f: refTestPred(C, ir.PPermanentCard, nil), want: "Permanent"},
		{name: "not permanent card", f: notP(refTestPred(C, ir.PPermanentCard, nil)), bad: "pred:permanent-card-not"},
		{name: "white", f: colour(ir.ColorW), want: "Permanent.White"},
		{name: "blue", f: colour(ir.ColorU), want: "Permanent.Blue"},
		{name: "black", f: colour(ir.ColorB), want: "Permanent.Black"},
		{name: "red", f: colour(ir.ColorR), want: "Permanent.Red"},
		{name: "green", f: colour(ir.ColorG), want: "Permanent.Green"},
		{name: "not red", f: notP(colour(ir.ColorR)), want: "Permanent.nonRed"},
		{name: "multi colours", f: colour(ir.ColorR | ir.ColorG), bad: "pred:color-multi"},
		{name: "not multi colours", f: notP(colour(ir.ColorR | ir.ColorG)), bad: "pred:color-multi"},
		{name: "colorless", f: refTestPerm(ir.PColorless, nil), want: "Permanent.Colorless"},
		{name: "not colorless", f: notP(refTestPerm(ir.PColorless, nil)), want: "Permanent.nonColorless"},
		{name: "multicolored", f: refTestPerm(ir.PMulticolored, nil), want: "Permanent.MultiColor"},
		{name: "not multicolored", f: notP(refTestPerm(ir.PMulticolored, nil)), bad: "pred:multicolored-not"},
		{name: "mana value", f: refTestNum(P, ir.PManaValue, ir.CmpGE, 3), want: "Permanent.cmcGE3"},
		{name: "mana value eq", f: refTestNum(P, ir.PManaValue, ir.CmpEQ, 0), want: "Permanent.cmcEQ0"},
		{name: "not mana value", f: notP(refTestNum(P, ir.PManaValue, ir.CmpGE, 3)), bad: "pred:mana-value-not"},
		{name: "mana value no cmp", f: refTestPerm(ir.PManaValue, nil), bad: "pred:mana-value"},
		{name: "power", f: refTestNum(P, ir.PPower, ir.CmpLE, 2), want: "Permanent.powerLE2"},
		{name: "toughness", f: refTestNum(P, ir.PToughness, ir.CmpGT, 4), want: "Permanent.toughnessGT4"},
		{name: "not power", f: notP(refTestNum(P, ir.PPower, ir.CmpLE, 2)), bad: "pred:power-not"},
		{name: "not toughness", f: notP(refTestNum(P, ir.PToughness, ir.CmpLE, 2)), bad: "pred:toughness-not"},
		{name: "token", f: refTestPerm(ir.PToken, nil), want: "Permanent.token"},
		{name: "not token", f: notP(refTestPerm(ir.PToken, nil)), want: "Permanent.!token"},
		{name: "tapped", f: refTestPerm(ir.PTapped, nil), want: "Permanent.tapped"},
		{name: "not tapped", f: notP(refTestPerm(ir.PTapped, nil)), want: "Permanent.untapped"},
		{name: "attacking", f: refTestPerm(ir.PAttacking, nil), want: "Permanent.attacking"},
		{name: "blocking", f: refTestPerm(ir.PBlocking, nil), want: "Permanent.blocking"},
		{name: "not attacking", f: notP(refTestPerm(ir.PAttacking, nil)), bad: "pred:attacking-not"},
		{name: "not blocking", f: notP(refTestPerm(ir.PBlocking, nil)), bad: "pred:blocking-not"},
		{name: "has keyword", f: w(P, ir.PHasKeyword, "Flying"), want: "Permanent.withFlying"},
		{name: "not has keyword", f: notP(w(P, ir.PHasKeyword, "Flying")), want: "Permanent.withoutFlying"},
		{name: "counters", f: refTestPerm(ir.PCounters, func(f *ir.Filter) { f.Word, f.Cmp, f.Value = "P1P1", ir.CmpGE, refTestVal(1) }),
			want: "Permanent.counters_GE1_P1P1"},
		{name: "not counters", f: notP(refTestPerm(ir.PCounters, func(f *ir.Filter) { f.Word, f.Cmp, f.Value = "P1P1", ir.CmpGE, refTestVal(1) })),
			want: "Permanent.counters_LT1_P1P1"},
		{name: "name", f: w(C, ir.PName, "Island"), want: "Card.namedIsland"},
		{name: "not name", f: notP(w(C, ir.PName, "Island")), want: "Card.!namedIsland"},
		{name: "controlled by you", f: ctrl(ir.PControlledBy, refTestRef(ir.RefYou)), want: "Permanent.YouCtrl"},
		{name: "controlled by you, all form", f: ctrl(ir.PControlledBy, refTestAllPlayers(refTestYou())), want: "Permanent.YouCtrl"},
		{name: "controlled by opponents", f: ctrl(ir.PControlledBy, refTestAllPlayers(refTestOpp())), want: "Permanent.OppCtrl"},
		{name: "controlled by active", f: ctrl(ir.PControlledBy, active), want: "Permanent.ActivePlayerCtrl"},
		{name: "controlled by target", f: ctrl(ir.PControlledBy, tgtPl), want: "Permanent.TargetedPlayerCtrl"},
		{name: "controlled by self", f: ctrl(ir.PControlledBy, refTestRef(ir.RefSelf)), bad: "pred:controlled-by"},
		{name: "not controlled by", f: notP(ctrl(ir.PControlledBy, refTestRef(ir.RefYou))), bad: "pred:controlled-by-not"},
		{name: "owned by you", f: refTestRefPred(C, ir.POwnedBy, refTestRef(ir.RefYou)), want: "Card.YouOwn"},
		{name: "owned by opponents", f: refTestRefPred(C, ir.POwnedBy, refTestAllPlayers(refTestOpp())), want: "Card.OppOwn"},
		{name: "owned by target", f: refTestRefPred(C, ir.POwnedBy, tgtPl), want: "Card.TargetedPlayerOwn"},
		{name: "owned by active", f: refTestRefPred(C, ir.POwnedBy, active), bad: "pred:owned-by"},
		{name: "not owned by", f: notP(refTestRefPred(C, ir.POwnedBy, refTestRef(ir.RefYou))), bad: "pred:owned-by-not"},
		{name: "is self", f: refTestRefPred(P, ir.PIs, refTestRef(ir.RefSelf)), want: "Permanent.Self"},
		{name: "is other", f: notP(refTestRefPred(P, ir.PIs, refTestRef(ir.RefSelf))), want: "Permanent.Other"},
		{name: "is target", f: refTestRefPred(P, ir.PIs, tgtPl), bad: "pred:is"},
		{name: "is event object, not self", f: refTestRefPred(P, ir.PIs, refTestRef(ir.RefEventObject)), bad: "pred:is"},
		{name: "player is you", f: refTestYou(), want: "You"},
		{name: "player is active", f: refTestRefPred(S, ir.PPlayerIs, active), want: "Player.Active"},
		{name: "player is target", f: refTestRefPred(S, ir.PPlayerIs, tgtPl), bad: "pred:player-is"},
		{name: "player is opponents", f: refTestRefPred(S, ir.PPlayerIs, refTestAllPlayers(refTestOpp())), bad: "pred:player-is"},
		{name: "player is not active", f: notP(refTestRefPred(S, ir.PPlayerIs, active)), want: "Player.!Active"},
		{name: "opponent of you", f: refTestOpp(), want: "Opponent"},
		{name: "opponent of active", f: refTestRefPred(S, ir.POpponentOf, active), bad: "pred:opponent-of"},
		{name: "not opponent of", f: notP(refTestOpp()), bad: "pred:opponent-of-not"},
		{name: "attacked this turn", f: refTestPred(S, ir.PAttackedThisTurn, nil), bad: "pred:attacked-this-turn"},
		{name: "cards in graveyard", f: refTestPred(S, ir.PCardsInGraveyard, nil), bad: "pred:cards-in-graveyard"},
		{name: "cards in hand", f: refTestPred(S, ir.PCardsInHand, nil), bad: "pred:cards-in-hand"},
		{name: "life total", f: refTestPred(S, ir.PLifeTotal, nil), bad: "pred:life-total"},
		{name: "controls", f: refTestPred(S, ir.PControls, nil), bad: "pred:controls"},
		{name: "none pred", f: refTestPred(P, ir.PNone, nil), bad: "pred:none"},
	}
	cases = append(cases,
		refTestValid{name: "all: type and two qualifiers", want: "Creature.YouCtrl+!token", f: refTestAll(P,
			w(P, ir.PCardType, "Creature"), ctrl(ir.PControlledBy, refTestRef(ir.RefYou)), refTestNot(refTestPerm(ir.PToken, nil)))},
		refTestValid{name: "all: later type words are qualifiers", want: "Creature.Artifact+Elf", f: refTestAll(P,
			w(P, ir.PCardType, "Creature"), w(P, ir.PCardType, "Artifact"), w(P, ir.PSubtype, "Elf"))},
		refTestValid{name: "all: repeated qualifier once", want: "Permanent.token", f: refTestAll(P,
			refTestPerm(ir.PToken, nil), refTestPerm(ir.PToken, nil))},
		refTestValid{name: "any: two types", want: "Creature,Planeswalker", f: refTestAny(
			w(P, ir.PCardType, "Creature"), w(P, ir.PCardType, "Planeswalker"))},
		refTestValid{name: "any target", want: "Any", f: any4(0, 1, 2, 3)},
		refTestValid{name: "any target, shuffled", want: "Any", f: any4(3, 2, 0, 1)},
		refTestValid{name: "not quite any target", want: "Creature,Planeswalker,Player", f: any4(0, 1, 3)},
		refTestValid{name: "all of any, 2x2", want: "Creature.YouCtrl,Creature.OppCtrl,Artifact.YouCtrl,Artifact.OppCtrl",
			f: refTestAll(P,
				refTestAny(w(P, ir.PCardType, "Creature"), w(P, ir.PCardType, "Artifact")),
				refTestAny(ctrl(ir.PControlledBy, refTestRef(ir.RefYou)), ctrl(ir.PControlledBy, refTestAllPlayers(refTestOpp()))))},
		refTestValid{name: "too wide", bad: "filter:too-wide", f: refTestAll(P,
			refTestAny(w(P, ir.PCardType, "Creature"), w(P, ir.PCardType, "Artifact"), w(P, ir.PCardType, "Land")),
			refTestAny(w(P, ir.PSubtype, "Elf"), w(P, ir.PSubtype, "Goblin"), w(P, ir.PSubtype, "Human")))},
		refTestValid{name: "not of a compound", bad: "filter:not", f: refTestNot(refTestAll(P,
			w(P, ir.PCardType, "Creature"), refTestPerm(ir.PToken, nil)))},
		refTestValid{name: "not of two", bad: "filter:not", f: &ir.Filter{Op: ir.FNot, Args: nil}},
		refTestValid{name: "player: you and tapped-free", want: "Player.Active+!Active", f: refTestAll(S,
			refTestRefPred(S, ir.PPlayerIs, active), refTestNot(refTestRefPred(S, ir.PPlayerIs, active)))},
		refTestValid{name: "empty any", bad: "filter:empty", f: refTestAny()},
	)
	return cases
}

func TestValidTerms(t *testing.T) {
	for _, tc := range refTestValidCases() {
		got, err := refTestPr(ir.EvNone, false, refTestPlayerTarget()).valid(tc.f)
		if tc.bad != "" {
			refTestUnprintable(t, err, tc.bad)
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("%s: got %q, %v, want %q", tc.name, got, err, tc.want)
		}
	}
	if _, err := (&pr{}).valid(nil); err == nil {
		t.Error("nil filter printed")
	}
}

// --- target declarations ---

func TestTargetParams(t *testing.T) {
	P, C, S := ir.SubjPermanent, ir.SubjCard, ir.SubjPlayer
	anyTgt := refTestAny(refTestWord(P, ir.PCardType, "Creature"), refTestWord(P, ir.PCardType, "Planeswalker"),
		refTestWord(P, ir.PCardType, "Battle"), refTestPred(S, ir.PAny, nil))
	gy := refTestAll(C, refTestWord(C, ir.PCardType, "Creature"), refTestRefPred(C, ir.POwnedBy, refTestRef(ir.RefYou)))
	gy.Zones = ir.MakeZoneSet(ir.ZoneGraveyard)
	spell := refTestPred(ir.SubjSpell, ir.PAny, nil)
	spell.Zones = ir.MakeZoneSet(ir.ZoneStack)
	bf := refTestWord(P, ir.PCardType, "Creature")
	bf.Zones = ir.MakeZoneSet(ir.ZoneBattlefield)
	cases := []struct {
		name string
		t    *ir.Target
		want []kv
		bad  string
	}{
		{"any target", &ir.Target{Min: 1, Max: 1, Filter: anyTgt}, []kv{{"ValidTgts", "Any"}}, ""},
		{"spell", &ir.Target{Min: 1, Max: 1, Filter: spell}, []kv{{"ValidTgts", "Card"}, {"TargetType", "Spell"}}, ""},
		{"graveyard card, up to one", &ir.Target{Min: 0, Max: 1, Filter: gy},
			[]kv{{"ValidTgts", "Creature.YouOwn"}, {"TgtZone", "Graveyard"}, {"TargetMin", "0"}, {"TargetMax", "1"}}, ""},
		{"battlefield zone is not printed", &ir.Target{Min: 1, Max: 1, Filter: bf}, []kv{{"ValidTgts", "Creature"}}, ""},
		{"value max", &ir.Target{Min: 0, Max: 0, MaxV: refTestVal(3), Filter: refTestWord(P, ir.PCardType, "Creature")},
			[]kv{{"ValidTgts", "Creature"}, {"TargetMin", "0"}, {"TargetMax", "3"}}, ""},
		{"distinct", &ir.Target{Min: 2, Max: 2, Distinct: true, Filter: refTestWord(P, ir.PCardType, "Creature")},
			[]kv{{"ValidTgts", "Creature"}, {"TargetMin", "2"}, {"TargetMax", "2"}, {"TargetUnique", "True"}}, ""},
		{"no filter", &ir.Target{Min: 1, Max: 1}, nil, "target:no-filter"},
		{"unprintable filter", &ir.Target{Min: 1, Max: 1, Filter: refTestPred(P, ir.PLifeTotal, nil)}, nil, "pred:life-total"},
	}
	for _, tc := range cases {
		got, err := (&pr{}).target(tc.t)
		if tc.bad != "" {
			refTestUnprintable(t, err, tc.bad)
			continue
		}
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %v, %v, want %v", tc.name, got, err, tc.want)
		}
	}
	if _, err := (&pr{}).target(nil); err == nil {
		t.Error("nil target printed")
	}
}

// --- attestation against the corpus ---

// corpusParams is the spellings the Forge corpus uses.
type corpusParamsSet struct {
	defined map[string]bool // values of the Defined-like parameters
	bases   map[string]bool // Valid list base words
	quals   map[string]bool // Valid list qualifier tokens
}

var corpusParamsDefinedKeys = []string{"Defined", "DefinedPlayer", "Affected", "ValidPlayer", "Activator", "UnlessPayer", "Controller"}

func corpusParamsIsValidKey(k string) bool {
	switch k {
	case "ChangeType", "ChangeValid", "Affected", "IsPresent", "ConditionPresent":
		return true
	}
	return strings.HasPrefix(k, "Valid")
}

func corpusParamsCollect(reg *cards.Registry) *corpusParamsSet {
	s := &corpusParamsSet{defined: map[string]bool{}, bases: map[string]bool{}, quals: map[string]bool{}}
	params := func(m map[string]string) {
		for _, k := range corpusParamsDefinedKeys {
			if v, ok := m[k]; ok {
				s.defined[v] = true
			}
		}
		for k, v := range m { // order-free: only fills sets
			if !corpusParamsIsValidKey(k) && k != "ValidPlayer" && k != "Activator" && k != "UnlessPayer" {
				continue
			}
			for _, term := range strings.Split(v, ",") {
				base, rest, dot := strings.Cut(term, ".")
				s.bases[base] = true
				if dot {
					for _, q := range strings.Split(rest, "+") {
						s.quals[q] = true
					}
				}
			}
		}
	}
	var sa func(a *cards.SA)
	sa = func(a *cards.SA) {
		for ; a != nil; a = a.Sub {
			params(a.Params)
		}
	}
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			for _, a := range f.Abilities {
				sa(a)
			}
			for _, tr := range f.Triggers {
				params(tr.Params)
				sa(tr.Effect)
			}
			for _, st := range f.Statics {
				params(st.Params)
			}
			for _, r := range f.Repls {
				params(r.Params)
				sa(r.With)
			}
		}
	}
	return s
}

// corpusParamsFamily maps a parametric token to its family: digits and the
// operand drop out (cmcGE3 is cmcGE, namedIsland is named).
func corpusParamsFamily(tok string) string {
	for _, pre := range []string{"named", "!named"} {
		if strings.HasPrefix(tok, pre) && len(tok) > len(pre) {
			return pre
		}
	}
	for _, pre := range []string{"cmc", "power", "toughness"} {
		if strings.HasPrefix(tok, pre) && len(tok) >= len(pre)+3 {
			return tok[:len(pre)+2]
		}
	}
	if strings.HasPrefix(tok, "counters_") && len(tok) > len("counters_GE") {
		return tok[:len("counters_GE")]
	}
	return tok
}

// corpusParamsAttested reports whether tok is a collected qualifier. The
// registry loads Forge's "Player.!Active" as "Player.NonActive", so a leading
// "!" on a player word is attested by either form.
func corpusParamsAttested(got *corpusParamsSet, families map[string]bool, tok string) bool {
	if families[corpusParamsFamily(tok)] {
		return true
	}
	if w, ok := strings.CutPrefix(tok, "!"); ok {
		return got.quals["Non"+w]
	}
	return false
}

func TestSpellingsAttested(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	got := corpusParamsCollect(reg)
	if len(got.defined) == 0 || len(got.quals) == 0 {
		t.Fatalf("collected %d defined values, %d qualifiers", len(got.defined), len(got.quals))
	}
	families := map[string]bool{}
	for q := range got.quals {
		families[corpusParamsFamily(q)] = true
	}
	var missing []string
	for _, tc := range refTestDefinedCases() {
		if tc.bad == "" && !got.defined[tc.want] {
			missing = append(missing, "Defined$ "+tc.want)
		}
	}
	for _, tc := range refTestValidCases() {
		if tc.bad != "" {
			continue
		}
		for _, term := range strings.Split(tc.want, ",") {
			base, rest, dot := strings.Cut(term, ".")
			if !got.bases[base] && !got.quals[base] {
				missing = append(missing, "base "+base)
			}
			if dot {
				for _, q := range strings.Split(rest, "+") {
					if !corpusParamsAttested(got, families, q) {
						missing = append(missing, "qualifier "+q)
					}
				}
			}
		}
	}
	sort.Strings(missing)
	var uniq []string
	for i, m := range missing {
		if i == 0 || m != missing[i-1] {
			uniq = append(uniq, m)
		}
	}
	if len(uniq) > 0 {
		t.Errorf("unattested spellings: %s", strings.Join(uniq, "; "))
	}
	t.Logf("%d Defined values, %d bases, %d qualifiers", len(got.defined), len(got.bases), len(got.quals))
}

// --- the proof slice ---

func refTestCollectTargets(v reflect.Value, out *[]*ir.Target) {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return
		}
		if t, ok := v.Interface().(*ir.Target); ok {
			*out = append(*out, t)
			return
		}
		refTestCollectTargets(v.Elem(), out)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			refTestCollectTargets(v.Field(i), out)
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			refTestCollectTargets(v.Index(i), out)
		}
	}
}

func TestProofSliceRefsFilters(t *testing.T) {
	testutil.CorpusRegistry(t)
	var targets []*ir.Target
	for _, c := range sliceCards(t) {
		refused := false
		for _, f := range c.Faces {
			if len(f.Refusals) > 0 {
				refused = true
			}
		}
		if !refused {
			refTestCollectTargets(reflect.ValueOf(c), &targets)
		}
	}
	printed := 0
	counts := map[string]int{}
	for _, tg := range targets {
		p := refTestPr(ir.EvNone, false, tg)
		_, err := p.target(tg)
		if err == nil {
			printed++
			continue
		}
		var u *Unprintable
		if !errors.As(err, &u) {
			t.Fatalf("not an Unprintable: %v", err)
		}
		counts[u.What]++
	}
	var names []string
	for w := range counts {
		names = append(names, w)
	}
	sort.Strings(names)
	t.Logf("%d target declarations, %d printed", len(targets), printed)
	for _, w := range names {
		t.Logf("  %s: %d", w, counts[w])
	}
	if printed < 23 {
		t.Errorf("printed %d of %d declarations, want at least 23", printed, len(targets))
	}
}
