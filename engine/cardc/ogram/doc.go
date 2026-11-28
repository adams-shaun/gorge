// Package ogram reads a pinned release of the oracle-grammar project's typed
// card faces: a second reading of every card, independent of the Forge
// scripts and of mtgish. It counts the faces the grammar reads uniquely,
// ambiguously or not at all, and lowers the object filters of the uniquely
// read faces to ir.Filter through mtgish's filter row table.
//
// The release (engine/cardc/OGRAM.lock, fetched by make ogram-fetch into
// .cards/ogram/<tag>/, never committed) is format_version 1, schema 193:
//
//   - manifest.json: format_version, tag, counts (the coverage numbers),
//     grammar.commit, model.schema_version, the sha256 of every asset.
//   - coverage.json: input_faces and counts.{unique, ambiguous, refused,
//     limit}, the release's own totals.
//   - outcomes.jsonl.gz: one record per input face, all outcomes. Fields:
//     format_version, schema_version, name (the face name), identity
//     {face_index, layout, provider_id, record_index, record_name}, status
//     ("unique", "ambiguous" or "refused"), readings (a list of {card, notes};
//     empty when refused), source (the source card record), and either
//     semantic_readings, context and work (read faces) or stage ("no_parse",
//     "unsupported_source", "lexical", "binding") and message (refused faces).
//   - faces.jsonl.gz: the outcomes records whose status is "unique", same
//     fields. A unique face may carry more than one syntactic reading;
//     semantic_readings is 1 and readings[0].card is the reading.
//   - oracle-source.jsonl.gz: the Scryfall oracle-cards snapshot the faces
//     were read from, one card record per line; identity.record_index is the
//     line number (from 0). legalities.commander and games carry legality.
//   - 193.json (the lock's schema line, from the repository at
//     grammar.commit): the type description of the readings, which the
//     operand walk needs because the readings carry no type names.
//
// A reading (readings[0].card) is the schema type Card: {schema_version,
// name, characteristics, abilities}. Its encoding is not mtgish's: an enum
// marked adjacent is {"kind":"Variant","args":...} (no "args" for a unit
// variant), any other enum is "Variant" for a unit variant and
// {"Variant":...} otherwise, and nothing names the enum. Object filters are
// the type ObjectFilter (AllOf, AnyOf, Not, CardType, Subtype, Color, ...);
// FilterOperands finds them by walking a reading with the schema, and
// LowerFilter translates one to mtgish's group-enum encoding and lowers it
// with mtgish.LowerFilterNode.
package ogram
