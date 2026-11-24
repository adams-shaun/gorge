package mtgish

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// Filter lowerings: mtgish's group enums to *ir.Filter, plus the small enums
// the filters lean on (CheckHasable, Color, NameFilter).

// filtEnum is one group enum, with the Subject and Zones every node of its
// tree carries and the variants of the shared row table it has.
type filtEnum struct {
	name     string
	subj     ir.Subject
	zones    ir.ZoneSet
	variants string // space-separated variant names
}

func (e *filtEnum) has(v string) bool {
	for _, w := range strings.Fields(e.variants) {
		if w == v {
			return true
		}
	}
	return false
}

func (e *filtEnum) player() bool { return e.subj == ir.SubjPlayer }

// node returns an empty filter node of the enum's Subject and Zones.
func (e *filtEnum) node(op ir.FilterOp) *ir.Filter {
	return &ir.Filter{Subject: e.subj, Zones: e.zones, Op: op}
}

func (e *filtEnum) pred(p ir.Pred) *ir.Filter {
	f := e.node(ir.FPred)
	f.Pred = p
	return f
}

func (e *filtEnum) not(f *ir.Filter) *ir.Filter {
	n := e.node(ir.FNot)
	n.Args = []*ir.Filter{f}
	return n
}

func (e *filtEnum) wordPred(p ir.Pred, word string) *ir.Filter {
	f := e.pred(p)
	f.Word = word
	return f
}

func zoneBit(z ir.Zone) ir.ZoneSet { return 1 << z }

// filtEnums lists the group enums. Order is registration order only.
var filtEnums = []filtEnum{
	{"Permanents", ir.SubjPermanent, zoneBit(ir.ZoneBattlefield),
		"IsNonToken IsToken TheTokensCreatedThisWay And AnyPermanent ControlledByAPlayer DoesntHaveAbility " +
			"HasACounterOfType HasAbility HasNoCountersOfType HasNumberCountersOfType IsArtifactType IsAttacking " +
			"IsBlocking IsCardtype IsColor IsColorless IsCreatureType IsLandType IsNamed IsNonCardtype IsNonColor " +
			"IsNonCreatureType IsNonEnchantmentType IsNonSupertype IsPermanent IsSupertype IsTapped IsUntapped " +
			"ManaValueIs Or Other OwnedByAPlayer PowerIs Ref_TargetPermanents SinglePermanent ToughnessIs"},
	{"Spells", ir.SubjSpell, zoneBit(ir.ZoneStack),
		"And Or Other AnySpell DoesntHaveAbility HasAbility IsArtifactType IsCardtype IsColor IsColorless " +
			"IsCreatureType IsNamed IsNonCardtype IsNonColor IsNonCreatureType IsNonSupertype IsPermanent " +
			"IsSupertype ManaValueIs PowerIs ToughnessIs ControlledByAPlayer OwnedByAPlayer SingleSpell"},
	{"Players", ir.SubjPlayer, 0,
		"AnyPlayer And Or Other SinglePlayer HasACounterOfType HasNoCountersOfType " +
			// Players only.
			"Opponent AttackedThisTurn ControlsA ControlsMorePermanentThanPlayer LifeTotalIs LostLifeThisTurn " +
			"NumCardsInGraveyardIs NumCardsInHandIs"},
	{"Cards", ir.SubjCard, 0,
		"And Or Other AnyCard DoesntHaveAbility HasAbility IsArtifactType IsCardtype IsColor IsColorless " +
			"IsCreatureType IsLandType IsNamed IsNonCardtype IsNonColor IsNonCreatureType IsNonEnchantmentType " +
			"IsNonSupertype IsPermanent IsSupertype ManaValueIs PowerIs ToughnessIs OwnedByAPlayer ControlledByAPlayer"},
	{"CardsInGraveyards", ir.SubjCard, zoneBit(ir.ZoneGraveyard),
		"AnyCardInAnyGraveyard And Or Other DoesntHaveAbility HasAbility InAPlayersGraveyard IsArtifactType " +
			"IsCardtype IsColor IsColorless IsCreatureType IsLandType IsNamed IsNonCardtype IsNonCreatureType " +
			"IsNonEnchantmentType IsNonSupertype IsPermanent IsSupertype ManaValueIs Ref_TargetGraveyardCards " +
			"PowerIs ToughnessIs"},
	{"CardsInExile", ir.SubjCard, zoneBit(ir.ZoneExile),
		"And Or Other AnyCard IsNamed HasAbility DoesntHaveAbility IsColor ManaValueIs IsCardtype IsCreatureType " +
			"IsNonCardtype IsNonEnchantmentType IsPermanent IsSupertype OwnedByAPlayer HasACounterOfType " +
			"HasNoCountersOfType TheCardsExiledThisWay"},
	{"CardsInLibrary", ir.SubjCard, zoneBit(ir.ZoneLibrary),
		"OwnedByAPlayer IsColor IsNonCreatureType IsColorless PowerIs ToughnessIs And HasAbility IsArtifactType " +
			"IsCardtype IsCreatureType IsLandType IsNamed IsNonCardtype IsNonSupertype IsPermanent IsSupertype " +
			"ManaValueIs Or"},
	{"CardsInHand", ir.SubjCard, zoneBit(ir.ZoneHand),
		"And Or AnyCard Other OwnedByAPlayer DoesntHaveAbility IsColorless IsLandType IsColor IsArtifactType " +
			"IsSupertype HasAbility IsCardtype IsCreatureType IsNonCardtype IsPermanent IsNamed ManaValueIs " +
			"ToughnessIs PowerIs IsNonColor"},
}

