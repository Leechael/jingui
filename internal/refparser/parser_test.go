package refparser

import "testing"

func TestParse_Valid(t *testing.T) {
	ref, err := Parse("jingui://gmail-app/user@example.com/client_id")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if ref.AppID != "gmail-app" {
		t.Errorf("AppID = %q, want %q", ref.AppID, "gmail-app")
	}
	if ref.SecretName != "user@example.com" {
		t.Errorf("SecretName = %q, want %q", ref.SecretName, "user@example.com")
	}
	if ref.FieldName != "client_id" {
		t.Errorf("FieldName = %q, want %q", ref.FieldName, "client_id")
	}
	if ref.Raw != "jingui://gmail-app/user@example.com/client_id" {
		t.Errorf("Raw = %q", ref.Raw)
	}
}

func TestParse_AllFields(t *testing.T) {
	tests := []struct {
		ref   string
		app   string
		name  string
		field string
	}{
		{"jingui://app1/user1/refresh_token", "app1", "user1", "refresh_token"},
		{"jingui://my-app/foo@bar.com/client_secret", "my-app", "foo@bar.com", "client_secret"},
	}
	for _, tt := range tests {
		r, err := Parse(tt.ref)
		if err != nil {
			t.Errorf("Parse(%q): %v", tt.ref, err)
			continue
		}
		if r.AppID != tt.app || r.SecretName != tt.name || r.FieldName != tt.field {
			t.Errorf("Parse(%q) = {%q, %q, %q}, want {%q, %q, %q}", tt.ref,
				r.AppID, r.SecretName, r.FieldName, tt.app, tt.name, tt.field)
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
