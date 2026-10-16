package main

// Serving the closed-list az policies: az-c4
// (docs/superpowers/reports/2026-10-14-az-c4-hosted.md) and az-c5
// (docs/superpowers/reports/2026-10-15-az-c5.md).
//
// Each is az-c3's search with its own network, playing CLOSED opponent lists
// only: it guesses the opponent's deck from a belief corpus and never reads
// the real one, so -- unlike az-c3 -- it is offered for games against a
// human and needs no -allow-decklist-bots. They differ only in the network,
// so one closedAZ serves both, each under its own flags (-az-c4-*,
// -az-c5-*). Their flags mirror az-c3's: -<policy>-net and -<policy>-decks
// come as a pair (a network trained on one deck distribution plays only
// those decks), and -<policy>-belief is required, not optional. The decks
// may be the same directory as -az-c3-decks or another closed policy's: a
// shared deck is reserved for every policy that names it and listed once.

import (
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azc4"
	"github.com/adams-shaun/gorge/bots/azc5"
	"github.com/adams-shaun/gorge/bots/azc6"
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/host"
	"github.com/adams-shaun/gorge/host/httpapi"
	"github.com/adams-shaun/gorge/internal/searchprobe"
	"github.com/adams-shaun/gorge/view"
)

// closedAZ is one closed-list az policy's flags. config holds one per policy
// (az4, az5, az6); closedAZs fills in which policy each is, so a hand-built
// config{} in a test needs only the flag values.
type closedAZ struct {
	// policy is the registry key and the flags' prefix ("az-c4").
	policy string
	// register is the policy package's Enable.
	register func(path string, b *searchprobe.Belief) error

	net, decks         string
	belief, beliefPool string
	demoTable          bool
}

// closedAZs returns every closed-list az policy's flags, in registration
// (and demo-table) order: az-c4, az-c5, then az-c6.
func (c config) closedAZs() []closedAZ {
	a4, a5, a6 := c.az4, c.az5, c.az6
	a4.policy, a4.register = azc4.Policy, azc4.Enable
	a5.policy, a5.register = azc5.Policy, azc5.Enable
	a6.policy, a6.register = azc6.Policy, azc6.Enable
	return []closedAZ{a4, a5, a6}
}

// closedAZ returns the named closed-list policy's flags.
func (c config) closedAZ(policy string) closedAZ {
	for _, a := range c.closedAZs() {
		if a.policy == policy {
			return a
		}
	}
	panic("gorged: no closed-list az policy " + policy)
}

// registerFlags registers policy's flags on fs, into a; what names the network in
// the -net help ("its .dzgw network").
func (a *closedAZ) registerFlags(fs *flag.FlagSet, policy, what string) {
	p := policy
	fs.StringVar(&a.net, p+"-net", "", "enable the "+p+" bot: the path of "+what+" (needs -"+p+"-decks, -"+p+"-belief, -auto-mana and -bot-auto-mana). "+p+" guesses its opponent's deck list instead of reading it, so it is offered for games against a human")
	fs.StringVar(&a.decks, p+"-decks", "", "directory of the decks "+p+" may play: the FDN limited decks it was trained on. They are reserved for it (and for az-c3 or another az policy too when it names the same directory), and it plays nothing else")
	fs.StringVar(&a.belief, p+"-belief", "", "with -"+p+"-net (required): the deck corpus "+p+" guesses its opponent's list from (the TRAIN split of its pool)")
	fs.StringVar(&a.beliefPool, p+"-belief-pool", "", "with -"+p+"-belief: a pool.tsv naming the belief decks; only its train split is read (default: every deck file in -"+p+"-belief)")
	fs.BoolVar(&a.demoTable, p+"-demo-table", true, "with -"+p+"-net: add one two-seat "+p+" vs "+p+" table, dealt from -"+p+"-decks, for spectators")
}

