package enrollment

import (
	"strings"
	"testing"
)

func TestLinuxScriptResetsExistingTailscaleConfig(t *testing.T) {
	script := LinuxScript("https://headscale.example.com", "tskey-example", "node-1")
	if !strings.Contains(script, "sudo tailscale up \\\n  --reset \\") {
		t.Fatalf("Linux enrollment script should reset existing Tailscale settings:\n%s", script)
	}
}
