package dto

import (
	"testing"

	"go.lumeweb.com/queryutil"
)

func TestQuotaPlanResponse_FieldEnums(t *testing.T) {
	schema := queryutil.NewSchemaProvider().ForType(&QuotaPlanResponse{})
	enums := schema.FieldEnums()

	if len(enums) != 0 {
		t.Errorf("QuotaPlanResponse should have no enum fields, got %d: %v", len(enums), enums)
	}
}

func TestAllowanceGrantResponse_FieldEnums(t *testing.T) {
	schema := queryutil.NewSchemaProvider().ForType(&AllowanceGrantResponse{})
	enums := schema.FieldEnums()

	type enumCheck struct {
		field   string
		values  []string
		exists  bool
	}

	expectations := []enumCheck{
		{"type", []string{"STORAGE", "UPLOAD", "DOWNLOAD"}, true},
		{"source", []string{"SUBSCRIPTION", "PAYG_ADDON", "BONUS", "PROMO"}, true},
	}

	for _, exp := range expectations {
		values, ok := enums[exp.field]
		if !ok {
			t.Errorf("expected enum field %q, not found in %v", exp.field, enums)
			continue
		}
		if len(values) != len(exp.values) {
			t.Errorf("field %q: expected %d enum values, got %d (%v)", exp.field, len(exp.values), len(values), values)
			continue
		}
		for i, v := range exp.values {
			if values[i] != v {
				t.Errorf("field %q[%d]: expected %q, got %q", exp.field, i, v, values[i])
			}
		}
	}

	// Verify no unexpected enum fields
	if len(enums) != len(expectations) {
		t.Errorf("expected %d enum fields, got %d: %v", len(expectations), len(enums), enums)
	}
}

func TestUserQuotaConfigResponse_FieldEnums(t *testing.T) {
	schema := queryutil.NewSchemaProvider().ForType(&UserQuotaConfigResponse{})
	enums := schema.FieldEnums()

	values, ok := enums["enforcement_policy"]
	if !ok {
		t.Fatalf("expected enum field \"enforcement_policy\", not found in %v", enums)
	}

	expected := []string{"HARD_LIMITS", "UNLIMITED", "ALLOWANCE", "THRESHOLD"}
	if len(values) != len(expected) {
		t.Fatalf("expected %d enum values, got %d (%v)", len(expected), len(values), values)
	}
	for i, v := range expected {
		if values[i] != v {
			t.Errorf("enforcement_policy[%d]: expected %q, got %q", i, v, values[i])
		}
	}
}
