package gorged

// Serving network-backed bot policies. A binary links one with RegisterNetBot
// before Main; gorged gives it flags, a reserved deck pool and a demo table.
//
// A network is trained on one deck distribution, so the server keeps the bot
// to it: -<policy>-decks names the decks it may play, those decks are reserved
// for it, and a game against it deals from them and nothing else. -<policy>-net
// and -<policy>-decks come as a pair for that reason. Policies may share a deck
// directory: a shared deck is reserved for every policy that names it and
// listed once.
//
// A policy guesses its opponent's deck from a belief corpus (-<policy>-belief,
// required) and never reads the real one, so it is offered for games against a
// human. A NetBot.ReadsDecklists policy does read the real lists, so it is
// never offered: it runs only on its bot-only demo table, needs
// -allow-decklist-bots, and takes -<policy>-belief as an option.

import (
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/host/httpapi"
)

// NetBot is a hosted policy whose seat loads a trained network at startup.
type NetBot struct {
	// NetHelp describes the network in -<policy>-net's help ("its .dzgw
	// network").
	NetHelp string
	// ReadsDecklists marks a policy that reads opponents' real deck lists
	// (bots.Info.Env without closed lists).
	ReadsDecklists bool
	// Enable loads the network at path and registers the policy in the bots
	// registry. belief is the corpus -<policy>-belief names, or nil for a
	// ReadsDecklists policy run without the flag. The registered bots.Info of
	// a policy that is not ReadsDecklists must set ClosedLists, or the host
	// refuses to seat it against a human.
	Enable func(path string, belief *searchprobe.Belief) error
}

// registeredNetBot is one RegisterNetBot call.
type registeredNetBot struct {
	policy string
	bot    NetBot
}

var (
	netBotsMu sync.Mutex
	// netBots is every RegisterNetBot registration, in registration order.
	netBots []registeredNetBot
)

// policyNameRE is what a RegisterNetBot policy name may be: it becomes a
// registry key and the prefix of five flags.
var policyNameRE = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// RegisterNetBot links policy into this binary and gives it the flags
// -<policy>-net, -decks, -belief, -belief-pool and -demo-table. Reserved pools
// and demo tables follow registration order.
//
// Call it before Main, Serve or ServeFlagSet (from an init or main): the flags
// are built from the registrations. policy must be a lower-case, dash-separated
// name not already registered and nb.Enable non-nil; a violation panics.
func RegisterNetBot(policy string, nb NetBot) {
	switch {
	case !policyNameRE.MatchString(policy):
		panic(fmt.Sprintf("gorged: RegisterNetBot: %q is not a lower-case dash-separated policy name", policy))
	case nb.Enable == nil:
		panic("gorged: RegisterNetBot: " + policy + " has no Enable")
	}
	netBotsMu.Lock()
	defer netBotsMu.Unlock()
	for _, r := range netBots {
		if r.policy == policy {
			panic("gorged: RegisterNetBot: " + policy + " registered twice")
		}
	}
	netBots = append(netBots, registeredNetBot{policy: policy, bot: nb})
}

// registeredNetBots returns RegisterNetBot's registrations in order.
func registeredNetBots() []registeredNetBot {
	netBotsMu.Lock()
	defer netBotsMu.Unlock()
	return append([]registeredNetBot(nil), netBots...)
}

// netPolicy is one registered policy and the values of its flags.
type netPolicy struct {
	policy string
	bot    NetBot

	net, decks         string
	belief, beliefPool string
	demoTable          bool
}

// registerFlags registers a's flags on fs.
func (a *netPolicy) registerFlags(fs *flag.FlagSet) {
	p := a.policy
	if a.bot.ReadsDecklists {
		fs.StringVar(&a.net, p+"-net", "", "enable the "+p+" bot: the path of "+a.bot.NetHelp+" (needs -"+p+"-decks, -allow-decklist-bots, -auto-mana and -bot-auto-mana; runs only on its bot-only demo table, never against a human)")
		fs.StringVar(&a.belief, p+"-belief", "", "with -"+p+"-net: close the opponents' deck lists -- "+p+" deals each opponent's unknown cards from lists drawn from the decks in this directory (the train split of its pool), never from the opponent's real list")
	} else {
		fs.StringVar(&a.net, p+"-net", "", "enable the "+p+" bot: the path of "+a.bot.NetHelp+" (needs -"+p+"-decks, -"+p+"-belief, -auto-mana and -bot-auto-mana). "+p+" guesses its opponent's deck list instead of reading it, so it is offered for games against a human")
		fs.StringVar(&a.belief, p+"-belief", "", "with -"+p+"-net (required): the deck corpus "+p+" guesses its opponent's list from (the train split of its pool)")
	}
	fs.StringVar(&a.decks, p+"-decks", "", "directory of the decks "+p+" may play: the decks its network was trained on. They are reserved for it (and for any other network bot that names the same directory), and it plays nothing else")
	fs.StringVar(&a.beliefPool, p+"-belief-pool", "", "with -"+p+"-belief: a pool.tsv naming the belief decks; only its train split is read (default: every deck file in -"+p+"-belief)")
	fs.BoolVar(&a.demoTable, p+"-demo-table", true, "with -"+p+"-net: add one two-seat "+p+" vs "+p+" table, dealt from -"+p+"-decks, for spectators")
}

