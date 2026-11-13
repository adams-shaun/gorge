package rules

// Dig's budget, NoLooking$, optional prompt and parameters: each drives a real engine through the mid-resolution
// ask and asserts what the offer is and what the answer does.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// artifactFixtureSrc is a nonland artifact fixture named name with mana value mv.
func artifactFixtureSrc(name string, mv int) string {
	pips := make([]string, mv)
	for i := range pips {
		pips[i] = "W"
	}
	return "Name:" + name + "\nManaCost:" + strings.Join(pips, " ") + "\nTypes:Artifact\nOracle:x\n"
}

func sorceryFixtureSrc(name, body string) string {
	return "Name:" + name + "\nManaCost:B\nTypes:Sorcery\n" + body + "\nOracle:x\n"
}

const budgetDigBody = "A:SP$ Dig | Defined$ You | DigNum$ 4 | ChangeNum$ Any | ChangeValid$ Artifact | WithTotalCMC$ 4 | DestinationZone$ Hand"

// TestKernelDigWithTotalCMCNarrowsEligible:
// a card whose own mana value exceeds the WithTotalCMC$ budget is never
// offered; window [5,2,2,2] budget 4 offers exactly the three 2-MV cards,
// each carrying its mana value, under a prompt naming the budget.
func TestKernelDigWithTotalCMCNarrowsEligible(t *testing.T) {
	t.Parallel()
	e, cfg, id := newFixtureGame(t, 101, sorceryFixtureSrc("BudgetDig", budgetDigBody),
		[]string{artifactFixtureSrc("ArtA", 5), artifactFixtureSrc("ArtB", 2), artifactFixtureSrc("ArtC", 2), artifactFixtureSrc("ArtD", 2)}, nil)
	addMana(t, e, 0, "B")
	ids := stackLibraryTop(t, e, 0, "ArtA", "ArtB", "ArtC", "ArtD")
	d := castSpellObj(t, e, id)
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("no dig ask: the budget forces a choice the forced take cannot make: %+v", d)
	}
	if d.MaxSum != 4 {
		t.Fatalf("MaxSum = %d, want 4", d.MaxSum)
	}
	if len(d.Options) != 3 {
		t.Fatalf("options = %+v, want exactly the three 2-MV cards", d.Options)
	}
	for _, o := range d.Options {
		if o.Obj == ids[0] {
			t.Fatalf("the 5-MV card was offered under a budget of 4")
		}
		if o.Value != 2 {
			t.Fatalf("option %+v Value = %d, want 2", o, o.Value)
		}
	}
	if !strings.Contains(d.Prompt, "total mana value 4 or less") {
		t.Fatalf("prompt = %q, want the budget named", d.Prompt)
	}
	// Taking two 2-MV cards (sum 4) moves exactly those.
	if p := answerKindFor(t, e, decision.KChoose, 0, 0, 2); p != nil {
		t.Fatalf("unexpected follow-up ask %+v", p)
	}
	if zoneOfID(e, d.Options[0].Obj) != state.ZHand || zoneOfID(e, d.Options[2].Obj) != state.ZHand {
		t.Fatal("the two picked artifacts are not in hand")
	}
	if zoneOfID(e, ids[0]) != state.ZLibrary || zoneOfID(e, d.Options[1].Obj) != state.ZLibrary {
		t.Fatal("an untaken card left the library")
	}
	replayCheck(t, e, cfg)
}

// TestKernelDigWithTotalCMCCapsCumulative:
// the budget is cumulative: window [2,2,2,2] budget 4 rejects a four-pick
// and accepts a two-pick, which moves exactly those two to hand.
func TestKernelDigWithTotalCMCCapsCumulative(t *testing.T) {
	t.Parallel()
	e, cfg, id := newFixtureGame(t, 102, sorceryFixtureSrc("BudgetDig", budgetDigBody),
		[]string{artifactFixtureSrc("ArtA", 2), artifactFixtureSrc("ArtB", 2), artifactFixtureSrc("ArtC", 2), artifactFixtureSrc("ArtD", 2)}, nil)
	addMana(t, e, 0, "B")
	ids := stackLibraryTop(t, e, 0, "ArtA", "ArtB", "ArtC", "ArtD")
	d := castSpellObj(t, e, id)
	if d == nil || d.Kind != decision.KChoose || len(d.Options) != 4 {
		t.Fatalf("dig ask = %+v, want all four affordable cards offered", d)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{0, 1, 2, 3}}); err == nil {
		t.Fatal("an over-budget four-pick was accepted")
	}
	if p := answerKindFor(t, e, decision.KChoose, 0, optionIdxOfObj(t, d, ids[0]), optionIdxOfObj(t, d, ids[1])); p != nil {
		t.Fatalf("unexpected follow-up ask %+v", p)
	}
	for i, want := range []state.Zone{state.ZHand, state.ZHand, state.ZLibrary, state.ZLibrary} {
		if z := zoneOfID(e, ids[i]); z != want {
			t.Fatalf("card %d zone = %s, want %s", i, z, want)
		}
	}
	replayCheck(t, e, cfg)
}

