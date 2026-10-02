package gitpanel

// editHunk is one changed region between the index side and the editor:
// old lines [os, oe) were replaced by new lines [ns, ne). Either range may
// be empty (pure insertion / deletion).
type editHunk struct{ os, oe, ns, ne int }

// lineDiff diffs two line slices into hunks. Common prefix and suffix are
// stripped first, so a single edit costs O(n); the rest is an LCS table.
// ponytail: above 4M cells the middle is one replaced block, Myers if that
// ever shows up in practice.
func lineDiff(a, b []string) []editHunk {
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	am, bm := a[pre:len(a)-suf], b[pre:len(b)-suf]
	n, m := len(am), len(bm)
	if n == 0 && m == 0 {
		return nil
	}
	if n == 0 || m == 0 || n*m > 4_000_000 {
		return []editHunk{{pre, pre + n, pre, pre + m}}
	}
	// lcs[i][j] = LCS length of am[i:], bm[j:].
	lcs := make([][]int32, n+1)
	for i := range lcs {
		lcs[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if am[i] == bm[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var out []editHunk
	i, j := 0, 0
	open := func() *editHunk {
		if len(out) == 0 || out[len(out)-1].oe != pre+i || out[len(out)-1].ne != pre+j {
			out = append(out, editHunk{pre + i, pre + i, pre + j, pre + j})
		}
		return &out[len(out)-1]
	}
	for i < n || j < m {
		switch {
		case i < n && j < m && am[i] == bm[j]:
			i, j = i+1, j+1
		case j < m && (i == n || lcs[i][j+1] >= lcs[i+1][j]):
			h := open()
			j++
			h.ne = pre + j
		default:
			h := open()
			i++
			h.oe = pre + i
		}
	}
	return out
}

// hunkAt is the hunk covering new line n (an insertion-free deletion sits on
// the line after it), or nil.
func (p *Panel) hunkAt(n int) *editHunk {
	for i := range p.editHunks {
		h := &p.editHunks[i]
		if n >= h.ns && (n < h.ne || (h.ns == h.ne && n == h.ns)) {
			return h
		}
	}
	return nil
}

// leftLine maps editor line n to the index-side line shown beside it:
// the unchanged counterpart, or the k-th removed line of its hunk (del =
// true). -1 when nothing pairs with it.
func (p *Panel) leftLine(n int) (old int, del bool) {
	shift := 0
	for _, h := range p.editHunks {
		if n >= h.ne { // past this hunk (a pure deletion ends where it starts)
			shift += (h.oe - h.os) - (h.ne - h.ns)
			continue
		}
		if n >= h.ns && n < h.ne {
			if k := h.os + (n - h.ns); k < h.oe {
				return k, true
			}
			return -1, false
		}
		break
	}
	return n + shift, false
}
