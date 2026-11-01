package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// redirectWildSlashSteps plays Wild Slash at the bears and Redirect Lightning
// paying {2} at it, leaving the redirect on the stack with both seats to pass.
const redirectWildSlashSteps = `{"op":"pass","seat":0},
	{"op":"cast","seat":1,"card":"p1:Wild Slash","targets":["p0:Grizzly Bears"],"mana":"R"},
	{"op":"pass","seat":1},
	{"op":"cast","seat":0,"card":"p0:Redirect Lightning","targets":["p1:Wild Slash"],"mana":"CCR","answers":[{"kind":"choose","pick":["2"]}]},
	{"op":"pass","seat":0}`

// TestRedirectChainLink: Wild Slash's one target rides a SubAbility$ link, and
// changing it is the whole of what Redirect Lightning does (CR 115.7a).
func TestRedirectChainLink(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct{ name, setup, steps, expect string }{
		{
			name:  "the new target takes the damage",
			setup: `{"p0":{"battlefield":["Grizzly Bears"],"hand":["Redirect Lightning"]},"p1":{"battlefield":["Hill Giant"],"hand":["Wild Slash"]}}`,
			steps: redirectWildSlashSteps + `,{"op":"pass","seat":1,"answers":[{"kind":"choose","pick":["p1:Hill Giant"]}]},{"op":"resolve"}`,
			expect: `[{"card":"p0:Grizzly Bears","zone":"battlefield","damage":0},{"card":"p1:Hill Giant","damage":2},
				{"card":"p1:Wild Slash","zone":"graveyard"}]`,
		},
		{
			// CR 608.2b judges the link's new target at resolution.
			name:  "losing the new target fizzles the spell",
			setup: `{"p0":{"battlefield":["Grizzly Bears"],"hand":["Redirect Lightning","Shock"],"mana":"R"},"p1":{"battlefield":["Elvish Mystic"],"hand":["Wild Slash"]}}`,
			steps: redirectWildSlashSteps + `,{"op":"pass","seat":1,"answers":[{"kind":"choose","pick":["p1:Elvish Mystic"]}]},
				{"op":"cast","seat":0,"card":"p0:Shock","targets":["p1:Elvish Mystic"],"mana":"R"},{"op":"resolve"}`,
			expect: `[{"card":"p0:Grizzly Bears","zone":"battlefield","damage":0},{"card":"p1:Elvish Mystic","zone":"graveyard"},
				{"card":"p1:Wild Slash","zone":"graveyard"}]`,
		},
		{
			// The new target becomes the target of the spell: Wild Slash and
			// the ward trigger are on the stack.
			name:   "a warded new target triggers ward",
			setup:  `{"p0":{"battlefield":["Grizzly Bears","Tolarian Terror"],"hand":["Redirect Lightning"]},"p1":{"hand":["Wild Slash"]}}`,
			steps:  redirectWildSlashSteps + `,{"op":"pass","seat":1,"answers":[{"kind":"choose","pick":["p0:Tolarian Terror"]}]}`,
			expect: `[{"stack_size":2}]`,
		},
		{
			// CR 115.7d: the root's ask comes first, then the link's.
			name:  "new targets for the root and the link",
			setup: `{"p0":{"battlefield":["Grizzly Bears","Hill Giant"],"hand":["Bite Down"]},"p1":{"battlefield":["Craw Wurm","Elvish Mystic"],"hand":["Deflecting Swat"]}}`,
			steps: `{"op":"cast","seat":0,"card":"p0:Bite Down","targets":["p0:Grizzly Bears"],"mana":"CG","answers":[{"kind":"target","pick":["p1:Craw Wurm"]}]},
				{"op":"pass","seat":0},
				{"op":"cast","seat":1,"card":"p1:Deflecting Swat","targets":["p0:Bite Down"],"mana":"CCR"},
				{"op":"resolve","answers":[{"kind":"choose","pick":["p0:Hill Giant"]},{"kind":"choose","pick":["p1:Elvish Mystic"]}]}`,
			expect: `[{"card":"p1:Elvish Mystic","zone":"graveyard"},{"card":"p1:Craw Wurm","damage":0}]`,
		},
		{
			// CR 115.7a: the root could move, the link could not, so neither does.
			name:  "changing the targets is all or none",
			setup: `{"p0":{"battlefield":["Grizzly Bears","Hill Giant"],"hand":["Bite Down"]},"p1":{"battlefield":["Goblin Flectomancer","Craw Wurm"]}}`,
			steps: `{"op":"cast","seat":0,"card":"p0:Bite Down","targets":["p0:Hill Giant"],"mana":"CG","answers":[{"kind":"target","pick":["p1:Craw Wurm"]}]},
				{"op":"pass","seat":0},
				{"op":"activate","seat":1,"card":"p1:Goblin Flectomancer","targets":["p0:Bite Down"],"ability_index":0},
				{"op":"pass","seat":1},{"op":"resolve"}`,
			expect: `[{"card":"p1:Craw Wurm","damage":3},{"card":"p0:Bite Down","zone":"graveyard"}]`,
		},
		{
			// CR 115.7d leaves any number in place, and the two asks answer separately.
			name:  "a new root target keeps the link",
			setup: `{"p0":{"battlefield":["Grizzly Bears","Hill Giant"],"hand":["Bite Down"]},"p1":{"battlefield":["Craw Wurm"],"hand":["Deflecting Swat"]}}`,
			steps: `{"op":"cast","seat":0,"card":"p0:Bite Down","targets":["p0:Grizzly Bears"],"mana":"CG","answers":[{"kind":"target","pick":["p1:Craw Wurm"]}]},
				{"op":"pass","seat":0},
				{"op":"cast","seat":1,"card":"p1:Deflecting Swat","targets":["p0:Bite Down"],"mana":"CCR"},
				{"op":"resolve","answers":[{"kind":"choose","pick":["p0:Hill Giant"]},{"kind":"choose","pick":[]}]}`,
			expect: `[{"card":"p1:Craw Wurm","damage":3},{"card":"p0:Bite Down","zone":"graveyard"}]`,
		},
		{
			// Legality is the blue spell's, whatever colour the redirect is.
			name:  "a red redirect can move a blue spell onto protection from red",
			setup: `{"p0":{"battlefield":["Grizzly Bears","Auriok Champion"],"hand":["Redirect Lightning"]},"p1":{"hand":["Unsummon"]}}`,
			steps: `{"op":"pass","seat":0},{"op":"cast","seat":1,"card":"p1:Unsummon","targets":["p0:Grizzly Bears"],"mana":"U"},{"op":"pass","seat":1},
				{"op":"cast","seat":0,"card":"p0:Redirect Lightning","targets":["p1:Unsummon"],"mana":"CCR","answers":[{"kind":"choose","pick":["2"]}]},
				{"op":"resolve","answers":[{"kind":"choose","pick":["p0:Auriok Champion"]}]}`,
			expect: `[{"card":"p0:Auriok Champion","zone":"hand"},{"card":"p0:Grizzly Bears","zone":"battlefield"}]`,
		},
		{
			// CR 115.9: "a spell that targets" reads the chain link's target.
			name:  "a chain link's target counts for a spell that targets a permanent you control",
			setup: `{"p0":{"battlefield":["Grizzly Bears"],"hand":["Not of This World"]},"p1":{"hand":["Wild Slash"]}}`,
			steps: `{"op":"pass","seat":0},{"op":"cast","seat":1,"card":"p1:Wild Slash","targets":["p0:Grizzly Bears"],"mana":"R"},{"op":"pass","seat":1},
				{"op":"cast","seat":0,"card":"p0:Not of This World","targets":["p1:Wild Slash"],"mana":"CCCCCCC"},{"op":"resolve"}`,
			expect: `[{"card":"p0:Grizzly Bears","zone":"battlefield","damage":0},{"card":"p1:Wild Slash","zone":"graveyard"}]`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc := `{"name":"redirect","cr":["115.7"],"why":"redirect","setup":` + tc.setup + `,"steps":[` + tc.steps + `],"expect":` + tc.expect + `}`
			res, err := RunOracleAuditScenarioJSON(reg, []byte(sc), true)
			if err != nil || len(res.Fails) != 0 || res.ReplayDiff != "" {
				t.Fatalf("err=%v fails=%v replay=%q\n%s", err, res.Fails, res.ReplayDiff, res.Transcript)
			}
		})
	}
}

