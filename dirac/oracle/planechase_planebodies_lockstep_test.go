package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The planebodies ticket's ChaosEnsues-body shapes, played in lockstep on
// synthetic probe planes. The corpus planes themselves stay refused on their
// TapsForMana shapes (the tapsformana ticket's: dirac/kgen
// TestPlanechasePlaneBodiesRefused), so the bodies lower under the planes'
// own printed SVar lines with no TapsForMana line beside them:
//
//   - the dig plane is The Fertile Lands of Saulvinia's chaos body verbatim:
//     its dig reveals the planar deck's face-down remainder (the face-up
//     current plane at the head is not part of the reveal), remembers the
//     found plane, moves it to the deck's bottom, and the chained DB$
//     ChaosEnsues | Defined$ Remembered erupts that plane -- bottom seat,
//     face down (gorge planar.go's membership boundary) -- before the
//     Cleanup clears the remember;
//   - the draw plane is a plane whose chaos ability draws a card, so the
//     remembered-plane eruption and every deck rotation are observable in
//     the games' hands.
//
// Both engines must queue the same chaos, rotate the planar deck the same
// way and draw the same cards.
const planebodiesDigPlaneSrc = `Name:Planebodies Dig Probe Plane
ManaCost:no cost
Types:Plane Probe
T:Mode$ ChaosEnsues | TriggerZones$ Command | Execute$ PlanebodiesDig | TriggerDescription$ chaos: dig the planar deck, erupt the found plane
SVar:PlanebodiesDig:DB$ DigUntil | Valid$ Plane | DigZone$ PlanarDeck | RememberFound$ True | FoundDestination$ PlanarDeck | FoundLibraryPosition$ -1 | RevealedDestination$ PlanarDeck | RevealedLibraryPosition$ -1 | SubAbility$ PlanebodiesChaos
SVar:PlanebodiesChaos:DB$ ChaosEnsues | Defined$ Remembered | SubAbility$ PlanebodiesCleanup
SVar:PlanebodiesCleanup:DB$ Cleanup | ClearRemembered$ True
Oracle:chaos: dig the planar deck, erupt the found plane
`

const planebodiesDrawPlaneSrc = `Name:Planebodies Draw Probe Plane
ManaCost:no cost
Types:Plane Probe
T:Mode$ ChaosEnsues | TriggerZones$ Command | Execute$ ChaosProbeDraw | TriggerDescription$ chaos: the probe's player draws
SVar:ChaosProbeDraw:DB$ Draw | NumCards$ 1 | Defined$ You
Oracle:chaos: the probe's player draws
`

const planebodiesProbeSrc = `Name:Planebodies Chaos Probe
ManaCost:1
Types:Sorcery
A:SP$ ChaosEnsues | Defined$ You | SpellDescription$ chaos erupts
Oracle:chaos erupts
`

var planebodiesDecks = [2][]cardset.Entry{
	{{Name: "Planebodies Chaos Probe", Count: 12}, {Name: "Island", Count: 28}},
	{{Name: "Planebodies Chaos Probe", Count: 12}, {Name: "Island", Count: 28}},
}

// The Selesnya Loft Gardens replacement probes: seat 0's planar deck leads
// with a plane carrying that card's two ActiveZones$ Command replacement
// bodies (renamed SVars, reworded text -- a fixture is never a corpus
// script), and both seats cast a token-creating and a counter-adding probe
// spell, so the doubling is observed on each seat's own creations and on the
// other seat's. Seat 1's planar deck has no doubling plane: the bodies are
// unscoped (no ValidToken$/ValidPlayer$), so only the one plane doubles.
const planebodiesReplPlaneSrc = `Name:Planebodies Repl Probe Plane
ManaCost:no cost
Types:Plane Probe
R:Event$ CreateToken | ActiveZones$ Command | ReplaceWith$ PlanebodiesReplTok | EffectOnly$ True | Description$ If an effect would make one or more tokens, twice that many of them are made.
SVar:PlanebodiesReplTok:DB$ ReplaceToken | Type$ Amount
R:Event$ AddCounter | ActiveZones$ Command | ValidCard$ Permanent.inZoneBattlefield | EffectOnly$ True | ReplaceWith$ PlanebodiesReplCtr | Description$ If an effect would put one or more counters on a permanent, twice that many go on it.
SVar:PlanebodiesReplCtr:DB$ ReplaceCounter | Amount$ PlanebodiesZ
SVar:PlanebodiesZ:ReplaceCount$CounterNum/Twice
Oracle:If an effect would make one or more tokens, twice that many of them are made.
If an effect would put one or more counters on a permanent, twice that many go on it.
`

