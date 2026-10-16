package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/bots/sbtactical"
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/azmcts"
	"github.com/adams-shaun/gorge/internal/gonet"
	"github.com/adams-shaun/gorge/internal/policynet"
	"github.com/adams-shaun/gorge/internal/searchprobe"
	"github.com/adams-shaun/gorge/internal/spellbench/builtins"
	"github.com/adams-shaun/gorge/seat"
)

// policySpec is one side's policy, parsed from "name[:key=value...]":
//
//	random                       uniform over the legal options, mana paid automatically (sb-uniform)
//	bot                          gorge's production heuristic, no search
//	sbtactical                   sb-tactical (bots/sbtactical, internal/spellbench/builtins.NewTactical):
//	                             the SpellBench scored heuristic, gorge's strongest no-search bot. Mana is
//	                             always auto-paid (builtins.AutoPay, the mode botbench's sb-tactical arm
//	                             and the hosted seat use), and the game runner hands it the engine as its
//	                             potential-play payment planner, as botbench does. Its keep/mulligan and
//	                             bottoming asks go to gorge's default policy, so :mull, :keep7 and
//	                             -mull-heuristic act on it exactly as on bot; other keys are ignored
//	az:sims=100                  AlphaZero-style PUCT search over the seat's own decisions
//	                             (internal/azmcts), honest worlds: each simulation re-deals the
//	                             cards the seat cannot see. Generation 0 without net=: a uniform
//	                             prior and gorge's heuristic leaf
//	az:sims=100:net=gen1.gpol    the same, the network's prior and value head
//	   :leaf=heuristic           keep the heuristic leaf (the net is the prior only)
//	   :prior=uniform            keep the uniform prior (the net is the leaf only)
//	   :worlds=K                 K deals per decision instead of one per simulation
//	   :explore[:nonoise]        generation: sample moves by visits on turns 1-4 (+ root noise)
//	   :cpuct=1.5 :fpu=0.1 :cands=N
//	   :exploreturns=N           with :explore, sample moves on turns <= N (default 4)
//	   :temp=T                   (PL-3) sample the move on EVERY searched decision by visits^(1/T), T>0
//	   :second=P                 (PL-3) with probability P play the second most-visited root move
//	   :aseed=N                  (PL-3) salt for the root noise / move randomness only (replays)
//	   :autopay                  mana paid automatically: every cast (and pass) at a priority stop is a
//	                             candidate, so the search decides what to cast and when (the seat's own
//	                             bot is gorge's auto-pay bot; azmcts Options.AutoPayment). bot:autopay is
//	                             that bot alone
//	   :topk=K                   the network chooses the candidates: every searched decision (the root
//	                             and every one inside a simulation) is enumerated in full and the bot's
//	                             answer is kept with the K-1 others the network's prior ranks highest,
//	                             instead of the first cands= in enumeration order (under autopay the
//	                             prior then also scores every cast). Needs net= or remote= and the
//	                             net's prior (inert under prior=uniform or with no network);
//	                             azmcts Options.PriorTopK
//	   :oppnodes                 the opponent in the tree: its searched decisions inside a simulation
//	                             (priority, attackers, blockers, single target, >= 2 candidates) are tree
//	                             points where it picks what is worst for the seat (PUCT on 1 - Q), its
//	                             prior the net's on the opponent's own view, instead of gorge's bot
//	                             answering them (azmcts Options.OpponentNodes)
//	   :oppfull                  with :oppnodes (which it needs): the opponent's points mirror the seat's
//	                             settings -- under :autopay its bot pays automatically and its casts are
//	                             searched candidates, under :topk=K the net's prior ranks and cuts its
//	                             candidates too (azmcts Options.OpponentMirror; gorge/patches/0007).
//	                             Without it the opponent's points ignore :autopay and :topk
//	   :skippass                 simulation shortcut: rollout engines auto-pass every priority window
//	                             outside a main phase with an empty stack (azmcts Options.SkipPass); the
//	                             real game and the root decision are untouched
//	   :mull                     the London mulligan by land count (botpolicy/mulligan.go): keep 2-5 lands
//	                             of 7 (also after one mulligan), after two keep unless 0 or 7, and
//	                             bottom toward ceil(K/2) lands, highest mana value spells first. Any
//	                             bot, sbtactical, az or prior seat; play -mull-heuristic sets it on every seat.
//	                             Without it the seat mulligans on gorge's 1/3 coin
//	   :keep7                    never mulligan (the control for :mull in a game with -mulligans N)
//	   :morekinds                the fuller search (gorge/patches/0007): besides priority, attackers,
//	                             blockers and single targets, a decision's modes, a may trigger's yes/no,
//	                             a choice among at most cands= options (X, a number, a name, cost cards,
//	                             dig/search picks; never a mana-payment window) and multi-target picks
//	                             are searched, each legal option set a candidate (the bot's first). With
//	                             :oppnodes the opponent's are tree points too, and with :oppfull:autopay
//	                             as well the opponent pays automatically and its casts are searched
//	                             candidates
//	   :kinds=LIST               the searched kinds by name, comma-separated: priority, attackers,
//	                             blockers, target, modes, optional, choose, multitarget, or all (=
//	                             :morekinds); the default is the first four
//	   :closedlist               the opponent's deck list is closed: the seat's own list stays known,
//	                             but every world deals the opponent's unknown hand and library from a
//	                             list drawn from the belief corpus (play -belief-decks/-belief-pool,
//	                             default the TRAIN split of -decks/-pool), conditioned on the colours
//	                             of the opponent's seen cards, never from its real list or its real
//	                             hidden cards (azmcts.WorldRedealClosed)
//	prior:net=gen1.gpol          the network's policy alone: the argmax of its prior over the
//	                             candidates the search would build, no simulation
//	az:sims=100:remote=unix:/tmp/dzg.sock
//	                             the network served by python -m dzg.serve (gorge/dzg): an MLP,
//	                             transformer or GNN reading gorge's entity encoding, in place
//	                             of a .gpol (also prior:remote=...)
//	az:sims=100:gonet=best.dzgw  dzg's mlp evaluated in this process (gonet.go), from python -m
//	                             dzg.export: the same network as remote= without the socket (also
//	                             prior:gonet=...; the path must not contain a colon)
type policySpec struct {
	Raw, Kind    string
	Sims         int
	Net          string
	Remote       string
	GoNet        string
	model        *policynet.Model
	HeurLeaf     bool
	UniPrior     bool
	Worlds       int
	Explore      bool
	NoNoise      bool
	ExploreTurns int     // :exploreturns=; 0 keeps the seat default
	Temp         float64 // :temp=; 0 off
	Second       float64 // :second=; 0 off
	ASeed        uint64  // :aseed=
	CPUCT        float64
	FPU          float64
	Cands        int
	AutoPay      bool
	Mull         botpolicy.MulliganRule // :mull / :keep7; zero is gorge's 1/3 coin
	TopK         int
	OppNodes     bool
	SkipPass     int
	OppFull      bool         // :oppfull
	Closed       bool         // :closedlist (the belief is set by play, setBelief)
	Kinds        azmcts.Kinds // :kinds= / :morekinds; zero is azmcts.AllKinds
	// lookup resolves card names for sbtactical (setLookup); nil reads View facts only.
	lookup builtins.CardLookup
	// belief is :closedlist's opponent-list belief (setBelief).
	belief *searchprobe.Belief
}

