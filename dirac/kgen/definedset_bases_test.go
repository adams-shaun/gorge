package kgen

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// setBaseTrigger is a synthetic card whose trigger (the T: line mode) runs
// body, the shape each referent's Defined$ arm is gated on.
func setBaseTrigger(mode, body string) string {
	return definedSetSrc("T:"+mode+" | TriggerZones$ Battlefield | Execute$ ProbeTrig", "SVar:ProbeTrig:"+body)
}

// setBaseRows are the shapes that lower onto a SetRefBase or SetBattlefield
// program: the SET line each leaves. Each also builds (TestSetBaseShapes).
func setBaseRows() []struct{ name, src, set string } {
	const attacks = "Mode$ Attacks | ValidCard$ Card.Self"
	const upkeep = "Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You"
	return []struct{ name, src, set string }{
		{"owner of the enchanted permanent", setBaseTrigger(upkeep, "DB$ DealDamage | Defined$ EnchantedOwner | NumDmg$ 1"),
			"SET 0 SetRefBase(Equipped) SetAndFilter(F="},
		{"complement of the enchanted permanent's controller", setBaseTrigger(upkeep, "DB$ LoseLife | Defined$ NonEnchantedController | LifeAmount$ 1"),
			"SET 0 SetRefBase(EnchantedCtrl) SetPlayersComplement"},
		{"complement of the defending player", setBaseTrigger(attacks, "DB$ LoseLife | Defined$ NonTriggeredDefendingPlayer | LifeAmount$ 1"),
			"SET 0 SetRefBase(Defending) SetPlayersComplement"},
		{"complement of the triggering target's controller", setBaseTrigger("Mode$ DamageDone | ValidSource$ Card.Self | ValidTarget$ Creature",
			"DB$ LoseLife | Defined$ NonTriggeredTargetController | LifeAmount$ 1"),
			"SET 0 SetRefBase(TriggeredTgtCtrl) SetPlayersComplement"},
		{"an untap sweep", definedSetSrc("A:AB$ Untap | Cost$ T | Defined$ Valid Creature.YouCtrl"),
			"SET 0 SetBattlefield SetAndFilter(F="},
		{"the enchanted creature by filter", setBaseTrigger("Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self",
			"DB$ DealDamage | Defined$ Valid Card.EnchantedBy | NumDmg$ 2"),
			"SET 0 SetBattlefield SetAndFilter(F="},
		{"a tap sweep", definedSetSrc("A:AB$ Tap | Cost$ 1 | Defined$ Valid Creature.YouDontCtrl"),
			"SET 0 SetBattlefield SetAndFilter(F="},
		{"destroying the enchanted creature by filter", setBaseTrigger(upkeep, "DB$ Destroy | Defined$ Valid Card.EnchantedBy"),
			"SET 0 SetBattlefield SetAndFilter(F="},
	}
}

// setObjectRows are Sacrifice and Discard over a set holding an object, the
// bearer sweep and a qualified base, and the reason the builder's walk guard
// gives for each: the kernel does not finish such a member's run as gorge
// does.
func setObjectRows() []struct{ name, src, reason string } {
	const upkeep = "Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You"
	const sac = "Defined$ set holding an object on kop Sacrifice: the kernel does not finish its run as gorge does"
	const disc = "Defined$ set holding an object on kop Discard: the kernel does not finish its run as gorge does"
	var rows []struct{ name, src, reason string }
	for _, def := range []struct{ name, v string }{{"the bearer", "Valid Card.EnchantedBy"}, {"a qualified source", "Self.Creature"}} {
		for _, body := range []struct{ name, api, params, reason string }{
			{"sacrificing itself", "Sacrifice", "SacValid$ Self", sac},
			{"sacrificing a land", "Sacrifice", "SacValid$ Land", sac},
			{"discarding", "Discard", "Mode$ TgtChoose | NumCards$ 1", disc},
		} {
			rows = append(rows, struct{ name, src, reason string }{def.name + " " + body.name,
				setBaseTrigger(upkeep, "DB$ "+body.api+" | Defined$ "+def.v+" | "+body.params), body.reason})
		}
	}
	return rows
}

