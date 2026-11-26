package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// choicesHost is a fakeHost with the ask seam: it records every decision
// posed through AskTape and answers with the option at pick (-1 = unserved).
type choicesHost struct {
	*fakeHost
	pick  int
	asked []*decision.Decision
}

func (h *choicesHost) AskCount() uint64 { return uint64(len(h.asked)) }

func (h *choicesHost) TapeAnswer(d *decision.Decision) (decision.Intent, bool) {
	cp := *d
	cp.Options = append([]decision.Option(nil), d.Options...)
	h.asked = append(h.asked, &cp)
	if h.pick < 0 {
		return decision.Intent{}, false
	}
	return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{h.pick}}, true
}

// cometBoard resolves Comet, Stellar Pup's real 4-5 branch (DB$ DealDamage |
// NumDmg$ X | CardChoices$ Creature | PlayerChoices$ Player, a resolution-time
// choice, CR 115.10) with five loyalty counters, one creature per seat. pick
// selects the answered option index (-1 leaves the ask unserved).
func cometBoard(t *testing.T, pick int) (h *choicesHost, comet, mine, theirs state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Comet, Stellar Pup")
	if !ok {
		t.Fatal("corpus has no Comet, Stellar Pup")
	}
	face := card.Faces[0]
	line := face.SVars["Damage"]
	for _, want := range []string{"CardChoices$ Creature", "PlayerChoices$ Player"} {
		if !strings.Contains(line, want) {
			t.Fatalf("corpus moved: Damage no longer carries %q: %q", want, line)
		}
	}
	h = &choicesHost{fakeHost: newHost(t, 2), pick: pick}
	cm := h.g.AddObject(card, 0)
	cm.Zone = state.ZBattlefield
	cm.AddCounter("LOYALTY", 5)
	bear := mkCard(t, "Name:Fixture Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	m := h.g.AddObject(bear, 0)
	m.Zone = state.ZBattlefield
	th := h.g.AddObject(bear, 1)
	th.Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{cm.ID, m.ID})
	h.g.SetZone(state.ZBattlefield, 1, []state.ObjID{th.ID})
	if got := cm.Counter("LOYALTY"); got != 5 {
		t.Fatalf("precondition: Comet has %d loyalty counters, want 5", got)
	}
	Resolve(h, &Ctx{Source: cm.ID, Controller: 0, SVars: face.SVars}, svarSA(t, face, "Damage"))
	return h, cm.ID, m.ID, th.ID
}

func optionFor(d *decision.Decision, p state.PlayerID, obj state.ObjID, player bool) int {
	for i, o := range d.Options {
		if player && o.Kind == "player" && o.Player == p {
			return i
		}
		if !player && o.Kind == "card" && o.Obj == obj {
			return i
		}
	}
	return -1
}

