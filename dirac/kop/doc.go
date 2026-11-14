// Package kop is dirac's card-effect vocabulary: the op enum
// generated and hand-written card tables target, and the Table / CardDef /
// Ability / Instr types they are written in.
//
// Interpreter contract (implemented by package dirac):
//
//   - A resolving spell or ability runs its Ability.Prog front to back. Each
//     Instr is executed against the stack entry's source, controller, chosen
//     targets, chosen modes, X and kicked flag.
//   - Obj refs to a target that has become illegal (left its zone, no longer
//     matches its filter, gained hexproof/protection) make that Instr a no-op;
//     if EVERY target of a targeted ability is illegal the whole program is
//     skipped (CR 608.2b fizzle).
//   - Branch ops (OpCharm, OpIfKicked, OpSkipIf, OpRepeat, OpRepeatEach,
//     OpGenericChoice) are structured: their skip/loop counts name the
//     instructions immediately following them.
//   - Decision-raising ops (OpChooseColor, OpChooseType, OpChooseCard,
//     OpGenericChoice, OpDiscard with a player chooser, OpScry, OpSurveil,
//     OpSacrifice, OpDig, OpSearch) suspend resolution; the kernel poses a
//     Decision and resumes at the same Instr when it is answered.
//   - An op the interpreter does not implement panics naming the op; it is
//     never a silent no-op.
//
// A constant's value never changes (testdata/ordinals.tsv records each). The
// iota lists are frozen: each ends at a fixed sentinel (OpCount,
// TrigEventCount, ReplCount, StatCount, ExCount_, CondCount_; the counter
// kinds at CtrM2M1, with NumCounterKinds3 their fixed cover), and a new code
// of a typed family is an entry of dirac's namespace manifest
// (dirac/manifest/<Family>/<Name>), which kmanifest generates into
// manifest_gen.go. Most vocabulary is pre-mapped there: activate the entry
// (status: active) rather than adding a constant. An op's own untyped operand
// codes stay hand-written in its file.
//
// # Pool tables
//
// A Table may hold a whole card pool rather than two decks (the closed-list
// belief deals any card of the pool's lists into a search world): CardID is
// 16 bits (NoCard 0xFFFF), subtype sets are SubMasks of up to MaxSubtypes,
// filter subtype indexes stay a uint8 index+1. The FDN pool table
// (cardset.Set.Pool) puts pool card i at CardID Set.PoolID(i), from the
// checked-in dirac/cardset/pools/fdn/ids.tsv so a CardID never moves (i itself, the
// Lookup index, until the pool changes; cards sorted by name, then tokens;
// a card that left the pool keeps its CardID as an inert placeholder); a card the generator does not
// support is an inert fallback (printed cost, types, subtypes, colours and
// P/T; no abilities, keywords or costs beyond the mana cost), which the
// kernel plays as a vanilla card.
package kop