// TestRetargetLinkKeepsAKeptTarget: a link's kept target is not targeted again;
// a changed one leaves the object's flat target list and the new one is targeted.
func TestRetargetLinkKeepsAKeptTarget(t *testing.T) {
	t.Parallel()
	e, _, mine, theirs := cr601Board(t, 60102,
		map[string]state.Zone{"Bite Down": state.ZHand, "Centaur Courser": state.ZBattlefield},
		map[string]state.Zone{"Grizzly Bears": state.ZBattlefield, "Craw Wurm": state.ZBattlefield})
	spell := mine["Bite Down"]
	bears, wurm := theirs["Grizzly Bears"], theirs["Craw Wurm"]
	addMana(t, e, 0, "G1")
	edrSeatZeroPriority(t, e)
	cr601Cast(t, e, spell, "")
	answerTargetAsk(t, e, []state.ObjID{mine["Centaur Courser"]})
	answerCastSubObj(t, e, wurm)
	link := e.G.Obj(spell).Face().SpellAbility().Sub

	targeted := len(subTargetEvents(e, spell))
	e.RetargetLink(spell, link, []state.Target{{Obj: wurm}})
	if got := len(subTargetEvents(e, spell)); got != targeted {
		t.Fatalf("keeping the target emitted %d targeting event(s)", got-targeted)
	}

	e.RetargetLink(spell, link, []state.Target{{Obj: bears}})
	if got := len(subTargetEvents(e, spell)); got != targeted+1 {
		t.Fatalf("changing the target emitted %d targeting event(s), want 1", got-targeted)
	}
	if got := e.G.Obj(spell).SubTargets; len(got) != 1 || got[0].Obj != bears {
		t.Fatalf("SubTargets = %+v, want only the bears", got)
	}
	if got := e.AnnouncedLinkTargets(spell, link); len(got) != 1 || got[0].Obj != bears {
		t.Fatalf("the link's record = %+v, want only the bears", got)
	}
}
