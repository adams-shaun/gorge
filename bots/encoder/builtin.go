package encoder

import (
	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// featureSetEncoder wraps one policynet.FeatureSet as a ViewEncoder.
type featureSetEncoder struct {
	fs     policynet.FeatureSet
	schema Schema
}

// builtinSets is every FeatureSet, in ordinal order.
var builtinSets = [...]policynet.FeatureSet{
	policynet.FeaturesV1, policynet.FeaturesMZ, policynet.FeaturesMZOppHand, policynet.FeaturesMZOracle,
	policynet.FeaturesEntity, policynet.FeaturesMZOwnLib, policynet.FeaturesMZGrave, policynet.FeaturesPod,
}

func init() {
	for _, fs := range builtinSets {
		Register(ForFeatureSet(fs))
	}
}

// ForFeatureSet is the ViewEncoder of fs; its Schema.Hash() is
// policynet.EncoderHashFor(fs).
func ForFeatureSet(fs policynet.FeatureSet) ViewEncoder {
	raw := 0
	if fs.Entity() {
		raw = fs.EntityRawW()
	}
	return &featureSetEncoder{fs: fs, schema: Schema{
		Name:       fs.String(),
		Version:    1,
		DenseW:     fs.DenseW(),
		SparseRows: policynet.TableRows,
		RawW:       raw,
		OptDenseW:  policynet.OptionDenseWidth,
		Hidden:     fs.Diagnostic(),
		Pinned:     policynet.EncoderHashFor(fs),
	}}
}

func (e *featureSetEncoder) Schema() Schema { return e.schema }

func (e *featureSetEncoder) EncodeState(buf *policynet.StateBuf, v *view.View, seat state.PlayerID, diag *Diag) policynet.State {
	return policynet.EncodeStateWithInto(buf, e.fs, v, seat, diag)
}

func (e *featureSetEncoder) EncodeOption(v *view.View, seat state.PlayerID, d decision.Kind, o decision.Option, idx, n int) policynet.Option {
	return policynet.EncodeOptionWith(e.fs, *v, seat, d, o, idx, n)
}

func (e *featureSetEncoder) EncodeOptions(v *view.View, seat state.PlayerID, d decision.Kind, opts []decision.Option, dst []policynet.Option) []policynet.Option {
	return policynet.EncodeOptionsWith(e.fs, v, seat, d, opts, dst)
}
