package kop

// ExtStackExileMark: an Effect's "if this spell would be put into a graveyard
// from the stack, exile it instead" replacement over one named spell (gorge
// effects/effect.go registers a live Event$ Moved | Origin$ Stack |
// Destination$ Graveyard | ValidCard$ Card.IsRemembered replacement whose body
// is a ChangeZone of the ReplacedCard to exile, with ExileOnMoved$ Stack
// ending the Effect as the spell leaves the stack). Lilah, Undefeated
// Slickshot (the body also makes the card plotted), Goliath Daydreamer (a
// dream counter, which the kernel cannot hold: it stalls) and Nita, Forum
// Conciliator (the exiled card, cast later this turn).
//
// C is the variant (StackExile* below). Two forms:
//   - the SpellCast trigger form (StackExileSpell): the triggering spell, if
//     still on the stack (and, with StackExileOwn, owned by the resolving
//     controller), is marked; every graveyard exit of its entry (resolved,
//     fizzled, countered) goes to exile instead.
//   - the grant form (StackExileGrant): each remembered card in exile gets the
//     mark on its may-play permission record, so the spell cast through that
//     permission is marked as it is cast.
//
// dirac/repl_stackexile.go.
const ExtStackExileMark ExtOp = 315

func init() { extOpNames[ExtStackExileMark] = "StackExileMark" }

// ExtStackExileMark's C bits.
const (
	StackExilePlot     = 1 << iota // the exiled card becomes plotted (AlterAttribute Plotted)
	StackExileDream                // the exiled card gets a dream counter (WithCountersType$ DREAM): a stall
	StackExileGrant                // the grant form: marks remembered cards' permission records
	StackExileOwn                  // the mark needs the spell's owner to be the controller (ValidCard$ ...+YouOwn)
	StackExileResolved             // the replacement is Fizzle$ False: only the spell's resolution move is exiled, not a fizzle or a counter
)
