package dirac

import (
	"errors"
	"math/rand/v2"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The classes, spelled out by name apart from the table they test, so a reason
// moving class is an edit in two places a reviewer sees. Capacity: a fixed array or
// budget ran out and gorge has no such limit. Unmodelled: gorge does something here
// the kernel does not.
var (
	capacityStalls = []string{"twoseat", "ctradded", "addphase", "untaplock", "turnrepl", "dmgmark", "board", "pending", "stack", "tables", "loop", "delayed", "remember", "imprint",
		"mayplay", "dprog", "emblem", "dmgmap", "xtgt", "used", "kwrepx", "links", "free", "ask", "counters", "lki", "tappick",
		"regen", "look", "grants", "acts", "linger", "sac", "entered", "convoke", "castring", "pt", "actring", "charmopts",
		"reflatch", "zone", "options", "srcmark", "dmglatch", "replcands", "replplusbatch", "prevcol", "colgrant", "restrx", "obssrc", "dynmana", "tapextra", "planalts",
		"exiledwith", "breach", "netlock", "panharmgrant", "exilegrant", "charmpick", "combathit", "finality", "persistmana", "optmoves", "mustblock",
		"restrsrc", "firebend", "exhaste", "effetbctr", "atkGrp", "paradigm", "filtx", "delve", "embrem", "dmgamt", "choosenumber"}
	unmodelledStalls = []string{"infect", "replplus", "liferepl", "drawrepl", "prolif", "ctrorder",
		"shield", "replctr", "replleave", "copyface", "rolemulti", "defenderpw", "tokatkask", "enterdefender",
		"endure", "linkctr", "numtimes", "aegis", "sacother", "copycast", "netmana", "tokenpt", "staticask", "handsize",
		"mayraise", "uncounterorder", "entryorder", "copyallask", "clonelayer", "gravepower", "landgrant",
		"outsidetype", "exileonmoved", "multiretarget", "l6walk", "lowering", "tokrewrite", "stackkwgrant", "paylifeshared", "noprevent", "dmgmarkorder", "fbgrantstamp", "restrdefer", "diamondmix", "extrablockreq", "paybeforeetb", "atktax", "blktax", "prevnext", "chooseplayerorder"}
)

// Every registered reason has a class, and it is the one the lists above name.
func TestEveryStallHasAClass(t *testing.T) {
	seen := map[string]bool{}
	for r, name := range stallNames {
		if name == "" {
			continue
		}
		seen[name] = true
		c := StallClassOf(uint8(r))
		if c != StallCapacity && c != StallUnmodelled {
			t.Errorf("reason %d (%s) has no class: add it to stallCapacityReasons or stallUnmodelledReasons with what it means", r, name)
		}
		if want := slices.Contains(capacityStalls, name); want && c != StallCapacity {
			t.Errorf("reason %d (%s) is class %d, want capacity", r, name, c)
		}
		if want := slices.Contains(unmodelledStalls, name); want && c != StallUnmodelled {
			t.Errorf("reason %d (%s) is class %d, want unmodelled", r, name, c)
		}
		if !slices.Contains(capacityStalls, name) && !slices.Contains(unmodelledStalls, name) {
			t.Errorf("reason %d (%s) is in neither list of this test", r, name)
		}
	}
	for _, n := range append(slices.Clone(capacityStalls), unmodelledStalls...) {
		if !seen[n] {
			t.Errorf("%s is listed here but registers no reason", n)
		}
	}
}

// A reason nobody classed fails closed: it cannot pass as a declared limit.
func TestUnclassedStallIsNotCapacity(t *testing.T) {
	for r := 0; r < NumStall; r++ {
		if stallNames[r] == "" && StallClassOf(uint8(r)) != stallUnclassed {
			t.Errorf("slot %d is classed with no name", r)
		}
	}
	if StallIsUnmodelled(StallNone) {
		t.Error("no stall is an unmodelled stall")
	}
	if !StallIsUnmodelled(NumStall) || !StallIsUnmodelled(255) {
		t.Error("a reason past the table must not read as capacity")
	}
	if StallIsUnmodelled(StallBoard) || !StallIsUnmodelled(StallInfect) {
		t.Error("board is capacity, infect is not")
	}
}

// A site that stops for a rule gap must not borrow a capacity reason's name. Each
// row is a site that once did: the reason it uses now and the class that makes it
// fail, read from the source so moving one back fails here.
func TestRuleGapSitesKeepTheirOwnReason(t *testing.T) {
	for _, c := range []struct {
		file, reason string
		want, gone   string
	}{
		{"act_choosetype.go", "StallOutsideType", "s.capFull(StallOutsideType)", "s.capFull(StallBoard)"},
		{"cast_mayplayremembered.go", "StallExileOnMoved", "s.capFull(StallExileOnMoved)", ""},
		{"act_stackrewrite.go", "StallMultiRetarget", "s.capFull(StallMultiRetarget)", "s.capFull(StallTables)"},
		{"act_optionaldecider.go", "StallLowering", "s.capFull(StallLowering)", "s.capFull(StallBoard)"},
		{"copy_riders.go", "StallLowering", "s.capFull(StallLowering) // kgen interns", ""},
		{"ctr_addedthisturn.go", "StallLowering", "s.capFull(StallLowering) // a reader", ""},
		{"act_addphase.go", "StallLowering", "s.capFull(StallLowering) // a second phase", ""},
		{"copy_counters.go", "StallCopyAllAsk", "s.stall(StallCopyAllAsk)", ""},
	} {
		b := readSource(t, c.file)
		if !strings.Contains(b, c.want) {
			t.Errorf("%s: no %q", c.file, c.want)
		}
		if c.gone != "" && strings.Contains(b, c.gone) {
			t.Errorf("%s: %q is back", c.file, c.gone)
		}
	}
	for _, r := range []uint8{StallOutsideType, StallExileOnMoved, StallMultiRetarget, StallL6Walk, StallLowering, StallCopyAllAsk} {
		if StallClassOf(r) != StallUnmodelled {
			t.Errorf("%s: class %d, want unmodelled", StallName(r), StallClassOf(r))
		}
	}
}

func readSource(t *testing.T, file string) string {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// stallPool is a hand-written pool holding the two halves of one interaction and
// vanilla filler, so every flag has a table: no registered pool holds most of them,
// which is the point of refusing the pair.
func stallPool(name string, halves []kop.CardDef, overrides []kop.Override, filters []kop.Filter, staticSubs map[int][]cardset.AbSubs) cardset.Set {
	filler := func(n string) kop.CardDef {
		return kop.CardDef{Name: n, Types: kop.TCreature, Power: 1, Toughness: 1}
	}
	defs := append(slices.Clone(halves), filler("Vanilla A"), filler("Vanilla B"), filler("Vanilla C"), filler("Vanilla D"))
	slices.SortFunc(defs, func(a, b kop.CardDef) int { return strings.Compare(a.Name, b.Name) })
	d := &cardset.PoolData{Name: name, NumCards: len(defs), NumIDs: len(defs), Subtypes: []string{"Elf"},
		Filters: append([]kop.Filter{{}}, filters...), Exprs: []kop.Expr{}, Conds: []kop.Cond{}, Overrides: overrides, Emblems: []kop.Emblem{}}
	if d.Overrides == nil {
		d.Overrides = []kop.Override{}
	}
	d.OverrideSubs = make([]cardset.OvSubs, len(d.Overrides))
	for i := range defs {
		d.Names = append(d.Names, defs[i].Name)
		d.PoolID = append(d.PoolID, uint16(i))
		d.Status = append(d.Status, cardset.StatusSupported)
		d.Reason = append(d.Reason, "")
		d.StaticSubs = append(d.StaticSubs, staticSubs[indexOfHalf(halves, defs[i].Name)])
		defs[i].Name = ""
		d.Defs = append(d.Defs, defs[i])
	}
	d.CardSubs = make([]kop.SubMask, len(defs))
	d.CardTokens = make([][]uint16, len(defs))
	d.CardRefSubs = make([][]uint8, len(defs))
	d.AbilityOrigin = make([][]uint8, len(defs))
	for i := range d.CardRefSubs {
		if len(d.StaticSubs[i]) > 0 || len(filters) > 0 {
			d.CardRefSubs[i] = []uint8{0} // the subtype is in every matchup that reads it
		}
	}
	return cardset.NewDataSet(d)
}

func indexOfHalf(halves []kop.CardDef, name string) int {
	return slices.IndexFunc(halves, func(c kop.CardDef) bool { return c.Name == name })
}

// stallFlagCase is one flag's two halves.
type stallFlagCase struct {
	reason uint8
	a, b   string
	set    cardset.Set
}

func stallFlagCases() []stallFlagCase {
	vt := func(name string, a ...kop.Ability) kop.CardDef {
		return kop.CardDef{Name: name, Types: kop.TEnchantment, Abilities: a}
	}
	fbKw := kop.KwFlashbackOwnCost // a variable: the stamp's mask is split into C (low 16 bits) and B.N, as build_ab.go emits it
	repl := func(k kop.ReplKind) kop.Ability { return kop.Ability{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: k}} }
	return []stallFlagCase{
		{StallInfect, "Half Infect", "Half Watcher", stallPool("infect", []kop.CardDef{
			{Name: "Half Infect", Types: kop.TCreature, Power: 1, Toughness: 1, Infect: true},
			vt("Half Watcher", kop.Ability{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigCounterAddedOnce}}),
		}, nil, nil, nil)},
		{StallLifeRepl, "Half Doubler", "Half Plus", stallPool("liferepl", []kop.CardDef{
			vt("Half Doubler", repl(kop.ReplLifeDouble)),
			vt("Half Plus", repl(kop.ReplLifePlus)),
		}, nil, nil, nil)},
		{StallL6Walk, "Half Loser", "Half Lord", stallPool("l6walk", []kop.CardDef{
			vt("Half Loser", kop.Ability{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatOverride, Override: 1}}),
			vt("Half Lord", kop.Ability{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatContinuous, Filter: 1}}),
		}, []kop.Override{{LoseAbilities: true}}, []kop.Filter{{Sub: 1}}, map[int][]cardset.AbSubs{1: {{Ab: 0, Subs: []uint8{0}}}})},
		{StallFbGrantStamp, "Half Fb Lotus", "Half Fb Stamper", stallPool("fbgrantstamp", []kop.CardDef{
			vt("Half Fb Lotus", kop.Ability{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatFbGrant, StFlags: kop.StFbOwnCost}}),
			vt("Half Fb Stamper", kop.Ability{Kind: kop.AbSpell, Prog: []kop.Instr{{
				Op: kop.OpGrantKeyword, C: int16(uint16(fbKw)),
				B: kop.ValueRef{K: kop.VConst, N: int16(uint16(fbKw >> 16))}}}}),
		}, nil, nil, nil)},
	}
}