// filtRow builds the filter of one variant of the enum e. v is the variant
// name and n the node.
type filtRow struct {
	variant string
	build   func(c *Ctx, e *filtEnum, v string, n node) (*ir.Filter, error)
}

// filtWord is the mtgish spelling of a unit-variant argument (a card type, a
// creature type, a supertype, ...): the gorge backend respells it.
func filtWord(x any) (string, error) {
	v := variantOf(x)
	if v == "" {
		return "", fmt.Errorf("cardc: expected a unit variant, got %T", x)
	}
	return v, nil
}

// filtStr lowers x with c.lower and wants a string (CheckHasable, NameFilter,
// CounterType).
func filtStr(c *Ctx, x any) (string, error) {
	v, err := c.lower(x)
	if err != nil {
		return "", err
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s lowers to %T, want string", qualified(x), v)
	}
	return s, nil
}

// filtColor lowers a Color node to its ir.ColorSet (returned by value).
func filtColor(c *Ctx, x any) (ir.ColorSet, error) {
	v, err := c.lower(x)
	if err != nil {
		return 0, err
	}
	cs, ok := v.(ir.ColorSet)
	if !ok {
		return 0, fmt.Errorf("%s lowers to %T, want ir.ColorSet", qualified(x), v)
	}
	return cs, nil
}

// filtCompare is a numeric predicate: PManaValue and friends.
func filtCompare(p ir.Pred) filtRow {
	return filtRow{build: func(c *Ctx, e *filtEnum, v string, n node) (*ir.Filter, error) {
		cmp, err := c.Compare(argN(n, 0))
		if err != nil {
			return nil, err
		}
		f := e.pred(p)
		f.Cmp, f.Value = cmp.Op, cmp.V
		return f, nil
	}}
}

// filtTypeWord is a pred on a unit-variant word, optionally negated.
func filtTypeWord(p ir.Pred, negate bool) filtRow {
	return filtRow{build: func(c *Ctx, e *filtEnum, v string, n node) (*ir.Filter, error) {
		w, err := filtWord(argN(n, 0))
		if err != nil {
			return nil, err
		}
		f := e.wordPred(p, w)
		if negate {
			f = e.not(f)
		}
		return f, nil
	}}
}

