package state

// GoadActive reports whether goad relationship ge on o is in force (CR
// 701.15): o is on the battlefield, and an "as long as" duration still
// holds -- AsLongAsInPlay while the goad's source is on the battlefield,
// AsLongAsControl while o's controller is the one it had when goaded. Every
// other duration lasts until the events that end it remove the entry. It
// reads state only (combat's goad requirements and the view's GoadedBy).
func (g *Game) GoadActive(o *Object, ge GoadEffect) bool {
	if o.Zone != ZBattlefield {
		return false
	}
	switch goadDurationCodes.Code(ge.Duration) {
	case goadAsLongAsInPlay:
		src := g.Obj(ge.Source)
		return src != nil && src.Zone == ZBattlefield
	case goadAsLongAsControl:
		return o.Controller == ge.Controller
	default:
		return true
	}
}

type goadDurationCode uint16

const (
	goadAsLongAsInPlay goadDurationCode = iota + 1
	goadAsLongAsControl
)

var goadDurationCodes = NewStrCodes(
	StrEntry[goadDurationCode]{Key: "AsLongAsInPlay", Val: goadAsLongAsInPlay},
	StrEntry[goadDurationCode]{Key: "AsLongAsControl", Val: goadAsLongAsControl},
)
