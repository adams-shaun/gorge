package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The escape-grant and graveyard-CantTarget locksteps (the AffectedZone$
// Graveyard statics the AffectedZone-without-MayPlay refusal class used to
// hold out).
//
// Escape (CR 702.42a): Underworld Breach grants every nonland card in its
// controller's graveyard escape at the card's mana cost plus three other
// cards from the same graveyard. The escape cast is offered from the
// graveyard on both sides when the floating pool pays the mana (gorge
// mirrors mayhem's surface handling), the fodder is exiled by a cost pick,
// and the spell returns to its owner's graveyard on resolution.
//
// CantTarget (CR 115.4): Ground Seal's class makes every card in every
// graveyard untargetable, so a Zombify with a creature card in its caster's
// graveyard and a live seal on either battlefield is offered by neither
// engine, and without a live seal both offer it.
//
// The decks avoid every card the kernel refuses (Burning-Tree Emissary's
// graveyard choice does not lower): the mana the escape cast needs is
// floated from Mountains, the graveyard is filled by Thrill of Possibility
// and Mulch.

var escapeDecks = [2][]cardset.Entry{
	{ // A: R
		{Name: "Underworld Breach", Count: 4}, {Name: "Akki Rockspeaker", Count: 4},
		{Name: "Lightning Bolt", Count: 10}, {Name: "Thrill of Possibility", Count: 8},
		{Name: "Mountain", Count: 14},
	},
	{ // B: plains do nothing
		{Name: "Plains", Count: 40},
	},
}

var sealDecks = [2][]cardset.Entry{
	{ // A: G/B
		{Name: "Ground Seal", Count: 4}, {Name: "Zombify", Count: 8},
		{Name: "Satyr Wayfinder", Count: 10}, {Name: "Forest", Count: 9}, {Name: "Swamp", Count: 9},
	},
	{ // B: G/B
		{Name: "Zombify", Count: 8}, {Name: "Satyr Wayfinder", Count: 10},
		{Name: "Forest", Count: 11}, {Name: "Swamp", Count: 11},
	},
}

type escapeStats struct {
	offers, casts int
}

// escapeProbe floats Mountain mana, loots with Thrill of Possibility and
// takes the escape casts, counting the offers gorge surfaces and the casts
// taken.
type escapeProbe struct {
	st   *escapeStats
	base Policy
	r    *rand.Rand
}

func (p *escapeProbe) pick(c *PickCtx) Canon {
	var esc, outlets, lands []Canon
	if c.G != nil {
		for _, x := range c.D.Options {
			o, ok := c.G.GorgeOption(x)
			if !ok {
				continue
			}
			switch {
			case o.Kind == "cast" && o.Mode == "escape":
				esc = append(esc, x)
			case o.Kind == "cast":
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
					n := ob.Face().Name
					// the ETB mana floater (Akki Rockspeaker adds R on
					// entering), the loot outlet and the grant source
					if n == "Thrill of Possibility" || n == "Underworld Breach" || n == "Akki Rockspeaker" {
						outlets = append(outlets, x)
					}
				}
			case o.Kind == "play_land":
				lands = append(lands, x)
			}
		}
		if len(esc) > 0 {
			p.st.offers++
		}
	}
	a := p.base.Pick(c)
	switch {
	case len(esc) > 0 && p.r.IntN(4) != 0:
		a = esc[p.r.IntN(len(esc))]
	case len(outlets) > 0 && p.r.IntN(4) != 0:
		a = outlets[p.r.IntN(len(outlets))]
	case len(lands) > 0 && p.r.IntN(2) != 0:
		a = lands[p.r.IntN(len(lands))]
	}
	if c.G != nil && a.Kind() == ActCast {
		if o, ok := c.G.GorgeOption(a); ok && o.Mode == "escape" {
			p.st.casts++
		}
	}
	return a
}

