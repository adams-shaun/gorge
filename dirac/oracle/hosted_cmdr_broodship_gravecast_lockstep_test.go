package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Exploration Broodship's STATION 8+ grant, played decision by decision
// against gorge (gorge leads): the outer self static (Card.Self with 8+
// CHARGE counters) grants the inner BroodshipStatic, "Once during each of
// your turns, you may cast a permanent spell from your graveyard by
// sacrificing a land in addition to paying its other costs" (Condition$
// PlayerTurn, MayPlayLimit$ 1, RaiseCost$ Sac<1/Land>). gorge reads the
// granted body through mayPlayBoardStatics (engine/rules/mayplay_granted_raise.go);
// kgen lowers it as the inner static ANDed with the station gate and your
// turn, the raise a MayRaise latent (dirac/kgen/lower_mayraise.go).
//
// "Station Titan Probe" is an inline {G} 8/8 that stations a Broodship to 8
// in one tap; "Bone Altar Probe" (a {1} artifact, "Sacrifice an artifact:
// gain 1 life") puts a second, un-stationed Broodship into the graveyard
// (Ashnod's Altar, the obvious outlet, is itself refused: its sacrifice
// mana source is a payment-planner gap). The case FAILS unless some seed casts a permanent spell from
// seat 0's graveyard and sacrifices a land for it.
const stationTitanProbeScript = "Name:Station Titan Probe\nManaCost:G\nTypes:Creature Giant\nPT:8/8\n" +
	"Oracle:A big body that stations a Spacecraft in one tap.\n"

const boneAltarProbeScript = "Name:Bone Altar Probe\nManaCost:1\nTypes:Artifact\n" +
	"A:AB$ GainLife | Cost$ Sac<1/Artifact> | LifeAmount$ 1 | SpellDescription$ You gain 1 life.\n" +
	"Oracle:Sacrifice an artifact: You gain 1 life.\n"

func TestHostedCmdrBroodshipStationGravecastLockstep(t *testing.T) {
	const ship, titan, altar = "Exploration Broodship", "Station Titan Probe", "Bone Altar Probe"
	dir := filters11Corpus(t, map[string]string{titan: stationTitanProbeScript, altar: boneAltarProbeScript}, ship, "Forest", "Grizzly Bears")
	reg, err := cards.SharedCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := [2][]string{
		append(append(deckOf(8, ship), deckOf(8, titan)...), append(deckOf(4, altar), deckOf(20, "Forest")...)...),
		append(deckOf(8, "Grizzly Bears"), deckOf(32, "Forest")...),
	}
	var decks [2][]*cards.Card
	for s, ns := range names {
		for _, n := range ns {
			c, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("%s not in the corpus", n)
			}
			decks[s] = append(decks[s], c)
		}
	}
	entries := func(ns []string) []cardset.Entry {
		es := make([]cardset.Entry, len(ns))
		for i, n := range ns {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	// printedDelveSetIn runs kgen over the decks: a refused card fails here.
	set := printedDelveSetIn(t, dir, names)
	tbl, _, _, err := set.Matchup(entries(names[0]), entries(names[1]))
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{}
	seen, played := false, 0
	for seed := uint64(1); seed <= 60 && !(seen && played >= 3); seed++ {
		m, err := NewMatchup(reg, [2]string{"hcb-ship-a", "hcb-ship-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 40}
		rc.Finish = func(g *GorgeSide) {
			if !seen && broodshipGravecastReached(g) {
				seen = true
			}
		}
		res := Lockstep(m, seed, k, broodshipGravecastPolicy(m, seed), rc)
		if res.Div != nil {
			d := res.Div
			CloseSide(k)
			t.Fatalf("seed %d: divergence at %d: %s %s %v\ngorge %v\ndirac %v", seed, d.At, d.What, d.Field, d.Diff, d.RefOptions, d.KerOptions)
		}
		if res.Decisions > 100 {
			played++
		}
		CloseSide(k)
	}
	if played < 3 {
		t.Fatalf("only %d seeds played a real game", played)
	}
	if !seen {
		t.Fatal("no seed cast a permanent spell from the graveyard by sacrificing a land, so the lockstep is vacuous")
	}
}

// broodshipGravecastReached: a seat-0 land was sacrificed. In these decks
// only the granted cast's RaiseCost$ Sac<1/Land> sacrifices a land (Bone
// Altar Probe takes an artifact; nothing else sacrifices), so the event is
// the graveyard cast paying its raise.
func broodshipGravecastReached(g *GorgeSide) bool {
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if !events.IsSacrifice(*e) {
			continue
		}
		if o := g.E.G.Obj(e.Obj); o != nil && o.Owner == 0 && o.Face() != nil && o.Face().IsLand() {
			return true
		}
	}
	return false
}

// broodshipGravecastPolicy: seat 0 casts from the graveyard first, stations,
// casts its cards, plays a land, then (with a Broodship at 8+ on the
// battlefield) sacrifices another, un-stationed Broodship to the Altar; a
// sacrifice or tap pick prefers an un-stationed Broodship, then the Titan. Seat 1 never attacks. The tail is the seed's own random stream.
func broodshipGravecastPolicy(m *Matchup, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	// ship reports whether id is a seat-0 Broodship on the battlefield, and
	// whether it holds 8+ CHARGE counters (the grant is live).
	ship := func(p *PickCtx, id state.ObjID) (is, at8 bool) {
		o := p.G.E.G.Obj(id)
		if o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 || o.Face() == nil || o.Face().Name != "Exploration Broodship" {
			return false, false
		}
		return true, o.Counter("CHARGE") >= 8
	}
	// feedable: a Broodship at 8+ and another one below it are both out.
	feedable := func(p *PickCtx) bool {
		live, spare := false, false
		for _, id := range p.G.E.G.Zone(state.ZBattlefield, 0) {
			if is, at8 := ship(p, id); is && at8 {
				live = true
			} else if is {
				spare = true
			}
		}
		return live && spare
	}
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.G == nil {
			return rnd.Pick(p)
		}
		if p.D.Kind == DAttackers && p.D.Player == 1 {
			for _, a := range p.D.Options {
				if a.Kind() != ActAttack {
					return a
				}
			}
		}
		if p.D.Kind == DPriority && p.D.Player == 0 {
			for _, a := range p.D.Options {
				if o, ok := p.G.GorgeOption(a); ok && a.Kind() == ActCast && o.Mode == "mayplay" {
					return a
				}
			}
			for _, a := range p.D.Options {
				if a.Kind() == ActSpecial {
					return a
				}
			}
			for _, a := range p.D.Options {
				if a.Kind() == ActCast {
					return a
				}
			}
			for _, a := range p.D.Options {
				if a.Kind() == ActPlayLand {
					return a
				}
			}
			if feedable(p) {
				for _, a := range p.D.Options {
					if a.Kind() == ActActivate && m.InstanceCard(int(a.Src())) == "Bone Altar Probe" {
						return a
					}
				}
			}
			return rnd.Pick(p)
		}
		if p.D.Kind != DPriority && p.D.Player == 0 {
			for _, a := range p.D.Options {
				if o, ok := p.G.GorgeOption(a); ok && o.Obj != 0 {
					if is, at8 := ship(p, o.Obj); is && !at8 {
						return a
					}
				}
			}
			for _, a := range p.D.Options {
				if o, ok := p.G.GorgeOption(a); ok && o.Obj != 0 {
					if ob := p.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Station Titan Probe" {
						return a
					}
				}
			}
		}
		return rnd.Pick(p)
	})
}
