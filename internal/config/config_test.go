package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "test-key")
	t.Setenv("DEEPSEEK_BASE_URL", "")
	t.Setenv("DEEPSEEK_MODEL", "")
	t.Setenv("REQUEST_TIMEOUT", "")
	t.Setenv("MAX_AGENT_STEPS", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model != "deepseek-flash" || cfg.MaxAgentSteps != 5 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}
