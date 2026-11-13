package oracle

import (
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The MBC chosen-player-card-6 spellings at lockstep
// (dirac/oracle/mbc_chosenplayer_test.go). The two MBC cards stay refused by
// other families' blockers (Plargg and Nassari's DBPlay, Maddening Hex's
// EnchantedBy player qualifier) and a matchup refuses a refused card
// (cardset's RefusedError), so the probes are synthetic: each is parsed from
// an inline script for the gorge side and grafted onto the chosenplayer6
// pool's donors for the kernel side — the donors' compiled defs carry the
// printed shapes (Curse of Opulence's Enchant:Player aura, Ajani's Mantra's
// your-upkeep trigger record) with the lowered chain as the program, renamed
// to the probe. The graft plays the runtime the lowering emits;
// dirac/kgen's TestChoosePlayerRandomLowers and
// TestChooseCardChosenPlayerChooserLowers prove the lowering emits it.
//
//   - Maddening Hex's re-attach chain (the Random$ True ChoosePlayer over
//     Player.Opponent+!EnchantedBy and the Attach of its pick): the probe is
//     an Enchant:Player aura; gorge picks the controller's living opponent the
//     aura is not attached to with one engine rng draw and never asks, and the
//     aura moves onto the pick. A pool of one is still one draw, so the pick
//     runs every upkeep while the aura sits on its controller.
//   - the Plargg class's mandatory ChooseCard asked of the chosen player
//     (Demonic Hordes' "a land of an opponent's choice"): the pool is the
//     RESOLVING CONTROLLER's lands — gorge cardChoices' activator perspective
//     (choose_control.go cardChoicesIn) — while the chosen player answers the
//     ask, and the chosen land is destroyed.
const (
	probeMaddening = `Name:Probe Maddening
ManaCost:2
Types:Enchantment Aura Curse
K:Enchant:Player
T:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ Trig
SVar:Trig:DB$ ChoosePlayer | Defined$ You | Choices$ Player.Opponent+!EnchantedBy | Random$ True | SubAbility$ DBAttach
SVar:DBAttach:DB$ Attach | Defined$ ChosenPlayer
Oracle:At the beginning of your upkeep, attach CARDNAME to another one of your opponents chosen at random.
`

	probePlargg = `Name:Probe Plargg
ManaCost:2
Types:Creature
PT:2/2
T:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ Trig
SVar:Trig:DB$ ChoosePlayer | Defined$ You | Choices$ Player.Opponent | SubAbility$ DBChoose
SVar:DBChoose:DB$ ChooseCard | Defined$ ChosenPlayer | Choices$ Land.YouCtrl | Mandatory$ True | SubAbility$ DBDest
SVar:DBDest:DB$ Destroy | Defined$ ChosenCard
Oracle:At the beginning of your upkeep, an opponent chooses a land you control. Destroy that land.
`
)

// mbcProbes is the two spellings: the script, the def the probe is grafted
// onto and the program that replaces the def's abilities.
var mbcProbes = []struct {
	name   string
	donor  string // the lowered def the probe takes its printed shape from
	script string
	graft  func(t *testing.T, tbl *kop.Table, d *kop.CardDef, trig kop.Trigger)
}{
	{
		name:   "Probe Maddening",
		donor:  "Curse of Opulence",
		script: probeMaddening,
		graft: func(t *testing.T, tbl *kop.Table, d *kop.CardDef, trig kop.Trigger) {
			d.Abilities = []kop.Ability{{
				Kind: kop.AbTriggered, Trig: trig,
				Prog: []kop.Instr{
					{Op: kop.OpChoosePlayer, Obj: kop.RefController, C: kop.ChPlRandom},
					{Op: kop.OpAttach, W: kop.RefChosenPlayer},
				},
			}}
			d.Cost, d.Colors, d.Identity = kop.ManaCost{Generic: 2}, 0, 0
		},
	},
	{
		name:   "Probe Plargg",
		donor:  "Ajani's Mantra",
		script: probePlargg,
		graft: func(t *testing.T, tbl *kop.Table, d *kop.CardDef, trig kop.Trigger) {
			// gorge's one-opponent ChoosePlayer records its only candidate
			// without an ask, so the lowering elides it and the ChooseCard's
			// chooser is the resolution's chosen player (EachOpp in a
			// two-seat game). The pool is Land.YouCtrl from the resolving
			// controller's perspective (interp_gen.go's chooseCardBase).
			tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TLand, Ctrl: kop.CtrlYou})
			f := kop.FilterID(len(tbl.Filters) - 1)
			d.Abilities = []kop.Ability{{
				Kind: kop.AbTriggered, Trig: trig,
				Prog: []kop.Instr{
					{Op: kop.OpChooseCard, Obj: kop.RefEachOpponent, F: f},
					{Op: kop.OpDestroy, Obj: kop.RefChosen},
				},
			}}
			d.Types = kop.TCreature
			d.Power, d.Toughness = 2, 2
			d.AtkFace, d.NoPT = false, false
			d.Cost = kop.ManaCost{Generic: 2}
			d.Colors, d.Identity = 0, 0
		},
	},
}

