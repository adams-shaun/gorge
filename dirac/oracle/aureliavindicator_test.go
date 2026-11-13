package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Aurelia's Vindicator lockstep: Disguise {X}{3}{W} turned up at X = 2 and
// its turn-face-up trigger's one target slot across the battlefield and the
// graveyards ("up to X other target creatures from the battlefield and/or
// creature cards from graveyards"). The policy puts a creature card in a
// graveyard with Shock, casts Aurelia face down from an Iridescent Tiger's
// floating mana, floats a second Tiger's {W}{U}{B}{R}{G} and a Crumbling
// Vestige's white (mana abilities are never actions, so only enters triggers
// float mana), turns up choosing X = 2 and picks one battlefield creature and
// one graveyard card. The facedown pool carries Tiger and Vestige for this.

var kaurDeck = []cardset.Entry{
	{Name: "Aurelia's Vindicator", Count: 5},
	{Name: "Mox Pearl", Count: 9},
	{Name: "Mox Jet", Count: 3},
	{Name: "Mox Ruby", Count: 4},
	{Name: "Iridescent Tiger", Count: 6},
	{Name: "Crumbling Vestige", Count: 4},
	{Name: "Shock", Count: 5},
	{Name: "Aven Farseer", Count: 4},
}

type kaurState struct {
	turnUps                  int
	tgtPhase                 bool
	pickedGrave, pickedField bool
}

// kaurCreatureInGrave reports a creature card in either graveyard.
func kaurCreatureInGrave(c *PickCtx) bool {
	g := c.G.E.G
	for p := state.PlayerID(0); p < 2; p++ {
		for _, id := range g.Zone(state.ZGraveyard, p) {
			if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().IsCreature() {
				return true
			}
		}
	}
	return false
}

func kaurPolicy(seed uint64, st *kaurState) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		return kaurPick(c, st, rnd)
	})
}

