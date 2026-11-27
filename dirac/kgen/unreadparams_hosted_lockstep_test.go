package kgen

import (
	"strings"
	"testing"
)

// The hosted-Commander unread-param and counter shapes of ticket
// cli-20261127T065129Z-e7b92793. Three lower (the lockstep arms live in
// dirac/oracle's TestUnreadParams*Lockstep, which play each shape against
// gorge); six stay refused by name, each with kgen's own reason, because the
// shape needs state the kernel does not carry yet (see unreadHostedRefused).

// unreadHostedLowered are the cards that lower with no refusals, and the shape
// each carries.
var unreadHostedLowered = []struct{ name, shape string }{
	{"Nick Valentine, Private Eye", "DB$ Investigate: Optional$ True (OpMayAsk before the Clue mint)"},
	{"Urza's Bauble", "AB$ Reveal: Random$ True over a target hand (ExtRevealRandom, one RNG draw)"},
	{"Chandra, Awakened Inferno", "AB$ Effect: EffectOwner$ Player.Opponent (CreateEmblem obj=EachOpp)"},
}

// unreadHostedRefused are the cards that stay refused: the card and the
// fragment kgen's refusal reason must still carry. Each needs a kernel state
// gorge keeps and dirac lacks:
//   - Captain Marvel: the triggering counter map (kind and number per counter put).
//   - Moira and Teshar: a leave-the-battlefield exile replacement on a Pump's target.
//   - Filigree Vector: a remembered target set read by a later PutCounterAll.
//   - Synth Eradicator: PutCounter Optional$ plus RememberPut$ read by a player-count condition.
//   - Rise and Shine: RememberAnimated$ feeding a chained PutCounter Defined$ Remembered.
//   - Jeweled Amulet: the noted mana type of the activation cost (Special LastNotedType).
//
// When one starts to lower, this test fails: move it to unreadHostedLowered
// and add its lockstep arm in dirac/oracle.
var unreadHostedRefused = []struct{ name, reason string }{
	{"Captain Marvel, Apex Avenger", "PutCounter without CounterType"},
	{"Moira and Teshar", "unread params [LeaveBattlefield]"},
	{"Filigree Vector", "unread params [RememberTargets]"},
	{"Synth Eradicator", "unread params [Optional RememberPut]"},
	{"Rise and Shine", "unread params [RememberAnimated]"},
	{"Jeweled Amulet", "unread params [RememberCostMana]"},
}

// TestUnreadParamsHostedLockstep: the three lowered cards carry no refusal;
// the six refused ones are still refused, by their own reason.
func TestUnreadParamsHostedLockstep(t *testing.T) {
	t.Parallel()
	var names []string
	shapes := map[string]string{}
	for _, c := range unreadHostedLowered {
		names = append(names, c.name)
		shapes[c.name] = c.shape
	}
	for _, c := range unreadHostedRefused {
		names = append(names, c.name)
	}
	reasons := map[string]string{}
	for _, c := range lowerNamed(t, names) {
		var rs []string
		for _, r := range c.Refusals {
			rs = append(rs, r.Reason)
		}
		reasons[c.Name] = strings.Join(rs, "; ")
	}
	for _, c := range unreadHostedLowered {
		if r := reasons[c.name]; r != "" {
			t.Errorf("%s (%s): refused: %s", c.name, c.shape, r)
		}
	}
	for _, c := range unreadHostedRefused {
		r := reasons[c.name]
		if r == "" {
			t.Errorf("%s now lowers: move it to unreadHostedLowered with a dirac/oracle lockstep arm", c.name)
		} else if !strings.Contains(r, c.reason) {
			t.Errorf("%s: refusal %q no longer names %q", c.name, r, c.reason)
		}
	}
}