const planebodiesTokenProbeSrc = `Name:Planebodies Token Probe
ManaCost:1
Types:Sorcery
A:SP$ Token | TokenAmount$ 1 | TokenScript$ w_1_1_spirit | TokenOwner$ You | SpellDescription$ Make a 1/1 Spirit token.
Oracle:Make a 1/1 Spirit token.
`

const planebodiesCounterProbeSrc = `Name:Planebodies Counter Probe
ManaCost:1
Types:Sorcery
A:SP$ PutCounter | ValidTgts$ Creature | CounterType$ P1P1 | CounterNum$ 2 | SpellDescription$ Two +1/+1 counters go on target creature.
Oracle:Two +1/+1 counters go on target creature.
`

// The CR 614.16 counter-cost phase's battlefield doubler: a non-EffectOnly
// any-kind doubler over every permanent's placement (Loading Zone's body over
// a permanent-wide ValidCard$), so a planeswalker's [+1] loyalty cost doubles
// in gorge's first pass and the reopened EffectOnly class -- whose collection
// ends in the planar scan -- admits the plane's doubler in the second (4
// total). Renamed SVars, reworded text.
const planebodiesCtrDoublerSrc = `Name:Planebodies Ctr Doubler
ManaCost:2
Types:Enchantment
R:Event$ AddCounter | ActiveZones$ Battlefield | ValidCard$ Permanent.inZoneBattlefield | ReplaceWith$ PlanebodiesDblCtr | Description$ If one or more counters would be put on a permanent, twice that many of them go on it instead.
SVar:PlanebodiesDblCtr:DB$ ReplaceCounter | Amount$ PlanebodiesDblY
SVar:PlanebodiesDblY:ReplaceCount$CounterNum/Twice
Oracle:If one or more counters would be put on a permanent, twice that many of them go on it instead.
`

