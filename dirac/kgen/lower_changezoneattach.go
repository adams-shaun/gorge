package kgen

// ChangeZone AttachedTo$ on a battlefield return
// from a graveyard. gorge reads it two ways (effects/zone_change.go
// markChangeZoneAttach, changeZoneAttachedTo): an Aura entering is marked
// with its named bearers before the move and stays in its zone when none can
// be enchanted (CR 303.4g); anything else (an Equipment) moves, then attaches
// to the first object the selector resolves. Both read the same bearer, so
// one instruction carries it (kop.RetAttachTo) and the kernel splits on the
// returned card's type. Only the two single-object spellings are read:
// "Self" for a one-target return (Magnetic Snuffler's Equipment) and
// "ParentTarget" for the source returning itself (Eagle's Rescue). A card
// filter bearer is refused: gorge attaches to the first creature it finds
// where Forge's ChangeZoneEffect lets the controller choose (One Last Job;
// gorge finding). Since gorge (b6a92e822) a filter bearer asks the
// controller which legal bearer the card attaches to (CR 303.4f), and the
// kernel poses the same choice (tgt_attachedto.go): the one filter
// read, "Creature.YouCtrl" on a one-target graveyard return (One Last Job),
// rides the instruction's F beside RetAttachTo. Every other filter stays
// refused (Retether's mass return, Storm Herald's and Nomad Mythmaker's
// hidden-origin searches, Runed Crown's shared-type filter are
// lockstep-unverified).

func (lc *lowerCtx) czAttachBearer(at string, moved lRef, origin int) (lRef, *lFilter, error) {
	if origin != zGraveyard {
		return lRef{}, nil, refuse("AttachedTo$ on a move from %s", zoneLabel[origin])
	}
	switch at {
	case "Creature.YouCtrl":
		if moved.Kind != rTarget {
			return lRef{}, nil, refuse("AttachedTo$ %q on a %s return", at, refNames[moved.Kind])
		}
		f, err := lc.filter(at)
		return lRef{}, f, err
	case "Self":
		if moved.Kind != rTarget {
			return lRef{}, nil, refuse("AttachedTo$ Self on a %s return", refNames[moved.Kind])
		}
	case "ParentTarget":
		if moved.Kind != rSelf {
			return lRef{}, nil, refuse("AttachedTo$ ParentTarget on a %s return", refNames[moved.Kind])
		}
	default:
		return lRef{}, nil, refuse("AttachedTo$ %q", at)
	}
	r, err := lc.defined(at)
	return r, nil, err
}
