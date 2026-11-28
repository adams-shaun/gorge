// cr_fourseat_commander_test.go — CR 903 in a four-player Commander pod.
// The duel suites (oracle_setup_commander_test.go, commander_*_test.go) cannot
// tell "per player" from "the active player's" or "per commander" from "per
// game": with two seats the owner, the caster and the defender coincide too
// often. Every scenario here seats four (p0..p3, p0 active on turn 1), and
// every seat not described commands Isamaru from the command zone at 40 life.
package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const fsIsamaru = "Isamaru, Hound of Konda"

// fsCmd is a seat that commands Isamaru from the command zone.
const fsCmd = `{"command":["` + fsIsamaru + `"]}`

// fsHead opens a four-seat Commander scenario; the caller closes setup.
func fsHead(name, cr string) string {
	return `{"name":"` + name + `","format":"commander","players":4,"cr":["` + cr + `"],"why":"four-seat commander pod",`
}

// fsRun plays raw and fails the test on any scenario failure.
func fsRun(t *testing.T, raw string) *oracleRun {
	t.Helper()
	run, fails := runOracleCommander(t, raw)
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", fails, strings.Join(run.log, "\n"))
	}
	return run
}

// fsRefused plays raw and fails the test unless the scenario fails with a
// message containing want.
func fsRefused(t *testing.T, raw, want string) {
	t.Helper()
	run, fails := runOracleCommander(t, raw)
	if len(fails) == 0 {
		t.Fatalf("scenario passed, want a failure naming %q:\n%s", want, strings.Join(run.log, "\n"))
	}
	if got := strings.Join(fails, "\n"); !strings.Contains(got, want) {
		t.Fatalf("fails = %q, want one naming %q", got, want)
	}
}

func fsLast(run *oracleRun) OracleSnapshot { return run.snaps[len(run.snaps)-1] }

// fsPodStarts: all four seats command Isamaru, nothing happens.
const fsPodStarts = `{"name":"fs-pod-starts","format":"commander","players":4,"cr":["903.6","903.7"],"why":"four-seat commander pod",
"setup":{"p0":` + fsCmd + `,"p1":` + fsCmd + `,"p2":` + fsCmd + `,"p3":` + fsCmd + `},"steps":[]}`

// TestCR903FourSeatPodStarts: CR 903.7 every seat starts at 40; CR 903.6 each
// seat's commander starts in that seat's own command zone; CR 103.8a in a
// game of more than two the starting player draws, so p0 alone holds a card.
func TestCR903FourSeatPodStarts(t *testing.T) {
	t.Parallel()
	run := fsRun(t, fsPodStarts)
	s := run.snaps[0]
	if len(s.Players) != 4 {
		t.Fatalf("players = %d, want 4", len(s.Players))
	}
	for i, p := range s.Players {
		if p.Life != 40 {
			t.Errorf("p%d life = %d, want 40 (CR 903.7)", i, p.Life)
		}
		if len(p.Command) != 1 || p.Command[0] != fsIsamaru {
			t.Errorf("p%d command zone = %v, want [%s] (CR 903.6)", i, p.Command, fsIsamaru)
		}
		ids := run.e.G.Players[i].Commanders
		if len(ids) != 1 {
			t.Fatalf("p%d Commanders = %v, want one", i, ids)
		}
		if o := run.e.G.Obj(ids[0]); o == nil || o.Owner != state.PlayerID(i) || o.Zone != state.ZCommand {
			t.Errorf("p%d's commander object = %+v, want owned by p%d in the command zone", i, o, i)
		}
		want := 0
		if i == 0 {
			want = 1
		}
		if len(p.Hand) != want {
			t.Errorf("p%d hand = %v, want %d card(s) (CR 103.8a)", i, p.Hand, want)
		}
	}
}

// fsTaxSetup: p0 has cast Isamaru twice, p1 once, p2 never; p3 commands it too.
const fsTaxSetup = `"setup":{"p0":{"command":["` + fsIsamaru + `"],"commander_casts":{"` + fsIsamaru + `":2}},` +
	`"p1":{"command":["` + fsIsamaru + `"],"commander_casts":{"` + fsIsamaru + `":1}},` +
	`"p2":` + fsCmd + `,"p3":` + fsCmd + `}`

func fsTaxCast(turn string, seat, mana string) string {
	return `{"name":"fs-tax","format":"commander","players":4,"turn":` + turn + `,"cr":["903.8"],"why":"four-seat commander pod",` +
		fsTaxSetup + `,"steps":[{"op":"cast","seat":` + seat + `,"card":"p` + seat + `:` + fsIsamaru + `","mana":"` + mana + `"}]}`
}

