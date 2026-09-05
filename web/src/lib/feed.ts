export interface FeedLine { table: string; match: number; seq: number; line: string }

export function pushFeed(lines: FeedLine[], l: FeedLine, cap = 200): FeedLine[] {
  if (!l.line) return lines;
  if (lines.some((x) => x.table === l.table && x.match === l.match && x.seq === l.seq)) return lines;
  const out = [...lines, l];
  return out.length > cap ? out.slice(out.length - cap) : out;
}
