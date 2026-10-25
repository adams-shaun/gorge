package events

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// CopyExceptions is a CopyExcept event's payload: the FINAL values a copy
// effect's exceptions give the copy (CR 707.9a/b), settled by the emitter
// (engine/effects/copy_except.go) against the copied face. The fold writes
// them onto the copy's layer-1 basis face, so they are the copy's copiable
// values and a later copy of the copy inherits them (CR 707.3). It rides the
// event's Text as JSON (Text/ParseCopyExceptions); encoding/json writes
// struct fields in declaration order, so the bytes are deterministic.
type CopyExceptions struct {
	// Types, when TypesSet, is the copy's whole type line.
	Types    []string `json:"types,omitempty"`
	TypesSet bool     `json:"types_set,omitempty"`
	// Colors, when ColorsSet, is the copy's colour set (cards.Colour* bits;
	// 0 is colourless).
	Colors    uint8 `json:"colors,omitempty"`
	ColorsSet bool  `json:"colors_set,omitempty"`
	// Power/Toughness, when PTSet, are the copy's P/T.
	Power     int32 `json:"power,omitempty"`
	Toughness int32 `json:"toughness,omitempty"`
	PTSet     bool  `json:"pt_set,omitempty"`
	// NoManaCost is "it has no mana cost" (Embalm, Eternalize: RemoveCost$).
	NoManaCost bool `json:"no_mana_cost,omitempty"`
	// Keywords are keyword lines the copy has (CR 707.9a), in grant order.
	Keywords []string `json:"keywords,omitempty"`
	// SVars are {name, body} pairs merged into the copy's SVar table, in
	// grant order, BEFORE Triggers are linked against it.
	SVars [][2]string `json:"svars,omitempty"`
	// Triggers are raw trigger bodies the copy has (CR 707.9a), in grant
	// order; each Execute$ is resolved against the merged SVar table.
	Triggers []string `json:"triggers,omitempty"`
}

// Empty reports whether x carries no exception.
func (x CopyExceptions) Empty() bool {
	return !x.TypesSet && !x.ColorsSet && !x.PTSet && !x.NoManaCost &&
		len(x.Keywords) == 0 && len(x.SVars) == 0 && len(x.Triggers) == 0
}

// Text is the CopyExcept event's Text for x.
func (x CopyExceptions) Text() string {
	b, _ := json.Marshal(x) // a plain struct of strings, ints and bools: never fails
	return string(b)
}

// ParseCopyExceptions reads Text's encoding. A malformed payload fails
// closed: ok is false and the fold applies nothing.
func ParseCopyExceptions(s string) (x CopyExceptions, ok bool) {
	if json.Unmarshal([]byte(s), &x) != nil {
		return CopyExceptions{}, false
	}
	return x, true
}

// foldTextChange folds Kind TextChange (CR 612): the substitution Counter ->
// Text joins Obj's text changes in event (timestamp) order.
func foldTextChange(g *state.Game, e *Event) {
	o := g.Obj(e.Obj)
	if o == nil {
		return
	}
	if e.Amount == TextChangeExpire {
		// CR 514.2: the cleanup step ends Obj's until-end-of-turn changes.
		kept := make([]cards.TextSub, 0, len(o.TextChanges))
		for _, s := range o.TextChanges {
			if !s.UntilEOT {
				kept = append(kept, s)
			}
		}
		if len(kept) == 0 {
			kept = nil
		}
		o.SetTextChanges(kept)
		return
	}
	if e.Counter == "" || e.Text == "" {
		return
	}
	subs := append(append([]cards.TextSub(nil), o.TextChanges...),
		cards.TextSub{From: e.Counter, To: e.Text, UntilEOT: e.Amount == TextChangeUntilEOT})
	o.SetTextChanges(subs)
}

// TextChange's Amount values: 0 an indefinite change, TextChangeUntilEOT a
// change that ends in the cleanup step, TextChangeExpire the cleanup-step
// record that ends Obj's until-end-of-turn changes (Counter/Text empty).
const (
	TextChangeUntilEOT int32 = 1
	TextChangeExpire   int32 = -1
)

// foldCopyExcept folds Kind CopyExcept into state. The copy's current face
// (its CopyFace basis, or a just-minted token copy's printed face) is
// value-copied and edited -- never mutated in place, since it may be the
// shared corpus face or another copy's basis (CR 707.2b) -- and installed
// as Obj's CopyFace. No derive() runs (the cards setters re-parse only the
// field they write), so a replay rebuilds a FaceEqual twin.
func foldCopyExcept(g *state.Game, e *Event) {
	o := g.Obj(e.Obj)
	if o == nil || o.Face() == nil {
		return
	}
	x, ok := ParseCopyExceptions(e.Text)
	if !ok {
		return
	}
	f := *o.Face()
	// The edited face is no longer the catalog row it may be bound to (a
	// compiled TypeMask or trigger-interest prefilter would answer for the
	// unedited text), so it takes the computed paths a token face takes.
	f.ForgetCompiled()
	if x.TypesSet {
		f.Types = append([]string(nil), x.Types...)
	}
	if x.ColorsSet {
		f.SetCopyColours(x.Colors)
	}
	if x.NoManaCost {
		f.SetManaCost("")
		f.SetCopyNoManaCost()
	}
	if x.PTSet {
		f.SetPT(strconv.Itoa(int(x.Power)) + "/" + strconv.Itoa(int(x.Toughness)))
		f.SetCopyPTSet()
		// CR 707.9d: a copy effect that provides a specific P/T does not copy
		// a characteristic-defining ability that defines P/T (Quicksilver
		// Gargantuan copying Tarmogoyf is 7/7).
		kept := make([]cards.Static, 0, len(f.Statics))
		for _, s := range f.Statics {
			_, setP := s.Param(cards.PKSetPower)
			_, setT := s.Param(cards.PKSetToughness)
			if s.ParamStr(cards.PKCharacteristicDefining) == "True" && (setP || setT) {
				continue
			}
			kept = append(kept, s)
		}
		f.Statics = kept
	}
	if len(x.Keywords) > 0 || len(x.SVars) > 0 || len(x.Triggers) > 0 {
		// Deep-copy every container a keyword expansion appends to or writes
		// (the ClonePermanent "ifnew" discipline): f aliases the source face.
		svars := make(map[string]string, len(f.SVars))
		for k, v := range f.SVars {
			svars[k] = v
		}
		f.SVars = svars
		f.Keywords = append([]string(nil), f.Keywords...)
		f.Abilities = append([]*cards.SA(nil), f.Abilities...)
		f.Triggers = append([]cards.Trigger(nil), f.Triggers...)
		f.Repls = append([]cards.Repl(nil), f.Repls...)
		f.Statics = append([]cards.Static(nil), f.Statics...)
		for _, sv := range x.SVars {
			f.SVars[sv[0]] = sv[1]
		}
		for _, k := range x.Keywords {
			if !f.ApplyKeywordExpansion(k) {
				f.Keywords = append(f.Keywords, k)
			}
		}
		for _, raw := range x.Triggers {
			if tr, ok := cards.ParseTriggerLine(raw); ok {
				if ex := strings.TrimSpace(tr.ParamStr(cards.PKExecute)); ex != "" {
					tr.Effect = cards.ResolveSVar(f.SVars, ex)
				}
				f.Triggers = append(f.Triggers, tr)
			}
		}
	}
	o.SetCopyFace(&f)
}
