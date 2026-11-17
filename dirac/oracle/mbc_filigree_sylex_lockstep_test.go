package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// The Filigree Sylex's third ability, "{T}, Remove ten oil counters from among
// permanents you control and sacrifice The Filigree Sylex", is paid from the
// Sylex's own counters: the counters come off before the sacrifice settles
// (CR 118.3, 601.2h), and the Sylex is the only permanent that carries oil.
// gorge reserved the Sylex for its own sacrifice before it counted the
// counters, so it never offered the ability that the kernel offers at ten
// counters (an `options options` divergence in tail-40).
//
// The game is recorded by TestMBCTheFiligreeSylexReproRecord (PINS_RECORD=1)
// on the kernel table built from the decks.
func sylexCase() effectLongCase {
	return effectLongCase{
		name: "sylex_removes_its_own_counters_before_its_sacrifice",
		decks: [2][]string{
			append(deckOf(4, "The Filigree Sylex"), deckOf(26, "Mountain")...),
			deckOf(30, "Mountain"),
		},
		surface: "autopay",
		reached: sylexTenDamageDealt,
	}
}

// sylexTenDamageDealt: ten damage was dealt to a player while the game ran.
func sylexTenDamageDealt(g *GorgeSide) bool {
	for _, e := range g.E.L.Events {
		if e.Kind == events.Damage && e.Amount >= 10 {
			return true
		}
	}
	return false
}

// sylexPolicy plays lands, casts the Sylex, then puts an oil counter on it
// every time it can, and takes the ten damage the moment it is offered. It
// never takes the destroy ability, which would end the Sylex early.
func sylexPolicy(m *Matchup, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind != DPriority {
			return rnd.Pick(p)
		}
		var land, cast, counter, dmg, pass Canon
		var haveLand, haveCast, haveCounter, haveDmg, havePass bool
		for _, o := range p.D.Options {
			switch {
			case o.Kind() == ActPlayLand:
				land, haveLand = o, true
			case o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == "The Filigree Sylex":
				cast, haveCast = o, true
			case o.Kind() == ActActivate && m.InstanceCard(int(o.Src())) == "The Filigree Sylex":
				switch o.Mode() {
				case 0:
					counter, haveCounter = o, true
				case 2:
					dmg, haveDmg = o, true
				}
			case o.Kind() == ActPass:
				pass, havePass = o, true
			}
		}
		switch {
		case haveDmg:
			return dmg
		case haveLand:
			return land
		case haveCast:
			return cast
		case haveCounter:
			return counter
		case havePass:
			return pass
		}
		return rnd.Pick(p)
	})
}

// TestMBCTheFiligreeSylexReprosLockstep replays the recorded game: both
// engines offer the Sylex's ten-damage ability at ten oil counters, and pay it.
func TestMBCTheFiligreeSylexReprosLockstep(t *testing.T) {
	stdReproNamed(t, sylexCase().name)
}

// TestMBCTheFiligreeSylexReproReaches replays the recorded game with its
// reach check: every scripted action offered and the ten damage dealt.
func TestMBCTheFiligreeSylexReproReaches(t *testing.T) {
	reg := synthReg(t)
	c := sylexCase()
	r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	script := &ScriptPolicy{Actions: r.Actions}
	res, reached := effectLongPlay(t, reg, c, r, printedDelveSet(t, r.Cards), script)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if !reached {
		t.Fatalf("the recorded game never deals the Sylex's ten damage")
	}
}

// TestMBCTheFiligreeSylexReproRecord plays sylexPolicy over seeds until one
// deals the ten damage, and writes the repro. It runs only with PINS_RECORD=1;
// the game it writes is the committed repro.
func TestMBCTheFiligreeSylexReproRecord(t *testing.T) {
	if os.Getenv("PINS_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	c := sylexCase()
	for seed := uint64(1); seed <= 40; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks,
			Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom", Pool: "custom",
			Why: "The Filigree Sylex removes its ten oil counters from among permanents (its own, the only ones) and then sacrifices itself (CR 118.3, 601.2h): both engines offer the ten-damage ability at ten counters"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		res, reached := effectLongPlay(t, reg, c, r, printedDelveSet(t, c.decks), sylexPolicy(m, seed))
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if !reached {
			continue
		}
		r.setActions(res.Actions)
		if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
			t.Fatal(err)
		}
		t.Logf("seed %d: %d decisions", seed, len(res.Actions))
		return
	}
	t.Fatalf("no seed dealt the Sylex's ten damage")
}
