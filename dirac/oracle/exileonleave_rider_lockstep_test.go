package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The counters-on-exiled-cards lockstep: Dauthi Voidwalker exiles a dying or
// discarded opponent card with a VOID counter, and its activated ChooseCard
// over Exile must find it and let it be played; Draugr Necromancer's ICE
// counter keys the may-play static over exile. The check proves the games
// reach the exiled counter, so a run that never lowers the rider cannot pass
// vacuously.
var exileOnLeaveRiderDecks = [5][2][]cardset.Entry{
	{
		{
			{Name: "Dauthi Voidwalker", Count: 4}, {Name: "Doom Blade", Count: 4},
			{Name: "Grizzly Bears", Count: 10}, {Name: "Swamp", Count: 22},
		},
		{
			{Name: "Grizzly Bears", Count: 12}, {Name: "Doom Blade", Count: 4},
			{Name: "Swamp", Count: 8}, {Name: "Forest", Count: 16},
		},
	},
	{
		{
			{Name: "Draugr Necromancer", Count: 4}, {Name: "Doom Blade", Count: 4},
			{Name: "Grizzly Bears", Count: 10}, {Name: "Swamp", Count: 22},
		},
		{
			{Name: "Grizzly Bears", Count: 12}, {Name: "Doom Blade", Count: 4},
			{Name: "Swamp", Count: 8}, {Name: "Forest", Count: 16},
		},
	},
	// both riders in one deck: with several Draugr in play the exiled card's
	// may-play grants come from each one, and the kernel offers the one cast
	{
		{
			{Name: "Dauthi Voidwalker", Count: 4}, {Name: "Draugr Necromancer", Count: 4},
			{Name: "Doom Blade", Count: 4}, {Name: "Grizzly Bears", Count: 6}, {Name: "Swamp", Count: 22},
		},
		{
			{Name: "Grizzly Bears", Count: 12}, {Name: "Doom Blade", Count: 4},
			{Name: "Swamp", Count: 8}, {Name: "Forest", Count: 16},
		},
	},
	// Kalitas: a dies-to-exile replacement whose rider mints a Zombie token
	// for Kalitas's controller after the exile move
	{
		{
			{Name: "Kalitas, Traitor of Ghet", Count: 4}, {Name: "Doom Blade", Count: 4},
			{Name: "Grizzly Bears", Count: 10}, {Name: "Swamp", Count: 22},
		},
		{
			{Name: "Grizzly Bears", Count: 12}, {Name: "Doom Blade", Count: 4},
			{Name: "Swamp", Count: 8}, {Name: "Forest", Count: 16},
		},
	},
	// Head of the Hunt: the reflexive rider form, where the exile move's
	// ImmediateTrigger puts a Wolf token on the stack after the move
	{
		{
			{Name: "Head of the Hunt", Count: 4}, {Name: "Doom Blade", Count: 4},
			{Name: "Grizzly Bears", Count: 10}, {Name: "Swamp", Count: 22},
		},
		{
			{Name: "Grizzly Bears", Count: 12}, {Name: "Doom Blade", Count: 4},
			{Name: "Swamp", Count: 8}, {Name: "Forest", Count: 16},
		},
	},
}

// riderTokens counts the name/pt/colors tokens player p controls on gorge's
// battlefield.
func riderTokens(g *state.Game, p state.PlayerID, name, pt, colors string) int {
	n := 0
	for _, o := range g.Zone(state.ZBattlefield, p) {
		ob := g.Obj(o)
		if !ob.IsToken || ob.Card == nil || len(ob.Card.Faces) == 0 {
			continue
		}
		if f := ob.Card.Faces[0]; f.Name == name && f.PT == pt && f.Colors == colors {
			n++
		}
	}
	return n
}

// riderToken is the token one death-rider case proves the games mint.
type riderToken struct {
	name, pt, colors string
}

func TestExileOnLeaveRiderLockstep(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		decks [2][]cardset.Entry
		kind  string
		token *riderToken
	}{
		{"dauthi", exileOnLeaveRiderDecks[0], "VOID", nil},
		{"draugr", exileOnLeaveRiderDecks[1], "ICE", nil},
		{"both", exileOnLeaveRiderDecks[2], "VOID", nil},
		{"kalitas", exileOnLeaveRiderDecks[3], "", &riderToken{name: "Zombie Token", pt: "2/2", colors: "black"}},
		{"headofthehunt", exileOnLeaveRiderDecks[4], "", &riderToken{name: "Wolf Token", pt: "2/2", colors: "green"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reg, set, gd := lockstepPool(t, "exileonleavectr", tc.decks)
			tbl, _, _, err := set.Matchup(tc.decks[0], tc.decks[1])
			if err != nil {
				t.Fatal(err)
			}
			seen := 0
			playedSeeds := 0
			for seed := uint64(1); seed <= 60; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"exilectr-a", "exilectr-b"}, gd, 1)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				saw, played := false, false
				tokens := 0
				prev := map[state.ObjID]*cards.Card{}
				rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30}
				rc.Check = func(ref *GorgeSide, _ Side) *Divergence {
					g := ref.E.G
					if tc.token != nil {
						tk := tc.token
						if z := riderTokens(g, 0, tk.name, tk.pt, tk.colors) + riderTokens(g, 1, tk.name, tk.pt, tk.colors); z > tokens {
							tokens, saw = z, true
						}
						return nil
					}
					cur := map[state.ObjID]*cards.Card{}
					for p := state.PlayerID(0); p < 2; p++ {
						for _, o := range g.Zone(state.ZExile, p) {
							if g.Obj(o).Counter(tc.kind) > 0 {
								cur[o], saw = g.Obj(o).Card, true
							}
						}
					}
					// A carrier that left exile and now has the same card on the
					// stack or the battlefield was played by Dauthi's grant: the
					// grant is the only permission to cast an opponent's exiled
					// card, and the cast moves it out of exile (``played'').
					for o, card := range prev {
						if _, still := cur[o]; still {
							continue
						}
						for p := state.PlayerID(0); p < 2; p++ {
							for _, so := range g.Zone(state.ZStack, p) {
								if g.Obj(so).Card == card {
									played = true
								}
							}
							for _, so := range g.Zone(state.ZBattlefield, p) {
								if g.Obj(so).Card == card {
									played = true
								}
							}
						}
					}
					prev = cur
					return nil
				}
				res := Lockstep(m, seed, k, castFirstPolicy(seed), rc)
				CloseSide(k)
				if d := res.Div; d != nil {
					t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
				}
				if saw {
					seen++
				}
				if played {
					playedSeeds++
				}
			}
			if tc.token != nil {
				if seen == 0 {
					t.Fatalf("no seed minted a %s from the replacement", tc.token.name)
				}
				t.Logf("%d of 60 seeds minted a %s token", seen, tc.token.name)
				return
			}
			if seen == 0 {
				t.Fatalf("no seed reached an exiled %s counter", tc.kind)
			}
			if tc.kind == "VOID" && playedSeeds == 0 {
				t.Fatal("no seed activated Dauthi Voidwalker to play the exiled card")
			}
			t.Logf("%d of 60 seeds reached an exiled %s counter, %d played it", seen, tc.kind, playedSeeds)
		})
	}
}
