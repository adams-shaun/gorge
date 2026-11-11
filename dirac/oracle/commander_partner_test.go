package oracle

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// partnerFixtures are four vanilla legendary Partner creatures (CR 702.124),
// a pair a seat in the colours of vanilla decks 0 (red-green) and 1
// (white-black): gorge reads the inline script, the kernel the CardDef.
var partnerFixtures = []struct {
	name, cost string
	pip        int
	col        kop.ColorMask
	p, t       int8
}{
	{"Partner Alpha", "1 G", kop.G, kop.ColG, 2, 2},
	{"Partner Beta", "1 R", kop.R, kop.ColR, 3, 1},
	{"Partner Gamma", "1 W", kop.W, kop.ColW, 2, 2},
	{"Partner Delta", "1 B", kop.B, kop.ColB, 2, 1},
}

// partnerDuel is vanilla decks 0 and 1, each led by its seat's partner pair,
// as a Commander matchup over a registry holding only these cards (the
// corpus's vanilla printings and tokens, plus the inline partners) and the
// kernel table that mirrors them.
func partnerDuel(t *testing.T) (*Matchup, *kop.Table) {
	t.Helper()
	c, _ := testCorpus(t)
	tbl := vanilla.NewTable()
	reg := cards.NewRegistry()
	for k, v := range c.Reg.Tokens {
		reg.Tokens[k] = v
	}
	for s := 0; s < 2; s++ {
		for _, id := range vanilla.Decks[s] {
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
	}
	var pids []kop.CardID
	for _, f := range partnerFixtures {
		src := fmt.Sprintf("Name:%s\nManaCost:%s\nTypes:Legendary Creature Elf\nPT:%d/%d\nK:Partner\nOracle:x\n", f.name, f.cost, f.p, f.t)
		cd, diags := cards.ParseBytes("t.txt", []byte(src))
		if len(diags) != 0 {
			t.Fatalf("%s: %v", f.name, diags)
		}
		cd.Link()
		for _, fc := range cd.Faces {
			fc.ApplyIntrinsics()
		}
		reg.Add(cd)
		cost := kop.ManaCost{Generic: 1}
		cost.Pips[f.pip] = 1
		pids = append(pids, kop.CardID(len(tbl.Cards)))
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: f.name, Cost: cost, Types: kop.TCreature | kop.TLegendary,
			Colors: f.col, Power: f.p, Toughness: f.t})
	}
	var decks [2][]kop.CardID
	for s := 0; s < 2; s++ {
		decks[s] = append([]kop.CardID{pids[2*s], pids[2*s+1]}, vanilla.Decks[s]...)
	}
	m, err := KernelMatchup(reg, tbl, [2]string{"partners-rg", "partners-wb"}, decks, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	m.Commander = true
	m.Commanders, m.Partners = [2]int{0, 0}, [2]int{1, 1}
	return m, tbl
}

// TestCommanderPartnerLockstep plays partner-pair Commander duels in gorge
// and the kernel in lockstep: both seat two commanders at genesis, cast each
// from the command zone at its own tax (CR 903.8) and tally each one's combat
// damage apart (CR 903.10a). The games must agree to the end; across the
// seeds both commanders of a pair are cast, a commander is cast again
// (taxed), a pair's two commanders both deal combat damage, and one
// commander alone reaches 21 (CR 903.10a), so the per-commander bookkeeping
// is exercised and not merely zero on both sides.
func TestCommanderPartnerLockstep(t *testing.T) {
	m, tbl := partnerDuel(t)
	if cfg := m.Config(1); len(cfg.Commanders) != 2 || len(cfg.Commanders[0]) != 2 || len(cfg.Commanders[1]) != 2 {
		t.Fatalf("gorge config commanders %v, want a pair a seat", cfg.Commanders)
	}
	if on, ix := m.kernelCommanders(); !on || ix[0] != [2]int16{0, 1} || ix[1] != [2]int16{0, 1} {
		t.Fatalf("kernel commanders %v %v, want a pair a seat", on, ix)
	}
	rc := RunConfig{Config: Config{PassElision: true}, MaxTurns: 60}
	mk := RealKernel(tbl, nil, rc.Config)
	var bothCast, recast, bothHit, lethal bool
	for _, seed := range []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 12, 19, 24} {
		lead, err := NewGorgeSide(m, seed, rc.Config, nil)
		if err != nil {
			t.Fatal(err)
		}
		for s := 0; s < 2; s++ {
			pl := &lead.E.G.Players[s]
			if len(pl.Commanders) != 2 || lead.E.G.Obj(pl.Commanders[0]).Zone != state.ZCommand || lead.E.G.Obj(pl.Commanders[1]).Zone != state.ZCommand {
				t.Fatalf("seed %d seat %d: gorge seated %v, want both partners in the command zone", seed, s, pl.Commanders)
			}
		}
		k, err := mk(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, lead, k, RandomPolicy(seed), rc)
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("seed %d: diverged at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if res.Stall != "" && !res.Clean() {
			t.Fatalf("seed %d: stall %q", seed, res.Stall)
		}
		for s := 0; s < 2; s++ {
			pl := &lead.E.G.Players[s]
			if pl.CmdCasts[0] > 0 && pl.CmdCasts[1] > 0 {
				bothCast = true
			}
			if pl.CmdCasts[0] > 1 || pl.CmdCasts[1] > 1 {
				recast = true
			}
			// the opponent's CmdDamage slots for seat s's pair: dense
			// index 2s and 2s+1
			od := lead.E.G.Players[1-s].CmdDamage
			if od[2*s] > 0 && od[2*s+1] > 0 {
				bothHit = true
			}
			if od[2*s] >= 21 || od[2*s+1] >= 21 {
				lethal = true
			}
		}
		t.Logf("seed %d: %d actions, stall %q, casts %v %v, damage %v %v", seed, len(res.Actions), res.Stall,
			lead.E.G.Players[0].CmdCasts, lead.E.G.Players[1].CmdCasts, lead.E.G.Players[0].CmdDamage, lead.E.G.Players[1].CmdDamage)
	}
	if !bothCast || !recast || !bothHit || !lethal {
		t.Fatalf("partner bookkeeping not exercised: both cast %v, a recast %v, both dealt damage %v, one reached 21 %v", bothCast, recast, bothHit, lethal)
	}
}