// TestPlanechasePlaneBodiesLockstep plays the chaos probe beside a
// three-plane planar deck on each seat: the dig plane's planar dig erupts a
// draw plane's chaos from its bottom seat and rotates the deck, and the game
// stays decision-for-decision with gorge.
func TestPlanechasePlaneBodiesLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	dig, diags := cards.ParseBytes("planebodies_dig_plane.txt", []byte(planebodiesDigPlaneSrc))
	if dig == nil {
		t.Fatalf("dig plane parse: %v", diags)
	}
	draw, diags := cards.ParseBytes("planebodies_draw_plane.txt", []byte(planebodiesDrawPlaneSrc))
	if draw == nil {
		t.Fatalf("draw plane parse: %v", diags)
	}
	probe, diags := cards.ParseBytes("planebodies_probe.txt", []byte(planebodiesProbeSrc))
	if probe == nil {
		t.Fatalf("probe parse: %v", diags)
	}
	if d := dig.Link(); len(d) != 0 {
		t.Fatalf("dig plane link: %v", d)
	}
	if d := draw.Link(); len(d) != 0 {
		t.Fatalf("draw plane link: %v", d)
	}
	if d := probe.Link(); len(d) != 0 {
		t.Fatalf("probe link: %v", d)
	}
	island, ok := reg.Lookup("Island")
	if !ok {
		t.Fatal("Island not in the corpus")
	}
	pd, err := kgen.TableFromCards(reg, []*cards.Card{dig, draw, probe, island}, "planebodies-oracle")
	if err != nil {
		t.Fatalf("table: %v", err)
	}
	set := cardset.Raw(cardset.NewDataSet(pd))
	// A zero-count entry puts the planes in the matchup table without dealing
	// them: the planar deck is genesis configuration, not a deck card.
	addPlane := func(es []cardset.Entry) []cardset.Entry {
		return append(es, cardset.Entry{Name: "Planebodies Dig Probe Plane"},
			cardset.Entry{Name: "Planebodies Draw Probe Plane"})
	}
	tbl, _, _, err := set.Matchup(addPlane(planebodiesDecks[0]), addPlane(planebodiesDecks[1]))
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s := range gd {
		for _, e := range planebodiesDecks[s] {
			c := probe
			if e.Name != "Planebodies Chaos Probe" {
				c = island
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
	}
	played := map[string]int{}
	for seed := uint64(1); seed <= 4; seed++ {
		m, err := NewMatchupPlanar(reg, [2]string{"planebodies-a", "planebodies-b"}, gd, [2][]*cards.Card{},
			[2][]*cards.Card{{dig, draw, draw}, {dig, draw, draw}}, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, castActivatePlayedPolicy(seed, played), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	if played["Planebodies Chaos Probe"] == 0 {
		t.Fatal("the chaos probe was never cast")
	}

	// The replacement phase: seat 0's planar deck is the Selesnya-shape
	// doubling plane alone -- a genesis shuffle would bury it, and only the
	// face-up current plane's abilities function, so a one-plane deck pins it
	// at the head for the whole game. Seat 1's deck is the draw plane: it
	// carries no doubling body. Both seats cast the token and counter probes,
	// so the unscoped bodies double each seat's own creations and the other
	// seat's, and both engines must agree decision for decision.
	repl, diags := cards.ParseBytes("planebodies_repl_plane.txt", []byte(planebodiesReplPlaneSrc))
	if repl == nil {
		t.Fatalf("repl plane parse: %v", diags)
	}
	tokP, diags := cards.ParseBytes("planebodies_token_probe.txt", []byte(planebodiesTokenProbeSrc))
	if tokP == nil {
		t.Fatalf("token probe parse: %v", diags)
	}
	ctrP, diags := cards.ParseBytes("planebodies_counter_probe.txt", []byte(planebodiesCounterProbeSrc))
	if ctrP == nil {
		t.Fatalf("counter probe parse: %v", diags)
	}
	if d := repl.Link(); len(d) != 0 {
		t.Fatalf("repl plane link: %v", d)
	}
	if d := tokP.Link(); len(d) != 0 {
		t.Fatalf("token probe link: %v", d)
	}
	if d := ctrP.Link(); len(d) != 0 {
		t.Fatalf("counter probe link: %v", d)
	}
	pd2, err := kgen.TableFromCards(reg, []*cards.Card{repl, draw, tokP, ctrP, island}, "planebodies-repl-oracle")
	if err != nil {
		t.Fatalf("repl table: %v", err)
	}
	set2 := cardset.Raw(cardset.NewDataSet(pd2))
	replDecks := [2][]cardset.Entry{
		{{Name: "Planebodies Token Probe", Count: 12}, {Name: "Planebodies Counter Probe", Count: 12}, {Name: "Island", Count: 28}},
		{{Name: "Planebodies Token Probe", Count: 12}, {Name: "Planebodies Counter Probe", Count: 12}, {Name: "Island", Count: 28}},
	}
	addReplPlane := func(es []cardset.Entry) []cardset.Entry {
		return append(es, cardset.Entry{Name: "Planebodies Repl Probe Plane"},
			cardset.Entry{Name: "Planebodies Draw Probe Plane"})
	}
	tbl2, _, _, err := set2.Matchup(addReplPlane(replDecks[0]), addReplPlane(replDecks[1]))
	if err != nil {
		t.Fatal(err)
	}
	var gd2 [2][]*cards.Card
	for s := range gd2 {
		for _, e := range replDecks[s] {
			c := tokP
			switch e.Name {
			case "Planebodies Counter Probe":
				c = ctrP
			case "Island":
				c = island
			}
			for i := 0; i < e.Count; i++ {
				gd2[s] = append(gd2[s], c)
			}
		}
	}
	planars := [2][]*cards.Card{{repl}, {draw}}
	tokCasts, ctrCasts, spirits, doubles, singles := 0, 0, 0, 0, 0
	for seed := uint64(1); seed <= 4; seed++ {
		tokBefore, ctrBefore := played["Planebodies Token Probe"], played["Planebodies Counter Probe"]
		m2, err := NewMatchupPlanar(reg, [2]string{"planebodies-repl-a", "planebodies-repl-b"}, gd2, [2][]*cards.Card{}, planars, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set2, tbl2, Config{})(m2, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m2, seed, Config{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m2, seed, g, k, castActivatePlayedPolicy(seed, played),
			RunConfig{Config: Config{}, MaxDecisions: 3000, MaxTurns: 30})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		if res.StallUnmodelled {
			t.Fatalf("seed %d: unmodelled kernel stall: %s", seed, res.Stall)
		}
		tokCasts += played["Planebodies Token Probe"] - tokBefore
		ctrCasts += played["Planebodies Counter Probe"] - ctrBefore
		for i := range g.E.L.Events {
			e := &g.E.L.Events[i]
			switch e.Kind {
			case events.TokenCreate:
				if strings.Contains(e.Text, "spirit") {
					spirits++
				}
			case events.CounterChange:
				if e.Counter != "P1P1" {
					continue
				}
				if e.Amount == 4 {
					doubles++
				} else if e.Amount == 2 {
					singles++
				}
			}
		}
	}
	if tokCasts == 0 || ctrCasts == 0 {
		t.Fatalf("a probe was never cast: %d token, %d counter", tokCasts, ctrCasts)
	}
	// Every token probe's cast mints one Spirit and the plane doubles them
	// all -- the plane is the face-up current plane for the whole game, and
	// no other source makes Spirits -- so the log holds exactly twice that
	// many TokenCreate mints.
	if spirits != 2*tokCasts {
		t.Fatalf("spirit mints %d for %d token-probe casts: the plane did not double every creation", spirits, tokCasts)
	}
	// Every counter probe's cast puts two +1/+1 counters; doubled, four. An
	// undoubled placement (2) means the plane's replacement missed it.
	if doubles == 0 || singles != 0 {
		t.Fatalf("counter placements: %d doubled, %d undoubled: the plane did not double every placement", doubles, singles)
	}

	// The CR 614.16 counter-cost phase: Kaya, Spirits' Justice's [+1] is a
	// counter COST (Cost$ AddCounter<1/LOYALTY>), whose provenance closes the
	// EffectOnly$ class in gorge's first pass -- the plane's doubler is inert
	// there -- and the Ctr Doubler (non-EffectOnly, permanent-wide) doubles
	// 1->2 in it. The second pass then reopens the class, and its collection
	// ends in the planar scan, so the plane doubles 2->4. The kernel's
	// replCostCtr second pass must count the face-up plane's doubler beside
	// the battlefield's (repl_addcounter.go), or the loyalty counters stop at
	// 2 and the engines diverge. Kaya's [+2] pays 2 loyalty counters at once:
	// doubled twice it is 8, so every positive loyalty add must be a multiple
	// of 4 (an undoubled once is 2 or 4-from-the-[+2], twice an undoubled
	// [+1] is 2, and an unopened EffectOnly class leaves 1).
	kaya, ok := reg.Lookup("Kaya, Spirits' Justice")
	if !ok {
		t.Fatal("Kaya, Spirits' Justice not in the corpus")
	}
	plains, ok := reg.Lookup("Plains")
	if !ok {
		t.Fatal("Plains not in the corpus")
	}
	swamp, ok := reg.Lookup("Swamp")
	if !ok {
		t.Fatal("Swamp not in the corpus")
	}
	dbl, diags := cards.ParseBytes("planebodies_ctr_doubler.txt", []byte(planebodiesCtrDoublerSrc))
	if dbl == nil {
		t.Fatalf("ctr doubler parse: %v", diags)
	}
	if d := dbl.Link(); len(d) != 0 {
		t.Fatalf("ctr doubler link: %v", d)
	}
	pd3, err := kgen.TableFromCards(reg, []*cards.Card{repl, draw, dbl, kaya, island, plains, swamp}, "planebodies-cost-oracle")
	if err != nil {
		t.Fatalf("cost table: %v", err)
	}
	set3 := cardset.Raw(cardset.NewDataSet(pd3))
	costDecks := [2][]cardset.Entry{
		{{Name: "Kaya, Spirits' Justice", Count: 2}, {Name: "Planebodies Ctr Doubler", Count: 6}, {Name: "Plains", Count: 22}, {Name: "Swamp", Count: 22}},
		{{Name: "Kaya, Spirits' Justice", Count: 2}, {Name: "Planebodies Ctr Doubler", Count: 6}, {Name: "Plains", Count: 22}, {Name: "Swamp", Count: 22}},
	}
	tbl3, _, _, err := set3.Matchup(addReplPlane(costDecks[0]), addReplPlane(costDecks[1]))
	if err != nil {
		t.Fatal(err)
	}
	var gd3 [2][]*cards.Card
	for s := range gd3 {
		for _, e := range costDecks[s] {
			c := kaya
			switch e.Name {
			case "Planebodies Ctr Doubler":
				c = dbl
			case "Plains":
				c = plains
			case "Swamp":
				c = swamp
			}
			for i := 0; i < e.Count; i++ {
				gd3[s] = append(gd3[s], c)
			}
		}
	}
	loyaltyAdds, doubledOnce, doubledTwice := 0, 0, 0
	for seed := uint64(1); seed <= 4; seed++ {
		m3, err := NewMatchupPlanar(reg, [2]string{"planebodies-cost-a", "planebodies-cost-b"}, gd3, [2][]*cards.Card{}, planars, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set3, tbl3, Config{})(m3, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m3, seed, Config{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m3, seed, g, k, castActivatePlayedPolicy(seed, played),
			RunConfig{Config: Config{}, MaxDecisions: 3000, MaxTurns: 30})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		if res.StallUnmodelled {
			t.Fatalf("seed %d: unmodelled kernel stall: %s", seed, res.Stall)
		}
		for i := range g.E.L.Events {
			e := &g.E.L.Events[i]
			if e.Kind == events.CounterChange && e.Counter == "LOYALTY" && e.Amount > 0 {
				if strings.Contains(e.Text, "__entry_counter_notice") {
					continue // the walker's own starting loyalty, not a cost
				}
				loyaltyAdds++
				if e.Amount == 4 {
					doubledOnce++ // the [+1], pass 1 and pass 2 both on it
				}
				if e.Amount == 8 {
					doubledTwice++ // the [+2], pass 1 and pass 2 both on it
				}
			}
		}
	}
	if loyaltyAdds == 0 {
		t.Fatal("Kaya's [+N] loyalty cost was never paid")
	}
	// An early [+N] before the Ctr Doubler enters is legitimately undoubled,
	// but once it is out every add is pass 1's doubler and, through the
	// reopened EffectOnly class, the plane's: a [+1] is 4, the [+2] is 8.
	if doubledOnce == 0 || doubledTwice == 0 {
		t.Fatalf("the CR 614.16 chain never completed: %d doubled-once [+1]s, %d doubled-twice [+2]s", doubledOnce, doubledTwice)
	}

	// The contested rewriter phase: Holo-Engineer's CreateToken rewriter
	// beside the plane's token doubler is gorge's CR 616.1 order ask, which
	// the kernel's token walk (replTokensOp's contested branch; the same
	// count's silent omission at the rewriter mint sites replTokensCard)
	// cannot pose -- it must stop on the declared StallTokRewrite, never omit
	// the plane's doubling. Any stall the phase reaches must be that one.
	holo, ok := reg.Lookup("Holo-Engineer")
	if !ok {
		t.Fatal("Holo-Engineer not in the corpus")
	}
	forest, ok := reg.Lookup("Forest")
	if !ok {
		t.Fatal("Forest not in the corpus")
	}
	pd4, err := kgen.TableFromCards(reg, []*cards.Card{repl, draw, holo, forest}, "planebodies-rewriter-oracle")
	if err != nil {
		t.Fatalf("rewriter table: %v", err)
	}
	set4 := cardset.Raw(cardset.NewDataSet(pd4))
	rewDecks := [2][]cardset.Entry{
		{{Name: "Holo-Engineer", Count: 12}, {Name: "Forest", Count: 40}},
		{{Name: "Holo-Engineer", Count: 12}, {Name: "Forest", Count: 40}},
	}
	tbl4, _, _, err := set4.Matchup(addReplPlane(rewDecks[0]), addReplPlane(rewDecks[1]))
	if err != nil {
		t.Fatal(err)
	}
	var gd4 [2][]*cards.Card
	for s := range gd4 {
		for _, e := range rewDecks[s] {
			c := holo
			if e.Name != "Holo-Engineer" {
				c = forest
			}
			for i := 0; i < e.Count; i++ {
				gd4[s] = append(gd4[s], c)
			}
		}
	}
	tokStalls := 0
	for seed := uint64(1); seed <= 4; seed++ {
		m4, err := NewMatchupPlanar(reg, [2]string{"planebodies-rew-a", "planebodies-rew-b"}, gd4, [2][]*cards.Card{}, planars, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set4, tbl4, Config{})(m4, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m4, seed, k, castActivatePlayedPolicy(seed, played), RunConfig{Config: Config{}, MaxDecisions: 3000, MaxTurns: 30})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		// StallTokRewrite is an unmodelled-class stall BY DESIGN here: gorge
		// poses the CR 616.1 order ask, the kernel declares the gap loudly
		// instead of silently omitting the plane's doubling. So the phase
		// asserts the stall's identity, not its class.
		if res.Stall != "" {
			if !strings.Contains(res.Stall, "tokrewrite") {
				t.Fatalf("seed %d: a stall other than the token-rewrite contest: %s", seed, res.Stall)
			}
			tokStalls++
		}
	}
	if tokStalls == 0 {
		t.Fatal("no token-rewrite stall: the rewriter and the plane never contested a creation")
	}
}