// TestKernelDigNoLooking: a bare NoLooking$ True is Forge's presentation
// rider, not a blind dig -- it still names the cards under a secret look. A
// blind-shaped dig (NoLooking$ with ExileFaceDown$ or RandomChange$) hides the
// window (no secret look, no names in prompt or options) while an ordinary
// Dig names the cards under a secret look and Reveal$ wins over NoLooking$
// with a public reveal of the whole window. In every case the answer moves
// the chosen object to the dig's destination.
func TestKernelDigNoLooking(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                              string
		params                            string
		dest                              state.Zone
		wantSecretLook, wantPublic, named bool
	}{
		{name: "names the cards under a secret look", params: " | NoLooking$ True", dest: state.ZHand,
			wantSecretLook: true, named: true},
		{name: "blind random change", params: " | NoLooking$ True | RandomChange$ True", dest: state.ZHand},
		{name: "blind face-down exile", params: " | NoLooking$ True | ExileFaceDown$ True | DestinationZone$ Exile", dest: state.ZExile},
		{name: "ordinary", dest: state.ZHand, wantSecretLook: true, named: true},
		{name: "reveal wins", params: " | NoLooking$ True | Reveal$ True", dest: state.ZHand, wantPublic: true, named: true},
	}
	for i, tc := range cases {
		tc := tc
		seed := uint64(110 + i)
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			src := sorceryFixtureSrc("BlindDig", "A:SP$ Dig | Defined$ You | DigNum$ 2 | ChangeNum$ 1 | ChangeValid$ Card | DestinationZone$ Hand"+tc.params)
			e, cfg, id := newFixtureGame(t, seed, src, []string{
				"Name:Alpha Identity\nManaCost:G\nTypes:Creature\nPT:2/2\nOracle:x\n",
				"Name:Beta Identity\nManaCost:G\nTypes:Creature\nPT:3/3\nOracle:x\n"}, nil)
			addMana(t, e, 0, "B")
			ids := stackLibraryTop(t, e, 0, "Alpha Identity", "Beta Identity")
			mark := len(e.L.Events)
			d := castSpellObj(t, e, id)
			if d == nil || d.Kind != decision.KChoose || len(d.Options) != 2 {
				t.Fatalf("decision = %+v, want a choice over both window cards", d)
			}
			optionIdxOfObj(t, d, ids[0])
			optionIdxOfObj(t, d, ids[1])
			if tc.named {
				if !strings.Contains(d.Prompt, "Look at") || !strings.Contains(d.Options[0].Label, "Identity") || !strings.Contains(d.Options[1].Label, "Identity") {
					t.Fatalf("ordinary/public decision should name the cards: %+v", d)
				}
			} else {
				if strings.Contains(d.Prompt, "Look at") || strings.Contains(d.Prompt, "Identity") {
					t.Fatalf("blind prompt identifies the window: %q", d.Prompt)
				}
				for _, o := range d.Options {
					if strings.Contains(o.Label, "Identity") {
						t.Fatalf("blind option identifies a card: %+v", o)
					}
				}
			}
			var secret, public []state.ObjID
			for _, ev := range e.L.Events[mark:] {
				if ev.Kind != events.Note {
					continue
				}
				if ev.Secret {
					secret = append(secret, ev.IDs...)
				} else {
					public = append(public, ev.IDs...)
				}
			}
			if (len(secret) != 0) != tc.wantSecretLook {
				t.Fatalf("private look IDs = %v, want private disclosure %t", secret, tc.wantSecretLook)
			}
			if tc.wantPublic {
				if len(public) != 2 || public[0] != ids[0] || public[1] != ids[1] {
					t.Fatalf("public reveal IDs = %v, want the window %v", public, ids)
				}
			} else if len(public) != 0 {
				t.Fatalf("unexpected public disclosure %v", public)
			}
			chosen := d.Options[1].Obj
			answerKindFor(t, e, decision.KChoose, 0, d.Options[1].Index)
			if zoneOfID(e, chosen) != tc.dest {
				t.Fatalf("answer did not move the chosen object %d to %s", chosen, tc.dest)
			}
			replayCheck(t, e, cfg)
		})
	}
}