var (
	fsTaxP0Short = fsTaxCast("1", "0", "WCC")
	fsTaxP0Paid  = fsTaxCast("1", "0", "WCCCC")
	fsTaxP1Short = fsTaxCast("2", "1", "W")
	fsTaxP1Paid  = fsTaxCast("2", "1", "WCC")
	fsTaxP2Paid  = fsTaxCast("3", "2", "W")
)

// TestCR903FourSeatTaxIsPerPlayer: CR 903.8 the tax counts the casts of THAT
// commander from the command zone, so each seat pays {2} per its own earlier
// cast and nothing for another seat's. p0 (two casts) pays {4}, p1 (one) {2},
// p2 (none) nothing, each on its own turn.
func TestCR903FourSeatTaxIsPerPlayer(t *testing.T) {
	t.Parallel()
	fsRefused(t, fsTaxP0Short, "Isamaru")
	fsRefused(t, fsTaxP1Short, "Isamaru")
	for _, c := range []struct {
		raw  string
		seat int
	}{{fsTaxP0Paid, 0}, {fsTaxP1Paid, 1}, {fsTaxP2Paid, 2}} {
		run := fsRun(t, c.raw)
		last := fsLast(run)
		if last.Active != c.seat {
			t.Fatalf("precondition: active p%d, want p%d", last.Active, c.seat)
		}
		ref := "p" + string(rune('0'+c.seat)) + ":" + fsIsamaru
		if len(last.Stack) != 1 || last.Stack[0].Source != ref {
			t.Fatalf("p%d cast: stack = %+v, want %s on the stack", c.seat, last.Stack, ref)
		}
		if last.Players[c.seat].Pool != "" {
			t.Errorf("p%d cast left %q in the pool: the tax charged was not its own", c.seat, last.Players[c.seat].Pool)
		}
	}
}

// fsDamagePerCommander: p0's and p2's Isamaru are on the battlefield as
// commanders; p3 has taken 20 from p2's.
const fsDamagePerCommanderSetup = `"setup":{` +
	`"p0":{"battlefield":["` + fsIsamaru + `"],"commanders":["` + fsIsamaru + `"]},"p1":` + fsCmd + `,` +
	`"p2":{"battlefield":["` + fsIsamaru + `"],"commanders":["` + fsIsamaru + `"]},` +
	`"p3":{"command":["` + fsIsamaru + `"],"commander_damage":{"p2:` + fsIsamaru + `":20}}}`

const fsSwingToMain2 = `{"op":"pass_to","step":"main2","active":"p0"}`

var fsDamagePerCommander = fsHead("fs-damage-per-commander", "903.10a") + fsDamagePerCommanderSetup +
	`,"steps":[{"op":"attack","seat":0,"attackers":["p0:` + fsIsamaru + `"],"defender":"p3"},` + fsSwingToMain2 + `]}`

// TestCR903FourSeatCommanderDamageIsPerCommander: CR 903.10a the tally is per
// commander, so 20 from p2's Isamaru plus 2 from p0's (the same card name)
// is not 21 from one commander and p3 lives.
func TestCR903FourSeatCommanderDamageIsPerCommander(t *testing.T) {
	t.Parallel()
	run := fsRun(t, fsDamagePerCommander)
	setup, last := run.snaps[0], fsLast(run)
	if got := setup.Players[3].CommanderDamage["p2:"+fsIsamaru]; got != 20 {
		t.Fatalf("precondition: p3's setup tally from p2 = %d, want 20", got)
	}
	if last.Players[3].Lost || last.Over {
		t.Fatalf("p3 lost (over=%v) to 20+2 from two commanders: %v", last.Over, last.Players[3].CommanderDamage)
	}
	want := map[string]int32{"p2:" + fsIsamaru: 20, "p0:" + fsIsamaru: 2}
	if got := last.Players[3].CommanderDamage; len(got) != 2 || got["p2:"+fsIsamaru] != 20 || got["p0:"+fsIsamaru] != 2 {
		t.Fatalf("p3 tally = %v, want %v", got, want)
	}
	if setup.Players[3].Life-last.Players[3].Life != 2 {
		t.Fatalf("p3 life %d -> %d, want a fall of 2", setup.Players[3].Life, last.Players[3].Life)
	}
}

