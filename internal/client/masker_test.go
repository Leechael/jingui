package client

import (
	"bytes"
	"testing"
)

func TestMaskingWriter_Basic(t *testing.T) {
	var buf bytes.Buffer
	mw := NewMaskingWriter(&buf, []string{"SECRET123", "TOKEN456"})

	mw.Write([]byte("hello SECRET123 world TOKEN456 end"))
	mw.Flush()

	got := buf.String()
	want := "hello [REDACTED_BY_JINGUI] world [REDACTED_BY_JINGUI] end"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestMaskingWriter_ChunkBoundary(t *testing.T) {
	var buf bytes.Buffer
	secret := "MYSECRET"
	mw := NewMaskingWriter(&buf, []string{secret})

	// Split secret across two writes
	mw.Write([]byte("prefix MYSE"))
	mw.Write([]byte("CRET suffix"))
	mw.Flush()

	got := buf.String()
	want := "prefix [REDACTED_BY_JINGUI] suffix"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestMaskingWriter_NoSecrets(t *testing.T) {
	var buf bytes.Buffer
	mw := NewMaskingWriter(&buf, nil)

	mw.Write([]byte("passthrough"))
	mw.Flush()

	if got := buf.String(); got != "passthrough" {
		t.Fatalf("got %q, want %q", got, "passthrough")
	}
}

func TestMaskingWriter_MultipleMatches(t *testing.T) {
	var buf bytes.Buffer
	mw := NewMaskingWriter(&buf, []string{"AAA", "BBB"})

	mw.Write([]byte("AAA and BBB and AAA"))
	mw.Flush()

	got := buf.String()
	want := "[REDACTED_BY_JINGUI] and [REDACTED_BY_JINGUI] and [REDACTED_BY_JINGUI]"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestMaskingWriter_EmptySecrets(t *testing.T) {
	var buf bytes.Buffer

	// Empty strings in secrets list should not cause panic or misbehavior
	mw := NewMaskingWriter(&buf, []string{"", "SECRET", ""})

	mw.Write([]byte("hello SECRET world"))
	mw.Flush()

	got := buf.String()
	want := "hello [REDACTED_BY_JINGUI] world"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestMaskingWriter_AllEmptySecrets(t *testing.T) {
	var buf bytes.Buffer

	// All empty → passthrough
	mw := NewMaskingWriter(&buf, []string{"", ""})

	mw.Write([]byte("passthrough"))
	mw.Flush()

	if got := buf.String(); got != "passthrough" {
		t.Fatalf("got %q, want %q", got, "passthrough")
	}
}

func TestMaskingWriter_LargeInput(t *testing.T) {
	var buf bytes.Buffer
	secret := "BIGSECRET"
	mw := NewMaskingWriter(&buf, []string{secret})

	// Write many chunks
	for i := 0; i < 100; i++ {
		mw.Write([]byte("data "))
	}
	mw.Write([]byte("BIGSECRET end"))
	mw.Flush()

	got := buf.String()
	if bytes.Contains([]byte(got), []byte(secret)) {
		t.Fatal("secret value leaked in output")
	}
	if !bytes.Contains([]byte(got), []byte("[REDACTED_BY_JINGUI]")) {
		t.Fatal("expected redaction placeholder in output")
	}
}
