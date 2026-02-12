package client

import (
	"io"
	"sync"

	aho "github.com/petar-dambovaliev/aho-corasick"
)

const redactedPlaceholder = "[REDACTED_BY_JINGUI]"

// MaskingWriter wraps an io.Writer and replaces any occurrence of secret values
// with [REDACTED_BY_JINGUI]. Uses Aho-Corasick for efficient multi-pattern matching.
// Handles matches that span across Write() call boundaries by buffering.
type MaskingWriter struct {
	mu           sync.Mutex
	out          io.Writer
	matcher      aho.AhoCorasick
	secrets      []string
	maxSecretLen int
	buf          []byte
}

// NewMaskingWriter creates a MaskingWriter that will redact all given secret values.
// If secrets is empty, writes pass through unmodified.
func NewMaskingWriter(out io.Writer, secrets []string) *MaskingWriter {
	// Filter out empty strings — they cause maxSecretLen==0 which breaks
	// the buffer arithmetic (safeEnd underflow) and are meaningless to match.
	var filtered []string
	for _, s := range secrets {
		if len(s) > 0 {
			filtered = append(filtered, s)
		}
	}

	mw := &MaskingWriter{
		out:     out,
		secrets: filtered,
	}

	if len(filtered) == 0 {
		return mw
	}

	for _, s := range filtered {
		if len(s) > mw.maxSecretLen {
			mw.maxSecretLen = len(s)
		}
	}

	builder := aho.NewAhoCorasickBuilder(aho.Opts{})
	mw.matcher = builder.Build(filtered)

	return mw
}

// Write implements io.Writer. Data may be buffered to handle cross-boundary matches.
func (mw *MaskingWriter) Write(p []byte) (int, error) {
	if len(mw.secrets) == 0 {
		return mw.out.Write(p)
	}

	mw.mu.Lock()
	defer mw.mu.Unlock()

	mw.buf = append(mw.buf, p...)

	if err := mw.processBuffer(false); err != nil {
		return 0, err
	}

	return len(p), nil
}

// Flush writes any remaining buffered data, performing final masking.
func (mw *MaskingWriter) Flush() error {
	if len(mw.secrets) == 0 {
		return nil
	}

	mw.mu.Lock()
	defer mw.mu.Unlock()

	return mw.processBuffer(true)
}

func (mw *MaskingWriter) processBuffer(flushAll bool) error {
	if len(mw.buf) == 0 {
		return nil
	}

	// Determine how far we can safely emit.
	// We retain maxSecretLen-1 bytes to handle cross-boundary matches,
	// unless we're flushing everything.
	safeEnd := len(mw.buf)
	if !flushAll {
		safeEnd = len(mw.buf) - (mw.maxSecretLen - 1)
		if safeEnd <= 0 {
			return nil
		}
	}

	// Search the ENTIRE buffer for matches (not just safe zone)
	// so we can detect matches that straddle the safe boundary.
	matches := mw.matcher.FindAll(string(mw.buf))

	var result []byte
	pos := 0
	consumedEnd := safeEnd

	for _, m := range matches {
		start := m.Start()
		end := m.End()

		if start < pos {
			continue // overlapping match
		}

		// Skip matches entirely beyond the safe boundary (they stay in buffer)
		if start >= safeEnd && !flushAll {
			break
		}

		// This match starts before safeEnd (or we're flushing all)
		result = append(result, mw.buf[pos:start]...)
		result = append(result, []byte(redactedPlaceholder)...)
		pos = end

		// If match crosses safeEnd boundary, advance consumedEnd past it
		if end > consumedEnd {
			consumedEnd = end
		}
	}

	// Emit any remaining non-matched bytes up to safeEnd
	if pos < safeEnd {
		result = append(result, mw.buf[pos:safeEnd]...)
	}

	if len(result) > 0 {
		if _, err := mw.out.Write(result); err != nil {
			return err
		}
	}

	// Retain unconsumed bytes
	remaining := make([]byte, len(mw.buf)-consumedEnd)
	copy(remaining, mw.buf[consumedEnd:])
	mw.buf = remaining

	return nil
}