// checkFlags validates a's flags alone. It runs before anything is loaded,
// so a misconfigured server fails fast.
func (a closedAZ) checkFlags(autoMana, botAutoPayMana bool) error {
	p := a.policy
	switch {
	case a.beliefPool != "" && a.belief == "":
		return fmt.Errorf("-%s-belief-pool needs -%s-belief", p, p)
	case a.net == "" && a.decks == "" && a.belief == "":
		return nil
	case a.net == "":
		return fmt.Errorf("-%s-decks and -%s-belief need -%s-net", p, p, p)
	case a.decks == "":
		return fmt.Errorf("-%s-net needs -%s-decks: the network was trained on one deck distribution and the server keeps the bot to it", p, p)
	case a.belief == "":
		return fmt.Errorf("-%s-net needs -%s-belief: %s plays humans only because it guesses the opponent's list from a belief corpus instead of reading it", p, p, p)
	case !autoMana || !botAutoPayMana:
		return fmt.Errorf("-%s-net needs -auto-mana and -bot-auto-mana (both default on): %s pays its mana through the engine's automatic payment plans", p, p)
	}
	return nil
}

// checkClosedAZFlags validates every closed-list policy's flags.
func (c config) checkClosedAZFlags() error {
	for _, a := range c.closedAZs() {
		if err := a.checkFlags(c.autoMana, c.botAutoPayMana); err != nil {
			return err
		}
	}
	return nil
}

// enableClosedAZ loads each closed-list policy's belief and registers it over
// its network with its opponents' lists closed under that belief. It runs
// before the policy flags are resolved (the offered set is read from the
// registry) and after the corpus is open (the belief's decks are resolved
// against it). A policy without its -net flag is not registered.
func (c config) enableClosedAZ(reg *cards.Registry) error {
	if err := c.checkClosedAZFlags(); err != nil {
		return err
	}
	for _, a := range c.closedAZs() {
		if a.net == "" {
			continue
		}
		p := a.policy
		b, err := loadBelief(reg, a.belief, a.beliefPool, "-"+p+"-belief", "-"+p+"-belief-pool")
		if err != nil {
			return err
		}
		if err := a.register(a.net, b); err != nil {
			return err
		}
	}
	return nil
}

// loadBelief is searchprobe.LoadBeliefDir with dirFlag and poolFlag naming
// the flags in its errors. It is the one reader behind -az-c3-belief
// (azc3Belief) and each closed-list policy's -belief.
func loadBelief(reg *cards.Registry, dir, pool, dirFlag, poolFlag string) (*searchprobe.Belief, error) {
	b, err := searchprobe.LoadBeliefDir(reg, dir, pool)
	var pe *searchprobe.PoolError
	var de *searchprobe.DirError
	switch {
	case err == nil:
		return b, nil
	case errors.As(err, &pe):
		return nil, fmt.Errorf("%s: %w", poolFlag, pe.Err)
	case errors.As(err, &de):
		return nil, fmt.Errorf("%s: %w", dirFlag, de.Err)
	}
	return nil, fmt.Errorf("%s %w", dirFlag, err)
}

// reservedDir is a reserved deck pool already loaded: the policy it is
// reserved for, the directory its ids resolve from, and the ids.
type reservedDir struct {
	policy, dir string
	pool        []string
}

// fdnArchetype is the catalogue archetype a reserved FDN deck without its own
// gets; plays names the policies it is reserved for.
const fdnArchetype = "FDN limited, plays vs "