// mbcProbeTable builds the chosenplayer6 matchup table with both probes
// grafted onto its two donors, each in place: each probe takes its donor's
// def — its printed shape and, for the Maddening probe, the Enchant:Player
// cast spec — and the Ajani's Mantra trigger record (the same Phase Upkeep
// You header both probes' scripts carry) with the lowered chain as the
// program, made mandatory. Renaming in place keeps the table's per-card
// arrays sized.
func mbcProbeTable(t *testing.T, set cardset.Set) *kop.Table {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Curse of Opulence", Count: 4}, {Name: "Ajani's Mantra", Count: 4},
		{Name: "Island", Count: 32},
	}
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	trig := -1
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Ajani's Mantra" {
			trig = i
			break
		}
	}
	if trig < 0 {
		t.Fatal("the trigger donor is not in the table")
	}
	record := tbl.Cards[trig].Abilities[0].Trig
	record.Flags = 0 // the probes' triggers are mandatory: no optional ask either side
	for _, p := range mbcProbes {
		di := -1
		for i := range tbl.Cards {
			if tbl.Cards[i].Name == p.donor {
				di = i
				break
			}
		}
		if di < 0 {
			t.Fatalf("%s: the %s def is not in the table", p.name, p.donor)
		}
		d := tbl.Cards[di]
		p.graft(t, tbl, &d, record)
		d.Name = p.name
		tbl.Cards[di] = d
	}
	return tbl
}

// mbcTally is one game's coverage: the probes' casts, the chosen player's
// ChooseCard asks the policy answered and the Maddening auras the pick moved.
type mbcTally struct {
	casts    [2]int // casts of each probe, in mbcProbes order
	asks     int    // the Plargg class's ChooseCard asks
	moved    int    // Probe Maddening auras whose AttachedPlayer changed
	lastAura map[state.ObjID]state.PlayerID
}

// mbcCheck observes the gorge state at every agreed decision: a Probe
// Maddening aura's AttachedPlayer only changes when the random pick ran and
// its Attach moved the aura.
func mbcCheck(t *testing.T, y *mbcTally, m *Matchup) func(*GorgeSide, Side) *Divergence {
	t.Helper()
	y.lastAura = map[state.ObjID]state.PlayerID{}
	return func(ref *GorgeSide, k Side) *Divergence {
		for _, o := range ref.E.G.Objs {
			if o.Zone != state.ZBattlefield || o.Face() == nil || o.Face().Name != "Probe Maddening" || !o.HasAttachedPlayer {
				continue
			}
			if prev, ok := y.lastAura[o.ID]; ok && prev != o.AttachedPlayer {
				y.moved++
			}
			y.lastAura[o.ID] = o.AttachedPlayer
		}
		return nil
	}
}