// One of each half refuses the pair by both names, and either half alone plays.
func TestMatchupRefusesEveryInteraction(t *testing.T) {
	for _, c := range stallFlagCases() {
		t.Run(StallName(c.reason), func(t *testing.T) {
			deck := func(n string) []cardset.Entry {
				return []cardset.Entry{{Name: n, Count: 1}, {Name: "Vanilla A", Count: 1}}
			}
			_, _, _, err := c.set.Matchup(deck(c.a), deck(c.b))
			var r *cardset.InteractionRefusal
			if !errors.As(err, &r) {
				t.Fatalf("Matchup: %v, want an InteractionRefusal", err)
			}
			if r.Reason != StallName(c.reason) || !slices.Contains(r.Cards, c.a) || !slices.Contains(r.Cards, c.b) {
				t.Errorf("refusal %+v, want reason %s naming %q and %q", r, StallName(c.reason), c.a, c.b)
			}
			if r.Why == "" {
				t.Error("a refusal says why")
			}
			for _, alone := range []string{c.a, c.b} {
				if _, _, _, err := c.set.Matchup(deck(alone), deck("Vanilla B")); err != nil {
					t.Errorf("%s alone: %v", alone, err)
				}
			}
			// A pool table holds both halves and is not a matchup: no refusal.
			if _, err := c.set.Pool(); err != nil {
				t.Errorf("Pool: %v", err)
			}
		})
	}
}

