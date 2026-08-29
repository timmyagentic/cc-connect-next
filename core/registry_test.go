package core

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type stubPlatform struct{ n string }

func (s *stubPlatform) Name() string                                   { return s.n }
func (s *stubPlatform) Start(MessageHandler) error                     { return nil }
func (s *stubPlatform) Reply(_ context.Context, _ any, _ string) error { return nil }
func (s *stubPlatform) Send(_ context.Context, _ any, _ string) error  { return nil }
func (s *stubPlatform) Stop() error                                    { return nil }

func TestRegisterAndCreatePlatform(t *testing.T) {
	RegisterPlatform("test-plat", func(opts map[string]any) (Platform, error) {
		return &stubPlatform{n: "test-plat"}, nil
	})

	p, err := CreatePlatform("test-plat", nil)
	if err != nil {
		t.Fatalf("CreatePlatform: %v", err)
	}
	if p.Name() != "test-plat" {
		t.Errorf("Name() = %q, want test-plat", p.Name())
	}
}

func TestCreatePlatform_Unknown(t *testing.T) {
	_, err := CreatePlatform("nonexistent-xyz", nil)
	if err == nil {
		t.Error("expected error for unknown platform")
	}
}

func TestValidatePlatformOptions_DoesNotConstructPlugin(t *testing.T) {
	constructed := 0
	RegisterPlatform("test-validated-plat", func(opts map[string]any) (Platform, error) {
		constructed++
		return &stubPlatform{n: "test-validated-plat"}, nil
	})
	RegisterPlatformOptionsValidator("test-validated-plat", func(opts map[string]any) error {
		return errors.New("invalid test options")
	})

	if err := ValidatePlatformOptions("test-validated-plat", map[string]any{"invalid": true}); err == nil {
		t.Fatal("ValidatePlatformOptions() accepted invalid options")
	}
	if constructed != 0 {
		t.Fatalf("side-effect-free validation constructed plugin %d times", constructed)
	}
	if _, err := CreatePlatform("test-validated-plat", map[string]any{"invalid": true}); err == nil {
		t.Fatal("CreatePlatform() bypassed the registered option validator")
	}
	if constructed != 0 {
		t.Fatalf("CreatePlatform() constructed plugin after validation failure %d times", constructed)
	}
}

func TestCreateAgent_Unknown(t *testing.T) {
	_, err := CreateAgent("nonexistent-xyz", nil)
	if err == nil {
		t.Error("expected error for unknown agent")
	}
}

func TestValidateConfigOptionContract_EnforcesRequiredTypeAndBounds(t *testing.T) {
	minimum, maximum := 1.0, 10.0
	options := []ConfigOption{
		{Key: "token", Type: "string", Requirement: ConfigRequirementRequired},
		{Key: "window", Type: "integer", Minimum: &minimum, Maximum: &maximum},
		{Key: "targets", Type: "string | string[]"},
		{Key: "mode", Type: "string", Values: []string{"safe", "fast"}, ClosedValues: true},
	}
	tests := []struct {
		name    string
		values  map[string]any
		wantErr string
	}{
		{name: "required", values: map[string]any{}, wantErr: `option "token" is required`},
		{name: "type", values: map[string]any{"token": "x", "window": "2"}, wantErr: `option "window" must be integer`},
		{name: "minimum", values: map[string]any{"token": "x", "window": int64(0)}, wantErr: `option "window" must be >= 1`},
		{name: "maximum", values: map[string]any{"token": "x", "window": 11}, wantErr: `option "window" must be <= 10`},
		{name: "enum", values: map[string]any{"token": "x", "window": 2, "mode": "unknown"}, wantErr: `option "mode" must be one of safe, fast`},
		{name: "string union", values: map[string]any{"token": "x", "window": 2, "targets": "a"}},
		{name: "array union", values: map[string]any{"token": "x", "window": 2, "targets": []any{"a", "b"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfigOptionContract("demo", options, tt.values)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateConfigOptionContract() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}
