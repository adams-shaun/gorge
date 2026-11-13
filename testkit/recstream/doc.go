// Package recstream reads and writes condensed record streams: JSON Lines
// in which each record is stored as a delta against the record before it,
// with a full keyframe every N records, compressed with gzip. Readers open
// plain .jsonl, gzipped .jsonl.gz and condensed files through one call
// (Open, NewReader), so a tool that switches its writer to the condensed
// form breaks no reader that goes through this package or its Python twin,
// scripts/recstream.py.
//
// It is public (testkit/, not internal/) so four-yay and gorge-compliance
// import it at their gorge pin. It uses only the standard library.
//
// # Format, version 1
//
// A condensed file is one or more gzip members (RFC 1952) laid end to end;
// `zcat` shows the frame stream. Decompressed, the stream is UTF-8 text, one
// frame per line, each line ending in '\n'. The first byte of a frame says
// what it is:
//
//	#recstream/1 ...   header; the first line of the stream. Any later '#'
//	                   line is a comment. Readers ignore both.
//	=TEXT              a keyframe: the record is TEXT, verbatim.
//	:PATCH             a keyframe as a patch: apply PATCH to {} and write the
//	                   result compactly (no whitespace).
//	+PATCH             a delta: apply PATCH to the previous record and write
//	                   the result compactly.
//	;PATCH  ~PATCH     as : and +, written in Python's default json.dumps
//	                   style (", " between items, ": " after keys).
//
// Every gzip member starts with a keyframe, so each member decodes on its
// own: a reader can start at any member, and a damaged member loses only
// the records in it (Options.Keyframe records at most). A writer buffers a
// block of Keyframe records (or BlockBytes of record text) and writes it as
// one member: its frames, or the same records as = keyframes when that
// compresses smaller, as it does for records that share little with their
// neighbours. A record that is not a JSON object, or does not reproduce
// byte for byte in one of the two styles (stray whitespace, CRLF, duplicate
// keys, a key containing a backslash escape), is always an = keyframe, and
// the record after it is a keyframe too.
//
// A PATCH is a JSON object read against the previous record, which is an
// object. Each member names a key; its value is an op:
//
//	scalar             set the key to this string, number, true, false or null
//	[]                 delete the key
//	[0, V]             set the key to V (any value)
//	[1, {...}]         apply the nested PATCH to the key's object
//	[2, P, S, [X...]]  splice the key's array: keep its first P and last S
//	                   elements and put X... between them
//	[3, {"I": op...}]  same-length array: apply op to element I (0-based)
//	[4, A, D, [X...], ...]  edit script: for each (A, D, X) triple, in rising
//	                   A, replace the old array's elements A..A+D-1 with X...
//	[5, V0, OP...]     a chain: element 0 is V0 (an op against nothing),
//	                   element i is OP applied to element i-1, [] repeating it
//
// Ops nest: an element op in [3] and [5] uses the same grammar (but [] in
// [3] is not allowed). A key
// that already exists keeps its place; a new key is appended, in PATCH
// order. Scalars keep their source text, so the Go reader reproduces every
// record byte for byte; the Python reader reproduces it semantically (same
// values, same key order), since Python re-spells string escapes.
//
// # Files
//
// Condensed files are named NAME.cjsonl.gz. The separate extension keeps a
// reader that does not know the format from misreading one as .jsonl.gz;
// readers detect the format by content, never by name. `recstream pack`
// converts .jsonl or .jsonl.gz to the condensed form and checks it record by
// record before reporting success; `recstream unpack` and `recstream cat`
// go back.
package recstream
