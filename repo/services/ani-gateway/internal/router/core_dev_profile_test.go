package router

import "testing"

func requireLocalCoreDevProfile(t *testing.T, got coreDevProfileResponse, provider string) {
	t.Helper()
	if got.Mode != "local" || got.Provider != provider || got.RealProvider {
		t.Fatalf("dev profile = %+v, want local %s with real_provider=false", got, provider)
	}
	if got.Reason == "" {
		t.Fatalf("dev profile reason is empty")
	}
}
