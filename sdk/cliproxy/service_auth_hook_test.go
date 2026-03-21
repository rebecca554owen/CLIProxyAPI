package cliproxy

import (
	"context"
	"os"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

func TestServiceAuthHookSyncsModelRegistryOnAuthUpdate(t *testing.T) {
	t.Parallel()

	svc := &Service{
		cfg: &config.Config{
			AuthDir: t.TempDir(),
		},
	}
	if err := svc.ensureDefaults(); err != nil {
		t.Fatalf("ensureDefaults() error = %v", err)
	}

	auth := &coreauth.Auth{
		ID:       "codex-test-plus.json",
		Provider: "codex",
		Label:    "codex-test",
		Attributes: map[string]string{
			"path":      "codex-test-plus.json",
			"plan_type": "plus",
		},
		Metadata: map[string]any{
			"type": "codex",
		},
		Status: coreauth.StatusActive,
	}

	reg := registry.GetGlobalRegistry()
	reg.UnregisterClient(auth.ID)
	t.Cleanup(func() {
		reg.UnregisterClient(auth.ID)
	})

	if _, err := svc.coreManager.Register(context.Background(), auth); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if got := reg.GetModelsForClient(auth.ID); len(got) == 0 {
		t.Fatalf("expected registered models for enabled auth, got none")
	}

	auth.Disabled = true
	auth.Status = coreauth.StatusDisabled
	if _, err := svc.coreManager.Update(context.Background(), auth); err != nil {
		t.Fatalf("Update(disable) error = %v", err)
	}
	if got := reg.GetModelsForClient(auth.ID); len(got) != 0 {
		t.Fatalf("expected no registered models for disabled auth, got %d", len(got))
	}

	auth.Disabled = false
	auth.Status = coreauth.StatusActive
	if _, err := svc.coreManager.Update(context.Background(), auth); err != nil {
		t.Fatalf("Update(enable) error = %v", err)
	}
	if got := reg.GetModelsForClient(auth.ID); len(got) == 0 {
		t.Fatalf("expected registered models after re-enable, got none")
	}
}

func TestServiceSyncLoadedAuthModelsRegistersPersistedAuths(t *testing.T) {
	t.Parallel()

	authDir := t.TempDir()
	raw := []byte(`{"type":"codex","disabled":false,"id_token":"header.payload.sig"}`)
	if err := os.WriteFile(authDir+"/codex-startup-plus.json", raw, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	tokenStore := sdkAuth.GetTokenStore()
	dirSetter, ok := tokenStore.(interface{ SetBaseDir(string) })
	if !ok {
		t.Fatal("token store does not support SetBaseDir")
	}
	dirSetter.SetBaseDir(authDir)

	svc := &Service{
		cfg: &config.Config{
			AuthDir: authDir,
		},
	}
	if err := svc.ensureDefaults(); err != nil {
		t.Fatalf("ensureDefaults() error = %v", err)
	}
	if err := svc.coreManager.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	auth, ok := svc.coreManager.GetByID("codex-startup-plus.json")
	if !ok || auth == nil {
		t.Fatal("expected persisted auth to be loaded")
	}
	auth.Attributes["plan_type"] = "plus"

	reg := registry.GetGlobalRegistry()
	reg.UnregisterClient(auth.ID)
	t.Cleanup(func() {
		reg.UnregisterClient(auth.ID)
	})

	svc.syncLoadedAuthModels()

	if got := reg.GetModelsForClient(auth.ID); len(got) == 0 {
		t.Fatalf("expected registered models for loaded auth, got none")
	}
}