func filtPlain(p ir.Pred, negate bool) filtRow {
	return filtRow{build: func(c *Ctx, e *filtEnum, v string, n node) (*ir.Filter, error) {
		f := e.pred(p)
		if negate {
			f = e.not(f)
		}
		return f, nil
	}}
}

// filtCombine is And (FAll) or Or (FAny) over a list of the same enum.
func filtCombine(op ir.FilterOp) filtRow {
	return filtRow{build: func(c *Ctx, e *filtEnum, v string, n node) (*ir.Filter, error) {
		f := e.node(op)
		for _, x := range list(rawArgs(n)) {
			a, err := c.Filter(x)
			if err != nil {
				return nil, err
			}
			f.Args = append(f.Args, a)
		}
		return f, nil
	}}
}

// filtPlayerRef is "<pred> by some player of the filter": ControlledBy,
// OwnedBy.
func filtPlayerRef(p ir.Pred) filtRow {
	return filtRow{build: func(c *Ctx, e *filtEnum, v string, n node) (*ir.Filter, error) {
		ps, err := c.Filter(argN(n, 0))
		if err != nil {
			return nil, err
		}
		f := e.pred(p)
		f.Ref = &ir.Ref{Kind: ir.RefAll, Filter: ps}
		return f, nil
	}}
}

// filtIs is PIs of the argument's reference (PPlayerIs for players);
// negate gives Other.
func filtIs(negate bool) filtRow {
	return filtRow{build: func(c *Ctx, e *filtEnum, v string, n node) (*ir.Filter, error) {
		r, err := c.Ref(argN(n, 0))
		if err != nil {
			return nil, err
		}
		p := ir.PIs
		if e.player() {
			p = ir.PPlayerIs
		}
		f := e.pred(p)
		f.Ref = r
		if negate {
			f = e.not(f)
		}
		return f, nil
	}}
}

func filtHasAbility(negate bool) filtRow {
	return filtRow{build: func(c *Ctx, e *filtEnum, v string, n node) (*ir.Filter, error) {
		k, err := filtStr(c, argN(n, 0))
		if err != nil {
			return nil, err
		}
		f := e.wordPred(ir.PHasKeyword, k)
		if negate {
			f = e.not(f)
		}
		return f, nil
	}}
}

// filtCounters is PCounters with the counter word; the number forms take a
// Comparison first.
func filtCounters(negate, withCmp bool) filtRow {
	return filtRow{build: func(c *Ctx, e *filtEnum, v string, n node) (*ir.Filter, error) {
		ctArg := argN(n, 0)
		f := e.pred(ir.PCounters)
		if withCmp {
			cmp, err := c.Compare(argN(n, 0))
			if err != nil {
				return nil, err
			}
			f.Cmp, f.Value = cmp.Op, cmp.V
			ctArg = argN(n, 1)
		} else {
			f.Cmp, f.Value = ir.CmpGE, &ir.Value{Kind: ir.VConst, N: 1}
		}
		w, err := filtStr(c, ctArg)
		if err != nil {
			return nil, err
		}
		f.Word = w
		if negate {
			return e.not(f), nil
		}
		return f, nil
	}}
}

func filtReceipt(rk ir.ReceiptKind) filtRow {
	return filtRow{build: func(c *Ctx, e *filtEnum, v string, n node) (*ir.Filter, error) {
		f := e.pred(ir.PIs)
		f.Ref = &ir.Ref{Kind: ir.RefReceipt, Receipt: rk}
		return f, nil
	}}
}

func filtTargetRef() filtRow {
	return filtRow{build: func(c *Ctx, e *filtEnum, v string, n node) (*ir.Filter, error) {
		slot, ok := c.TargetSlot(v)
		if !ok {
			return nil, fmt.Errorf("cardc: %s: %s names no declared target", c.Src(), v)
		}
		f := e.pred(ir.PIs)
		f.Ref = &ir.Ref{Kind: ir.RefTarget, Slot: slot}
		return f, nil
	}}
}

func filtCmpPlayer(p ir.Pred) filtRow { return filtCompare(p) }