var fsDamageLoss = fsHead("fs-damage-loss", "903.10a") + `"setup":{` +
	`"p0":{"battlefield":["` + fsIsamaru + `"],"commanders":["` + fsIsamaru + `"]},` +
	`"p1":{"command":["` + fsIsamaru + `"],"commander_damage":{"p0:` + fsIsamaru + `":20}},` +
	`"p2":` + fsCmd + `,"p3":` + fsCmd + `},` +
	`"steps":[{"op":"attack","seat":0,"attackers":["p0:` + fsIsamaru + `"],"defender":"p1"},` + fsSwingToMain2 + `]}`

// TestCR903FourSeatCommanderDamageLossLeavesThree: 22 from one commander
// loses (CR 903.10a, 704.6c) at 38 life; with three left the game goes on
// and every object p1 owns leaves it (CR 800.4a).
func TestCR903FourSeatCommanderDamageLossLeavesThree(t *testing.T) {
	t.Parallel()
	run := fsRun(t, fsDamageLoss)
	last := fsLast(run)
	if !last.Players[1].Lost {
		t.Fatalf("p1 not lost at 22 commander damage: %v", last.Players[1].CommanderDamage)
	}
	if got := run.e.G.Players[1].Life; got != 38 {
		t.Fatalf("p1 life = %d, want 38 (lost on commander damage, not life)", got)
	}
	if last.Over {
		t.Fatal("game over with three seats left (CR 800.4a)")
	}
	for _, i := range []int{0, 2, 3} {
		if last.Players[i].Lost {
			t.Errorf("p%d lost, want only p1", i)
		}
	}
	g := run.e.G
	for z := state.ZLibrary; z <= state.ZPlanarDeck; z++ {
		for p := range g.Players {
			for _, id := range g.Zone(z, state.PlayerID(p)) {
				if o := g.Obj(id); o != nil && o.Owner == 1 {
					t.Errorf("p1's object %d (%s) still in zone %v of p%d (CR 800.4a)", id, o.Card.Faces[0].Name, z, p)
				}
			}
		}
	}
}

// fsDeadCommander: p0 Doom Blades p2's Isamaru; the 903.9a ask is answered
// pick, with seat asserting who it is posed to.
func fsDeadCommander(pick, seat string) string {
	return fsHead("fs-dead-commander", "903.9a") + `"setup":{` +
		`"p0":{"command":["` + fsIsamaru + `"],"hand":["Doom Blade"]},"p1":` + fsCmd + `,` +
		`"p2":{"battlefield":["` + fsIsamaru + `"],"commanders":["` + fsIsamaru + `"]},"p3":` + fsCmd + `},` +
		`"steps":[{"op":"cast","seat":0,"card":"p0:Doom Blade","mana":"BC","targets":["p2:` + fsIsamaru + `"]},` +
		`{"op":"resolve","seat":0,"answers":[{"kind":"commander_zone","pick":["` + pick + `"],"seat":` + seat + `}]}]}`
}

var (
	fsDeadYes      = fsDeadCommander("command_zone", "2")
	fsDeadNo       = fsDeadCommander("leave", "2")
	fsDeadToActive = fsDeadCommander("command_zone", "0")
)

// fsAskedSeat returns the seat the one commander_zone (yesno) decision of run
// was posed to.
func fsAskedSeat(t *testing.T, run *oracleRun) int {
	t.Helper()
	seat, n := -1, 0
	for _, d := range run.decisions {
		if d.Kind == "yesno" {
			seat, n = d.Seat, n+1
		}
	}
	if n != 1 {
		t.Fatalf("yes/no decisions = %d, want the one commander-zone ask: %+v", n, run.decisions)
	}
	return seat
}

// fsCommanderObj returns p's one commander object.
func fsCommanderObj(t *testing.T, run *oracleRun, p int) *state.Object {
	t.Helper()
	ids := run.e.G.Players[p].Commanders
	if len(ids) != 1 {
		t.Fatalf("p%d Commanders = %v, want one", p, ids)
	}
	return run.e.G.Obj(ids[0])
}

// TestCR903_9aFourSeatOwnerChoosesForADeadCommander: CR 903.9a "its owner may
// put it into the command zone": p0 kills p2's commander, the choice is p2's,
// not the active player's (a "seat":0 answer is refused).
func TestCR903_9aFourSeatOwnerChoosesForADeadCommander(t *testing.T) {
	t.Parallel()
	run := fsRun(t, fsDeadYes)
	if got := fsAskedSeat(t, run); got != 2 {
		t.Fatalf("CR 903.9a ask posed to p%d, want p2 (the owner)", got)
	}
	if o := fsCommanderObj(t, run, 2); o.Zone != state.ZCommand || o.Owner != 2 {
		t.Fatalf("yes: p2's Isamaru zone/owner = %v/p%d, want command/p2", o.Zone, o.Owner)
	}
	if cz := fsLast(run).Players[2].Command; len(cz) != 1 || cz[0] != fsIsamaru {
		t.Fatalf("yes: p2 command zone = %v, want Isamaru", cz)
	}

	run = fsRun(t, fsDeadNo)
	if got := fsAskedSeat(t, run); got != 2 {
		t.Fatalf("no: ask posed to p%d, want p2", got)
	}
	if gy := fsLast(run).Players[2].Graveyard; len(gy) != 1 || gy[0] != fsIsamaru {
		t.Fatalf("no: p2 graveyard = %v, want Isamaru", gy)
	}
	fsRefused(t, fsDeadToActive, "posed to p2")
}

