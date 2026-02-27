package refparser

import "testing"

func TestParse_Valid(t *testing.T) {
	ref, err := Parse("jingui://gmail/user@example.com/token")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if ref.Vault != "gmail" {
		t.Errorf("Vault = %q, want %q", ref.Vault, "gmail")
	}
	if ref.Item != "user@example.com" {
		t.Errorf("Item = %q, want %q", ref.Item, "user@example.com")
	}
	if ref.FieldName != "token" {
		t.Errorf("FieldName = %q, want %q", ref.FieldName, "token")
	}
	if ref.Section != "" {
		t.Errorf("Section = %q, want empty", ref.Section)
	}
	if ref.Raw != "jingui://gmail/user@example.com/token" {
		t.Errorf("Raw = %q", ref.Raw)
	}
}

func TestParse_OpPrefix(t *testing.T) {
	ref, err := Parse("op://gmail/user@example.com/token")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if ref.Vault != "gmail" {
		t.Errorf("Vault = %q, want %q", ref.Vault, "gmail")
	}
	if ref.Item != "user@example.com" {
		t.Errorf("Item = %q, want %q", ref.Item, "user@example.com")
	}
	if ref.FieldName != "token" {
		t.Errorf("FieldName = %q, want %q", ref.FieldName, "token")
	}
	if ref.Section != "" {
		t.Errorf("Section = %q, want empty", ref.Section)
	}
	if ref.Raw != "op://gmail/user@example.com/token" {
		t.Errorf("Raw = %q", ref.Raw)
	}
}

func TestParse_OpPrefix_FourSegment(t *testing.T) {
	ref, err := Parse("op://gmail/user@ex.com/oauth/token")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if ref.Vault != "gmail" {
		t.Errorf("Vault = %q, want %q", ref.Vault, "gmail")
	}
	if ref.Item != "user@ex.com" {
		t.Errorf("Item = %q, want %q", ref.Item, "user@ex.com")
	}
	if ref.Section != "oauth" {
		t.Errorf("Section = %q, want %q", ref.Section, "oauth")
	}
	if ref.FieldName != "token" {
		t.Errorf("FieldName = %q, want %q", ref.FieldName, "token")
	}
}

func TestParse_AllFields(t *testing.T) {
	tests := []struct {
		ref   string
		vault string
		item  string
		field string
	}{
		{"jingui://gmail/work/token", "gmail", "work", "token"},
		{"jingui://github/foo@bar.com/pat", "github", "foo@bar.com", "pat"},
		{"op://gmail/work/token", "gmail", "work", "token"},
		{"op://github/foo@bar.com/pat", "github", "foo@bar.com", "pat"},
	}
	for _, tt := range tests {
		r, err := Parse(tt.ref)
		if err != nil {
			t.Errorf("Parse(%q): %v", tt.ref, err)
			continue
		}
		if r.Vault != tt.vault || r.Item != tt.item || r.FieldName != tt.field {
			t.Errorf("Parse(%q) = {%q, %q, %q}, want {%q, %q, %q}", tt.ref,
				r.Vault, r.Item, r.FieldName, tt.vault, tt.item, tt.field)
		}
	}
}

func TestParse_FourSegment(t *testing.T) {
	ref, err := Parse("jingui://gmail/user@ex.com/oauth/token")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if ref.Vault != "gmail" {
		t.Errorf("Vault = %q, want %q", ref.Vault, "gmail")
	}
	if ref.Item != "user@ex.com" {
		t.Errorf("Item = %q, want %q", ref.Item, "user@ex.com")
	}
	if ref.Section != "oauth" {
		t.Errorf("Section = %q, want %q", ref.Section, "oauth")
	}
	if ref.FieldName != "token" {
		t.Errorf("FieldName = %q, want %q", ref.FieldName, "token")
	}
}

func TestParse_Invalid(t *testing.T) {
	invalids := []string{
		"",
		"not-a-ref",
		"jingui://",
		"jingui://app",
		"jingui://app/name",
		"jingui:///name/field",
		"jingui://app//field",
		"jingui://app/name/",
		"jingui://a/b/c/d/e", // 5 segments — invalid
		"op://",
		"op://app",
		"op://app/name",
		"op:///name/field",
		"op://app//field",
		"op://app/name/",
		"op://a/b/c/d/e", // 5 segments — invalid
	}
	for _, ref := range invalids {
		_, err := Parse(ref)
		if err == nil {
			t.Errorf("Parse(%q) should fail", ref)
		}
	}
}

func TestIsRef(t *testing.T) {
	if !IsRef("jingui://foo/bar/baz") {
		t.Error("expected true for jingui:// prefix")
	}
	if !IsRef("op://foo/bar/baz") {
		t.Error("expected true for op:// prefix")
	}
	if IsRef("https://foo/bar") {
		t.Error("expected false for non-jingui prefix")
	}
	if IsRef("") {
		t.Error("expected false for empty string")
	}
}
