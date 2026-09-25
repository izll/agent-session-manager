package session

import (
	"strings"
	"testing"
)

const codexBypassFlag = "--dangerously-bypass-approvals-and-sandbox"

// Codex removed --full-auto, and exits with "unexpected argument" when given
// it: a YOLO Codex tab has to be started with the bypass flag instead.
func TestCodexYoloUsesTheBypassFlag(t *testing.T) {
	if got := AgentConfigs[AgentCodex].AutoYesFlag; got != codexBypassFlag {
		t.Errorf("Codex's YOLO flag = %q, want %q", got, codexBypassFlag)
	}

	cmd := (&Instance{}).buildAgentCommand(AgentCodex, "", true, "")
	fields := strings.Fields(cmd)
	if indexOfField(fields, codexBypassFlag) < 0 {
		t.Errorf("a YOLO Codex tab starts without the bypass flag: %q", cmd)
	}
	if indexOfField(fields, "--full-auto") >= 0 {
		t.Errorf("a YOLO Codex tab still gets the removed --full-auto: %q", cmd)
	}
}

func indexOfField(fields []string, want string) int {
	for index, field := range fields {
		if field == want {
			return index
		}
	}
	return -1
}
