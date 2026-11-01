package env

import (
	"encoding/hex"
	"regexp"
	"runtime/debug"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/wire"
)

const AdapterVersion = "0.1.0"

// DecisionKinds are the 25 kinds the engine emits. distribute is not one:
// combat damage follows the declared engine_order default (Section 7.6).
// choose_option is the two-pile pick (a counted option list); it joins the
// advertised set because the mapping poses it.
var DecisionKinds = []string{"pass", "play_land", "cast_spell", "activate_mana_ability", "activate_ability",
	"special_action", "choose_target", "finish_target_selection", "choose_cost_target", "choose_spell_mode",
	"choose_color", "choose_number", "choose_boolean", "choose_name", "select_object", "finish_selection",
	"optional_cost", "optional_cast", "mulligan", "order_pick", "arrange_card", "choose_replacement",
	"declare_attack", "declare_block", "choose_option"}

// corpusRef is the Forge corpus ref this process was started against, set by
// the command from cards.lock. It is part of the engine identity, not a pin:
// a different corpus is simply a different identity.
var corpusRef = "unknown"

// SetCorpusRef records the Forge ref the running corpus was fetched at.
func SetCorpusRef(ref string) {
	if ref != "" {
		corpusRef = ref
	}
}

// gorgeVersion reports the running gorge build: the module version, or the
// VCS revision when built from a checkout.
func gorgeVersion() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		v := bi.Main.Version
		if v != "" && v != "(devel)" {
			return v
		}
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" && s.Value != "" {
				return s.Value
			}
		}
	}
	return "devel"
}

// pseudoVersion matches what ends a Go pseudo-version: its timestamp, its
// commit and any build metadata (v0.0.0-20261031120000-96610c40a1f8+dirty).
var pseudoVersion = regexp.MustCompile(`[-.][0-9]{14}-([0-9a-f]{12})(\+[0-9A-Za-z.-]+)?$`)

// short names a revision for an identity string: a pseudo-version by its commit,
// keeping the +dirty of a build from a modified tree; a tagged version whole;
// anything else, a commit hash, by its first 12 characters.
func short(s string) string {
	if m := pseudoVersion.FindStringSubmatch(s); m != nil {
		return m[1] + m[2]
	}
	if len(s) > 12 && !strings.HasPrefix(s, "v") {
		return s[:12]
	}
	return s
}

func engineIdentity(sourceRevision *string, reg *cards.Registry) protocol.Engine {
	var ids []string
	for _, d := range catalog.Decks() {
		ids = append(ids, d.DeckID())
	}
	catalogID := wire.DomainID(ids) // the five catalog deck ids, distinct by construction
	corpus := "unknown"
	if reg != nil {
		if c := reg.Catalog(); c != nil {
			corpus = hex.EncodeToString(c.Identity.CorpusHash[:])[:16]
		}
	}
	version := gorgeVersion()
	return protocol.Engine{Name: "gorge", Version: "gorge-" + short(version) + "/spellbench-adapter-" + AdapterVersion,
		SourceRevision:   sourceRevision,
		RulesSnapshotID:  "gorge/" + short(version) + "/ir-" + cards.CompilerFingerprint,
		CardPoolIdentity: "forge-" + short(corpusRef) + "/corpus-" + corpus + "/catalog-" + catalogID[7:23]}
}

func provenance(e protocol.Engine) protocol.Provenance {
	return protocol.Provenance{EngineName: e.Name, EngineVersion: e.Version, RulesSnapshotID: e.RulesSnapshotID, CardPoolIdentity: e.CardPoolIdentity}
}

// engineOrder is the declared combat damage default (Section 7.6): gorge
// assigns combat damage in engine order (Task 16).
var engineOrder = "engine_order"

func helloOK(id string, e protocol.Engine, flags map[string]bool, autoPay bool) protocol.HelloOK {
	var cat []protocol.CatalogDeck
	for _, d := range catalog.Decks() {
		cd := protocol.CatalogDeck{CatalogID: d.CatalogID, Name: d.Name}
		for _, r := range d.Rows {
			cd.Decklist = append(cd.Decklist, protocol.DeckRow{Name: r.Name, Count: uint32(r.Count)})
		}
		cat = append(cat, cd)
	}
	var manaPayment *string
	if autoPay {
		m := "engine_autopay"
		manaPayment = &m
	}
	return protocol.HelloOK{ResponseType: "hello_ok", Protocol: protocol.Name, RequestID: id, ProtocolMinor: 0, Engine: e,
		Formats: []string{"pauper-bo1"}, DeckSources: []string{"catalog"}, Catalog: cat,
		RulesSupported: map[string][]string{"mulligan": {"london", "none"}, "starting_player": {"host_assigned"}},
		Observation:    flags, DecisionKinds: DecisionKinds,
		EngineDefaults: map[string]*string{"trigger_order": nil, "replacement_order": nil, "combat_damage_assignment": &engineOrder, "mana_payment": manaPayment},
		Rewind:         false, Fairness: map[string]bool{"noninterference_probe": false},
		Extensions: []protocol.Extension{{Name: "x_gorge_view_v1", NativeIDs: false}}}
}

// EngineIdentity exports the identity the v2 engine role advertises, for a
// seat adapter that must send the same engine block to an agent.
func EngineIdentity(sourceRevision *string, reg *cards.Registry) protocol.Engine {
	return engineIdentity(sourceRevision, reg)
}