// setBelief gives a :closedlist seat its belief (loaded after the specs are parsed).
func (p *policySpec) setBelief(b *searchprobe.Belief) {
	if p.Closed {
		p.belief = b
	}
}

// setLookup gives an sbtactical seat its card lookup over the game's corpus (the
// registry is loaded after the specs are parsed).
func (p *policySpec) setLookup(reg *cards.Registry) {
	if p.Kind == "sbtactical" {
		p.lookup = builtins.NewRegistryLookup(reg)
	}
}

// flagKeys are the spec keys that take no value: present means on.
var flagKeys = map[string]bool{
	"explore": true, "nonoise": true, "autopay": true, "mull": true, "keep7": true,
	"oppnodes": true, "oppfull": true, "closedlist": true, "morekinds": true,
}

func parsePolicy(s string) (*policySpec, error) {
	p, err := lexPolicy(s)
	if err != nil {
		return nil, err
	}
	if err := p.load(s); err != nil {
		return nil, err
	}
	return p, nil
}

// newPolicy is a spec of kind with every key at its default.
func newPolicy(raw, kind string) (*policySpec, error) {
	p := &policySpec{Raw: raw, Kind: kind, Sims: azmcts.DefaultOptions().Sims, CPUCT: -1, FPU: -1, Cands: -1}
	switch p.Kind {
	case "random", "bot", "sbtactical", "az", "prior":
	default:
		return nil, fmt.Errorf("policy %q: want random, bot, sbtactical, az or prior", raw)
	}
	return p, nil
}

