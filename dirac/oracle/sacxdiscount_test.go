package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// sacDiscountScript is Dargo, the Shipwrecker's announced sacrifice and its
// X-reading self discount, without the SacrificedThisTurn tally the kernel
// has no counter for (the full card still refuses in kgen): "sacrifice any
// number of artifacts and/or creatures ... this spell costs {2} less to cast
// for each permanent sacrificed this way". The {6}{R} spell is announced
// with X permanents sacrificed and costs {2} less for each (CR 601.2b,
// 601.2f), so {1}{R} pays it at X = 3. gorge reads this script; the kernel
// reads sacDiscountDef, the CardDef kgen lowers it to (dirac/kgen
// sacxdiscount_test.go).
const sacDiscountScript = "Name:Sac Discount Probe\nManaCost:6 R\nTypes:Creature\nPT:7/5\n" +
	"A:SP$ PermanentCreature | Cost$ 6 R Sac<X/Artifact;Creature/any number of artifacts and/or creatures> | AdditionalDesc$ This spell costs {2} less to cast for each permanent sacrificed this way.\n" +
	"S:Mode$ ReduceCost | Type$ Spell | Relative$ True | ValidCard$ Card.Self | Amount$ Y | EffectZone$ All\n" +
	"SVar:X:Count$xPaid\nSVar:Y:SVar$X/Times.2\nOracle:x\n"

// sacRitualScript floats {R}{R}: gorge offers a Sac<X> cast only from a
// floating pool (the planner over untapped lands declines it, and the
// kernel's gorgeLowers mirrors that), and the canonical surface has no mana
// activations, so a spell adds the pool.
const sacRitualScript = "Name:Sac Ritual Probe\nManaCost:R\nTypes:Instant\nA:SP$ Mana | Produced$ R | Amount$ 2 | SpellDescription$ Add {R}{R}.\nOracle:Add {R}{R}.\n"

// sacDiscountDeck is seat 0's 40 cards: the probe, the ritual, red lands and
// one-mana creatures to sacrifice.
func sacDiscountDeck(probe, ritual kop.CardID) []kop.CardID {
	var d []kop.CardID
	for i := 0; i < 8; i++ {
		d = append(d, probe)
	}
	for i := 0; i < 8; i++ {
		d = append(d, ritual)
	}
	for i := 0; i < 10; i++ {
		d = append(d, vanilla.Mountain)
	}
	for i := 0; i < 14; i++ {
		d = append(d, vanilla.RagingGoblin)
	}
	return d
}

// sacDiscountDuel is the probe's seat against vanilla deck 1, over a registry
// holding the probe, the corpus's vanilla printings and tokens, and the
// kernel table that mirrors them.
func sacDiscountDuel(t *testing.T) (*Matchup, *kop.Table) {
	t.Helper()
	c, _ := testCorpus(t)
	tbl := vanilla.NewTable()
	reg := cards.NewRegistry()
	for k, v := range c.Reg.Tokens {
		reg.Tokens[k] = v
	}
	probe := kop.CardID(len(tbl.Cards))
	ritual := probe + 1
	decks := [2][]kop.CardID{sacDiscountDeck(probe, ritual), vanilla.Decks[1]}
	for _, id := range append(append([]kop.CardID{}, decks[0]...), decks[1]...) {
		if int(id) >= len(tbl.Cards) {
			continue
		}
		n := tbl.Cards[id].Name
		if _, ok := reg.Lookup(n); ok {
			continue
		}
		cd, ok := c.Reg.Lookup(n)
		if !ok {
			t.Skipf("%s not in the corpus", n)
		}
		reg.Add(cd)
	}
	for _, src := range []string{sacDiscountScript, sacRitualScript} {
		cd, diags := cards.ParseBytes("t.txt", []byte(src))
		if len(diags) != 0 {
			t.Fatalf("probe: %v", diags)
		}
		cd.Link()
		for _, fc := range cd.Faces {
			fc.ApplyIntrinsics()
		}
		reg.Add(cd)
	}

	// the table rows dirac/kgen builds for the script: the sacrifice filter
	// (an artifact or a creature you control), the card itself on the stack
	// and X times two
	fCreature := kop.FilterID(len(tbl.Filters))
	fSac := fCreature + 1
	fSelf := fCreature + 2
	tbl.Filters = append(tbl.Filters,
		kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou},
		kop.Filter{Types: kop.TArtifact, Ctrl: kop.CtrlYou, Or: fCreature},
		kop.Filter{Flags: kop.FSelf, Zone: kop.ZoneStack})
	if len(tbl.Exprs) == 0 {
		tbl.Exprs = append(tbl.Exprs, kop.Expr{}) // Exprs[0] is unused
	}
	x2 := int16(len(tbl.Exprs))
	tbl.Exprs = append(tbl.Exprs, kop.Expr{Kind: kop.ExX, Mul: 2})
	cost := kop.ManaCost{Generic: 6}
	cost.Pips[kop.R] = 1
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Sac Discount Probe", Cost: cost, Types: kop.TCreature, Colors: kop.ColR, Power: 7, Toughness: 5,
		SpellCost: kop.Cost{Flags: kop.CostSac, Filter: fSac},
		Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{
			Kind: kop.StatReduceCost, Filter: fSelf, VA: kop.ValueRef{K: kop.VExpr, N: x2},
			StFlags: kop.StDynamic, Zone: kop.ZoneCommand}}},
	}, kop.CardDef{
		Name: "Sac Ritual Probe", Cost: kop.ManaCost{Pips: [6]uint8{kop.R: 1}}, Types: kop.TInstant, Colors: kop.ColR, Timing: kop.TimingInstant,
		Abilities: []kop.Ability{{Kind: kop.AbSpell,
			Prog: []kop.Instr{{Op: kop.OpAddMana, Obj: kop.RefController, A: kop.Const(2), C: int16(kop.ColR)}}}},
	})
	m, err := KernelMatchup(reg, tbl, [2]string{"sacdiscount-r", "sacdiscount-wb"}, decks, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	return m, tbl
}