func kaurPick(c *PickCtx, st *kaurState, rnd Policy) Canon {
	xCode, _ := chooseCode("x")
	colorCode, _ := chooseCode("color")
	manaCode, _ := chooseCode("mana")
	if c.G == nil {
		return rnd.Pick(c)
	}
	g := c.G.E.G
	me := state.PlayerID(c.D.Player)
	pool := 0
	if int(me) < len(g.Players) {
		pool = int(g.Players[me].Pool.Total())
	}
	faceDown, untappedMox, avenOnBF := false, 0, false
	for _, id := range g.Zone(state.ZBattlefield, me) {
		o := g.Obj(id)
		switch {
		case o == nil:
		case o.FaceDownHidden():
			faceDown = true
		case o.Face() != nil && isMoxName(o.Face().Name) && !o.Tapped:
			untappedMox++
		}
	}
	for p := state.PlayerID(0); p < 2; p++ {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Aven Farseer" {
				avenOnBF = true
			}
		}
	}
	grave := kaurCreatureInGrave(c)
	aurInHand := false
	for _, id := range g.Zone(state.ZHand, me) {
		if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Aurelia's Vindicator" {
			aurInHand = true
		}
	}

	var done, x2, xLast Canon
	var haveDone, haveX2, haveX bool
	var tgtGrave, tgtField, tgtAven, tgtAny Canon
	var haveGrave, haveField, haveAven, haveAny bool
	var mox, aurDown, aven, shock, priest, turnUp Canon
	var land, white Canon
	var haveLand, haveWhite bool
	var haveMox, haveAur, haveAven2, haveShock, havePriest, haveTurnUp bool
	for _, a := range c.D.Options {
		n := optionFaceName(c, a)
		switch {
		case a.Kind() == ActPlayLand && n == "Crumbling Vestige":
			land, haveLand = a, true
		case (a.Kind() == ActChoose+CanonKind(colorCode) || a.Kind() == ActChoose+CanonKind(manaCode)) && int(a.Mode()) == strings.Index(manaSyms, "W"):
			white, haveWhite = a, true
		case a.Kind() == ActDone:
			done, haveDone = a, true
		case a.Kind() == ActChoose+CanonKind(xCode):
			xLast, haveX = a, true
			if a.Mode() == 2 {
				x2, haveX2 = a, true
			}
		case a.Kind() == ActTarget && int(a.Tgt()) < int(RefPlayer):
			o := g.Obj(c.G.id.obj[a.Tgt()])
			if o == nil {
				continue
			}
			isCreature := o.Face() != nil && o.Face().IsCreature()
			switch {
			case o.Zone == state.ZGraveyard && isCreature:
				tgtGrave, haveGrave = a, true
			case o.Zone == state.ZBattlefield && o.Face() != nil && o.Face().Name == "Aven Farseer":
				tgtAven, haveAven = a, true
				fallthrough
			case o.Zone == state.ZBattlefield && isCreature:
				tgtField, haveField = a, true
			}
			tgtAny, haveAny = a, true
		case a.Kind() == ActCast && isMoxName(n):
			mox, haveMox = a, true
		case a.Kind() == ActCast && n == "Aven Farseer":
			aven, haveAven2 = a, true
		case a.Kind() == ActCast && n == "Shock":
			shock, haveShock = a, true
		case a.Kind() == ActCast && n == "Iridescent Tiger":
			priest, havePriest = a, true
		case a.Kind() == ActCastFace && n == "Aurelia's Vindicator":
			aurDown, haveAur = a, true
		case a.Kind() == ActSpecial && a.Mode() == kfacedownTurnUp && n == "Aurelia's Vindicator":
			turnUp, haveTurnUp = a, true
		}
	}
	switch {
	case haveWhite:
		return white
	case haveX:
		if haveX2 {
			return x2
		}
		return xLast
	case st.tgtPhase && (haveGrave || haveField || haveDone):
		switch {
		case haveGrave && !st.pickedGrave:
			st.pickedGrave = true
			return tgtGrave
		case haveField && !st.pickedField:
			st.pickedField = true
			return tgtField
		case haveDone:
			st.tgtPhase = false
			return done
		}
	case haveAny && !st.tgtPhase:
		if haveAven {
			return tgtAven
		}
		return tgtAny
	case haveMox:
		return mox
	case haveTurnUp && pool >= 6:
		st.turnUps++
		st.tgtPhase, st.pickedGrave, st.pickedField = true, false, false
		return turnUp
	case haveLand && faceDown && pool >= 5:
		return land // its enters trigger floats the white
	case haveAur && !faceDown:
		return aurDown
	case haveAven2 && !grave && !avenOnBF:
		return aven
	case haveShock && !grave && avenOnBF:
		return shock
	case havePriest && !faceDown && aurInHand && pool < 3:
		return priest // the face-down cast is priced from the floating pool
	case havePriest && faceDown && grave && pool < 5 && untappedMox >= 5:
		return priest
	}
	var rest []Canon
	for _, a := range c.D.Options {
		n := optionFaceName(c, a)
		switch {
		case a.Kind() == ActSpecial && a.Mode() == kfacedownTurnUp:
		case a.Kind() == ActCast && (n == "Iridescent Tiger" || n == "Shock" || n == "Aven Farseer" || n == "Aurelia's Vindicator"):
		case a.Kind() == ActActivate && n == "Mox Pearl":
		case a.Kind() == ActCastFace, a.Kind() == ActAttack:
		default:
			rest = append(rest, a)
		}
	}
	if len(rest) == 0 {
		return rnd.Pick(c)
	}
	d := *c.D
	d.Options = rest
	return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
}

func TestAureliaVindicatorLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, set, decks, tbl := kfacedownDeckEnv(t, kaurDeck)
	seeds := uint64(16)
	if testing.Short() {
		seeds = 4
	}
	var turnUps, fromField, fromGrave, xTwo int
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"aur-a", "aur-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var st kaurState
		finish := func(g *GorgeSide) {
			for _, ev := range g.E.L.Events {
				switch {
				case ev.Kind == events.TurnFaceUp && ev.Amount >= 2:
					xTwo++
				case ev.Kind == events.MoveZone && ev.To == state.ZExile && ev.From == state.ZBattlefield:
					fromField++
				case ev.Kind == events.MoveZone && ev.To == state.ZExile && ev.From == state.ZGraveyard:
					fromGrave++
				}
			}
		}
		res := Lockstep(m, seed, k, kaurPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30, Finish: finish})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("seed %d: stall %q", seed, res.Stall)
		}
		turnUps += st.turnUps
	}
	t.Logf("turn-ups=%d X>=2 events=%d exiled from battlefield=%d from graveyard=%d", turnUps, xTwo, fromField, fromGrave)
	if turnUps == 0 || xTwo == 0 {
		t.Errorf("no X >= 2 turn-up (turnUps=%d, TurnFaceUp events with Amount >= 2: %d)", turnUps, xTwo)
	}
	if fromField == 0 || fromGrave == 0 {
		t.Errorf("the trigger exiled %d from the battlefield and %d from a graveyard, want both", fromField, fromGrave)
	}
}