func runDecks(t *testing.T, seeds uint64, decks [2][]cardset.Entry, name string, policy func(seed uint64) Policy) (diverged bool) {
	t.Helper()
	cardsDir, err := cardset.CorpusDir()
	if err != nil {
		// a box without the corpus skips the corpus tests (testCorpus'
		// convention), rather than failing a fresh clone
		t.Skip(err)
	}
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set := printedDelveSet(t, [2][]string{deckNames(decks[0]), deckNames(decks[1])})
	var full [2][]*cards.Card
	for s, d := range decks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				full[s] = append(full[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{name + "-a", name + "-b"}, full, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, policy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		if d := res.Div; d != nil {
			diverged = true
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

func deckNames(d []cardset.Entry) []string {
	var names []string
	for _, e := range d {
		for i := 0; i < e.Count; i++ {
			names = append(names, e.Name)
		}
	}
	return names
}

// TestEscapeGrantLockstep plays the Underworld Breach decks over seeds and
// requires the escape casts to be offered and taken on both sides identically.
func TestEscapeGrantLockstep(t *testing.T) {
	t.Parallel()
	st := &escapeStats{}
	div := runDecks(t, 128, escapeDecks, "escape", func(seed uint64) Policy {
		p := &escapeProbe{st: st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x65736361))}
		return PolicyFunc(p.pick)
	})
	t.Logf("128 games, escape %+v, %d divergences", *st, b2i(div))
	if div {
		t.Fatal("diverged")
	}
	for _, c := range []struct {
		n, min int
		s      string
	}{
		{st.offers, 10, "escape casts offered"},
		{st.casts, 5, "escape casts taken"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, c.n, c.min)
		}
	}
}

// sealStats tallies the Ground Seal lockstep: the seal's casts, the
// decisions with a live seal, the blocked observations (a live seal, a
// creature card in the decider's graveyard, no Zombify offered) and the
// Zombify casts taken while no seal holds.
type sealStats struct {
	sealCasts, sealLive, blocked, zombCasts int
}

type sealProbe struct {
	st   *sealStats
	base Policy
	r    *rand.Rand
}

func (p *sealProbe) pick(c *PickCtx) Canon {
	var seals, zombs, millers, lands []Canon
	if c.G != nil {
		live := false
		for pl := uint8(0); pl < 2; pl++ {
			for _, id := range c.G.E.G.Zone(state.ZBattlefield, state.PlayerID(pl)) {
				if ob := c.G.E.G.Obj(id); ob != nil && ob.Face() != nil && ob.Face().Name == "Ground Seal" {
					live = true
				}
			}
		}
		graveCreature := false
		for _, id := range c.G.E.G.Zone(state.ZGraveyard, state.PlayerID(c.D.Player)) {
			if ob := c.G.E.G.Obj(id); ob != nil && ob.Face() != nil && ob.Face().IsCreature() {
				graveCreature = true
			}
		}
		if live {
			p.st.sealLive++
		}
		offered := false
		for _, x := range c.D.Options {
			o, ok := c.G.GorgeOption(x)
			if !ok {
				continue
			}
			switch {
			case o.Kind == "cast":
				n := ""
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
					n = ob.Face().Name
				}
				switch n {
				case "Ground Seal":
					seals = append(seals, x)
				case "Zombify":
					offered = true
					zombs = append(zombs, x)
				case "Satyr Wayfinder":
					millers = append(millers, x)
				}
			case o.Kind == "play_land":
				lands = append(lands, x)
			}
		}
		if live && graveCreature && !offered {
			p.st.blocked++
		}
	}
	a := p.base.Pick(c)
	switch {
	case len(zombs) > 0 && p.r.IntN(2) != 0:
		a = zombs[p.r.IntN(len(zombs))]
		if c.G != nil {
			p.st.zombCasts++
		}
	case len(seals) > 0 && p.r.IntN(4) != 0:
		a = seals[p.r.IntN(len(seals))]
		if c.G != nil {
			p.st.sealCasts++
		}
	case len(millers) > 0 && p.r.IntN(2) != 0:
		a = millers[p.r.IntN(len(millers))]
	case len(lands) > 0 && p.r.IntN(2) != 0:
		a = lands[p.r.IntN(len(lands))]
	}
	return a
}

// TestGroundSealLockstep plays the Ground Seal decks over seeds and requires
// the Zombify target offer to track the live seal on both sides identically.
func TestGroundSealLockstep(t *testing.T) {
	t.Parallel()
	st := &sealStats{}
	div := runDecks(t, 128, sealDecks, "seal", func(seed uint64) Policy {
		p := &sealProbe{st: st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x7365616c))}
		return PolicyFunc(p.pick)
	})
	t.Logf("128 games, seal %+v, %d divergences", *st, b2i(div))
	if div {
		t.Fatal("diverged")
	}
	for _, c := range []struct {
		n, min int
		s      string
	}{
		{st.sealCasts, 5, "Ground Seals cast"},
		{st.blocked, 10, "blocked observations (live seal, grave creature, no Zombify)"},
		{st.zombCasts, 5, "Zombify casts while no seal holds"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, c.n, c.min)
		}
	}
}