// TestSetBaseShapes: a referent defined() lowers that has no set kind of its
// own is wrapped in SetRefBase, Valid <filter> is a battlefield sweep, and the
// zone sweeps, the referents the kernel's set arm cannot read and a set under
// an op that takes none refuse by name. A shape that lowers must also build:
// the builder's walk guard (build_setexpr.go walksSet) refuses a set of more
// than one member under an op whose member run can ask, and any set holding an
// object under Sacrifice or Discard.
func TestSetBaseShapes(t *testing.T) {
	for _, tc := range setBaseRows() {
		t.Run(tc.name, func(t *testing.T) {
			c, lir := definedSetLIR(t, tc.src)
			if len(c.Refusals) != 0 {
				t.Fatalf("refused: %s\n%s", abReasons(c), lir)
			}
			if !strings.Contains(lir, tc.set) {
				t.Fatalf("no %q line:\n%s", tc.set, lir)
			}
			if !strings.Contains(lir, "obj=Set0") {
				t.Fatalf("no instruction reads Set0:\n%s", lir)
			}
			pt := newPoolTables(&lowered{cards: []*lCard{c}}, nil)
			if b := buildCard(pt, c); len(b.refusals) != 0 || len(pt.sets) != 1 {
				t.Fatalf("lowered but the builder refused: %+v (sets %v)", b.refusals, pt.sets)
			}
		})
	}
	buildRefuses := []struct{ name, src, reason string }{
		// a damage sweep of several creatures lowers, but one member's damage
		// can ask, which ends the kernel's walk
		{"a damage sweep", definedSetSrc("A:AB$ DealDamage | Cost$ T | Defined$ Valid Creature.YouCtrl | NumDmg$ 1"),
			`the battlefield walk order differs from gorge's on DealDamage`},
	}
	for _, tc := range buildRefuses {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := definedSetLIR(t, tc.src)
			if got := abReasons(c); !strings.HasSuffix(strings.TrimSpace(got), tc.reason) {
				t.Fatalf("want a lowering refusal ending %q, got:\n%s", tc.reason, got)
			}
		})
	}
	for _, tc := range setObjectRows() {
		t.Run(tc.name, func(t *testing.T) {
			c, lir := definedSetLIR(t, tc.src)
			if len(c.Refusals) != 0 || !strings.Contains(lir, "obj=Set0") || len(c.Sets) != 1 || c.Sets[0].Players || !c.Sets[0].One {
				t.Fatalf("precondition: a lowered read of one set of at most one object, refusals %q\n%s", abReasons(c), lir)
			}
			pt := newPoolTables(&lowered{cards: []*lCard{c}}, nil)
			b := buildCard(pt, c)
			if len(pt.sets) != 0 || !slices.ContainsFunc(b.refusals, func(r refusal) bool { return strings.HasSuffix(r.Reason, tc.reason) }) {
				t.Fatalf("want a build refusal ending %q, got %+v (sets %v)", tc.reason, b.refusals, pt.sets)
			}
		})
	}
	// a set of players under the same two ops still builds
	for _, tc := range []struct{ name, body string }{
		{"players sacrificing", "DB$ Sacrifice | Defined$ NonEnchantedController | SacValid$ Land"},
		{"one player discarding", "DB$ Discard | Defined$ EnchantedOwner | Mode$ TgtChoose | NumCards$ 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, lir := definedSetLIR(t, setBaseTrigger("Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You", tc.body))
			if len(c.Refusals) != 0 || len(c.Sets) != 1 || !c.Sets[0].Players {
				t.Fatalf("precondition: a lowered player set, refusals %q\n%s", abReasons(c), lir)
			}
			pt := newPoolTables(&lowered{cards: []*lCard{c}}, nil)
			if b := buildCard(pt, c); len(b.refusals) != 0 || len(pt.sets) != 1 {
				t.Fatalf("lowered but the builder refused: %+v (sets %v)", b.refusals, pt.sets)
			}
		})
	}
	loseLife := func(v string) string {
		return definedSetSrc("A:AB$ LoseLife | Cost$ T | Defined$ " + v + " | LifeAmount$ 1")
	}
	tap := func(v string) string { return definedSetSrc("A:AB$ Tap | Cost$ 1 | Defined$ " + v) }
	newCardMove := func(dest string) string {
		return setBaseTrigger("Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Creature.YouCtrl",
			"DB$ ChangeZone | Origin$ Graveyard | Destination$ "+dest+" | Defined$ TriggeredNewCard.Creature")
	}
	refuses := []struct{ name, src, reason string }{
		{"graveyard sweep", tap("ValidGraveyard Creature"), `Defined$ "ValidGraveyard Creature": zone base ValidGraveyard`},
		{"exile sweep", tap("ValidExile Creature"), `Defined$ "ValidExile Creature": zone base ValidExile`},
		{"hand sweep", tap("ValidHand Creature"), `Defined$ "ValidHand Creature": zone base ValidHand`},
		{"library sweep", tap("ValidLibrary Creature"), `Defined$ "ValidLibrary Creature": zone base ValidLibrary`},
		{"stack sweep", tap("ValidStack Spell"), `Defined$ "ValidStack Spell": zone base ValidStack`},
		{"a host", loseLife("OriginalHostController"), `Defined$ "OriginalHostController": base Host has no set kind`},
		{"the remembered set", loseLife("RememberedOwner"), `Defined$ "RememberedOwner": base Remembered has no set kind`},
		// gorge's ChosenCard names every picked card, kop.RefChosen the first
		// a zone move keys its origin guard on the referent kind, which a set
		// subject hides (instr_zones.go)
		{"a qualified new card to hand", newCardMove("Hand"), `Defined$ "TriggeredNewCard.Creature": base TriggeredNew has no set kind`},
		{"a qualified new card to the library", newCardMove("Library | LibraryPosition$ 0"), `Defined$ "TriggeredNewCard.Creature": base TriggeredNew has no set kind`},
		{"a qualified new card to exile", newCardMove("Exile"), `Defined$ "TriggeredNewCard.Creature": base TriggeredNew has no set kind`},
		{"controller of the chosen card", loseLife("ChosenCardController"), `Defined$ "ChosenCardController": base Chosen has no set kind`},
		{"controller of two chosen cards", definedSetSrc(
			"A:AB$ ChooseCard | Cost$ T | Defined$ You | Choices$ Creature | Amount$ 2 | Mandatory$ True | SubAbility$ ProbeTwo",
			"SVar:ProbeTwo:DB$ LoseLife | Defined$ ChosenCardController | LifeAmount$ 2"),
			`Defined$ "ChosenCardController": base Chosen has no set kind`},
		{"a sweep beside the source", tap("Self & Valid Creature"), `Defined$ "Self & Valid Creature": the parts may name the same object`},
		{"a destroy sweep", definedSetSrc("A:AB$ Destroy | Cost$ 1 | Defined$ Valid Creature"),
			`Defined$ "Valid Creature": the battlefield walk order differs from gorge's on Destroy`},
		{"a life loss over a sweep", loseLife("Valid Creature"),
			`Defined$ "Valid Creature": the battlefield walk order differs from gorge's on LoseLife`},
		{"a destroy sweep of more than the bearer", definedSetSrc("A:AB$ Destroy | Cost$ 1 | Defined$ Valid Card.EnchantedBy,Creature.YouCtrl"),
			`Defined$ "Valid Card.EnchantedBy,Creature.YouCtrl": the battlefield walk order differs from gorge's on Destroy`},
		{"a sweep under an op that takes none", definedSetSrc("A:AB$ Mill | Cost$ T | Defined$ Valid Creature | NumCards$ 1"),
			`Defined$ "Valid Creature": a set read outside an op that takes one`},
	}
	for _, tc := range refuses {
		t.Run(tc.name, func(t *testing.T) {
			c, lir := definedSetLIR(t, tc.src)
			if strings.Contains(lir, "  SET ") {
				t.Fatalf("a refused shape kept a SET line:\n%s", lir)
			}
			got := abReasons(c)
			if !slices.ContainsFunc(strings.Split(got, "\n"), func(r string) bool { return strings.HasSuffix(r, tc.reason) }) {
				t.Fatalf("want a refusal ending %q, got:\n%s", tc.reason, got)
			}
		})
	}
	t.Run("corpus carriers", testSetBaseCarriers)
}

