package oracle

import (
	"github.com/adams-shaun/gorge/dirac/canon"
)

// The oracle speaks the canonical vocabulary dirac/canon owns.
type (
	Canon     = canon.Canon
	CanonKind = canon.CanonKind
	DKind     = canon.DKind
	Ref       = canon.Ref
)

const (
	ActPass             = canon.ActPass
	ActPlayLand         = canon.ActPlayLand
	ActCast             = canon.ActCast
	ActActivate         = canon.ActActivate
	ActAttack           = canon.ActAttack
	ActBlock            = canon.ActBlock
	ActDone             = canon.ActDone
	ActTarget           = canon.ActTarget
	ActMode             = canon.ActMode
	ActKeep             = canon.ActKeep
	ActMulligan         = canon.ActMulligan
	ActBottom           = canon.ActBottom
	ActOrder            = canon.ActOrder
	ActYes              = canon.ActYes
	ActNo               = canon.ActNo
	ActArrange          = canon.ActArrange
	ActReplacement      = canon.ActReplacement
	ActSpecial          = canon.ActSpecial
	ActStartPlayer      = canon.ActStartPlayer
	ActCommanderZone    = canon.ActCommanderZone
	ActCastFace         = canon.ActCastFace
	ActCastCasualty     = canon.ActCastCasualty
	ActCastImpended     = canon.ActCastImpended
	ActCastAirbend      = canon.ActCastAirbend
	ActChoose           = canon.ActChoose
	DPriority           = canon.DPriority
	DAttackers          = canon.DAttackers
	DBlockers           = canon.DBlockers
	DTarget             = canon.DTarget
	DModes              = canon.DModes
	DChoose             = canon.DChoose
	DTriggerOrder       = canon.DTriggerOrder
	DMulligan           = canon.DMulligan
	DNone               = canon.DNone
	DTriggerOptional    = canon.DTriggerOptional
	DArrange            = canon.DArrange
	DReplacement        = canon.DReplacement
	DStartingPlayer     = canon.DStartingPlayer
	DCommanderZone      = canon.DCommanderZone
	RefStack            = canon.RefStack
	RefPlayer           = canon.RefPlayer
	RefNone             = canon.RefNone
	castModeOther       = canon.CastModeOther
	chooseOther         = canon.ChooseOther
	manaSyms            = canon.ManaSyms
	maxSpeedBase        = canon.MaxSpeedBase
	faceCodeSneak       = canon.FaceCodeSneak
	faceCodeOmen        = canon.FaceCodeOmen
	faceCodePrepared    = canon.FaceCodePrepared
	faceCodeRoomAlt     = canon.FaceCodeRoomAlt
	faceCodeModal       = canon.FaceCodeModal
	faceCodeAdventure   = canon.FaceCodeAdventure
	faceCodeRecast      = canon.FaceCodeRecast
	faceCodeSplit       = canon.FaceCodeSplit
	faceCodeAftermath   = canon.FaceCodeAftermath
	faceCodeMorphed     = canon.FaceCodeMorphed
	faceCodeMegamorphed = canon.FaceCodeMegamorphed
	faceCodeDisguised   = canon.FaceCodeDisguised
)

var (
	castModeCode      = canon.CastModeCode
	chooseCode        = canon.ChooseCode
	specialCode       = canon.SpecialCode
	turnUpModeCode    = canon.TurnUpModeCode
	unlockModeCode    = canon.UnlockModeCode
	landFaceCode      = canon.LandFaceCode
	chooseKinds       = canon.ChooseKinds
	faceCastCode      = canon.FaceCastCode
	castOptCanon      = canon.CastOptCanon
	beamAlt           = canon.BeamAlt
	followedFaceRoute = canon.FollowedFaceRoute
	faceDownCastCanon = canon.FaceDownCastCanon
	maxSpeedCode      = canon.MaxSpeedCode
	impendTickAb      = canon.ImpendTickAb
	Pack              = canon.Pack
	CastModeName      = canon.CastModeName
	GorgeDKind        = canon.GorgeDKind
	GorgeKind         = canon.GorgeKind
	kernelDKind       = canon.KernelDKind
	fdnSubKind        = canon.GeneratedKind
)