// countAsDecks play the inert count-as carriers: Pardic Firecat and Diligent
// Farmhand were refused whole on their "CARDNAME count as Flame Burst."
// AffectedZone$ Graveyard static, which gorge's printed Continuous walk
// registers nothing for, so both engines now lower the static to nothing and
// the cards play as their other abilities. Flame Burst, the consumer, stays
// refused by name: its count reads the count-as filter no engine models.
var countAsDecks = [2][]cardset.Entry{
	{ // A: R/G
		{Name: "Pardic Firecat", Count: 4}, {Name: "Diligent Farmhand", Count: 4},
		{Name: "Mountain", Count: 16}, {Name: "Forest", Count: 16},
	},
	{ // B: plains do nothing
		{Name: "Plains", Count: 40},
	},
}

type countAsStats struct {
	firecat, farmhand int
}

type countAsProbe struct {
	st   *countAsStats
	base Policy
	r    *rand.Rand
}

func (p *countAsProbe) pick(c *PickCtx) Canon {
	var bodies, lands []Canon
	if c.G != nil {
		for _, x := range c.D.Options {
			o, ok := c.G.GorgeOption(x)
			if !ok {
				continue
			}
			if o.Kind == "cast" {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
					switch ob.Face().Name {
					case "Pardic Firecat":
						bodies = append(bodies, x)
					case "Diligent Farmhand":
						bodies = append(bodies, x)
					}
				}
			} else if o.Kind == "play_land" {
				lands = append(lands, x)
			}
		}
	}
	a := p.base.Pick(c)
	switch {
	case len(bodies) > 0 && p.r.IntN(4) != 0:
		a = bodies[p.r.IntN(len(bodies))]
	case len(lands) > 0 && p.r.IntN(2) != 0:
		a = lands[p.r.IntN(len(lands))]
	}
	if c.G != nil && a.Kind() == ActCast {
		if o, ok := c.G.GorgeOption(a); ok {
			if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
				switch ob.Face().Name {
				case "Pardic Firecat":
					p.st.firecat++
				case "Diligent Farmhand":
					p.st.farmhand++
				}
			}
		}
	}
	return a
}

// TestCountAsStaticLockstep plays the inert count-as carriers over seeds and
// requires both to cast, attack and die identically on both sides.
func TestCountAsStaticLockstep(t *testing.T) {
	t.Parallel()
	st := &countAsStats{}
	div := runDecks(t, 128, countAsDecks, "countas", func(seed uint64) Policy {
		p := &countAsProbe{st: st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x636f756e))}
		return PolicyFunc(p.pick)
	})
	t.Logf("128 games, countas %+v, %d divergences", *st, b2i(div))
	if div {
		t.Fatal("diverged")
	}
	for _, c := range []struct {
		n, min int
		s      string
	}{
		{st.firecat, 2, "Pardic Firecat casts"},
		{st.farmhand, 2, "Diligent Farmhand casts"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, c.n, c.min)
		}
	}
}

// addCostDecks reach an escape of an additional-cost card: Underworld Breach
// is on the battlefield, Village Rites ({B}, sacrifice a creature) is in the
// graveyard, two Dark Rituals have floated black mana, and a Memnite waits to
// be sacrificed. Village Rites' own sacrifice must be posed before the
// escape fodder (gorge costObjectAsks: sacrifice, subcounter, discard, then
// exAsk); the pre-fix kernel returned the fodder first instead.
var addCostDecks = [2][]cardset.Entry{
	{ // A: B/R
		{Name: "Underworld Breach", Count: 4}, {Name: "Village Rites", Count: 8},
		{Name: "Dark Ritual", Count: 8}, {Name: "Memnite", Count: 8},
		{Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 4},
	},
	{ // B: plains do nothing
		{Name: "Plains", Count: 40},
	},
}

type addCostStats struct {
	offers, casts int
}

// addCostProbe plays the deck's line: Breach, a sacrifice body, Village
// Rites from hand, then the Dark Rituals that float the escape's mana, and
// finally the escape cast.
type addCostProbe struct {
	st   *addCostStats
	base Policy
	r    *rand.Rand
}

