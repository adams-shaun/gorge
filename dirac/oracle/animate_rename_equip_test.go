package oracle

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// Tenth District Hero's second ability (it becomes a legendary creature named
// Mileva, the Stalwart) and The Irencrag's trigger (it becomes Everflame,
// Heroes' Legacy, gains equip {3} and "Equipped creature gets +3/+3" and loses
// all other abilities) are the lowered Animate Name$ and Keywords$ Equip:N.
// gorge never offers Everflame's granted equip (the same Animate's
// RemoveAllAbilities$ hides it), so The Irencrag's game reaches the
// animation only.
// TestStdSetReprosLockstep replays one recorded game of each;
// TestAnimateRenameEquipReprosReach requires the recorded game to reach the
// animation, since a lockstep entry alone would pass a script that never did.
//
// The games are recorded by TestAnimateRenameEquipRecord
// (ANIMATE_RENAME_EQUIP_RECORD=1) on animateRenameEquipPolicy.
type animateRenameEquipCase struct {
	name   string
	prefer []string // cast first, in order; their activations go before any cast
	decks  [2][]string
	// reached reads the gorge side at each decision: the animation's
	// effect is visible on the board.
	reached func(g *GorgeSide) bool
}

func animateRenameEquipCases() []animateRenameEquipCase {
	return []animateRenameEquipCase{{
		// Lightning Bolts fill the graveyard for both evidence costs
		name:   "animate_name_tenth_district_hero_becomes_mileva",
		prefer: []string{"Tenth District Hero", "Lightning Bolt"},
		decks: [2][]string{
			append(deckOf(5, "Tenth District Hero"), append(deckOf(12, "Lightning Bolt"), append(deckOf(7, "Plains"), deckOf(6, "Mountain")...)...)...),
			append(deckOf(8, "Grizzly Bears"), deckOf(16, "Forest")...),
		},
		reached: mileva,
	}, {
		// gorge offers no equip activation beside the same Animate's
		// RemoveAllAbilities$ (its granted Equip:3 is listed but never
		// offered), so the game reaches the animation, not an equip
		name:   "animate_equip_the_irencrag_becomes_everflame",
		prefer: []string{"The Irencrag", "Isamaru, Hound of Konda"},
		decks: [2][]string{
			append(deckOf(4, "The Irencrag"), append(deckOf(10, "Isamaru, Hound of Konda"), deckOf(14, "Plains")...)...),
			append(deckOf(8, "Grizzly Bears"), deckOf(16, "Forest")...),
		},
		reached: everflame,
	}, {
		// Domesticated Watercourse's Animate keeps its abilities and grants
		// it equip {2}, so gorge offers the equip and the kernel hosts the
		// latent on the animated permanent itself; the policy activates the
		// highest mode (the equip) and attaches it to a creature.
		name:   "animate_equip_domesticated_watercourse_equips_a_creature",
		prefer: []string{"Merfolk of the Pearl Trident", "Typhoid Rats", "Dark Ritual", "Domesticated Watercourse"},
		decks: [2][]string{
			append(deckOf(4, "Domesticated Watercourse"), append(deckOf(4, "Dark Ritual"), append(deckOf(4, "Merfolk of the Pearl Trident"), append(deckOf(4, "Typhoid Rats"), append(deckOf(16, "Island"), deckOf(8, "Swamp")...)...)...)...)...),
			deckOf(40, "Forest"),
		},
		reached: watercourseEquipped,
	}}
}

// mileva: a Tenth District Hero on the battlefield is named Mileva, the
// Stalwart with base 5/5.
func mileva(g *GorgeSide) bool {
	gm := g.E.G
	for i := range gm.Objs {
		o := &gm.Objs[i]
		if o.Zone != state.ZBattlefield || o.Face() == nil || o.Face().Name != "Tenth District Hero" {
			continue
		}
		for _, n := range g.E.EffectiveNames() {
			if n.ID == o.ID && n.Name == "Mileva, the Stalwart" {
				if ch := g.E.Chars(o.ID); ch != nil && ch.BasePower == 5 && ch.BaseToughness == 5 {
					return true
				}
			}
		}
	}
	return false
}

// everflame: The Irencrag on the battlefield is a legendary Equipment
// artifact named Everflame, Heroes' Legacy, and has lost its mana ability.
func everflame(g *GorgeSide) bool {
	gm := g.E.G
	for i := range gm.Objs {
		o := &gm.Objs[i]
		if o.Zone != state.ZBattlefield || o.Face() == nil || o.Face().Name != "The Irencrag" {
			continue
		}
		named := false
		for _, n := range g.E.EffectiveNames() {
			named = named || n.ID == o.ID && n.Name == "Everflame, Heroes' Legacy"
		}
		ch := g.E.Chars(o.ID)
		if named && ch != nil && slices.Contains(ch.Types, "Equipment") && slices.Contains(ch.Types, "Legendary") {
			return true
		}
	}
	return false
}

