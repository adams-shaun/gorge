package rules

import "testing"

// CR 400.7, 701.50b: Baron Strucker's optional connive trigger for A.I.M. Bot
// resolves after the Bot died and Thunderbolts Conspiracy returned it. The
// returned Bot is a new object, so the connive still draws and discards (the
// drawn Grizzly Bears, a nonland card) but puts no +1/+1 counter on it. The
// returned Bot's own entry trigger is declined, so the old trigger is the
// only connive. The test checks the Bot is back on the battlefield and the
// Bears was discarded, so the 2/3 means no counter was put on it.
func TestConniveTriggeredReferentReturnedGetsNoCounter(t *testing.T) {
	t.Parallel()
	playRuleScenario(t, `{"name":"connive-triggered-referent-returned","cr":["400.7","701.50b"],"why":"the villain Strucker's trigger names died and returned: draw and discard, no counter on the new object","setup":{"p0":{"hand":["A.I.M. Bot"],"battlefield":["Baron Strucker, HYDRA Overlord","Thunderbolts Conspiracy"],"library_top":["Grizzly Bears","Island"],"mana":"UU"},"p1":{"hand":["Lightning Strike"],"mana":"RR"}},"steps":[{"op":"cast","seat":0,"card":"p0:A.I.M. Bot","mana":"UU"},{"op":"pass","seat":0},{"op":"pass","seat":1},{"op":"pass","seat":0,"expect":[{"stack_size":1}]},{"op":"cast","seat":1,"card":"p1:Lightning Strike","mana":"RR","targets":["p0:A.I.M. Bot"]},{"op":"resolve","answers":[{"kind":"trigger_optional","pick":["no"]},{"kind":"trigger_optional","pick":["yes"]}]}],"expect":[{"card":"p0:A.I.M. Bot","zone":"battlefield"},{"card":"p0:Grizzly Bears","zone":"graveyard"},{"hand_size":{"p0":0}},{"card":"p0:A.I.M. Bot","pt":"2/3"}]}`)
}