func (p *addCostProbe) pick(c *PickCtx) Canon {
	if c.G == nil {
		return p.base.Pick(c)
	}
	eng := c.G.E
	me := c.D.Player
	var esc, breach, rites, ritual, memnites, lands []Canon
	creatures, breachLive, ritesGrave := 0, false, false
	for pl := uint8(0); pl < 2; pl++ {
		for _, id := range eng.G.Zone(state.ZBattlefield, state.PlayerID(pl)) {
			ob := eng.G.Obj(id)
			if ob == nil || ob.Face() == nil {
				continue
			}
			f := ob.Face()
			if f.IsCreature() {
				creatures++
			}
			if pl == me && f.Name == "Underworld Breach" {
				breachLive = true
			}
		}
	}
	for _, id := range eng.G.Zone(state.ZGraveyard, state.PlayerID(me)) {
		if ob := eng.G.Obj(id); ob != nil && ob.Face() != nil && ob.Face().Name == "Village Rites" {
			ritesGrave = true
		}
	}
	for _, x := range c.D.Options {
		o, ok := c.G.GorgeOption(x)
		if !ok {
			continue
		}
		switch {
		case o.Kind == "cast" && o.Mode == "escape":
			if ob := eng.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Village Rites" {
				esc = append(esc, x)
			}
		case o.Kind == "cast":
			if ob := eng.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
				switch ob.Face().Name {
				case "Underworld Breach":
					breach = append(breach, x)
				case "Village Rites":
					rites = append(rites, x)
				case "Dark Ritual":
					ritual = append(ritual, x)
				case "Memnite":
					memnites = append(memnites, x)
				}
			}
		case o.Kind == "play_land":
			lands = append(lands, x)
		}
	}
	if len(esc) > 0 {
		p.st.offers++
	}
	a := p.base.Pick(c)
	switch {
	case len(esc) > 0:
		a = esc[p.r.IntN(len(esc))]
	case len(breach) > 0:
		a = breach[p.r.IntN(len(breach))]
	case breachLive && len(memnites) > 0 && creatures < 2:
		a = memnites[p.r.IntN(len(memnites))]
	case breachLive && len(rites) > 0 && creatures > 0:
		a = rites[p.r.IntN(len(rites))]
	case breachLive && ritesGrave && len(ritual) > 0 && creatures > 0:
		a = ritual[p.r.IntN(len(ritual))]
	case len(lands) > 0 && p.r.IntN(2) != 0:
		a = lands[p.r.IntN(len(lands))]
	}
	if a.Kind() == ActCast {
		if o, ok := c.G.GorgeOption(a); ok && o.Mode == "escape" {
			if ob := eng.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Village Rites" {
				p.st.casts++
			}
		}
	}
	return a
}

// TestEscapeAdditionalCostLockstep escapes Village Rites, whose own
// additional sacrifice must precede the escape fodder on both sides. The
// discard/sacrifice ordering is compared at every decision by Lockstep, so a
// build that poses the three exile picks first diverges.
func TestEscapeAdditionalCostLockstep(t *testing.T) {
	t.Parallel()
	st := &addCostStats{}
	div := runDecks(t, 128, addCostDecks, "addcost", func(seed uint64) Policy {
		p := &addCostProbe{st: st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x61646463))}
		return PolicyFunc(p.pick)
	})
	t.Logf("128 games, addcost %+v, %d divergences", *st, b2i(div))
	if div {
		t.Fatal("diverged")
	}
	for _, c := range []struct {
		n, min int
		s      string
	}{
		{st.offers, 2, "Village Rites escape casts offered"},
		{st.casts, 1, "Village Rites escape casts taken"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, c.n, c.min)
		}
	}
}

// mokDecks exercise the typed-filter grant: The Master of Keys gives escape
// only to Enchantment.YouOwn, unlike Underworld Breach's Card.YouOwn+nonLand.
// Seal of Fire (an enchantment sacrificed to its own ability) is the escape
// fodder's card; Lightning Bolt and Akki Rockspeaker are the non-enchantment
// controls that must never be offered escape under the Keys.
var mokDecks = [2][]cardset.Entry{
	{ // A: W/U/B/R
		{Name: "The Master of Keys", Count: 4}, {Name: "Seal of Fire", Count: 8},
		{Name: "Akki Rockspeaker", Count: 8}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Arcane Sanctum", Count: 8}, {Name: "Mountain", Count: 8},
	},
	{ // B: plains do nothing
		{Name: "Plains", Count: 40},
	},
}