// The test's named example, on its own.
func TestMatchupRefusesInfectPair(t *testing.T) {
	c := stallFlagCases()[0]
	_, _, _, err := c.set.Matchup([]cardset.Entry{{Name: c.a, Count: 1}}, []cardset.Entry{{Name: c.b, Count: 1}})
	if err == nil || !strings.Contains(err.Error(), c.a) || !strings.Contains(err.Error(), c.b) || !strings.Contains(err.Error(), "infect") {
		t.Fatalf("error %v, want infect naming %q and %q", err, c.a, c.b)
	}
}

// randomEntries draws a deck of n distinct cards from names.
func randomEntries(r *rand.Rand, names []string, n int) []cardset.Entry {
	out := make([]cardset.Entry, 0, n)
	for _, i := range r.Perm(len(names))[:min(n, len(names))] {
		out = append(out, cardset.Entry{Name: names[i], Count: 1})
	}
	return out
}

// compareRefusalWithInit plays pairs random deck pairs of set and holds Matchup's
// refusal to the stall Init starts the same table with. It reports how many pairs
// were refused and how many played. Not parallel: it clears kop.PairGuard, and a
// serial test has the binary to itself.
func compareRefusalWithInit(t *testing.T, set cardset.Set, names []string, deckSize, pairs int, seed uint64) (refused, played int) {
	t.Helper()
	guard := kop.PairGuard
	t.Cleanup(func() { kop.PairGuard = guard })
	r := rand.New(rand.NewPCG(seed, 1))
	for i := 0; i < pairs; i++ {
		a, b := randomEntries(r, names, deckSize), randomEntries(r, names, deckSize)
		kop.PairGuard = guard
		_, _, _, gerr := set.Matchup(a, b)
		kop.PairGuard = nil
		tbl, da, db, uerr := set.Matchup(a, b)
		kop.PairGuard = guard
		if uerr != nil { // an older guard, or a deck the pool does not hold: both must say so
			if gerr == nil || gerr.Error() != uerr.Error() {
				t.Fatalf("pair %d: unguarded %v, guarded %v", i, uerr, gerr)
			}
			continue
		}
		var s State
		s.Init(tbl, da, db, 1, Config{})
		why := s.StallReason()
		ReleaseTable(tbl)
		var ref *cardset.InteractionRefusal
		switch {
		case errors.As(gerr, &ref):
			refused++
			if ref.Reason != StallName(why) {
				t.Fatalf("pair %d: refused %s, Init stalls %q", i, ref.Reason, StallName(why))
			}
			if len(ref.Cards) == 0 {
				t.Fatalf("pair %d: refusal %+v names no card", i, ref)
			}
		case gerr != nil:
			t.Fatalf("pair %d: %v", i, gerr)
		default:
			played++
			if why != StallNone {
				t.Fatalf("pair %d: not refused, Init stalls %s", i, StallName(why))
			}
		}
	}
	return refused, played
}