// filtRows is the shared and Players-only row table. A row registers for
// every enum in filtEnums that lists its variant.
var filtRows = func() []filtRow {
	named := func(v string, r filtRow) filtRow { r.variant = v; return r }
	return []filtRow{
		named("And", filtCombine(ir.FAll)),
		named("Or", filtCombine(ir.FAny)),
		named("IsCardtype", filtTypeWord(ir.PCardType, false)),
		named("IsNonCardtype", filtTypeWord(ir.PCardType, true)),
		named("IsCreatureType", filtTypeWord(ir.PSubtype, false)),
		named("IsLandType", filtTypeWord(ir.PSubtype, false)),
		named("IsArtifactType", filtTypeWord(ir.PSubtype, false)),
		named("IsNonCreatureType", filtTypeWord(ir.PSubtype, true)),
		named("IsNonEnchantmentType", filtTypeWord(ir.PSubtype, true)),
		named("IsSupertype", filtTypeWord(ir.PSupertype, false)),
		named("IsNonSupertype", filtTypeWord(ir.PSupertype, true)),
		named("IsColor", filtColorRow(false)),
		named("IsNonColor", filtColorRow(true)),
		named("IsColorless", filtPlain(ir.PColorless, false)),
		named("IsPermanent", filtPlain(ir.PPermanentCard, false)),
		named("ManaValueIs", filtCompare(ir.PManaValue)),
		named("PowerIs", filtCompare(ir.PPower)),
		named("ToughnessIs", filtCompare(ir.PToughness)),
		named("ControlledByAPlayer", filtPlayerRef(ir.PControlledBy)),
		named("OwnedByAPlayer", filtPlayerRef(ir.POwnedBy)),
		named("InAPlayersGraveyard", filtPlayerRef(ir.POwnedBy)),
		named("HasAbility", filtHasAbility(false)),
		named("DoesntHaveAbility", filtHasAbility(true)),
		named("IsToken", filtPlain(ir.PToken, false)),
		named("IsNonToken", filtPlain(ir.PToken, true)),
		named("IsTapped", filtPlain(ir.PTapped, false)),
		named("IsUntapped", filtPlain(ir.PTapped, true)),
		named("IsAttacking", filtPlain(ir.PAttacking, false)),
		named("IsBlocking", filtPlain(ir.PBlocking, false)),
		named("Other", filtIs(true)),
		named("SinglePermanent", filtIs(false)),
		named("SingleSpell", filtIs(false)),
		named("SinglePlayer", filtIs(false)),
		named("Ref_TargetPermanents", filtTargetRef()),
		named("Ref_TargetGraveyardCards", filtTargetRef()),
		named("AnyPermanent", filtPlain(ir.PAny, false)),
		named("AnySpell", filtPlain(ir.PAny, false)),
		named("AnyCard", filtPlain(ir.PAny, false)),
		named("AnyCardInAnyGraveyard", filtPlain(ir.PAny, false)),
		named("AnyPlayer", filtPlain(ir.PAny, false)),
		named("IsNamed", filtNamed()),
		named("HasACounterOfType", filtCounters(false, false)),
		named("HasNoCountersOfType", filtCounters(true, false)),
		named("HasNumberCountersOfType", filtCounters(false, true)),
		named("TheCardsExiledThisWay", filtReceipt(ir.RcExiled)),
		named("TheTokensCreatedThisWay", filtReceipt(ir.RcCreated)),

		// Players only.
		named("Opponent", filtRow{build: func(c *Ctx, e *filtEnum, v string, n node) (*ir.Filter, error) {
			f := e.pred(ir.POpponentOf)
			f.Ref = &ir.Ref{Kind: ir.RefYou}
			return f, nil
		}}),
		named("AttackedThisTurn", filtPlain(ir.PAttackedThisTurn, false)),
		named("ControlsA", filtRow{build: func(c *Ctx, e *filtEnum, v string, n node) (*ir.Filter, error) {
			ps, err := c.Filter(argN(n, 0))
			if err != nil {
				return nil, err
			}
			f := e.pred(ir.PControls)
			f.Args = []*ir.Filter{ps}
			return f, nil
		}}),
		named("ControlsMorePermanentThanPlayer", filtRow{build: func(c *Ctx, e *filtEnum, v string, n node) (*ir.Filter, error) {
			r, err := c.Ref(argN(n, 0))
			if err != nil {
				return nil, err
			}
			ps, err := c.Filter(argN(n, 1))
			if err != nil {
				return nil, err
			}
			f := e.pred(ir.PControlsMoreThan)
			f.Ref, f.Args = r, []*ir.Filter{ps}
			return f, nil
		}}),
		named("LifeTotalIs", filtCmpPlayer(ir.PLifeTotal)),
		named("LostLifeThisTurn", filtPlain(ir.PLostLifeThisTurn, false)),
		named("NumCardsInGraveyardIs", filtCmpPlayer(ir.PCardsInGraveyard)),
		named("NumCardsInHandIs", filtCmpPlayer(ir.PCardsInHand)),
	}
}()

