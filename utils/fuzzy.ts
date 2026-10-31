const isBoundary = (text: string, i: number) => i === 0 || /[\s/_\-.()]/.test(text[i - 1]);

// Score one token: a substring beats a scattered match, and a match at the
// start of a word beats one in the middle. null when the letters are not there in order.
function tokenScore(token: string, text: string): number | null {
  const idx = text.indexOf(token);
  if (idx >= 0) {
    let best = idx;
    // Prefer a later occurrence that starts a word ("env" in "my environment").
    for (let i = idx; i >= 0 && !isBoundary(text, best); i = text.indexOf(token, i + 1)) {
      if (isBoundary(text, i)) best = i;
    }
    return 100 + token.length * 10 + (best === 0 ? 60 : 0) + (isBoundary(text, best) ? 40 : 0) - Math.min(best, 30);
  }

  let score = 0;
  let prev = -2;
  let from = 0;
  for (const ch of token) {
    const i = text.indexOf(ch, from);
    if (i < 0) return null;
    score += 5 + (i === prev + 1 ? 8 : 0) + (isBoundary(text, i) ? 6 : 0) - Math.min(i - from, 10) * 0.5;
    prev = i;
    from = i + 1;
  }
  return score;
}

/**
 * Fuzzy score of `query` against `text`, higher is better, null for no match.
 * Every space-separated word of the query must match.
 */
export function fuzzyScore(query: string, text: string): number | null {
  const tokens = query.toLowerCase().split(/\s+/).filter(Boolean);
  const hay = text.toLowerCase();
  let total = 0;
  for (const token of tokens) {
    const s = tokenScore(token, hay);
    if (s === null) return null;
    total += s;
  }
  return total;
}