// fsBounced: p0 Unsummons p3's Isamaru; the 903.9b ask answered pick.
func fsBounced(pick, seat string) string {
	return fsHead("fs-bounced-commander", "903.9b") + `"setup":{` +
		`"p0":{"command":["` + fsIsamaru + `"],"hand":["Unsummon"]},"p1":` + fsCmd + `,"p2":` + fsCmd + `,` +
		`"p3":{"battlefield":["` + fsIsamaru + `"],"commanders":["` + fsIsamaru + `"]}},` +
		`"steps":[{"op":"cast","seat":0,"card":"p0:Unsummon","mana":"U","targets":["p3:` + fsIsamaru + `"]},` +
		`{"op":"resolve","seat":0,"answers":[{"kind":"commander_zone","pick":["` + pick + `"],"seat":` + seat + `}]}]}`
}

var (
	fsBouncedYes      = fsBounced("command_zone", "3")
	fsBouncedNo       = fsBounced("leave", "3")
	fsBouncedToActive = fsBounced("command_zone", "0")
)

// TestCR903_9bFourSeatOwnerChoosesForABouncedCommander: CR 903.9b "its owner
// may put it into the command zone instead": p0 bounces p3's commander, the
// choice is p3's.
func TestCR903_9bFourSeatOwnerChoosesForABouncedCommander(t *testing.T) {
	t.Parallel()
	run := fsRun(t, fsBouncedYes)
	if got := fsAskedSeat(t, run); got != 3 {
		t.Fatalf("CR 903.9b ask posed to p%d, want p3 (the owner)", got)
	}
	last := fsLast(run)
	if o := fsCommanderObj(t, run, 3); o.Zone != state.ZCommand || o.Owner != 3 {
		t.Fatalf("yes: p3's Isamaru zone/owner = %v/p%d, want command/p3", o.Zone, o.Owner)
	}
	if cz := last.Players[3].Command; len(cz) != 1 || cz[0] != fsIsamaru {
		t.Fatalf("yes: p3 command zone = %v, want Isamaru", cz)
	}
	for _, n := range last.Players[3].Hand {
		if n == fsIsamaru {
			t.Fatalf("yes: Isamaru also in p3's hand %v", last.Players[3].Hand)
		}
	}

	run = fsRun(t, fsBouncedNo)
	if got := fsAskedSeat(t, run); got != 3 {
		t.Fatalf("no: ask posed to p%d, want p3", got)
	}
	if o := fsCommanderObj(t, run, 3); o.Zone != state.ZHand || o.Owner != 3 {
		t.Fatalf("no: p3's Isamaru zone/owner = %v/p%d, want hand/p3", o.Zone, o.Owner)
	}
	fsRefused(t, fsBouncedToActive, "posed to p3")
}

const (
	fsTymna    = "Tymna the Weaver"
	fsThrasios = "Thrasios, Triton Hero"
)

func fsPartners(card, mana string) string {
	return fsHead("fs-partners", "903.8") + `"setup":{` +
		`"p0":{"command":["` + fsTymna + `","` + fsThrasios + `"],"commander_casts":{"` + fsTymna + `":1}},` +
		`"p1":{"command":["` + fsIsamaru + `"],"commander_damage":{"p0:` + fsTymna + `":12,"p0:` + fsThrasios + `":12}},` +
		`"p2":` + fsCmd + `,"p3":` + fsCmd + `},` +
		`"steps":[{"op":"cast","seat":0,"card":"p0:` + card + `","mana":"` + mana + `"}]}`
}

var (
	fsPartnerThrasios   = fsPartners(fsThrasios, "GU")
	fsPartnerTymnaShort = fsPartners(fsTymna, "WBC")
	fsPartnerTymnaPaid  = fsPartners(fsTymna, "WBCCC")
)