func filtColorRow(negate bool) filtRow {
	return filtRow{build: func(c *Ctx, e *filtEnum, v string, n node) (*ir.Filter, error) {
		cs, err := filtColor(c, argN(n, 0))
		if err != nil {
			return nil, err
		}
		f := e.pred(ir.PColor)
		f.Colors = cs
		if negate {
			f = e.not(f)
		}
		return f, nil
	}}
}

// filtNamed is IsNamed: its argument is a NameFilter, lowered to a string.
func filtNamed() filtRow {
	return filtRow{build: func(c *Ctx, e *filtEnum, v string, n node) (*ir.Filter, error) {
		w, err := filtStr(c, argN(n, 0))
		if err != nil {
			return nil, err
		}
		return e.wordPred(ir.PName, w), nil
	}}
}

func init() {
	for i := range filtEnums {
		e := &filtEnums[i]
		for _, r := range filtRows {
			if !e.has(r.variant) {
				continue
			}
			r := r
			register(e.name+"."+r.variant, func(c *Ctx, n node) (any, error) {
				f, err := r.build(c, e, r.variant, n)
				if err != nil {
					return nil, err
				}
				return f, nil
			})
		}
	}

	// CheckHasable lowers to the keyword's rules name, a Go string. Only the
	// 13 no-argument variants that are also a keyword.
	for _, k := range []struct{ variant, name string }{
		{"Flying", "Flying"}, {"Deathtouch", "Deathtouch"}, {"Defender", "Defender"}, {"Flash", "Flash"},
		{"Haste", "Haste"}, {"Indestructible", "Indestructible"}, {"Lifelink", "Lifelink"},
		{"Menace", "Menace"}, {"Reach", "Reach"}, {"Trample", "Trample"}, {"Vigilance", "Vigilance"},
		{"DoubleStrike", "Double Strike"}, {"FirstStrike", "First Strike"},
	} {
		name := k.name
		register("CheckHasable."+k.variant, func(c *Ctx, n node) (any, error) { return name, nil })
	}

	// Color lowers to an ir.ColorSet BY VALUE, not a pointer: callers (CC-13)
	// call c.lower on a Color node and type-assert ir.ColorSet.
	for _, k := range []struct {
		variant string
		cs      ir.ColorSet
	}{
		{"White", ir.ColorW}, {"Blue", ir.ColorU}, {"Black", ir.ColorB}, {"Red", ir.ColorR}, {"Green", ir.ColorG},
	} {
		cs := k.cs
		register("Color."+k.variant, func(c *Ctx, n node) (any, error) { return cs, nil })
	}

	// NameFilter.NamedCard(s) lowers to the Go string s.
	register("NameFilter.NamedCard", func(c *Ctx, n node) (any, error) {
		s, ok := rawArgs(n).(string)
		if !ok {
			return nil, fmt.Errorf("cardc: %s: NamedCard wants a string, got %T", c.Src(), rawArgs(n))
		}
		return s, nil
	})
}
