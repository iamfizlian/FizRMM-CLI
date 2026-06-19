package enrollment

import (
	"strings"
	"testing"
)

func TestLinuxScriptResetsExistingTailscaleConfig(t *testing.T) {
	script := LinuxScript("https://headscale.example.com", "tskey-example", "node-1", "", "")
	if !strings.Contains(script, "sudo tailscale up \\\n  --reset \\") {
		t.Fatalf("Linux enrollment script should reset existing Tailscale settings:\n%s", script)
	}
	if !strings.Contains(script, "\n  --force-reauth \\") {
		t.Fatalf("Linux enrollment script should force reauth when changing login server:\n%s", script)
	}
}

func TestLinuxScriptInstallsSSHKeyWhenConfigured(t *testing.T) {
	script := LinuxScript("https://headscale.example.com", "tskey-example", "node-1", "rmm", "ssh-ed25519 AAAAexample")
	for _, expected := range []string{
		`SSH_USER="rmm"`,
		`SSH_PUBLIC_KEY="ssh-ed25519 AAAAexample"`,
		`authorized_keys`,
		`systemctl enable --now ssh`,
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("Linux enrollment script missing %q:\n%s", expected, script)
		}
	}
}
