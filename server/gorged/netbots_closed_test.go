package gorged

import (
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// withExtraClosed runs with policy registered through RegisterClosedNetBot
// over a stub implementation, restoring the linked set afterwards. The bots
// registry entry an Enable adds stays (it is process-wide), so each test
// uses its own policy name.
func withExtraClosed(t *testing.T, policy string) {
	t.Helper()
	netBotsMu.Lock()
	savedBots := make(map[string]NetBot, len(netBots))
	for k, v := range netBots {
		savedBots[k] = v
	}
	savedExtra := append([]extraClosedNetBot(nil), extraClosed...)
	netBotsMu.Unlock()
	t.Cleanup(func() {
		netBotsMu.Lock()
		netBots, extraClosed = savedBots, savedExtra
		netBotsMu.Unlock()
	})
	sb := &stubNetBot{info: stubInfo(policy, true)}
	RegisterClosedNetBot(policy, "its .dzgw network (a test net)", sb.enable)
}

// A closed-list policy gorged does not name is served by the same machinery
// as az-c4 and az-c5: its five flags exist and parse, it is ordered after
// the named policies, its flags are checked the same way, and with them set
// it is registered closed over its belief and offered to draft events.
func TestRegisterClosedNetBotServesAnUnnamedPolicy(t *testing.T) {
	const policy = "az-cx-served"
	withExtraClosed(t, policy)
	dir := t.TempDir()
	path, err := writeStubNet(dir, "net.dzgw")
	if err != nil {
		t.Fatal(err)
	}
	belief := filepath.Join(dir, "belief")
	if err := os.Mkdir(belief, 0o755); err != nil {
		t.Fatal(err)
	}
	writeDeck(t, belief, "b1", "")
	writeDeck(t, belief, "b2", "")
	fs, c := serveFlags()
	fs.SetOutput(nil)
	args := []string{"-" + policy + "-net", path, "-" + policy + "-decks", dir, "-" + policy + "-belief", belief, "-" + policy + "-demo-table=false"}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	if f := fs.Lookup(policy + "-net"); f == nil || !strings.Contains(f.Usage, "a test net") {
		t.Fatalf("-%s-net flag %+v: want the registered help", policy, f)
	}
	var order []string
	for _, a := range c.closedAZs() {
		order = append(order, a.policy)
	}
	if want := []string{policyAZC4, policyAZC5, policy}; !reflect.DeepEqual(order, want) {
		t.Fatalf("closed-list order %v, want %v", order, want)
	}
	a := c.closedAZ(policy)
	if a.net != path || a.decks != dir || a.belief != belief || a.demoTable || a.register == nil {
		t.Fatalf("parsed flags %+v", a)
	}
	bad := *c
	bad.extraAZ = []closedAZ{{policy: policy, net: path, decks: dir}}
	if err := bad.checkClosedAZFlags(); err == nil || !strings.Contains(err.Error(), "-"+policy+"-net needs -"+policy+"-belief") {
		t.Errorf("no belief: error %v", err)
	}
	c.autoMana, c.botAutoPayMana = true, true
	if err := c.enableClosedAZ(testutil.CorpusRegistry(t)); err != nil {
		t.Fatal(err)
	}
	e, ok := bots.Lookup(policy)
	if !ok || !e.ClosedLists || bots.ReadsDecklists(policy) {
		t.Fatalf("after enable: registered %v, closed %v", ok, e.ClosedLists)
	}
	c.botPolicies = []string{bots.Default, policy}
	if got := c.eventPolicies(); !reflect.DeepEqual(got, c.botPolicies) {
		t.Errorf("event policies %v, want %v", got, c.botPolicies)
	}
}

func TestRegisterClosedNetBotRefusals(t *testing.T) {
	withExtraClosed(t, "az-cx-dup")
	sb := &stubNetBot{info: stubInfo("x", true)}
	for _, policy := range []string{policyAZC3, policyAZC4, policyAZC5, "az-cx-dup", "", "Az-C6", "az c6", "-az", "az-"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("RegisterClosedNetBot(%q) accepted", policy)
				}
			}()
			RegisterClosedNetBot(policy, "net", sb.enable)
		}()
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("RegisterClosedNetBot accepted a nil enable")
			}
		}()
		RegisterClosedNetBot("az-cx-nil", "net", nil)
	}()
}

// Extensions.Flags puts a binary's own flags on gorged's command line: the
// flag set Main and Serve parse, and ServeFlagSetFor's, carry them, and
// gorge's own ServeFlagSet does not.
func TestExtensionFlags(t *testing.T) {
	var got string
	ext := Extensions{Flags: func(fs *flag.FlagSet) { fs.StringVar(&got, "ext-test-flag", "def", "an extension's flag") }}
	fs, c := flagsFor(ext)
	fs.SetOutput(nil)
	if err := fs.Parse([]string{"-ext-test-flag", "v", "-events"}); err != nil {
		t.Fatal(err)
	}
	if got != "v" || !c.events || c.ext.Flags == nil {
		t.Fatalf("ext flag %q, events %v, ext carried %v", got, c.events, c.ext.Flags != nil)
	}
	if err := ServeFlagSetFor(ext).Parse([]string{"-ext-test-flag", "w"}); err != nil || got != "w" {
		t.Fatalf("ServeFlagSetFor: %v (value %q)", err, got)
	}
	if err := ServeFlagSet().Parse([]string{"-ext-test-flag", "w"}); err == nil {
		t.Fatal("gorge's own flag set accepted an extension's flag")
	}
}
