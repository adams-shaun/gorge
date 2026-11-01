package oracle

// kapi.go is the only file that names the kernel's API types, spelled the way
// dirac/types.go spells them (Ak*/Dk*): nothing else in the package names a
// kernel type.
//
// The oracle itself speaks its own canonical vocabulary (canon.go: Canon,
// CDecision, Projection) through the Side interface; kernelSide adapts a
// Kernel to a Side.

// CardID indexes the matchup's card table; 255 = none.
type CardID = uint8

// Slot is a kernel zone slot index; 255 = none.
type Slot uint8

// NoCard and NoSlot are the contract's "none" values.
const (
	NoCard CardID = 255
	NoSlot Slot   = 255
)

// Action is the kernel's packed answer: kind<<24 | source<<16 | target<<8 | mode.
type Action uint32

// ActionKind is an Action's kind (kernel-core's numbering).
type ActionKind uint8

const (
	AkPass ActionKind = iota
	AkPlayLand
	AkCast
	AkActivate
	AkAttack
	AkBlock
	AkDone
	AkTarget
	AkMode
	AkChoose
	AkKeep
	AkMulligan
	AkOrder
)

// DecisionKind is what a kernel Decision asks (kernel-core's numbering).
type DecisionKind uint8

const (
	DkNone DecisionKind = iota
	DkPriority
	DkAttackers
	DkBlockers
	DkTarget
	DkModes
	DkChoose
	DkTriggerOrder
	DkMulligan
)

// MaxOptions bounds a kernel Decision's options.
const MaxOptions = 128 // dirac.MaxOptions

// Decision is one kernel decision: N options in Options[:N].
type Decision struct {
	Kind    DecisionKind
	Player  uint8
	N       uint8
	Options [MaxOptions]Action
}

// KernelProjection is the contract's Projection. Until the kernel lands it
// is the oracle's own (project.go); at integration it aliases
// dirac.Projection and toProjection converts.
type KernelProjection = Projection

func toProjection(src *KernelProjection, dst *Projection) { *dst = *src }

// Kernel is the contract's game surface on one game.
type Kernel interface {
	Step() Decision
	Apply(a Action)
	Terminal() (over bool, winner int8)
	Project(p *KernelProjection)
}

// KernelSide adapts a Kernel to the oracle's Side: it canonicalises each
// posed option and maps a canonical answer back to the kernel's own action.
func KernelSide(k Kernel) Side { return &kernelSide{k: k} }

type kernelSide struct {
	k    Kernel
	d    Decision
	opts []Canon
	kp   KernelProjection
}

func (s *kernelSide) Step() CDecision {
	s.d = s.k.Step()
	s.opts = s.opts[:0]
	for _, a := range s.d.Options[:s.d.N] {
		s.opts = append(s.opts, s.canon(a))
	}
	out := CDecision{Kind: dkind(s.d.Kind), Player: s.d.Player, Options: append([]Canon(nil), s.opts...)}
	sortCanon(out.Options)
	return out
}

func (s *kernelSide) Apply(c Canon) {
	for i, x := range s.opts {
		if x == c {
			s.k.Apply(s.d.Options[i])
			return
		}
	}
	s.k.Apply(Action(c)) // not offered: let the kernel reject it
}

func (s *kernelSide) Terminal() (bool, int8) { return s.k.Terminal() }

// Decided forwards a kernel's decided result when it has one; the contract's
// Terminal is the fallback (a kernel ends the game when it is decided).
func (s *kernelSide) Decided() (bool, int8) {
	if d, ok := s.k.(interface{ Decided() (bool, int8) }); ok {
		return d.Decided()
	}
	return s.k.Terminal()
}

// Dirty forwards a kernel's dirty flag when it has one; without it a
// projection is due at every posed decision.
func (s *kernelSide) Dirty() bool {
	if d, ok := s.k.(interface{ Dirty() bool }); ok {
		return d.Dirty()
	}
	return true
}

// Turn forwards a kernel's turn when it has one.
func (s *kernelSide) Turn() int {
	if t, ok := s.k.(interface{ Turn() int }); ok {
		return t.Turn()
	}
	return 0
}

// Err forwards a kernel's deferred failure when it has one.
func (s *kernelSide) Err() error {
	if e, ok := s.k.(interface{ Err() error }); ok {
		return e.Err()
	}
	return nil
}

func (s *kernelSide) Project(p *Projection) {
	s.k.Project(&s.kp)
	toProjection(&s.kp, p)
}

// ChanceTranscript forwards a kernel's chance transcript (Replay's forced
// outcomes), when it records one.
func (s *kernelSide) ChanceTranscript() []ChanceDraw {
	if cs, ok := s.k.(ChanceSource); ok {
		return cs.ChanceTranscript()
	}
	return nil
}

// canon is a kernel action's canonical form. The stub kernel (stubKernel)
// packs canonical actions, so it is the identity; the real kernel's
// translation (Ak* -> Act*, its trigger index -> the trigger's source,
// its attack -> the opponent's player reference) replaces it at integration.
func (s *kernelSide) canon(a Action) Canon { return Canon(a) }

// dkind is a kernel decision kind's canonical kind. The stub kernel reports
// canonical kinds as DecisionKind(DKind)+1 (DkNone stays 0).
func dkind(k DecisionKind) DKind {
	if k == DkNone {
		return DNone
	}
	return DKind(k - 1)
}

// stubKernel exposes a Side through the contract's Kernel surface, packing
// canonical actions as kernel Actions: GorgeKernel's API path, so the
// self-check runs through kernelSide exactly as the real kernel will. It is
// deleted with the rest of this file at integration.
type stubKernel struct{ s Side }

func (k stubKernel) Step() Decision {
	cd := k.s.Step()
	d := Decision{Player: cd.Player}
	if cd.Kind != DNone {
		d.Kind = DecisionKind(cd.Kind) + 1
	}
	for _, c := range cd.Options {
		if int(d.N) >= MaxOptions {
			break
		}
		d.Options[d.N] = Action(c)
		d.N++
	}
	return d
}
func (k stubKernel) Apply(a Action)              { k.s.Apply(Canon(a)) }
func (k stubKernel) Terminal() (bool, int8)      { return k.s.Terminal() }
func (k stubKernel) Decided() (bool, int8)       { return k.s.Decided() }
func (k stubKernel) Dirty() bool                 { return k.s.Dirty() }
func (k stubKernel) Turn() int                   { return k.s.Turn() }
func (k stubKernel) Err() error                  { return k.s.Err() }
func (k stubKernel) Project(p *KernelProjection) { k.s.Project(p) }
func (k stubKernel) ChanceTranscript() []ChanceDraw {
	if cs, ok := k.s.(ChanceSource); ok {
		return cs.ChanceTranscript()
	}
	return nil
}

// GorgeKernelAPI is GorgeKernel behind the contract's Kernel surface,
// adapted back through KernelSide: the self-check through the exact path the
// real kernel will take, contract limits included (a decision with more than
// MaxOptions options is truncated, as the kernel's would be).
func GorgeKernelAPI(cfg Config) SideFactory {
	return func(m *Matchup, seed uint64) (Side, error) {
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			return nil, err
		}
		return KernelSide(stubKernel{g}), nil
	}
}