// TestDealDamageCometPosesOneChoiceOverCreaturesAndPlayers: the 4-5 branch asks
// the controller one 1..1 choose over every creature and every player (Comet is
// a planeswalker, so it is not an option) and deals the loyalty count to the
// chosen one.
func TestDealDamageCometPosesOneChoiceOverCreaturesAndPlayers(t *testing.T) {
	for _, tc := range []struct {
		name   string
		player bool
		seat   state.PlayerID
		victim string
	}{{"opponent", true, 1, "player1"}, {"creature", false, 0, "theirs"}, {"self", true, 0, "player0"}} {
		t.Run(tc.name, func(t *testing.T) {
			// Probe first (unserved) to learn the option order, then answer.
			probe, _, _, theirsP := cometBoard(t, -1)
			if len(probe.asked) != 1 {
				t.Fatalf("asked %d times, want exactly 1", len(probe.asked))
			}
			d := probe.asked[0]
			if d.Player != 0 || d.Min != 1 || d.Max != 1 || len(d.Options) != 4 {
				t.Fatalf("decision = %+v, want seat 0's 1..1 pick over 2 creatures + 2 players", d)
			}
			idx := optionFor(d, tc.seat, theirsP, tc.player)
			if idx < 0 {
				t.Fatalf("no option for %s in %+v", tc.victim, d.Options)
			}
			h, comet, mine, theirs := cometBoard(t, idx)
			life0, life1 := h.g.Players[0].Life, h.g.Players[1].Life
			startLife := state.NewGame(names(2)).Players[0].Life
			if life0 != startLife-boolInt(tc.player && tc.seat == 0)*5 || life1 != startLife-boolInt(tc.player && tc.seat == 1)*5 {
				t.Fatalf("life = %d/%d after choosing %s, want 5 damage to that player only", life0, life1, tc.victim)
			}
			if got := h.g.Obj(theirs).Damage; got != 5*int32(boolInt(!tc.player)) {
				t.Fatalf("their creature damage = %d (choice %s)", got, tc.victim)
			}
			if h.g.Obj(mine).Damage != 0 || h.g.Obj(comet).Damage != 0 {
				t.Fatalf("an unchosen object took damage: mine %d comet %d", h.g.Obj(mine).Damage, h.g.Obj(comet).Damage)
			}
			for _, e := range h.log {
				if e.Kind == events.Note && strings.Contains(e.Text, "unread") {
					t.Fatalf("CardChoices$/PlayerChoices$ still Noted as unread: %q", e.Text)
				}
			}
		})
	}
}

func boolInt(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

// TestDealDamageChoicesPoolIgnoresHexproof: the choice is not a target, so a
// hexproof creature stays in the pool (CR 115.10).
func TestDealDamageChoicesPoolIgnoresHexproof(t *testing.T) {
	h := &choicesHost{fakeHost: newHost(t, 2), pick: -1}
	src := h.g.AddObject(mkCard(t, "Name:Src\nTypes:Sorcery\nOracle:x\n"), 0)
	hex := h.g.AddObject(mkCard(t, "Name:Hexy\nTypes:Creature Bear\nPT:2/2\nK:Hexproof\nOracle:x\n"), 1)
	hex.Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 1, []state.ObjID{hex.ID})
	Resolve(h, &Ctx{Source: src.ID, Controller: 0},
		sa(t, "SP$ DealDamage | NumDmg$ 3 | CardChoices$ Creature | PlayerChoices$ Player"))
	if len(h.asked) != 1 {
		t.Fatalf("asked %d times, want 1", len(h.asked))
	}
	if optionFor(h.asked[0], 0, hex.ID, false) < 0 {
		t.Fatalf("hexproof creature missing from %+v", h.asked[0].Options)
	}
	// Unserved: the first option (the creature, cards before players) takes it.
	if hex.Damage != 3 {
		t.Fatalf("unserved stand-in: hexproof creature damage = %d, want 3 (first option)", hex.Damage)
	}
}

// TestDealDamageChoicesRandomDrawsInsteadOfAsking: Random$ True (Whimsy,
// Faerie Dragon) never asks; the pick comes from the engine rng.
func TestDealDamageChoicesRandomDrawsInsteadOfAsking(t *testing.T) {
	h := &choicesHost{fakeHost: newHost(t, 2), pick: 0}
	src := h.g.AddObject(mkCard(t, "Name:Src\nTypes:Sorcery\nOracle:x\n"), 0)
	Resolve(h, &Ctx{Source: src.ID, Controller: 0},
		sa(t, "SP$ DealDamage | NumDmg$ 3 | Random$ True | CardChoices$ Creature | PlayerChoices$ Player"))
	if len(h.asked) != 0 {
		t.Fatalf("a Random$ choice asked %d times", len(h.asked))
	}
	if h.n != 1 {
		t.Fatalf("rng draws = %d, want 1", h.n)
	}
	if h.g.Players[0].Life != 17 {
		t.Fatalf("seat 0 life = %d, want 17 (Rand 0 picks the first seat)", h.g.Players[0].Life)
	}
}