// TestKernelDigOptionalPromptParametersAskForOptionalTake: both optional-prompt
// spellings make the take a 0..ChangeNum ask, and declining moves nothing.
func TestKernelDigOptionalPromptParametersAskForOptionalTake(t *testing.T) {
	t.Parallel()
	for i, param := range []string{"PromptToSkipOptionalAbility$ True", "OptionalAbilityPrompt$ Would you like to put the land onto the battlefield tapped?"} {
		param := param
		seed := uint64(120 + i)
		t.Run(strings.SplitN(param, "$", 2)[0], func(t *testing.T) {
			t.Parallel()
			src := sorceryFixtureSrc("OptDig", "A:SP$ Dig | Defined$ You | DigNum$ 3 | ChangeNum$ 2 | "+param+" | ChangeValid$ Land | DestinationZone$ Hand | DestinationZone2$ Library | LibraryPosition2$ 0")
			e, cfg, id := newFixtureGame(t, seed, src, []string{"Name:Isle\nTypes:Basic Land Island\nOracle:x\n"}, nil)
			addMana(t, e, 0, "B")
			stackLibraryTop(t, e, 0, "Isle")
			hand := len(e.G.Zone(state.ZHand, 0))
			d := castSpellObj(t, e, id)
			if d == nil || d.Kind != decision.KChoose || d.Min != 0 || d.Max != 2 {
				t.Fatalf("decision = %+v, want an optional 0..2 take ask", d)
			}
			lib := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, 0)...)
			if p := answerKindFor(t, e, decision.KChoose, 0); p != nil {
				t.Fatalf("decline produced a follow-up ask: %+v", p)
			}
			if got := len(e.G.Zone(state.ZHand, 0)); got != hand-1 {
				t.Fatalf("hand = %d, want %d (only the cast spell left it)", got, hand-1)
			}
			if got := e.G.Zone(state.ZLibrary, 0); !sameObjIDs(got, lib) {
				t.Fatalf("library after decline = %v, want unchanged %v", got, lib)
			}
			replayCheck(t, e, cfg)
		})
	}
}

// TestKernelDigNumXUsesTheTriggerSvar (Keldon Flamesage's shape): a triggered Dig has no paid X, so DigNum$ X with
// SVar:X:Count$CardPower sizes the window by the source's power (2) — the
// instant at position 2 is offered, which a zero or one-card window misses.
func TestKernelDigNumXUsesTheTriggerSvar(t *testing.T) {
	t.Parallel()
	src := "Name:Flamer\nManaCost:R\nTypes:Creature Human Shaman\nPT:2/3\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigDig | TriggerDescription$ x\n" +
		"SVar:TrigDig:DB$ Dig | DigNum$ X | ChangeNum$ 1 | Optional$ True | ChangeValid$ Instant.cmcLEX,Sorcery.cmcLEX | DestinationZone$ Exile | DestinationZone2$ Library | LibraryPosition$ -1\n" +
		"SVar:X:Count$CardPower\nOracle:x\n"
	e, _, id := newFixtureGame(t, 130, src, []string{"Name:Zap\nManaCost:R\nTypes:Instant\nOracle:x\n"}, nil)
	addMana(t, e, 0, "R")
	ids := stackLibraryTop(t, e, 0, "Mountain", "Zap")
	d := castSpellObj(t, e, id)
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("decision = %+v, want a Dig ask over the power-sized window", d)
	}
	if len(d.Options) != 1 || d.Options[0].Obj != ids[1] {
		t.Fatalf("options = %+v, want exactly the instant at window position 2", d.Options)
	}
	answerKindFor(t, e, decision.KChoose, 0, 0)
	if zoneOfID(e, ids[1]) != state.ZExile {
		t.Fatalf("taken instant zone = %s, want exile", zoneOfID(e, ids[1]))
	}
}
