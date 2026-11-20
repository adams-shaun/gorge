package kop

// Filter programs. A Filter with Pred == PredProg reads PredN as a
// FilterProgID: the filter matches an object when its struct fields (types,
// colours, controller, zone, flags, the Or chain) hold, which are the
// program's first conjuncts, and every literal of Table.FilterProgs[PredN]
// holds. The Filter stays one struct, so a clause with more conjuncts than the
// struct has slots (power >= 2 and mana value <= 3) spills into a program
// instead of growing it.

// FilterProgID indexes Table.FilterProgs; 0 is no program.
type FilterProgID = int16

// FilterWord names the test one literal of a filter program applies. The
// codes are the manifest's FilterWord family (dirac/manifest/FilterWord, Fw*);
// 0 is no word. Each word has one kernel handler (//kmanifest:fword, one file
// dirac/filter_<name>.go) and declares the matcher kinds it supports in its
// manifest entry's contexts line, which kgen holds a lowering to.
type FilterWord uint16

// FilterProg is a conjunction of literals, evaluated in order and stopping at
// the first that fails.
type FilterProg struct {
	Lits []FLit
}

// FLit is one literal of a FilterProg: the word, whether its result is
// inverted, and the word's operands. Which operands a word reads is its own
// manifest doc; a word reading an operand its literal leaves zero is
// authored wrong, and the kernel evaluates it as the zero operand says.
type FLit struct {
	W   FilterWord
	Neg bool // the literal holds when the word does not
	// Op is a comparison word's operator (CmpGE, CmpLE, CmpEQ, ...) against V.
	Op CmpOp
	// Arg is the word's integer argument: a counter kind, a keyword bit or a
	// subtype index.
	Arg int16
	// V is the value operand (a comparison's bound).
	V ValueRef
	// R is a reference operand (a word reading a referent, such as
	// sharesNameWith Targeted).
	R Ref
	// F is a nested filter (sharesCreatureTypeWith Valid ...), one level.
	F FilterID
}