// For each flag, Matchup refuses exactly when Init would stall, over 200 random
// pairs of a pool that holds both halves.
func TestInteractionRefusalMatchesInitStall(t *testing.T) {
	for _, c := range stallFlagCases() {
		t.Run(StallName(c.reason), func(t *testing.T) {
			refused, played := compareRefusalWithInit(t, c.set, c.set.Names(), 3, 200, 1)
			if refused == 0 || played == 0 {
				t.Errorf("%d refused, %d played: the pairs must fall both ways", refused, played)
			}
		})
	}
}

// The same over real generated pools: two that hold a real infect interaction,
// where some pair must refuse, and the two pools the gates play, where none does.
func TestInteractionRefusalMatchesInitStallOnPools(t *testing.T) {
	for _, c := range []struct {
		pool     string
		wantRefs bool
	}{{"mixedarms", true}, {"defined", true}, {"fdn", false}, {"blb", false}} {
		t.Run(c.pool, func(t *testing.T) {
			set := cardsettest.Set(t, c.pool)
			sup := set.Supported()
			var names []string
			for i := 0; i < set.NumCards(); i++ {
				if sup[i] {
					names = append(names, set.Names()[i])
				}
			}
			refused, played := compareRefusalWithInit(t, set, names, 15, 200, 2)
			t.Logf("%s: %d refused, %d played of 200", c.pool, refused, played)
			if c.wantRefs && refused == 0 {
				t.Error("no pair refused: the pool's interaction was not reached")
			}
			if !c.wantRefs && refused != 0 {
				t.Errorf("%d pairs refused: this pool's games would have stalled at their start", refused)
			}
			if played == 0 {
				t.Error("no pair played")
			}
		})
	}
}