// testSetBaseCarriers: the corpus carriers read their sets, and what still
// refuses them is another line: Enslave's DamageSource$ amount, Earthbind's
// granted static and its self-enters condition. Listing every blocker lowers
// the bodies behind them.
func testSetBaseCarriers(t *testing.T) {
	carriers := []struct{ card, set, blocker string }{
		{"Enslave", "SET 0 SetRefBase(Equipped) SetAndFilter(F={types=0x0 EnchantedBy}) SetOwnerOf",
			"DamageSource$ with an amount other than the source's power"},
		{"Earthbind", "SET 0 SetBattlefield SetAndFilter(F={types=0x0 EnchantedBy})", "Animate staticAbilities$"},
	}
	p, err := loadCardNames([]string{"Earthbind", "Enslave"}, ratchetCards(t))
	if err != nil {
		t.Fatal(err)
	}
	p.allBlockers = true
	byName := map[string]*lCard{}
	for _, c := range lowerPool(p).cards {
		byName[c.Name] = c
	}
	for _, f := range carriers {
		c := byName[f.card]
		if c == nil {
			t.Errorf("%s: not lowered", f.card)
			continue
		}
		var b bytes.Buffer
		dumpLIR(&b, c)
		if !strings.Contains(b.String(), f.set) {
			t.Errorf("%s: no %q line:\n%s", f.card, f.set, b.String())
		}
		got := abReasons(c)
		if !strings.Contains(got, f.blocker) || strings.Contains(got, "no set kind") || strings.Contains(got, `Defined$ "`) {
			t.Errorf("%s: want only other blockers, among them %q, got:\n%s", f.card, f.blocker, got)
		}
	}
}