// TestCR903FourSeatPartnersAreTaxedAndTalliedApart: CR 702.124 two partners
// are two commanders, each with its own tax (CR 903.8) and its own damage
// tally (CR 903.10a). Tymna's earlier cast taxes Tymna, not Thrasios; 12 and
// 12 from the two partners is not 21 from one.
func TestCR903FourSeatPartnersAreTaxedAndTalliedApart(t *testing.T) {
	t.Parallel()
	run := fsRun(t, fsPartnerThrasios)
	setup, last := run.snaps[0], fsLast(run)
	if got := setup.Players[0].CommanderCasts; got[fsTymna] != 1 || got[fsThrasios] != 0 {
		t.Fatalf("precondition: p0 casts = %v, want Tymna 1, Thrasios 0", got)
	}
	if len(last.Stack) != 1 || last.Stack[0].Source != "p0:"+fsThrasios {
		t.Fatalf("Thrasios for {G}{U}: stack = %+v, want Thrasios", last.Stack)
	}
	if got := setup.Players[1].CommanderDamage; len(got) != 2 || got["p0:"+fsTymna] != 12 || got["p0:"+fsThrasios] != 12 {
		t.Fatalf("p1 tally = %v, want 12 from each partner", got)
	}
	if last.Players[1].Lost || last.Over {
		t.Fatalf("p1 lost to 12+12 from two partners (over=%v)", last.Over)
	}

	fsRefused(t, fsPartnerTymnaShort, fsTymna)
	run = fsRun(t, fsPartnerTymnaPaid)
	if st := fsLast(run).Stack; len(st) != 1 || st[0].Source != "p0:"+fsTymna {
		t.Fatalf("Tymna for {W}{B}{3}: stack = %+v, want Tymna", st)
	}
}

const fsLurrus = "Lurrus of the Dream-Den"

var fsCompanion = fsHead("fs-companion", "702.139") + `"setup":{` +
	`"p0":{"command":["` + fsIsamaru + `"],"sideboard":["` + fsLurrus + `"],"mana":"CCC"},` +
	`"p1":` + fsCmd + `,"p2":` + fsCmd + `,"p3":` + fsCmd + `},"steps":[]}`

// TestCR903FourSeatCompanionIsOutsideTheGame: CR 702.139a and 903.11 — the
// companion waits outside the game, and any time its owner could cast a
// sorcery they may pay {3} to put it into their hand; no other seat may.
//
// DEVIATION (pinned): the engine has no companion special action (see
// 702-keyword-abilities.md, 702.139a-d). Lurrus sits in p0's sideboard, and
// p0's main-1 priority with {3} floating offers nothing that reaches it, nor
// does any other seat's. The rule requires p0 (and only p0) to be offered
// the pay-{3} action and Lurrus to end in p0's hand after taking it.
func TestCR903FourSeatCompanionIsOutsideTheGame(t *testing.T) {
	t.Parallel()
	run := fsRun(t, fsCompanion)
	e := run.e
	sb := e.G.Zone(state.ZSideboard, 0)
	if len(sb) != 1 || e.G.Obj(sb[0]).Card.Faces[0].Name != fsLurrus {
		t.Fatalf("precondition: p0 sideboard = %v, want Lurrus", sb)
	}
	lurrus := sb[0]
	if e.G.Step != state.StepMain1 || e.G.Active != 0 || e.G.Players[0].Pool[state.MC] != 3 {
		t.Fatalf("precondition: step %v active p%d pool C=%d, want main1 p0 with {3}",
			e.G.Step, e.G.Active, e.G.Players[0].Pool[state.MC])
	}
	seen := map[state.PlayerID]bool{}
	for range 4 {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("pending = %+v, want a priority decision", d)
		}
		seen[d.Player] = true
		for _, o := range d.Options {
			if o.Obj == lurrus || strings.Contains(strings.ToLower(string(o.Kind)), "companion") {
				t.Fatalf("p%d offered %+v: the companion action exists now; update this pin (and the 702.139 rows)", d.Player, o)
			}
		}
		idx := pickPass(d)
		if idx < 0 {
			t.Fatalf("p%d priority has no pass", d.Player)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
			t.Fatal(err)
		}
		if e.G.Step != state.StepMain1 {
			break
		}
	}
	if len(seen) < 4 {
		t.Fatalf("priority reached seats %v, want all four in main 1", seen)
	}
	if o := e.G.Obj(lurrus); o.Zone != state.ZSideboard || o.Owner != 0 {
		t.Fatalf("Lurrus zone/owner = %v/p%d, want sideboard/p0 (no companion action taken)", o.Zone, o.Owner)
	}
}