// lexPolicy reads a spec string's kind and keys and runs check: everything
// but loading a network and building a test seat (load).
func lexPolicy(s string) (*policySpec, error) {
	// remote=unix:/path and remote=tcp:host:port contain colons: take the rest of the spec.
	rest := ""
	if i := strings.Index(s, ":remote="); i >= 0 {
		s, rest = s[:i], s[i+len(":remote="):]
	}
	parts := strings.Split(s, ":")
	p, err := newPolicy(s, parts[0])
	if err != nil {
		return nil, err
	}
	for _, kv := range parts[1:] {
		k, v, hasValue := strings.Cut(kv, "=")
		if hasValue && flagKeys[k] {
			// A flag key reads no value: autopay=0 would otherwise turn autopay ON.
			return nil, fmt.Errorf("policy %q: %s is a flag and takes no value (write :%s, or leave it out)", s, kv, k)
		}
		if k == "remote" {
			// Only a bare ":remote" reaches here (":remote=" took the rest above).
			return nil, fmt.Errorf("policy %q: unknown key %q", s, k)
		}
		if err := p.setKey(s, k, v); err != nil {
			return nil, err
		}
	}
	if rest != "" {
		p.Raw, p.Remote = s+":remote="+rest, rest
	}
	if err := p.check(p.Raw); err != nil {
		return nil, err
	}
	return p, nil
}

// setKey applies one key of a spec (label names the spec in an error). The
// string form and the object form (policy_object.go) both set every key here,
// so the two cannot drift apart.
func (p *policySpec) setKey(label, k, v string) error {
	var err error
	switch k {
	case "sims":
		p.Sims, err = strconv.Atoi(v)
	case "net":
		p.Net = v
	case "gonet":
		p.GoNet = v
	case "remote":
		p.Remote = v
	case "leaf":
		if v != "heuristic" && v != "net" {
			return fmt.Errorf("policy %q: leaf=%s, want heuristic or net", label, v)
		}
		p.HeurLeaf = v == "heuristic"
	case "prior":
		if v != "uniform" && v != "net" {
			return fmt.Errorf("policy %q: prior=%s, want uniform or net", label, v)
		}
		p.UniPrior = v == "uniform"
	case "worlds":
		p.Worlds, err = strconv.Atoi(v)
	case "explore":
		p.Explore = true
	case "nonoise":
		p.NoNoise = true
	case "exploreturns":
		p.ExploreTurns, err = strconv.Atoi(v)
		if err == nil && p.ExploreTurns < 0 {
			err = fmt.Errorf("want >= 0")
		}
	case "temp":
		p.Temp, err = strconv.ParseFloat(v, 64)
		if err == nil && !(p.Temp > 0) {
			err = fmt.Errorf("want > 0")
		}
	case "second":
		p.Second, err = strconv.ParseFloat(v, 64)
		if err == nil && !(p.Second > 0 && p.Second <= 1) {
			err = fmt.Errorf("want in (0,1]")
		}
	case "aseed":
		p.ASeed, err = strconv.ParseUint(v, 10, 64)
	case "cpuct":
		p.CPUCT, err = strconv.ParseFloat(v, 64)
	case "fpu":
		p.FPU, err = strconv.ParseFloat(v, 64)
	case "cands":
		p.Cands, err = strconv.Atoi(v)
	case "autopay":
		p.AutoPay = true
	case "mull":
		p.Mull = botpolicy.MulliganLands
	case "keep7":
		p.Mull = botpolicy.MulliganNever
	case "topk":
		p.TopK, err = strconv.Atoi(v)
	case "oppnodes":
		p.OppNodes = true
	case "oppfull":
		p.OppFull = true
	case "closedlist":
		p.Closed = true
	case "morekinds":
		p.Kinds = azmcts.FullKinds()
	case "kinds":
		p.Kinds, err = azmcts.ParseKinds(v)
	case "skippass":
		p.SkipPass = 1
		if v != "" {
			p.SkipPass, err = strconv.Atoi(v)
		}
	default:
		return fmt.Errorf("policy %q: unknown key %q", label, k)
	}
	if err != nil {
		return fmt.Errorf("policy %q: %s: %w", label, k, err)
	}
	return nil
}

// check is the spec's cross-key rules (no file is read).
func (p *policySpec) check(label string) error {
	if p.Kind == "prior" && p.Net == "" && p.Remote == "" && p.GoNet == "" {
		return fmt.Errorf("policy %q: prior needs net=, remote= or gonet=", label)
	}
	if nets := btoi(p.Net != "") + btoi(p.Remote != "") + btoi(p.GoNet != ""); nets > 1 {
		return fmt.Errorf("policy %q: net=, remote= and gonet= are exclusive", label)
	}
	if (p.ExploreTurns != 0 || p.Temp != 0 || p.Second != 0 || p.ASeed != 0) && p.Kind != "az" {
		return fmt.Errorf("policy %q: exploreturns, temp, second and aseed are az keys", label)
	}
	if p.Closed && p.Kind != "az" {
		return fmt.Errorf("policy %q: closedlist is an az key", label)
	}
	return nil
}