type mokStats struct {
	sealOffers, sealCasts, nonEnchOffers, mokCasts int
}

// mokProbe plays the Keys, bins a Seal of Fire to its own sacrifice ability,
// floats red with Akki Rockspeaker and takes the Seal's escape cast.
type mokProbe struct {
	st   *mokStats
	base Policy
	r    *rand.Rand
}

func (p *mokProbe) pick(c *PickCtx) Canon {
	if c.G == nil {
		return p.base.Pick(c)
	}
	eng := c.G.E
	me := c.D.Player
	var esc, moks, seals, akkis, sacs, lands []Canon
	mokLive := false
	for pl := uint8(0); pl < 2; pl++ {
		for _, id := range eng.G.Zone(state.ZBattlefield, state.PlayerID(pl)) {
			if ob := eng.G.Obj(id); ob != nil && ob.Face() != nil && pl == me && ob.Face().Name == "The Master of Keys" {
				mokLive = true
			}
		}
	}
	for _, x := range c.D.Options {
		o, ok := c.G.GorgeOption(x)
		if !ok {
			continue
		}
		ob := eng.G.Obj(o.Obj)
		name := ""
		ench := false
		if ob != nil && ob.Face() != nil {
			name = ob.Face().Name
			ench = ob.Face().IsEnchantment()
		}
		switch {
		case o.Kind == "cast" && o.Mode == "escape":
			if name == "Seal of Fire" {
				esc = append(esc, x)
			} else if !ench {
				p.st.nonEnchOffers++
			}
		case o.Kind == "cast":
			switch name {
			case "The Master of Keys":
				moks = append(moks, x)
			case "Seal of Fire":
				seals = append(seals, x)
			case "Akki Rockspeaker":
				akkis = append(akkis, x)
			}
		case o.Kind == "ability" && name == "Seal of Fire":
			sacs = append(sacs, x)
		case o.Kind == "play_land":
			lands = append(lands, x)
		}
	}
	if len(esc) > 0 {
		p.st.sealOffers++
	}
	a := p.base.Pick(c)
	switch {
	case len(esc) > 0:
		a = esc[p.r.IntN(len(esc))]
	case mokLive && len(seals) > 0:
		a = seals[p.r.IntN(len(seals))]
	case mokLive && len(sacs) > 0:
		a = sacs[p.r.IntN(len(sacs))]
	case mokLive && len(akkis) > 0:
		a = akkis[p.r.IntN(len(akkis))]
	case len(moks) > 0:
		a = moks[p.r.IntN(len(moks))]
	case len(lands) > 0 && p.r.IntN(2) != 0:
		a = lands[p.r.IntN(len(lands))]
	}
	if a.Kind() == ActCast {
		if o, ok := c.G.GorgeOption(a); ok {
			if ob := eng.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
				switch {
				case o.Mode == "escape" && ob.Face().Name == "Seal of Fire":
					p.st.sealCasts++
				case ob.Face().Name == "The Master of Keys":
					p.st.mokCasts++
				}
			}
		}
	}
	return a
}

// TestMasterOfKeysGrantLockstep plays the typed-filter escape grant and
// requires the Keys' enchantment-only offer on both sides: an enchantment
// (Seal of Fire) is offered and escaped, a non-enchantment never is.
func TestMasterOfKeysGrantLockstep(t *testing.T) {
	t.Parallel()
	st := &mokStats{}
	div := runDecks(t, 128, mokDecks, "mok", func(seed uint64) Policy {
		p := &mokProbe{st: st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x6b657973))}
		return PolicyFunc(p.pick)
	})
	t.Logf("128 games, mok %+v, %d divergences", *st, b2i(div))
	if div {
		t.Fatal("diverged")
	}
	for _, c := range []struct {
		n, min int
		s      string
	}{
		{st.mokCasts, 1, "The Master of Keys casts"},
		{st.sealOffers, 2, "Seal of Fire escape casts offered"},
		{st.sealCasts, 1, "Seal of Fire escape casts taken"},
		{st.nonEnchOffers, 0, "non-enchantment escape offers under the Keys"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, c.n, c.min)
		}
	}
}