// TestSetBaseRefsAllowed: the refs kgen wraps in SetRefBase are the kernel's
// allowlist less those with a set kind of their own, and the builder refuses a
// ref outside it rather than emit one the kernel panics on.
func TestSetBaseRefsAllowed(t *testing.T) {
	cb := &cardBuilder{}
	wrapped := map[kop.Ref]bool{}
	for k, players := range setRefPlayers {
		r, err := cb.ref(lRef{Kind: k})
		if err != nil || !kop.SetRefBaseOK(r) {
			t.Errorf("%s builds to ref %d (%v), which the kernel's set arm refuses", refNames[k], r, err)
		}
		wrapped[r] = true
		term, err := setRefBaseTerm("v", lRef{Kind: k})
		if err != nil || term.players != players || !term.one {
			t.Errorf("%s: term %+v, %v", refNames[k], term, err)
		}
	}
	own := map[kop.Ref]bool{kop.RefSelf: true, kop.RefController: true, kop.RefTriggerObj: true, kop.RefTriggerPlayer: true, kop.RefTriggerSource: true}
	for r := 0; r < 256; r++ {
		if ref := kop.Ref(r); kop.SetRefBaseOK(ref) && !wrapped[ref] && !own[ref] {
			t.Errorf("ref %d is allowed but no kgen base reaches it", r)
		}
	}
	// TriggeredDefenderOrPlaneswalker's rDefending says more than its ref
	if _, err := setRefBaseTerm("v", lRef{Kind: rDefending, N: 1}); err == nil {
		t.Error("a ref with an N was wrapped")
	}
	// RefChosen reads one chosen card, gorge's ChosenCard all of them
	if _, err := setRefBaseTerm("v", lRef{Kind: rChosen}); err == nil {
		t.Error("rChosen was wrapped")
	}
	// a program naming a ref outside the list refuses at build time
	c := abLower(t, setBaseTrigger("Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You", "DB$ LoseLife | Defined$ NonEnchantedController | LifeAmount$ 1"), false)
	if len(c.Refusals) != 0 || len(c.Sets) != 1 || c.Sets[0].Ops[0].Kind != kop.SetRefBase {
		t.Fatalf("precondition: one lowered SetRefBase set, refusals %q", abReasons(c))
	}
	c.Sets[0].Ops[0].Base = lRef{Kind: rSacrificed}
	pt := newPoolTables(&lowered{cards: []*lCard{c}}, nil)
	built := buildCard(pt, c)
	if !slices.ContainsFunc(built.refusals, func(r refusal) bool {
		return strings.Contains(r.Reason, `Defined$ "EnchantedController": base Sacrificed has no set kind`)
	}) {
		t.Errorf("a ref outside the list was not refused: %+v", built.refusals)
	}
	c.Sets[0].Ops[0].Base = lRef{Kind: rEnchantedCtrl}
	pt = newPoolTables(&lowered{cards: []*lCard{c}}, nil)
	if b := buildCard(pt, c); len(b.refusals) != 0 || len(pt.sets) != 1 || pt.sets[0][0] != (kop.SetOp{Kind: kop.SetRefBase, Arg: uint16(kop.RefEnchantedCtrl)}) {
		t.Errorf("built sets %v, refusals %+v", pt.sets, b.refusals)
	}
}
