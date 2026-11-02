package oracle

import (
	"github.com/adams-shaun/gorge/dirac"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Pauper keyword-mechanics / mana-misc / singletons lockstep
// (CARD-CENSUS.md): constructed matchups over the generated
// pauperkw pool, one test per carrier group, each counting on gorge's side
// that its carriers did what they print.

// pkPolicy casts or plays a land whenever it can (else random, activations
// included); every fourth seed plays fully at random.
func pkPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	if seed%4 == 0 {
		return rnd
	}
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DPriority {
			buf = buf[:0]
			for _, a := range c.D.Options {
				if k := a.Kind(); k == ActCast || k == ActPlayLand {
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 {
				d := *c.D
				d.Options = buf
				return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
		}
		return rnd.Pick(c)
	})
}

// pkRun plays seeds 1..n of every matchup in lockstep, calling check (may be
// nil) at every decision, and fails on any divergence or stall.
func pkRun(t *testing.T, matchups [][2][]cardset.Entry, n uint64, maxTurns int32, check func(*GorgeSide, Side) *Divergence) int {
	t.Helper()
	games, _ := pkRunQ(t, matchups, n, maxTurns, check, false)
	return games
}

// pkRunQ is pkRun returning the diverging games too; quiet logs them
// instead of failing (a mutation test wants them).
func pkRunQ(t *testing.T, matchups [][2][]cardset.Entry, n uint64, maxTurns int32, check func(*GorgeSide, Side) *Divergence, quiet bool) (int, int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("pauperkw")
	if err != nil {
		t.Fatal(err)
	}
	games, divs := 0, 0
	for mi, md := range matchups {
		var decks [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], c)
				}
			}
			if len(decks[s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= n; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"pk-a", "pk-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, pkPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: maxTurns, Check: check})
			CloseSide(k)
			games++
			if res.Stall != "" && res.Stall != "turns" {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				divs++
				lo := max(d.At-12, 0)
				if quiet {
					continue
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs
}

// pkFaceName is o's face name, "" for none.
func pkFaceName(o *state.Object) string {
	if o == nil || o.Face() == nil {
		return ""
	}
	return o.Face().Name
}

// Troll of Khazad-dum: S:Mode$ MinMaxBlocker Min$ 3 (CR 509.1b,
// CardDef.MinBlockers) and swampcycling. Counted: Troll attacks, attacks
// blocked (by three or more), and attacks into a board with one or two
// untapped creatures (where gorge offers no block of it at all).
func TestPauperkwTrollLockstep(t *testing.T) {
	t.Parallel()
	decks := [][2][]cardset.Entry{{
		{
			{Name: "Troll of Khazad-dûm", Count: 12}, {Name: "Walking Corpse", Count: 6},
			{Name: "Shock", Count: 4}, {Name: "Swamp", Count: 10}, {Name: "Mountain", Count: 8},
		},
		{
			{Name: "Llanowar Elves", Count: 6}, {Name: "Grizzly Bears", Count: 6}, {Name: "Savannah Lions", Count: 6},
			{Name: "Raging Goblin", Count: 4}, {Name: "Forest", Count: 9}, {Name: "Plains", Count: 9},
		},
	}}
	type key struct {
		turn int32
		obj  state.ObjID
	}
	attacks := map[key]int{}
	cycles, evAt := 0, 0
	check := func(ref *GorgeSide, _ Side) *Divergence {
		g := ref.E.G
		for p := range g.Players {
			for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
				o := g.Obj(id)
				if o == nil || !o.IsAttacking || pkFaceName(o) != "Troll of Khazad-dûm" {
					continue
				}
				kk := key{g.Turn, id}
				if n := len(o.BlockedBy); n+1 > attacks[kk] {
					attacks[kk] = n + 1
				}
			}
		}
		evs := ref.E.L.Events
		if len(evs) < evAt {
			evAt = 0
		}
		for j := evAt; j < len(evs); j++ {
			if e := &evs[j]; e.Kind == events.MoveZone && e.From == state.ZHand && e.To == state.ZGraveyard &&
				g.Step != state.StepCleanup && pkFaceName(g.Obj(e.Obj)) == "Troll of Khazad-dûm" {
				cycles++
			}
		}
		evAt = len(evs)
		return nil
	}
	games := pkRun(t, decks, 80, 60, check)
	blocked, short := 0, 0
	for _, n := range attacks {
		switch {
		case n-1 >= 3:
			blocked++
		case n-1 > 0:
			short++
		}
	}
	t.Logf("%d games: %d Troll attacks, %d blocked by three or more, %d by fewer; %d Trolls hand to graveyard (swampcycling)", games, len(attacks), blocked, short, cycles)
	if short != 0 {
		t.Errorf("%d Troll attacks blocked by one or two creatures", short)
	}
	if len(attacks) < 20 || blocked < 5 || cycles < 10 {
		t.Errorf("too little coverage: %d attacks, %d blocked, %d cycled", len(attacks), blocked, cycles)
	}
}

// pkEvents calls f for every gorge event logged since the last call (the
// cursor resets when a new game's log is shorter).
type pkEvents struct{ at int }

func (c *pkEvents) each(ref *GorgeSide, f func(e *events.Event)) {
	evs := ref.E.L.Events
	if len(evs) < c.at {
		c.at = 0
	}
	for j := c.at; j < len(evs); j++ {
		f(&evs[j])
	}
	c.at = len(evs)
}

// Winding Way: ChooseType | Type$ Card | ValidTypes$ Creature,Land, then a
// Dig of the chosen type (OpChooseType C = card types; F2ChosenType reading
// the chosen card type). Counted: each answer, and the cards the dig put
// into a hand and a graveyard.
func TestPauperkwWindingWayLockstep(t *testing.T) {
	t.Parallel()
	decks := [][2][]cardset.Entry{{
		{
			{Name: "Winding Way", Count: 10}, {Name: "Llanowar Elves", Count: 8}, {Name: "Grizzly Bears", Count: 6},
			{Name: "Shock", Count: 2}, {Name: "Forest", Count: 10}, {Name: "Mountain", Count: 4},
		},
		{
			{Name: "Winding Way", Count: 6}, {Name: "Savannah Lions", Count: 8}, {Name: "Raging Goblin", Count: 6},
			{Name: "Shock", Count: 4}, {Name: "Forest", Count: 8}, {Name: "Plains", Count: 4}, {Name: "Mountain", Count: 4},
		},
	}}
	chosen := map[string]int{}
	var cur pkEvents
	toHand, toGrave := 0, 0
	check := func(ref *GorgeSide, _ Side) *Divergence {
		g := ref.E.G
		cur.each(ref, func(e *events.Event) {
			if e.Kind == events.Choose && e.Counter == "type" && pkFaceName(g.Obj(e.Obj)) == "Winding Way" {
				chosen[e.Text]++
			}
			if e.Kind == events.MoveZone && e.From == state.ZLibrary {
				switch e.To {
				case state.ZHand:
					toHand++
				case state.ZGraveyard:
					toGrave++
				}
			}
		})
		return nil
	}
	games := pkRun(t, decks, 80, 40, check)
	t.Logf("%d games: Winding Way chose %v; library to hand %d (draws included), to graveyard %d", games, chosen, toHand, toGrave)
	if chosen["Creature"] < 20 || chosen["Land"] < 20 || toGrave < 40 {
		t.Errorf("too little coverage: chose %v, %d milled", chosen, toGrave)
	}
}

// Masked Vandal: its enters trigger's own ExileFromGrave<1/Creature> cost
// (PayExileGrave: gorge's trigger_cost pay/decline, then the exile_cost
// pick). Counted: Vandal triggers paid (a creature card exiled from a
// graveyard), and artifacts / enchantments exiled from the battlefield.
func TestPauperkwMaskedVandalLockstep(t *testing.T) {
	t.Parallel()
	decks := [][2][]cardset.Entry{{
		{
			{Name: "Masked Vandal", Count: 12}, {Name: "Grizzly Bears", Count: 6}, {Name: "Shock", Count: 6},
			{Name: "Forest", Count: 9}, {Name: "Mountain", Count: 7},
		},
		{
			{Name: "Ornithopter", Count: 6}, {Name: "Pacifism", Count: 6}, {Name: "Bonesplitter", Count: 4},
			{Name: "Savannah Lions", Count: 6}, {Name: "Masked Vandal", Count: 4}, {Name: "Plains", Count: 8}, {Name: "Forest", Count: 6},
		},
	}}
	var cur pkEvents
	graveEx, permEx, picks := 0, 0, 0
	check := func(ref *GorgeSide, _ Side) *Divergence {
		cur.each(ref, func(e *events.Event) {
			if e.Kind != events.MoveZone || e.To != state.ZExile {
				return
			}
			switch e.From {
			case state.ZGraveyard:
				graveEx++
			case state.ZBattlefield:
				permEx++
			}
		})
		if d := ref.E.Pending(); d != nil {
			for _, o := range d.Options {
				if o.Kind == "exile_cost" {
					picks++
					break
				}
			}
		}
		return nil
	}
	games := pkRun(t, decks, 80, 40, check)
	t.Logf("%d games: graveyard exiles %d (exile_cost frames %d), battlefield exiles %d", games, graveEx, picks, permEx)
	if graveEx < 30 || permEx < 20 || picks < 10 {
		t.Errorf("too little coverage: %d graveyard exiles, %d picks, %d permanents exiled", graveEx, picks, permEx)
	}
}

// Highway Robbery: the controller's resolution GenericChoice over two
// UnlessSwitched branches (discard a card / sacrifice a land, then draw
// two), every branch offered payable or not (OpChooseBranch C 1), and plot.
// Counted: each branch's answer, the draws it paid for, and plot exiles.
func TestPauperkwHighwayRobberyLockstep(t *testing.T) {
	t.Parallel()
	decks := [][2][]cardset.Entry{{
		{
			{Name: "Highway Robbery", Count: 12}, {Name: "Raging Goblin", Count: 6}, {Name: "Shock", Count: 6},
			{Name: "Mountain", Count: 16},
		},
		{
			{Name: "Highway Robbery", Count: 8}, {Name: "Savannah Lions", Count: 8}, {Name: "Grizzly Bears", Count: 6},
			{Name: "Plains", Count: 6}, {Name: "Forest", Count: 6}, {Name: "Mountain", Count: 6},
		},
	}}
	var cur pkEvents
	modes := map[int]int{}
	sacLand, discards, plots := 0, 0, 0
	var lastD any
	check := func(ref *GorgeSide, _ Side) *Divergence {
		g := ref.E.G
		if d := ref.E.Pending(); d != nil && any(d) != lastD {
			lastD = d
			if d.ResumeKind == "modes" && pkFaceName(g.Obj(d.Source)) == "Highway Robbery" {
				modes[len(d.Options)]++
			}
		}
		cur.each(ref, func(e *events.Event) {
			if e.Kind != events.MoveZone {
				return
			}
			o := g.Obj(e.Obj)
			switch {
			case e.From == state.ZHand && e.To == state.ZExile && pkFaceName(o) == "Highway Robbery":
				plots++
			case e.From == state.ZBattlefield && e.To == state.ZGraveyard && o != nil && o.Face() != nil && o.Face().IsLand():
				sacLand++
			case e.From == state.ZHand && e.To == state.ZGraveyard && g.Step != state.StepCleanup:
				discards++
			}
		})
		return nil
	}
	games := pkRun(t, decks, 80, 40, check)
	t.Logf("%d games: Highway Robbery mode asks %v; lands sacrificed %d, discards %d, plotted %d", games, modes, sacLand, discards, plots)
	if modes[2] < 30 || sacLand < 10 || discards < 10 || plots < 5 {
		t.Errorf("too little coverage")
	}
}

// Sacred Cat: embalm (CR 702.128a): a graveyard activation (W, exile this
// card) making a token copy with copy exceptions white, Zombie, no mana
// cost (Override.SetColors / AddSub / NoCost). Counted: embalm exiles and
// token Sacred Cats on the battlefield.
func TestPauperkwEmbalmLockstep(t *testing.T) {
	t.Parallel()
	decks := [][2][]cardset.Entry{{
		{
			{Name: "Sacred Cat", Count: 12}, {Name: "Savannah Lions", Count: 6}, {Name: "Shock", Count: 6},
			{Name: "Plains", Count: 10}, {Name: "Mountain", Count: 6},
		},
		{
			{Name: "Shock", Count: 8}, {Name: "Raging Goblin", Count: 8}, {Name: "Grizzly Bears", Count: 6},
			{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 8},
		},
	}, {
		// blue embalm carriers against colour- and MV-sensitive removal: the
		// white, mana-value-0 token is no Sunlance target (nonwhite) and is
		// a Smother target (mana value 3 or less) where the card is not
		{
			{Name: "Aven Initiate", Count: 8}, {Name: "Tah-Crop Skirmisher", Count: 8}, {Name: "Shock", Count: 4},
			{Name: "Island", Count: 14}, {Name: "Mountain", Count: 6},
		},
		{
			{Name: "Smother", Count: 6}, {Name: "Sunlance", Count: 6}, {Name: "Shock", Count: 6}, {Name: "Raging Goblin", Count: 4},
			{Name: "Swamp", Count: 6}, {Name: "Plains", Count: 6}, {Name: "Mountain", Count: 6},
		},
	}}
	var cur pkEvents
	embalmed, tokens := 0, 0
	tokTargets := map[string]int{}
	check := func(ref *GorgeSide, _ Side) *Divergence {
		g := ref.E.G
		cur.each(ref, func(e *events.Event) {
			if e.Kind != events.MoveZone {
				return
			}
			o := g.Obj(e.Obj)
			if n := pkFaceName(o); n != "Sacred Cat" && n != "Aven Initiate" && n != "Tah-Crop Skirmisher" {
				return
			}
			if e.From == state.ZGraveyard && e.To == state.ZExile {
				embalmed++
			}
			if e.To == state.ZBattlefield && o.IsToken {
				tokens++
			}
		})
		if d := ref.E.Pending(); d != nil && d.Kind == decision.KTarget {
			if src := pkFaceName(g.Obj(d.Source)); src == "Smother" || src == "Sunlance" {
				for _, op := range d.Options {
					if o := g.Obj(op.Obj); o != nil && o.IsToken {
						tokTargets[src]++
					}
				}
			}
		}
		return nil
	}
	games := pkRun(t, decks, 80, 40, check)
	t.Logf("%d games: embalm exiles %d, embalm tokens entered %d, token target options %v", games, embalmed, tokens, tokTargets)
	if tokTargets["Sunlance"] != 0 {
		t.Errorf("Sunlance offered a white embalm token %d times", tokTargets["Sunlance"])
	}
	if embalmed < 20 {
		t.Errorf("too little coverage")
	}
}

// Chain Lightning: after its damage, the targeted player or the targeted
// permanent's controller may pay {R}{R} (UnlessSwitched, payer
// TargetedOrController) to copy the resolving spell (OpCopySpell of Self,
// C&4: the copy is theirs; C&1: they may choose a new target). Counted:
// Chain Lightning resolutions and copies.
func TestPauperkwChainLightningLockstep(t *testing.T) {
	t.Parallel()
	decks := [][2][]cardset.Entry{{
		{
			{Name: "Chain Lightning", Count: 14}, {Name: "Raging Goblin", Count: 8}, {Name: "Mountain", Count: 18},
		},
		{
			{Name: "Chain Lightning", Count: 10}, {Name: "Grizzly Bears", Count: 6}, {Name: "Savannah Lions", Count: 6},
			{Name: "Mountain", Count: 12}, {Name: "Forest", Count: 6},
		},
	}}
	var cur pkEvents
	copies, pays := 0, 0
	var lastD any
	check := func(ref *GorgeSide, _ Side) *Divergence {
		g := ref.E.G
		if d := ref.E.Pending(); d != nil && any(d) != lastD {
			lastD = d
			if pkFaceName(g.Obj(d.Source)) == "Chain Lightning" && d.ResumeKind != "" {
				pays++
			}
		}
		cur.each(ref, func(e *events.Event) {
			if e.Kind == events.StackCopy {
				copies++
			}
		})
		return nil
	}
	games := pkRun(t, decks, 80, 40, check)
	t.Logf("%d games: Chain Lightning resolution asks %d, copies %d", games, pays, copies)
	if copies < 10 {
		t.Errorf("too little coverage")
	}
}

// Sheoldred, the Apocalypse: its Drawn triggers, ValidCard$ Card.YouCtrl
// (gain 2) and Card.OppCtrl (the drawing opponent loses 2; a card drawn is
// controlled by its owner). Counted: life gained and lost to the triggers.
func TestPauperkwSheoldredLockstep(t *testing.T) {
	t.Parallel()
	decks := [][2][]cardset.Entry{{
		{
			{Name: "Sheoldred, the Apocalypse", Count: 10}, {Name: "Night's Whisper", Count: 6}, {Name: "Walking Corpse", Count: 8},
			{Name: "Swamp", Count: 16},
		},
		{
			{Name: "Divination", Count: 8}, {Name: "Sheoldred, the Apocalypse", Count: 4}, {Name: "Grizzly Bears", Count: 8},
			{Name: "Island", Count: 10}, {Name: "Swamp", Count: 6}, {Name: "Forest", Count: 4},
		},
	}}
	seen := map[state.ObjID]bool{}
	gains, losses := 0, 0
	check := func(ref *GorgeSide, _ Side) *Divergence {
		g := ref.E.G
		for _, id := range g.Stack {
			o := g.Obj(id)
			if o == nil || o.Ability == nil || seen[id] || pkFaceName(g.Obj(o.Source)) != "Sheoldred, the Apocalypse" {
				continue
			}
			seen[id] = true
			if o.Ability.API == "GainLife" {
				gains++
			} else {
				losses++
			}
		}
		return nil
	}
	games := pkRun(t, decks, 80, 40, check)
	t.Logf("%d games: Sheoldred life gains %d, losses %d", games, gains, losses)
	if gains < 20 || losses < 20 {
		t.Errorf("too little coverage")
	}
}

// Dusk Rose Reliquary: a one-part AlternateAdditionalCost (sacrifice an
// artifact or creature: CardDef.SpellCost, gorge altAddAsk's lone part), ward
// {2} and an until-leaves exile. Counted: Reliquary casts' sacrifices and
// its exiles.
func TestPauperkwDuskRoseLockstep(t *testing.T) {
	t.Parallel()
	decks := [][2][]cardset.Entry{{
		{
			{Name: "Dusk Rose Reliquary", Count: 10}, {Name: "Ornithopter", Count: 8}, {Name: "Savannah Lions", Count: 8},
			{Name: "Plains", Count: 14},
		},
		{
			{Name: "Grizzly Bears", Count: 8}, {Name: "Ornithopter", Count: 6}, {Name: "Shock", Count: 8},
			{Name: "Forest", Count: 9}, {Name: "Mountain", Count: 9},
		},
	}}
	var cur pkEvents
	sacs, exiles := 0, 0
	check := func(ref *GorgeSide, _ Side) *Divergence {
		g := ref.E.G
		cur.each(ref, func(e *events.Event) {
			if e.Kind != events.MoveZone || e.From != state.ZBattlefield {
				return
			}
			if e.To == state.ZExile {
				exiles++
			}
			if e.To == state.ZGraveyard && len(g.Stack) > 0 && pkFaceName(g.Obj(g.Stack[len(g.Stack)-1])) == "Dusk Rose Reliquary" {
				sacs++
			}
		})
		return nil
	}
	games := pkRun(t, decks, 80, 40, check)
	t.Logf("%d games: sacrifices under a Reliquary cast %d, battlefield exiles %d", games, sacs, exiles)
	if exiles < 20 {
		t.Errorf("too little coverage")
	}
}

// Maelstrom Pulse: destroy the target and every other permanent with its
// name (PredSameNameT0). Counted: Pulse resolutions and the permanents
// they destroyed beside the target.
func TestPauperkwMaelstromPulseLockstep(t *testing.T) {
	t.Parallel()
	decks := [][2][]cardset.Entry{{
		{
			{Name: "Maelstrom Pulse", Count: 12}, {Name: "Walking Corpse", Count: 6}, {Name: "Llanowar Elves", Count: 4},
			{Name: "Swamp", Count: 8}, {Name: "Forest", Count: 10},
		},
		{
			{Name: "Grizzly Bears", Count: 12}, {Name: "Ornithopter", Count: 8}, {Name: "Llanowar Elves", Count: 4},
			{Name: "Forest", Count: 8}, {Name: "Plains", Count: 8},
		},
	}}
	var cur pkEvents
	multi, kills := 0, 0
	check := func(ref *GorgeSide, _ Side) *Divergence {
		g := ref.E.G
		n, pulsed := 0, false
		cur.each(ref, func(e *events.Event) {
			if e.Kind != events.MoveZone {
				return
			}
			if e.From == state.ZBattlefield && e.To == state.ZGraveyard {
				n++
			}
			if e.From == state.ZStack && pkFaceName(g.Obj(e.Obj)) == "Maelstrom Pulse" {
				pulsed = true
			}
		})
		if pulsed {
			kills += n
			if n > 1 {
				multi++
			}
		}
		return nil
	}
	games := pkRun(t, decks, 80, 40, check)
	t.Logf("%d games: Pulse kills %d, resolutions destroying two or more %d", games, kills, multi)
	if kills < 30 || multi < 5 {
		t.Errorf("too little coverage")
	}
}

// Fiery Temper: madness (CR 702.35a-b; CardDef.HasMadness, the native
// abMadness trigger). Discards from additional costs (Thrill of
// Possibility, Tormenting Voice), an effect (Faithless Looting, the
// opponent's Mind Rot) and cleanup. Counted: madness exiles, madness casts
// (Fiery Temper pushed onto the stack from exile), declines (exile to
// graveyard).
func kpauperkwMadnessRun(t *testing.T, n uint64, quiet bool) (games, divs, exiled, cast, declined int) {
	decks := [][2][]cardset.Entry{{
		{
			{Name: "Fiery Temper", Count: 10}, {Name: "Thrill of Possibility", Count: 4}, {Name: "Tormenting Voice", Count: 4},
			{Name: "Faithless Looting", Count: 4}, {Name: "Raging Goblin", Count: 4}, {Name: "Mountain", Count: 14},
		},
		{
			{Name: "Mind Rot", Count: 8}, {Name: "Walking Corpse", Count: 8}, {Name: "Fiery Temper", Count: 6},
			{Name: "Swamp", Count: 10}, {Name: "Mountain", Count: 8},
		},
	}}
	var cur pkEvents
	check := func(ref *GorgeSide, _ Side) *Divergence {
		g := ref.E.G
		cur.each(ref, func(e *events.Event) {
			switch {
			case e.Kind == events.MoveZone && e.From == state.ZHand && e.To == state.ZExile && pkFaceName(g.Obj(e.Obj)) == "Fiery Temper":
				exiled++
			case e.Kind == events.MoveZone && e.From == state.ZExile && e.To == state.ZGraveyard && pkFaceName(g.Obj(e.Obj)) == "Fiery Temper":
				declined++
			case e.Kind == events.PutOnStack && e.From == state.ZExile && pkFaceName(g.Obj(e.Obj)) == "Fiery Temper":
				cast++
			}
		})
		return nil
	}
	games, divs = pkRunQ(t, decks, n, 40, check, quiet)
	return
}

func TestPauperkwMadnessLockstep(t *testing.T) {
	t.Parallel()
	// gorge bugs fixed in the pin: askMadnessCast gated "yes" on the floating
	// pool only (82b67aeb0); resolveMadnessChoice skipped the CR 117.3b
	// priority reset after a madness cast (1eeda1c30).
	games, _, exiled, cast, declined := kpauperkwMadnessRun(t, 600, false)
	t.Logf("%d games: Fiery Temper madness exiles %d, casts %d, declined %d", games, exiled, cast, declined)
	if exiled < 200 || cast < 50 || declined < 50 {
		t.Errorf("too little coverage")
	}
}

// TestPauperkwMadnessLockstepMutation: with the madness cast's mana paid
// by the planner at the mana step (gorge pays it through its window, first
// source in battlefield order) some seed diverges.
func TestPauperkwMadnessLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.SetDebugMadnessMutate(1)
	defer dirac.SetDebugMadnessMutate(0)
	_, divs, _, _, _ := kpauperkwMadnessRun(t, 100, true)
	if divs == 0 {
		t.Errorf("no seed diverged with the madness window payment dropped")
	}
}
