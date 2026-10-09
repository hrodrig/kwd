// Package report renders check verdicts into a table and folds them into an
// exit code.
package report

import (
	"fmt"
	"io"
	"sort"

	"github.com/hrodrig/kwd/internal/check"
)

// StatusSymbol returns a colored-capable status glyph. Color is applied by the
// CLI; this returns a plain text marker per status.
func StatusSymbol(s check.Status) string {
	switch s {
	case check.Ready:
		return "READY"
	case check.NotReady:
		return "NOT-READY"
	case check.Errored:
		return "ERRORED"
	default:
		return "UNKNOWN"
	}
}

// WriteTable renders one row per verdict: `STATUS  kind.namespace/name  reason`.
func WriteTable(w io.Writer, verdicts []check.Verdict) {
	// Stable, grouped output: sort by status severity then ref.
	sorted := append([]check.Verdict(nil), verdicts...)
	sort.SliceStable(sorted, func(i, j int) bool {
		si, sj := severity(sorted[i].Status), severity(sorted[j].Status)
		if si != sj {
			return si < sj
		}
		return sorted[i].Ref.String() < sorted[j].Ref.String()
	})

	for _, v := range sorted {
		_, _ = fmt.Fprintf(w, "%-10s %s  %s\n", StatusSymbol(v.Status), v.Ref.String(), v.Reason)
	}
}

// AnyNotReady reports whether any verdict is not Ready.
func AnyNotReady(verdicts []check.Verdict) bool {
	for _, v := range verdicts {
		if v.Status != check.Ready {
			return true
		}
	}
	return false
}

// severity orders Errored < NotReady < Ready (worst first) for a stable table.
func severity(s check.Status) int {
	switch s {
	case check.Errored:
		return 0
	case check.NotReady:
		return 1
	case check.Ready:
		return 2
	default:
		return 3
	}
}
