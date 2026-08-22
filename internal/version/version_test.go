package version

import "testing"

func TestVersionSetAtBuildTime(t *testing.T) {
	Version = "1.2.3"
	defer func() { Version = "" }()
	if got := compute(); got != "1.2.3" {
		t.Errorf("compute() = %q", got)
	}
}

func TestVersionFallback(t *testing.T) {
	if got := compute(); got == "" {
		t.Error("empty version")
	}
}
