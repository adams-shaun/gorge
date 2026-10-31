package gorged

import (
	"strings"
	"testing"
)

// A net flag naming a policy this binary does not link is a startup error
// that says how to get one; with no net flags nothing is asked of the
// registry.
func TestNetBotNotLinked(t *testing.T) {
	netBotsMu.Lock()
	saved := netBots
	netBots = map[string]NetBot{}
	netBotsMu.Unlock()
	t.Cleanup(func() {
		netBotsMu.Lock()
		netBots = saved
		netBotsMu.Unlock()
	})
	if err := (config{}).enableAZC3(); err != nil {
		t.Errorf("no az-c3 flags: %v", err)
	}
	if err := (config{}).checkClosedAZFlags(); err != nil {
		t.Errorf("no closed-list flags: %v", err)
	}
	az3 := config{az3Net: "n.dzgw", az3Decks: "d", allowDecklistBots: true, autoMana: true, botAutoPayMana: true}
	if err := az3.enableAZC3(); err == nil || !strings.Contains(err.Error(), "-az-c3-net: az-c3 is not linked into this binary") {
		t.Errorf("az-c3 unlinked: error %v", err)
	}
	for _, policy := range closedPolicies {
		c := withClosedAZ(config{autoMana: true, botAutoPayMana: true}, policy, closedAZ{net: "n.dzgw", decks: "d", belief: "b"})
		want := "-" + policy + "-net: " + policy + " is not linked into this binary (a net policy's implementation is registered with gorged.RegisterNetBot)"
		if err := c.checkClosedAZFlags(); err == nil || err.Error() != want {
			t.Errorf("%s unlinked: error %v, want %q", policy, err, want)
		}
	}
}

func TestRegisterNetBotRefusesAnUnknownPolicy(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("RegisterNetBot accepted a policy gorged does not serve")
		}
	}()
	RegisterNetBot("az-c9", NetBot{Enable: stubNetBots[policyAZC3].enable})
}
