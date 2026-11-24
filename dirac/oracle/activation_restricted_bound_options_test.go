package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestActivationRestrictedBoundOptionsLockstep: a plain battlefield activation
// the payment-restricted potential bound cannot afford is no offer. gorge
// prices an activation offer against the bound built for that payment
// (pay.AvailableFor, CR 106.6: a restricted source whose RestrictValid$
// refuses the payment adds nothing to it), so Jeweled Lotus's commander-only
// mana is not in the bound an Urza, Lord High Artificer {5} shuffle is priced
// against. The kernel's arm is restrBoundDrops' actBoundDrops
// (cast_restrbound.go), first pinned by a kfuzz recording (synth tail-64 pair
// 137, decision 575).
//
// That recording drifted when gorge began rechecking Hydromorph Guardian's
// TargetValidTargeting$ at resolution (#4414), and by then it no longer
// reached the refusal (its replay agreed with actBoundDrops disabled), so the
// position is a scripted start from its board: Urza cast from hand beside
// Jeweled Lotus, Rustvale Bridge, Thorn of Amethyst and eight lands, both
// seats casting and activating whenever they can. Every game must stay in
// lockstep and reach the refusal: Urza activated once, Jeweled Lotus still
// untapped, and the second activation not offered.
func TestActivationRestrictedBoundOptionsLockstep(t *testing.T) {
	t.Parallel()
	sc, err := ParseScenario([]byte(`{"format":"gorge-start-1","focus":"Urza, Lord High Artificer","players":2,"opponent":"passive","active":0,"max_turns":4,
 "setup":{"p0":{"battlefield":["Swamp","Swamp","Swamp","Island","Island","Island","Island","Island","Jeweled Lotus","Rustvale Bridge","Thorn of Amethyst"],"hand":["Urza, Lord High Artificer"]},
          "p1":{"battlefield":["Island"]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	c, err := OpenCorpus(corpusDirOrSkip(t), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m, err := c.ScenarioMatchup(sc, 0)
	if err != nil {
		t.Fatal(err)
	}
	var names [2][]string
	for p := range names {
		for _, c := range m.SeatDeck(p) {
			names[p] = append(names[p], cardName(c))
		}
	}
	set := cardset.Raw(printedDelveSet(t, names)) // the decks' own table (pool "custom")
	entries := func(d []*cards.Card) []cardset.Entry {
		var out []cardset.Entry
		for _, c := range d {
			n := cardName(c)
			i := slices.IndexFunc(out, func(e cardset.Entry) bool { return e.Name == n })
			if i < 0 {
				out = append(out, cardset.Entry{Name: n})
				i = len(out) - 1
			}
			out[i].Count++
		}
		return out
	}
	tbl, _, _, err := set.Matchup(entries(m.SeatDeck(0)), entries(m.SeatDeck(1)))
	if err != nil {
		t.Fatal(err)
	}
	rc := RunConfig{Config: Config{}, MaxDecisions: 600, MaxTurns: 4}
	const seeds = 8
	next, refusals := uint64(1), 0
	for i := 0; i < seeds; i++ {
		seed := sc.GameSeed(next)
		next = seed + 1
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		rp := RandomPolicy(seed)
		activated := false
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.D.Kind == DPriority && c.D.Player == 0 && activated && c.G != nil && urzaRefusalPosition(c) {
				refusals++
			}
			for _, a := range c.D.Options {
				if k := a.Kind(); k == ActActivate || k == ActCast {
					if k == ActActivate {
						activated = true
					}
					return a
				}
			}
			return rp.Pick(c)
		})
		res := Lockstep(m, seed, k, pol, rc)
		CloseSide(k)
		if !res.Clean() {
			if res.Div == nil {
				t.Fatalf("seed %d: not clean without a divergence: stall %q", seed, res.Stall)
			}
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v (%s)", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail)
		}
	}
	if refusals == 0 {
		t.Fatalf("no game reached the refused second Urza activation beside an untapped Jeweled Lotus in %d games", seeds)
	}
	t.Logf("%d games clean; %d refused-activation decisions", seeds, refusals)
}

// urzaRefusalPosition reports a p0 priority decision with Urza and an
// untapped Jeweled Lotus on p0's battlefield and no activation offered.
func urzaRefusalPosition(c *PickCtx) bool {
	for _, a := range c.D.Options {
		if a.Kind() == ActActivate {
			return false
		}
	}
	g := c.G.E.G
	urza, lotus := false, false
	for _, id := range g.Zone(state.ZBattlefield, 0) {
		o := g.Obj(id)
		switch cardName(o.Card) {
		case "Urza, Lord High Artificer":
			urza = true
		case "Jeweled Lotus":
			lotus = lotus || !o.Tapped
		}
	}
	return urza && lotus
}