// watercourseEquipped: Domesticated Watercourse on the battlefield is an
// Equipment artifact (its Animate resolved) attached to a creature.
func watercourseEquipped(g *GorgeSide) bool {
	gm := g.E.G
	for i := range gm.Objs {
		o := &gm.Objs[i]
		if o.Zone != state.ZBattlefield || o.Face() == nil || o.Face().Name != "Domesticated Watercourse" || o.AttachedTo == 0 {
			continue
		}
		// Chars returns an engine-owned view the next call may reuse, so
		// read the Equipment check before asking about the host.
		if ch := g.E.Chars(o.ID); ch == nil || !slices.Contains(ch.Types, "Equipment") {
			continue
		}
		host := gm.Obj(o.AttachedTo)
		if host == nil || host.Zone != state.ZBattlefield {
			continue
		}
		if hch := g.E.Chars(host.ID); hch != nil && slices.Contains(hch.Types, "Creature") {
			return true
		}
	}
	return false
}

// animateRenameEquipPolicy activates the case's cards' abilities first,
// then casts its cards, plays a land, attacks with every creature offered and
// answers yes; else random.
func animateRenameEquipPolicy(m *Matchup, c animateRenameEquipCase, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		opts := p.D.Options
		for _, o := range opts {
			if o.Kind() == ActYes {
				return o
			}
		}
		switch p.D.Kind {
		case DAttackers:
			for _, o := range opts {
				if o.Kind() == ActAttack {
					return o
				}
			}
		case DPriority:
			for _, want := range c.prefer {
				// the card's last ability first: Tenth District Hero's
				// second, not the first again on the same evidence
				var best Canon
				found := false
				for _, o := range opts {
					if o.Kind() == ActActivate && m.InstanceCard(int(o.Src())) == want && (!found || o.Mode() > best.Mode()) {
						best, found = o, true
					}
				}
				if found {
					return best
				}
			}
			for _, want := range c.prefer {
				for _, o := range opts {
					if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == want {
						return o
					}
				}
			}
			for _, o := range opts {
				if o.Kind() == ActPlayLand {
					return o
				}
			}
		}
		return rnd.Pick(p)
	})
}

// observed is pol with reached read at each decision the gorge side leads.
func observed(pol Policy, reached func(*GorgeSide) bool, hit *bool) Policy {
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.G != nil && !*hit && reached(p.G) {
			*hit = true
		}
		return pol.Pick(p)
	})
}

func animateRenameEquipPlay(t *testing.T, reg *cards.Registry, c animateRenameEquipCase, r *Repro, set cardset.Set, pol Policy) (Result, bool) {
	t.Helper()
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 200
	if len(r.Actions) == 0 {
		rc.MaxDecisions = 3000 // recording
	}
	if pol == nil {
		pol = animateRenameEquipPolicy(m, c, r.Seed)
	}
	hit := false
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	res := Lockstep(m, r.Seed, k, observed(pol, c.reached, &hit), rc)
	return res, hit
}

// TestAnimateRenameEquipReprosReach replays the recorded games: no
// divergence, every scripted action offered, and the animation reached.
func TestAnimateRenameEquipReprosReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range animateRenameEquipCases() {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			script := &ScriptPolicy{Actions: r.Actions}
			res, hit := animateRenameEquipPlay(t, reg, c, r, charmRepeatSet(t, r.Cards), script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if !hit {
				t.Fatal("the recorded game never reached the animation")
			}
		})
	}
}

// TestAnimateRenameEquipRecord plays the cases' policies over seeds until one
// reaches the animation, and writes the repro. It runs only with
// ANIMATE_RENAME_EQUIP_RECORD=1; the games it writes are the committed repros.
func TestAnimateRenameEquipRecord(t *testing.T) {
	if os.Getenv("ANIMATE_RENAME_EQUIP_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range animateRenameEquipCases() {
		t.Run(c.name, func(t *testing.T) {
			set := charmRepeatSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: "autopay", Kernel: "custom"}
				res, hit := animateRenameEquipPlay(t, reg, c, r, set, nil)
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
				if !hit {
					continue
				}
				r.setActions(res.Actions)
				if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
					t.Fatal(err)
				}
				t.Logf("seed %d: %d decisions", seed, len(res.Actions))
				return
			}
			t.Fatalf("no seed reached %s", c.name)
		})
	}
}