// load loads the spec's network (net=, gonet= or remote=) and, for a search
// seat, builds one seat to validate the configuration.
func (p *policySpec) load(label string) error {
	if p.GoNet != "" {
		m, err := gonet.Model(p.GoNet)
		if err != nil {
			return fmt.Errorf("policy %q: %w", label, err)
		}
		p.model = m
		if g, err := gonet.Shared(p.GoNet); err == nil {
			warnUncheckedNet(p.GoNet, g)
		}
	}
	if p.Remote != "" {
		m, err := remoteModel(p.Remote)
		if err != nil {
			return fmt.Errorf("policy %q: %w", label, err)
		}
		p.model = m
	}
	if p.Net != "" {
		m, err := policynet.LoadCheckpointFile(p.Net)
		if err != nil {
			return fmt.Errorf("policy %q: %w", label, err)
		}
		p.model = m
	}
	if p.Kind == "az" || p.Kind == "prior" {
		// Validated open: the closed world's belief is loaded later (setBelief).
		q := *p
		q.Closed = false
		if _, err := q.seat(1); err != nil {
			return fmt.Errorf("policy %q: %w", label, err)
		}
	}
	return nil
}

// warnedNets are the gonet= paths warnUncheckedNet already warned about.
var warnedNets sync.Map

// warnUncheckedNet prints one stderr warning per path for a .dzgw whose header
// carries no encoder_hash: it loaded, but nothing proved it was trained on this
// build's entity encoding (contract dzgw, docs/contracts.md).
func warnUncheckedNet(path string, g *gonet.Net) {
	if g.EncoderHashChecked() {
		return
	}
	if _, dup := warnedNets.LoadOrStore(path, true); dup {
		return
	}
	fmt.Fprintf(os.Stderr, "dzgorge: warning: %s has no encoder_hash in its header; it is assumed to match this build's entity encoder %016x (re-export it with the field to have that checked)\n",
		path, policynet.EncoderHashFor(policynet.FeaturesEntity))
}

// recordFeatures is the encoding visit corpora are written in (play -record-features).
var recordFeatures = policynet.FeaturesMZ

// recordEvery keeps one searched decision in K in the corpus (play -record-every).
var recordEvery = 1

func (p *policySpec) searches() bool { return p.Kind == "az" || p.Kind == "prior" }

func (p *policySpec) azConfig() azmcts.SeatConfig {
	cfg := azmcts.DefaultSeatConfig()
	cfg.Search.Sims = p.Sims
	cfg.Search.Kinds = azmcts.AllKinds()
	if p.Kinds != (azmcts.Kinds{}) {
		cfg.Search.Kinds = p.Kinds
	}
	cfg.Search.HeuristicLeaf = p.HeurLeaf
	cfg.Search.UniformPrior = p.UniPrior
	if p.CPUCT >= 0 {
		cfg.Search.CPUCT = p.CPUCT
	}
	if p.FPU >= 0 {
		cfg.Search.FPU = p.FPU
	}
	if p.Cands > 0 {
		cfg.Search.Limit = p.Cands
	}
	cfg.Search.AutoPayment = p.AutoPay
	cfg.Mulligan = p.Mull
	cfg.Search.PriorTopK = p.TopK
	cfg.Search.OpponentNodes = p.OppNodes
	cfg.Search.SkipPass = p.SkipPass
	cfg.Search.OpponentMirror = p.OppFull
	// Honest worlds only: the seat never searches the real engine's hidden zones. The
	// prior-only student asks for no world at all; redeal just satisfies NewSeat.
	cfg.World = azmcts.WorldRedeal
	if p.Closed {
		cfg.World, cfg.Belief = azmcts.WorldRedealClosed, p.belief
	}
	cfg.Worlds = p.Worlds
	cfg.Explore, cfg.NoNoise = p.Explore, p.NoNoise
	if p.ExploreTurns > 0 {
		cfg.ExploreTurns = int32(p.ExploreTurns)
	}
	cfg.Search.Temp, cfg.Search.Second, cfg.Search.ChoiceSalt = p.Temp, p.Second, p.ASeed
	cfg.PriorOnly = p.Kind == "prior"
	cfg.RecordFeatures = recordFeatures
	return cfg
}

// seat builds one seat of one game from its per-seat seed.
func (p *policySpec) seat(seed uint64) (seat.Seat, error) {
	switch p.Kind {
	case "random":
		return builtins.New(builtins.Uniform, builtins.AutoPay, seed^builtins.UniformSeed), nil
	case "bot":
		b := seat.NewBot(seed)
		if p.AutoPay {
			b = b.EnableAutoPayMana()
		}
		if p.Mull != botpolicy.MulliganCoin {
			b = b.WithMulligan(p.Mull)
		}
		return b, nil
	case "sbtactical":
		t := builtins.NewTactical(builtins.AutoPay, seed, p.lookup, sbtactical.Hosted())
		t.SetMulligan(p.Mull)
		return t, nil
	default:
		return azmcts.NewSeat(seed, p.model, p.azConfig())
	}
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}
