package services

import (
	"strings"
	"testing"

	"minidb/internal/storage"
)

func TestGetAnalyticsConfigCreatesStableInstallationID(t *testing.T) {
	store, err := storage.OpenStore(t.TempDir() + "/data.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	svc := NewSettingsService(store)
	first, err := svc.GetAnalyticsConfig()
	if err != nil {
		t.Fatal(err)
	}
	if first.Enabled {
		t.Fatal("analytics should be disabled by default")
	}
	if !strings.HasPrefix(first.InstallationID, "minidb_") {
		t.Fatalf("installation id = %q, want minidb_ prefix", first.InstallationID)
	}

	second, err := svc.GetAnalyticsConfig()
	if err != nil {
		t.Fatal(err)
	}
	if second.InstallationID != first.InstallationID {
		t.Fatalf("installation id changed: first=%q second=%q", first.InstallationID, second.InstallationID)
	}
}

func TestSaveAnalyticsConfigPreservesInstallationID(t *testing.T) {
	store, err := storage.OpenStore(t.TempDir() + "/data.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	svc := NewSettingsService(store)
	original, err := svc.GetAnalyticsConfig()
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SaveAnalyticsConfig(AnalyticsConfig{
		Enabled:        true,
		InstallationID: "client_supplied_id",
	}); err != nil {
		t.Fatal(err)
	}

	saved, err := svc.GetAnalyticsConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !saved.Enabled {
		t.Fatal("analytics should be enabled after save")
	}
	if saved.InstallationID != original.InstallationID {
		t.Fatalf("installation id = %q, want preserved id %q", saved.InstallationID, original.InstallationID)
	}
}
