package session

import (
	"context"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Progress describes one synchronization attempt. BytesProcessed is logical data
// reconstructed by rsync, including reused blocks, rather than network traffic.
type Progress struct {
	Stage          string    `json:"stage"`
	Percent        *int      `json:"percent,omitempty"`
	Estimated      bool      `json:"estimated,omitempty"`
	BytesProcessed int64     `json:"bytes_processed"`
	Rate           string    `json:"rate,omitempty"`
	ETA            string    `json:"eta,omitempty"`
	Transfers      int64     `json:"transfers,omitempty"`
	Checked        int64     `json:"checked,omitempty"`
	Total          int64     `json:"total,omitempty"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type progressKey struct{}
type progressReporter func(Progress)

func reportProgress(ctx context.Context, p Progress) {
	if report, ok := ctx.Value(progressKey{}).(progressReporter); ok {
		p.UpdatedAt = time.Now()
		report(p)
	}
}

var rsyncProgress = regexp.MustCompile(`^\s*([0-9,]+)\s+([0-9]{1,3})%\s+([0-9.]+[kMGTPE]?B/s)\s+([0-9]+:[0-9]{2}:[0-9]{2}|\?\?:\?\?:\?\?)(?:\s+\(xfr#([0-9]+),\s*(ir-chk|to-chk)=([0-9]+)/([0-9]+)\))?\s*$`)

func parseProgress(line string) (Progress, bool) {
	m := rsyncProgress.FindStringSubmatch(line)
	if m == nil {
		return Progress{}, false
	}
	bytes, err := strconv.ParseInt(strings.ReplaceAll(m[1], ",", ""), 10, 64)
	percent, _ := strconv.Atoi(m[2])
	if err != nil || percent > 100 {
		return Progress{}, false
	}
	// rsync can reach 100% before deletion, metadata and Git state are saved.
	percent = min(percent, 99)
	p := Progress{Stage: "transferring", Percent: &percent, BytesProcessed: bytes, Rate: m[3], Estimated: true}
	if m[5] == "" {
		// Completed transfer lines contain elapsed time, not remaining time.
		if !strings.Contains(m[4], "?") {
			p.ETA = m[4]
		}
	} else {
		p.Transfers, _ = strconv.ParseInt(m[5], 10, 64)
		remaining, _ := strconv.ParseInt(m[7], 10, 64)
		p.Total, _ = strconv.ParseInt(m[8], 10, 64)
		p.Checked = max(0, p.Total-remaining)
	}
	return p, true
}

// progressOutput consumes CR-delimited updates without accumulating them in the
// session log. Both its line buffer and the downstream log tail are bounded.
type progressOutput struct {
	ctx      context.Context
	log      io.Writer
	line     []byte
	overflow bool
	last     Progress
}

func (w *progressOutput) Write(data []byte) (int, error) {
	for _, b := range data {
		if b == '\r' || b == '\n' {
			w.flush()
			continue
		}
		w.line = append(w.line, b)
		if len(w.line) >= 4096 {
			_, _ = w.log.Write(w.line)
			w.line = w.line[:0]
			w.overflow = true
		}
	}
	return len(data), nil
}

func (w *progressOutput) flush() {
	if len(w.line) != 0 || w.overflow {
		if p, ok := parseProgress(string(w.line)); ok && !w.overflow {
			w.last = p
			reportProgress(w.ctx, p)
		} else {
			_, _ = w.log.Write(append(w.line, '\n'))
		}
	}
	w.line = w.line[:0]
	w.overflow = false
}