// mbcProbesDuel builds the game's registry (a fresh mutable one: the probes
// are synthetic scripts linked and intrinsified into it, the Islands are the
// corpus's) and both decks: each seat 2 copies of each probe and 36 Islands.
func mbcProbesDuel(t *testing.T, corpus *cards.Registry) (*cards.Registry, [2][]*cards.Card) {
	t.Helper()
	reg := cards.NewRegistry()
	for k, v := range corpus.Tokens {
		reg.Tokens[k] = v
	}
	island, ok := corpus.Lookup("Island")
	if !ok {
		t.Fatal("Island not in the corpus")
	}
	reg.Add(island)
	var decks [2][]*cards.Card
	for _, p := range mbcProbes {
		c, diags := cards.ParseBytes("probes.txt", []byte(p.script))
		if len(diags) > 0 {
			t.Fatalf("%s: %v", p.name, diags)
		}
		c.Link()
		for _, fc := range c.Faces {
			fc.ApplyIntrinsics()
		}
		reg.Add(c)
		for s := range decks {
			for j := 0; j < 4; j++ {
				decks[s] = append(decks[s], c)
			}
		}
	}
	for s := range decks {
		for j := 0; j < 32; j++ {
			decks[s] = append(decks[s], island)
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d has %d cards", s, len(decks[s]))
		}
	}
	return reg, decks
}

// TestMBCChosenPlayerCard6Lowers plays one lockstep game per seed with both
// probes in both decks, so each game exercises each spelling from either
// seat. The probes drive the game: the policy casts whichever probe it is
// offered, and sends the Maddening aura's enchant target to its own
// controller, so the random pick runs (the aura sits on its controller, its
// pool of opponents is the other seat) and the moved aura is the pick's
// Attach. The rest is the random policy's. The spells a given seed draws are
// the shuffle's, so the coverage counts pool over the seeds: every seed must
// agree in lockstep, and together they must have cast each probe, answered
// the chosen player's ChooseCard and moved a Maddening aura.
func TestMBCChosenPlayerCard6Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	set := cardsettest.Set(t, "chosenplayer6")
	corpus := synthReg(t)
	reg, decks := mbcProbesDuel(t, corpus)
	cardKind := ActChoose + CanonKind(mustChooseCode("card"))
	var cov struct {
		sync.Mutex
		casts [2]int
		asks  int
		moved int
	}
	seedChunks(t, 1, 4, 2, func(t *testing.T, lo, hi uint64, mu *sync.Mutex) {
		for seed := lo; seed <= hi; seed++ {
			// the table is built per seed: CloseSide releases it (the
			// kernel's prep), like fixtureLockstep's own per-seed Matchup
			tbl := mbcProbeTable(t, set)
			cfg := Config{GorgeAutopaySurface: true}
			m, err := NewMatchup(reg, [2]string{"probe-a", "probe-b"}, decks, 0)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			g, err := NewGorgeSide(m, seed, cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			var y mbcTally
			inner := RandomPolicy(seed)
			pol := PolicyFunc(func(c *PickCtx) Canon {
				for _, a := range c.D.Options {
					if a.Kind() == cardKind {
						y.asks++
						break
					}
				}
				if c.D.Kind == DTarget {
					for _, a := range c.D.Options {
						if a.Tgt() >= RefPlayer && a.Tgt() == RefPlayer+Ref(c.D.Player) {
							return a
						}
					}
				}
				// the Maddening probe first: its pick runs while the aura
				// sits on its controller, and the aura's re-attach is the
				// pick's only witness
				for _, want := range mbcProbes {
					for _, a := range c.D.Options {
						if strings.HasPrefix(a.String(), "cast(") && firstObjectCard(m, a.String()) == want.name {
							return a
						}
					}
				}
				return inner.Pick(c)
			})
			res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 12, Check: mbcCheck(t, &y, m)})
			CloseSide(k)
			if d := res.Div; d != nil {
				t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
			}
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Fatalf("seed %d: %s", seed, res.Stall)
			}
			for _, a := range res.Actions {
				s := a.String()
				name := firstObjectCard(m, s)
				for i, p := range mbcProbes {
					if name == p.name && strings.HasPrefix(s, "cast(") {
						y.casts[i]++
					}
				}
			}
			mu.Lock()
			for i := range cov.casts {
				cov.casts[i] += y.casts[i]
			}
			cov.asks += y.asks
			cov.moved += y.moved
			mu.Unlock()
		}
	})
	for i, p := range mbcProbes {
		if cov.casts[i] == 0 {
			t.Fatalf("%s was never cast in any seed; the spelling would not be exercised", p.name)
		}
	}
	if cov.asks == 0 {
		t.Fatal("no chosen-player ChooseCard ask; the Plargg spelling was not exercised")
	}
	if cov.moved == 0 {
		t.Fatal("no Probe Maddening aura ever moved: the random pick's Attach never ran")
	}
}
