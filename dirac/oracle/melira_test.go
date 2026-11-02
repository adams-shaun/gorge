package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	gcost "github.com/adams-shaun/gorge/engine/rules/cost"
)

// skipWithoutGorgeNickname skips the Melira lockstep at a gorge pin whose
// cost parser keeps Exile<1/NICKNAME>'s spec raw: the battlefield exile
// part then matches no permanent (the filter reads CARDNAME, not NICKNAME),
// so gorge never offers "Exile Melira" (gorge engine/rules/cost/parse.go,
// the bare Exile<N/Spec> head). The kernel pays it as the source itself.
func skipWithoutGorgeNickname(t *testing.T) {
	t.Helper()
	c := gcost.ParseCost("Exile<1/NICKNAME>")
	if len(c.Exile) != 1 || c.Exile[0].Spec != "CARDNAME" {
		t.Skip("gorge cannot pay Exile<1/NICKNAME>")
	}
}

// The Melira lockstep: Melira, the Living Cure's "Exile Melira:
// when another target creature or artifact is put into a graveyard this
// turn, return it" (a ChangesZone delayed trigger: the kernel's this-turn
// leave-return record) and its poison replacement (you get one poison
// counter, then no more this turn: ReplOnePoison), against an infect and
// toxic deck (Glistener Elf, Plague Stinger, Blight Mamba, Bloated
// Contaminator, Ichorclaw Myr) with removal. The expected answer at every
// decision is gorge's.
var kmeliraDecks = [2][]cardset.Entry{
	{
		{Name: "Melira, the Living Cure", Count: 4}, {Name: "Plains", Count: 9}, {Name: "Forest", Count: 9},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Savannah Lions", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Ornithopter", Count: 2}, {Name: "Swords to Plowshares", Count: 4},
	},
	{
		{Name: "Glistener Elf", Count: 4}, {Name: "Plague Stinger", Count: 4}, {Name: "Blight Mamba", Count: 3},
		{Name: "Bloated Contaminator", Count: 3}, {Name: "Ichorclaw Myr", Count: 3}, {Name: "Forest", Count: 8},
		{Name: "Swamp", Count: 9}, {Name: "Doom Blade", Count: 3}, {Name: "Disfigure", Count: 3},
	},
}

func TestMeliraLockstep(t *testing.T) {
	t.Parallel()
	skipWithoutGorgeNickname(t)
	reg, set, decks, tbl := p3Setup(t, "melira", kmeliraDecks)
	p3Lockstep(t, "melira", reg, set, decks, tbl, 600)
}

// kmeliraPoisonDecks: Melira and lands only against the infect deck
// without removal, so several infect attackers meet her replacement in one
// combat.
var kmeliraPoisonDecks = [2][]cardset.Entry{
	{
		{Name: "Melira, the Living Cure", Count: 4}, {Name: "Plains", Count: 18}, {Name: "Forest", Count: 18},
	},
	{
		{Name: "Glistener Elf", Count: 4}, {Name: "Plague Stinger", Count: 4}, {Name: "Blight Mamba", Count: 4},
		{Name: "Bloated Contaminator", Count: 4}, {Name: "Ichorclaw Myr", Count: 4}, {Name: "Forest", Count: 10},
		{Name: "Swamp", Count: 10},
	},
}

// castNoActPolicy is castFirstPolicy without activations: Melira stays on
// the battlefield, so the infect deck's poison meets her replacement.
func castNoActPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActCastFace || k == ActPlayLand {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			for _, a := range c.D.Options {
				if a.Kind() != ActActivate {
					buf = append(buf, a)
				}
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// TestMeliraPoisonLockstep plays the Melira pool with no activations, so
// her poison replacement (one counter, then none this turn) meets the
// infect and toxic attackers.
func TestMeliraPoisonLockstep(t *testing.T) {
	t.Parallel()
	skipWithoutGorgeNickname(t)
	reg, set, decks, tbl := p3Setup(t, "melira", kmeliraPoisonDecks)
	clean := 0
	for seed := uint64(1); seed <= 300; seed++ {
		m, err := NewMatchup(reg, [2]string{"kmelira-a", "kmelira-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, castNoActPolicy(seed), RunConfig{MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			continue
		}
		clean++
	}
	t.Logf("300 seeds, %d clean", clean)
}

// TestMeliraPoisonMutation proves the poison pool exercises the
// replacement: with it dropped, some seed diverges from gorge.
func TestMeliraPoisonMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	skipWithoutGorgeNickname(t)
	reg, set, decks, tbl := p3Setup(t, "melira", kmeliraPoisonDecks)
	dirac.DebugNoOnePoison = true
	defer func() { dirac.DebugNoOnePoison = false }()
	for seed := uint64(1); seed <= 40; seed++ {
		m, err := NewMatchup(reg, [2]string{"kmelira-a", "kmelira-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, castNoActPolicy(seed), RunConfig{MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if res.Div != nil {
			t.Logf("seed %d diverges without the replacement (decision %d: %s)", seed, res.Div.At, res.Div.What)
			return
		}
	}
	t.Fatal("no seed diverged with Melira's poison replacement dropped: the pool does not exercise it")
}
