package params

import "slices"

// DescriptiveKeys lists the Forge parameter keys that carry presentation text,
// never a rules instruction: captions, prompts and UI descriptions that a
// text-change effect may rewrite. A per-API compiler must not report one of
// these as an unread parameter -- that Note is the loud-degrade signal for a
// rule the engine does not implement, and a caption never is one.
//
// Contents are sorted for slices.BinarySearch (zero allocation: no map on the
// compile hot path). Sources, all in a Forge checkout:
//
//   - Description, SpellDescription, StackDescription, TriggerDescription,
//     ChangeTypeDesc and ValidTgtsDesc are Forge's CardTraitBase.descriptiveKeys
//     (forge-game/src/main/java/forge/game/CardTraitBase.java:62-66), the text
//     parameters a text-change effect rewrites.
//   - GiftDescription is the gift caption; forge-game/src/main/java/forge/game/card/CardFactoryUtil.java:1142
//     and forge-game/src/main/java/forge/game/ability/SpellAbilityEffect.java:117.
//   - DefinedDesc is the Play/Animate caption; forge-game/src/main/java/forge/game/ability/effects/PlayEffect.java:68-69
//     and AnimateEffect.java:236.
//   - IfDesc is the Play conditional caption; forge-game/src/main/java/forge/game/ability/effects/PlayEffect.java:76-77.
//   - ChangeValidDesc is the Dig search caption; forge-game/src/main/java/forge/game/ability/effects/DigEffect.java:85.
//   - PrimaryPrompt is the Dig primary pick prompt; forge-game/src/main/java/forge/game/ability/effects/DigEffect.java:321-322.
//   - NoPutDesc is the DigUntil caption; forge-game/src/main/java/forge/game/ability/effects/DigUntilEffect.java:69.
//   - TgtPrompt is the target prompt; forge-game/src/main/java/forge/game/card/CardFactoryUtil.java.
//   - ChoiceTitle is the choice-dialog title; forge-game/src/main/java/forge/game/card/CardFactoryUtil.java.
var DescriptiveKeys = [...]string{
	"ChangeTypeDesc",
	"ChangeValidDesc",
	"ChoiceTitle",
	"DefinedDesc",
	"Description",
	"GiftDescription",
	"IfDesc",
	"NoPutDesc",
	"PrimaryPrompt",
	"SpellDescription",
	"StackDescription",
	"TgtPrompt",
	"TriggerDescription",
	"ValidTgtsDesc",
}

// IsDescriptiveKey reports whether k is one of DescriptiveKeys.
func IsDescriptiveKey(k string) bool {
	_, ok := slices.BinarySearch(DescriptiveKeys[:], k)
	return ok
}
