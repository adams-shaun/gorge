package scriptfacts

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// Heads is the set of PRINTED keyword heads a face carries, over the
// heads the offer walk's hand loop reads off a card's front face through
// cards.Face.KeywordParam (the keyword-cost offer family: kicker, surge,
// entwine, replicate, ... and the face-down cast family). Each of those
// readers answers "absent" -- and prices nothing -- exactly when no entry
// of f.Keywords has a KeywordHead that strings.EqualFold-matches its head,
// so a clear bit lets the walk skip the reader (a keyword lookup and a
// zeroed Cost return per head, per hand card, per walk) without changing
// what it offers. A set bit only means "ask the reader", which still decides.
type Heads uint32

const (
	Kicker Heads = 1 << iota
	Surge
	Entwine
	Replicate
	Multikicker
	Squad
	Evoke
	Dash
	Overload
	Warp
	Emerge
	Bestow
	Mutate
	Buyback
	Suspend
	Plot
	Morph
	Megamorph
	Disguise
	MayFlashCost
	AlternateAdditionalCost
	Impending
	Prowl
	Freerunning
	Awaken
	Spectacle
	Offering
	// All answers "maybe" for every head.
	All Heads = 1<<iota - 1
)

// Has reports whether any head in bits may be printed.
func (h Heads) Has(bits Heads) bool { return h&bits != 0 }

// Of returns f's printed Heads. The fold is exact: for an
// all-ASCII head, strings.EqualFold against an ASCII name is the ASCII
// lower-case comparison below; a head carrying any non-ASCII byte could
// match through Unicode simple folding (the KELVIN SIGN folds to k), so it
// answers All and every reader is asked.
func Of(f *cards.Face) Heads {
	var m Heads
	for _, k := range f.Keywords {
		h := cards.KeywordHead(k)
		var buf [len("alternateadditionalcost")]byte
		ascii := true
		for i := 0; i < len(h); i++ {
			if h[i] >= 0x80 {
				ascii = false
				break
			}
		}
		if !ascii {
			return All
		}
		if len(h) > len(buf) {
			continue // longer than every head: no ASCII head can match
		}
		for i := 0; i < len(h); i++ {
			c := h[i]
			if 'A' <= c && c <= 'Z' {
				c += 'a' - 'A'
			}
			buf[i] = c
		}
		switch headsOfCodes.Code(string(string(buf[:len(h)]))) {
		case headOfKicker:
			m |= Kicker
		case headOfSurge:
			m |= Surge
		case headOfEntwine:
			m |= Entwine
		case headOfReplicate:
			m |= Replicate
		case headOfMultikicker:
			m |= Multikicker
		case headOfSquad:
			m |= Squad
		case headOfEvoke:
			m |= Evoke
		case headOfDash:
			m |= Dash
		case headOfOverload:
			m |= Overload
		case headOfWarp:
			m |= Warp
		case headOfEmerge:
			m |= Emerge
		case headOfBestow:
			m |= Bestow
		case headOfMutate:
			m |= Mutate
		case headOfBuyback:
			m |= Buyback
		case headOfSuspend:
			m |= Suspend
		case headOfPlot:
			m |= Plot
		case headOfMorph:
			m |= Morph
		case headOfMegamorph:
			m |= Megamorph
		case headOfDisguise:
			m |= Disguise
		case headOfMayflashcost:
			m |= MayFlashCost
		case headOfAlternateAdditionalCost:
			m |= AlternateAdditionalCost
		case headOfImpending:
			m |= Impending
		case headOfProwl:
			m |= Prowl
		case headOfFreerunning:
			m |= Freerunning
		case headOfAwaken:
			m |= Awaken
		case headOfSpectacle:
			m |= Spectacle
		case headOfOffering:
			m |= Offering
		}
	}
	return m
}

// Names maps each bit to the head its reader asks KeywordParam
// for (the verify check and the unit test walk it).
var Names = [...]struct {
	Bit  Heads
	Head string
}{
	{Kicker, "Kicker"}, {Surge, "Surge"}, {Entwine, "Entwine"}, {Replicate, "Replicate"},
	{Multikicker, "Multikicker"}, {Squad, "Squad"}, {Evoke, "Evoke"}, {Dash, "Dash"},
	{Overload, "Overload"}, {Warp, "Warp"}, {Emerge, "Emerge"}, {Bestow, "Bestow"},
	{Mutate, "Mutate"}, {Buyback, "Buyback"}, {Suspend, "Suspend"}, {Plot, "Plot"},
	{Morph, "Morph"}, {Megamorph, "Megamorph"}, {Disguise, "Disguise"},
	{MayFlashCost, "MayFlashCost"}, {AlternateAdditionalCost, "AlternateAdditionalCost"}, {Impending, "Impending"},
	{Prowl, "Prowl"}, {Freerunning, "Freerunning"}, {Awaken, "Awaken"},
	{Spectacle, spectacleHead}, {Offering, "Offering"},
}