// sacDiscountTally counts the probe's casts. An X ask with one option is
// answered in place (elision), so the casts are counted where they are
// taken and where they end.
type sacDiscountTally struct {
	offers  int // priority windows offering the probe's cast
	short   int // casts taken with fewer than seven mana floating: the unreduced {6}{R} is out of reach, only an announced X pays
	asks    int // X asks posed (more than one value paid)
	maxX    int // the largest X offered
	cast    int // such casts that reached the stack
	pending bool
}

// sacDiscountSource is the name of the decision's source object, "" for none.
func sacDiscountSource(g *GorgeSide, id state.ObjID) string {
	if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
		return o.Face().Name
	}
	return ""
}

// sacDiscountPolicy takes the probe's cast whenever it is offered and answers
// its X ask with the largest value half the time, the smallest otherwise.
func sacDiscountPolicy(seed uint64, tl *sacDiscountTally) Policy {
	rnd := RandomPolicy(seed)
	r := rand.New(rand.NewPCG(seed, 0x73616364)) // "sacd": the bias's own stream
	xcode, _ := chooseCode("x")
	return PolicyFunc(func(c *PickCtx) Canon {
		a := rnd.Pick(c)
		if c.G == nil {
			return a
		}
		if tl.pending {
			for _, id := range c.G.E.G.Stack {
				if sacDiscountSource(c.G, id) == "Sac Discount Probe" {
					tl.cast++
					tl.pending = false
				}
			}
		}
		if c.D.Kind == DPriority {
			// a land first; with the probe in hand and two permanents to
			// sacrifice the ritual's cast floats the pool; the probe's cast is
			// taken at once
			probeInHand := false
			for _, id := range c.G.E.G.Zone(state.ZHand, c.G.E.G.Priority) {
				probeInHand = probeInHand || c.G.E.G.Obj(id).Face().Name == "Sac Discount Probe"
			}
			sacrificable, landsUntapped, ritualsInHand := 0, 0, 0
			for _, id := range c.G.E.G.Zone(state.ZBattlefield, c.G.E.G.Priority) {
				if !objectHasType(c.G.E.G.Obj(id), "Land") {
					sacrificable++
				} else if !c.G.E.G.Obj(id).Tapped {
					landsUntapped++
				}
			}
			for _, id := range c.G.E.G.Zone(state.ZHand, c.G.E.G.Priority) {
				if sacDiscountSource(c.G, id) == "Sac Ritual Probe" {
					ritualsInHand++
				}
			}
			rank, pick := 0, a
			var ritual, probe Canon
			haveRitual, haveProbe := false, false
			for _, o := range c.D.Options {
				r := 0
				g, ok := c.G.GorgeOption(o)
				switch {
				case !ok:
				case o.Kind() == ActPlayLand:
					r = 4
				case o.Kind() == ActCast && g.Kind == "cast":
					r = 1
					if probeInHand && c.G.E.G.Players[c.G.E.G.Priority].Pool.Total() > 0 {
						r = 0 // floating mana is saved for the probe
					}
					switch c.G.E.G.Obj(g.Obj).Face().Name {
					case "Sac Discount Probe":
						r, probe, haveProbe = 5, o, true
					case "Sac Ritual Probe":
						r = 0
						if probeInHand && sacrificable >= 2 {
							r = 3
							ritual, haveRitual = o, true
						}
					}
				}
				if r > rank {
					rank, pick = r, o
				}
			}
			if rank == 0 {
				for _, o := range c.D.Options {
					if o.Kind() == ActPass {
						pick = o // nothing to take: pass, so no ritual or probe is cast at random
					}
				}
			}
			if rank == 5 && haveRitual && c.G.E.G.Players[c.G.E.G.Priority].Pool.Total() < 3 && r.IntN(4) != 0 {
				pick = ritual // float more first: the ask then holds several announcements
			}
			if haveProbe && pick == probe {
				tl.offers++
				if c.G.E.G.Players[c.G.E.G.Priority].Pool.Total() < 7 {
					tl.short++
					tl.pending = true
				}
			}
			return pick
		}
		if c.D.Kind != DChoose || len(c.D.Options) == 0 || c.D.Options[0].Kind() != ActChoose+CanonKind(xcode) {
			return a
		}
		pd := c.G.Pending()
		if pd == nil || sacDiscountSource(c.G, pd.Source) != "Sac Discount Probe" {
			return a
		}
		lo, hi := 1<<30, 0
		for _, o := range c.D.Options {
			lo, hi = min(lo, int(o.Mode())), max(hi, int(o.Mode()))
		}
		if r.IntN(2) == 0 {
			a = c.D.Options[len(c.D.Options)-1]
		} else {
			a = c.D.Options[0]
		}
		tl.asks++
		tl.maxX = max(tl.maxX, hi)
		return a
	})
}