// loadPool loads the decks a may play from its -decks directory, exactly as
// azc3Pool loads az-c3's. prior is every reserved pool loaded before it
// (az-c3's, then the closed-list policies' in order) and catalogue their
// catalogue entries. A deck id a shares with a prior pool must come from the
// same directory (otherwise the id would be ambiguous to the loader); such a
// deck is reserved for both and is not catalogued twice: its entry in
// catalogue is re-tagged instead. It returns a's ids and the reserved
// catalogue with a's fresh decks appended (catalogue itself is not written).
func (a closedAZ) loadPool(reg *cards.Registry, cmdPool, conPool []string, prior []reservedDir, catalogue []httpapi.DeckInfo) ([]string, []httpapi.DeckInfo, error) {
	if a.decks == "" {
		return nil, catalogue, nil
	}
	p := a.policy
	commander, constructed, err := splitDecks(reg, a.decks)
	if err != nil {
		return nil, nil, fmt.Errorf("-%s-decks: %w", p, err)
	}
	if len(commander) > 0 {
		return nil, nil, fmt.Errorf("-%s-decks: %q names a commander; %s plays 1v1 constructed decks only", p, commander[0], p)
	}
	if len(constructed) == 0 {
		return nil, nil, fmt.Errorf("-%s-decks: no deck files in %s", p, a.decks)
	}
	var fresh []string
next:
	for _, id := range constructed {
		if containsDeck(cmdPool, id) || containsDeck(conPool, id) {
			return nil, nil, fmt.Errorf("-%s-decks: deck %q has the same id as a deck in -decks", p, id)
		}
		for _, r := range prior {
			if !containsDeck(r.pool, id) {
				continue
			}
			if filepath.Clean(r.dir) != filepath.Clean(a.decks) {
				return nil, nil, fmt.Errorf("-%s-decks: deck %q has the same id as a deck in -%s-decks (a different directory); share one directory or rename it", p, id, r.policy)
			}
			continue next
		}
		fresh = append(fresh, id)
	}
	out := catalogue
	if len(fresh) < len(constructed) {
		out = append([]httpapi.DeckInfo(nil), catalogue...)
		for i := range out {
			if !containsDeck(constructed, out[i].ID) {
				continue
			}
			wasDefault := out[i].Archetype == fdnArchetype+joinOr(strings.Split(out[i].Pool, ","))
			out[i].Pool = httpapi.JoinPools(out[i].Pool, p)
			if wasDefault {
				out[i].Archetype = fdnArchetype + joinOr(strings.Split(out[i].Pool, ","))
			}
		}
	}
	added, err := loadDeckCatalogue(a.decks, nil, fresh)
	if err != nil {
		return nil, nil, fmt.Errorf("-%s-decks: %w", p, err)
	}
	for i := range added {
		added[i].Pool = p
		if added[i].Archetype == "" {
			added[i].Archetype = fdnArchetype + p
		}
	}
	return constructed, append(out, added...), nil
}

// joinOr joins names as "a", "a or b", "a, b or c".
func joinOr(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1]
}

// reservedLoader wraps load so the ids in pool resolve from dir. A shared
// directory (az-c3 and a closed-list policy on one -decks path) is the same
// loader twice, which resolves every shared id identically.
func reservedLoader(reg *cards.Registry, load func(string) (host.Deck, error), dir string, pool []string) func(string) (host.Deck, error) {
	if dir == "" || len(pool) == 0 {
		return load
	}
	az := deckLoader(reg, dir)
	return func(name string) (host.Deck, error) {
		if containsDeck(pool, name) {
			return az(name)
		}
		return load(name)
	}
}

// reservedOwners returns every bot policy a deck is reserved for, sorted;
// reservedFor (azc3.go) joins them.
func (c config) reservedOwners(id string) []string {
	var owners []string
	for policy, pool := range c.policyPools {
		if containsDeck(pool, id) {
			owners = append(owners, policy)
		}
	}
	sort.Strings(owners)
	return owners
}

// reservedPick is the policy a reserved deck picks for a request that names
// none: the first owner that may play a human (one that does not read
// decklists), else the first owner -- which createGame then refuses with
// the decklist rule, as it always has for a deck reserved only for az-c3.
func (c config) reservedPick(id string) string {
	owners := c.reservedOwners(id)
	for _, o := range owners {
		if !bots.ReadsDecklists(o) {
			return o
		}
	}
	if len(owners) > 0 {
		return owners[0]
	}
	return ""
}

// table is the demo's bot-versus-bot table for a, dealt from its pool,
// playing match after match for anyone watching; id names it. Nil when a is
// off or its demo table is switched off (-<policy>-demo-table=false). It
// needs no -allow-decklist-bots: a closed-list policy never reads a
// decklist.
func (a closedAZ) table(c config, id host.TableID, seed uint64, pool []string, vis view.Visibility) *host.TableConfig {
	if a.net == "" || !a.demoTable || len(pool) == 0 {
		return nil
	}
	return &host.TableConfig{
		ID: id, Name: a.policy + " vs " + a.policy + " (FDN)", Seats: 2, Decks: pool,
		Seed: seed, Pace: c.pace, Spectator: vis, Perpetual: c.perpetual,
		Mulligans: c.mulligans, Format: host.FormatConstructed, BotPolicy: a.policy,
		BotAutoPayMana: c.botAutoPayMana, AutoMana: c.autoMana,
	}
}

// reservedPolicyNames is every policy with a reserved deck pool. Draft
// events deal their own decks, so they never offer one.
func (c config) reservedPolicyNames() []string {
	var out []string
	for p := range c.policyPools {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
