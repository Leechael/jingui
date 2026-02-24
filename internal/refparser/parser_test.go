package refparser

import "testing"

func TestParse_Valid(t *testing.T) {
	ref, err := Parse("jingui://gmail/user@example.com/token")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if ref.Service != "gmail" {
		t.Errorf("Service = %q, want %q", ref.Service, "gmail")
	}
	if ref.Slug != "user@example.com" {
		t.Errorf("Slug = %q, want %q", ref.Slug, "user@example.com")
	}
	if ref.FieldName != "token" {
		t.Errorf("FieldName = %q, want %q", ref.FieldName, "token")
	}
	if ref.Raw != "jingui://gmail/user@example.com/token" {
		t.Errorf("Raw = %q", ref.Raw)
	}
	// Transitional aliases
	if ref.AppID != ref.Service || ref.SecretName != ref.Slug {
		t.Errorf("transitional aliases not populated")
	}
}

func TestParse_AllFields(t *testing.T) {
	tests := []struct {
		ref     string
		service string
		slug    string
		field   string
	}{
		{"jingui://gmail/work/token", "gmail", "work", "token"},
		{"jingui://github/foo@bar.com/pat", "github", "foo@bar.com", "pat"},
	}
	for _, tt := range tests {
		r, err := Parse(tt.ref)
		if err != nil {
			t.Errorf("Parse(%q): %v", tt.ref, err)
			continue
		}
		if r.Service != tt.service || r.Slug != tt.slug || r.FieldName != tt.field {
			t.Errorf("Parse(%q) = {%q, %q, %q}, want {%q, %q, %q}", tt.ref,
				r.Service, r.Slug, r.FieldName, tt.service, tt.slug, tt.field)
		}
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
	if IsRef("https://foo/bar") {
		t.Error("expected false for non-jingui prefix")
	}
	if IsRef("") {
		t.Error("expected false for empty string")
	}
}