const spectacleHead = "Spectacle"

// lowerASCII lower-cases an ASCII head name (no strings.* call: the strings
// ratchet counts them).
func lowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

// Verify panics when a clear bit of h is contradicted by f.KeywordParam
// (derivedMemoVerify mode: the offer walk checks every hand card it
// summarises).
func Verify(f *cards.Face, h Heads) {
	for _, n := range Names {
		if _, ok := f.KeywordParam(n.Head); ok && !h.Has(n.Bit) {
			panic("rules: printed keyword summary of " + f.Name + " misses " + n.Head)
		}
	}
}

type headCode uint16

const (
	headOfKicker headCode = iota + 1
	headOfSurge
	headOfEntwine
	headOfReplicate
	headOfMultikicker
	headOfSquad
	headOfEvoke
	headOfDash
	headOfOverload
	headOfWarp
	headOfEmerge
	headOfBestow
	headOfMutate
	headOfBuyback
	headOfSuspend
	headOfPlot
	headOfMorph
	headOfMegamorph
	headOfDisguise
	headOfMayflashcost
	headOfAlternateAdditionalCost
	headOfImpending
	headOfProwl
	headOfFreerunning
	headOfAwaken
	headOfSpectacle
	headOfOffering
)

var headsOfCodes = state.NewStrCodes(
	state.StrEntry[headCode]{Key: "kicker", Val: headOfKicker},
	state.StrEntry[headCode]{Key: "surge", Val: headOfSurge},
	state.StrEntry[headCode]{Key: "entwine", Val: headOfEntwine},
	state.StrEntry[headCode]{Key: "replicate", Val: headOfReplicate},
	state.StrEntry[headCode]{Key: "multikicker", Val: headOfMultikicker},
	state.StrEntry[headCode]{Key: "squad", Val: headOfSquad},
	state.StrEntry[headCode]{Key: "evoke", Val: headOfEvoke},
	state.StrEntry[headCode]{Key: "dash", Val: headOfDash},
	state.StrEntry[headCode]{Key: "overload", Val: headOfOverload},
	state.StrEntry[headCode]{Key: "warp", Val: headOfWarp},
	state.StrEntry[headCode]{Key: "emerge", Val: headOfEmerge},
	state.StrEntry[headCode]{Key: "bestow", Val: headOfBestow},
	state.StrEntry[headCode]{Key: "mutate", Val: headOfMutate},
	state.StrEntry[headCode]{Key: "buyback", Val: headOfBuyback},
	state.StrEntry[headCode]{Key: "suspend", Val: headOfSuspend},
	state.StrEntry[headCode]{Key: "plot", Val: headOfPlot},
	state.StrEntry[headCode]{Key: "morph", Val: headOfMorph},
	state.StrEntry[headCode]{Key: "megamorph", Val: headOfMegamorph},
	state.StrEntry[headCode]{Key: "disguise", Val: headOfDisguise},
	state.StrEntry[headCode]{Key: "mayflashcost", Val: headOfMayflashcost},
	state.StrEntry[headCode]{Key: "alternateadditionalcost", Val: headOfAlternateAdditionalCost},
	state.StrEntry[headCode]{Key: "impending", Val: headOfImpending},
	state.StrEntry[headCode]{Key: "prowl", Val: headOfProwl},
	state.StrEntry[headCode]{Key: "freerunning", Val: headOfFreerunning},
	state.StrEntry[headCode]{Key: "awaken", Val: headOfAwaken},
	// Lower-cased from the head name: rules/altcast_modes.go owns the
	// lower-case "spectacle" word (TestAltCastModeLiteralsLiveInTheDescriptor).
	state.StrEntry[headCode]{Key: lowerASCII(spectacleHead), Val: headOfSpectacle},
	state.StrEntry[headCode]{Key: "offering", Val: headOfOffering},
)