// checkFlags validates a's flags alone. It runs before anything is loaded, so
// a misconfigured server fails fast.
func (a netPolicy) checkFlags(c config) error {
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
	case a.bot.ReadsDecklists && !c.allowDecklistBots:
		return fmt.Errorf("-%s-net needs -allow-decklist-bots: %s reads opponents' decklists and runs only on its bot-only demo table, never against a human", p, p)
	case !a.bot.ReadsDecklists && a.belief == "":
		return fmt.Errorf("-%s-net needs -%s-belief: %s plays humans only because it guesses the opponent's list from a belief corpus instead of reading it", p, p, p)
	case !c.autoMana || !c.botAutoPayMana:
		return fmt.Errorf("-%s-net needs -auto-mana and -bot-auto-mana (both default on): %s pays its mana through the engine's automatic payment plans", p, p)
	}
	return nil
}

// checkNetFlags validates every network policy's flags.
func (c config) checkNetFlags() error {
	for _, a := range c.nets {
		if err := a.checkFlags(c); err != nil {
			return err
		}
	}
	return nil
}

// enableNetBots loads each asked-for policy's belief and registers it over its
// network. It runs before the policy flags are resolved (the offered set is
// read from the registry) and after the corpus is open (a belief's decks are
// resolved against it). A policy without its -net flag is not registered.
func (c config) enableNetBots(reg *cards.Registry) error {
	if err := c.checkNetFlags(); err != nil {
		return err
	}
	for _, a := range c.nets {
		if a.net == "" {
			continue
		}
		var belief *searchprobe.Belief
		if a.belief != "" {
			var err error
			if belief, err = loadBelief(reg, a.belief, a.beliefPool, "-"+a.policy+"-belief", "-"+a.policy+"-belief-pool"); err != nil {
				return err
			}
		}
		if err := a.bot.Enable(a.net, belief); err != nil {
			return err
		}
	}
	return nil
}

// loadBelief is searchprobe.LoadBeliefDir with dirFlag and poolFlag naming
// the flags in its errors.
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

// reservedArchetype is the catalogue archetype of a reserved deck without its
// own; the picker shows "name - archetype", and without one these decks look
// like ids among the hand-named ones. The policies it is reserved for follow.
const reservedArchetype = "plays vs "

// loadPool loads the decks a may play from its -decks directory: their ids
// (the pool createGame deals from) and catalogue entries tagged with the
// policy. A deck that names a commander is refused (these policies play 1v1
// constructed), and so is an id that shadows a deck in -decks, because the
// loader resolves a table's decks by id alone. No flag, no pool.
//
// prior is every reserved pool loaded before it and catalogue their catalogue
// entries. A deck id a shares with a prior pool must come from the same
// directory (otherwise the id would be ambiguous to the loader); such a deck is
// reserved for both and is not catalogued twice: its entry in catalogue is
// re-tagged instead. It returns a's ids and the reserved catalogue with a's
// fresh decks appended (catalogue itself is not written).
func (a netPolicy) loadPool(reg *cards.Registry, cmdPool, conPool []string, prior []reservedDir, catalogue []httpapi.DeckInfo) ([]string, []httpapi.DeckInfo, error) {
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
			wasDefault := out[i].Archetype == reservedArchetype+joinOr(strings.Split(out[i].Pool, ","))
			out[i].Pool = httpapi.JoinPools(out[i].Pool, p)
			if wasDefault {
				out[i].Archetype = reservedArchetype + joinOr(strings.Split(out[i].Pool, ","))
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
			added[i].Archetype = reservedArchetype + p
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

// reservedLoader wraps load so the ids in pool resolve from dir. Policies
// sharing a directory make this the same loader twice, which resolves every
// shared id identically.
func reservedLoader(reg *cards.Registry, load func(string) (host.Deck, error), dir string, pool []string) func(string) (host.Deck, error) {
	if dir == "" || len(pool) == 0 {
		return load
	}
	reserved := deckLoader(reg, dir)
	return func(name string) (host.Deck, error) {
		if containsDeck(pool, name) {
			return reserved(name)
		}
		return load(name)
	}
}

// reservedOwners returns every bot policy a deck is reserved for, sorted.
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

// reservedFor returns the bot policies a deck is reserved for, joined; "" when
// none.
func (c config) reservedFor(id string) string {
	return strings.Join(c.reservedOwners(id), ", ")
}

// reservedPick is the policy a reserved deck picks for a request that names
// none: the first owner that may play a human (one that does not read
// decklists), else the first owner -- which createGame then refuses with the
// decklist rule.
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

// table is the demo's bot-versus-bot table for a, dealt from its pool, playing
// match after match for anyone watching; id names it. Nil when a is off or its
// demo table is switched off (-<policy>-demo-table=false). A ReadsDecklists
// policy's table is the one effect of -allow-decklist-bots: all its seats are
// bots, and the host refuses such a policy at any table with a human even with
// the bit set.
func (a netPolicy) table(c config, id host.TableID, seed uint64, pool []string, vis view.Visibility) *host.TableConfig {
	if a.net == "" || !a.demoTable || len(pool) == 0 {
		return nil
	}
	return &host.TableConfig{
		ID: id, Name: a.policy + " vs " + a.policy, Seats: 2, Decks: pool,
		Seed: seed, Pace: c.pace, Spectator: vis, Perpetual: c.perpetual,
		Mulligans: c.mulligans, Format: host.FormatConstructed, BotPolicy: a.policy,
		BotAutoPayMana: c.botAutoPayMana, AutoMana: c.autoMana,
		AllowDecklistBots: a.bot.ReadsDecklists,
	}
}