// runSacDiscount plays seeds lo..hi of the probe's duel and returns the
// divergences and the tally.
func runSacDiscount(t *testing.T, lo, hi int) (games, divs int, tl sacDiscountTally) {
	t.Helper()
	m, tbl := sacDiscountDuel(t)
	for seed := uint64(lo); seed <= uint64(hi); seed++ {
		rc := RunConfig{Config: Config{PassElision: true, GorgeAutopaySurface: seed%2 == 0}, MaxDecisions: 2000, MaxTurns: 20}
		g, err := NewGorgeSide(m, seed, rc.Config, nil)
		if err != nil {
			t.Fatal(err)
		}
		k, err := RealKernel(tbl, nil, rc.Config)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		tl.pending = false
		res := LockstepLed(m, seed, g, k, sacDiscountPolicy(seed, &tl), rc)
		CloseSide(k)
		games++
		if d := res.Div; d != nil {
			divs++
			from := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[from:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		if res.Stall != "" && !res.Clean() {
			t.Errorf("seed %d: stall %q", seed, res.Stall)
		}
	}
	return games, divs, tl
}

// TestSacDiscountLockstepFixture pins the table the lockstep plays: the
// probe's kernel row is what the script lowers to.
func TestSacDiscountLockstepFixture(t *testing.T) {
	_, tbl := sacDiscountDuel(t)
	var d kop.CardDef
	for _, c := range tbl.Cards {
		if c.Name == "Sac Discount Probe" {
			d = c
		}
	}
	if d.SpellCost.Flags&kop.CostSac == 0 || d.SpellCost.N != 0 {
		t.Fatalf("probe SpellCost %+v: want an announced sacrifice", d.SpellCost)
	}
	if f := tbl.Filters[d.SpellCost.Filter]; f.Types != kop.TArtifact || tbl.Filters[f.Or].Types != kop.TCreature {
		t.Fatalf("sacrifice filter %+v", f)
	}
}

// TestSacXDiscountLockstep: Dargo-shaped cast (an announced Sac<X> and a
// self ReduceCost reading X) in gorge and the kernel. With six or fewer
// lands the unreduced {6}{R} is unaffordable and gorge still offers the
// cast, its X ask holding exactly the announcements the discount pays; the
// kernel must offer the same.
func TestSacXDiscountLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, tl := runSacDiscount(t, 1, 600)
	t.Logf("%d games, %d divergences; %+v", games, divs, tl)
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	if tl.short < 20 || tl.cast < 20 || tl.asks < 10 || tl.maxX < 2 {
		t.Errorf("the discount was not what paid: casts with fewer than seven mana floating %d (want 20), such casts that reached the stack %d (want 20), X asks with several values %d (want 10), the largest X offered %d (want 2)", tl.short, tl.cast, tl.asks, tl.maxX)
	}
}
