package session

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// The help of a codex that has the background server (0.157), trimmed.
const helpWithDaemon = `Codex CLI

Usage: codex [OPTIONS] [PROMPT]

Options:
      --dangerously-bypass-approvals-and-sandbox
          Skip all confirmation prompts and execute commands without sandboxing
      --no-daemon
          Run without the shared background server, even if it is already running
`

// The same from a codex that predates it (0.155, 0.156), and would refuse the
// flag.
const helpWithoutDaemon = `Codex CLI

Usage: codex [OPTIONS] [PROMPT]

Options:
      --dangerously-bypass-approvals-and-sandbox
          Skip all confirmation prompts and execute commands without sandboxing
`

// fakeCodex answers `codex --help` with help, or fails when helpErr is set, and
// counts how often it was asked.
type fakeCodex struct {
	help    string
	helpErr error
	asked   int
}

// installFakeCodex puts a fake codex in place of the real one, from the
// defaults: no daemon, nothing probed, no config file.
func installFakeCodex(t *testing.T, help string) *fakeCodex {
	t.Helper()
	fake := &fakeCodex{help: help}

	oldHelp, oldResolve, oldConfig := agentHelpText, resolveAgentCommand, appConfigPath
	agentHelpText = func(command string) (string, error) {
		fake.asked++
		if fake.helpErr != nil {
			return "", fake.helpErr
		}
		return fake.help, nil
	}
	resolveAgentCommand = func(command string) (string, error) {
		return "/fake/bin/" + command, nil
	}
	missing := filepath.Join(t.TempDir(), "config.json")
	appConfigPath = func() (string, error) { return missing, nil }

	SetCodexUseDaemon(false)
	resetAgentFlagProbes()
	t.Cleanup(func() {
		agentHelpText, resolveAgentCommand, appConfigPath = oldHelp, oldResolve, oldConfig
		SetCodexUseDaemon(false)
		resetAgentFlagProbes()
	})
	return fake
}

func codexYoloInstance() *Instance {
	return &Instance{Agent: AgentCodex, AutoYes: true}
}

// A new YOLO Codex session is kept off the background server, and the bypass
// flag is still passed alongside.
//
// In daemon mode the session's approval and sandbox settings do not reach the
// thread, so without --no-daemon a YOLO session asks for approval in a sandbox.
func TestCodexStartsWithoutItsDaemonByDefault(t *testing.T) {
	fake := installFakeCodex(t, helpWithDaemon)
	inst := codexYoloInstance()
	config := inst.GetAgentConfig()

	got := config.CommandLine(inst.startArgs(config, "")...)

	want := "codex " + codexBypassFlag + " --no-daemon"
	if got != want {
		t.Errorf("start command = %q, want %q", got, want)
	}
	if fake.asked != 1 {
		t.Errorf("codex --help ran %d times, want once", fake.asked)
	}
}

// A resume is a subcommand, and the flag has to land on it — after the app's
// own arguments, not before `resume`.
func TestAResumedCodexSessionGetsTheFlagOnTheSubcommand(t *testing.T) {
	installFakeCodex(t, helpWithDaemon)
	id := "019a0000-0000-7000-8000-000000000001"
	inst := codexYoloInstance()
	config := inst.GetAgentConfig()

	got := config.CommandLine(inst.startArgs(config, id)...)

	want := "codex resume " + codexBypassFlag + " " + id + " --no-daemon"
	if got != want {
		t.Errorf("resume command = %q, want %q", got, want)
	}
}

// A stopped tab restarted on its conversation goes the same way.
func TestARestartedCodexTabGetsTheFlagAfterItsResume(t *testing.T) {
	installFakeCodex(t, helpWithDaemon)
	id := "019a0000-0000-7000-8000-000000000002"

	got := (&Instance{}).buildAgentCommand(AgentCodex, "", true, id)

	want := "codex resume " + id + " " + codexBypassFlag + " --no-daemon"
	if got != want {
		t.Errorf("restart command = %q, want %q", got, want)
	}
}

// The resume picker (`codex resume` with no id) is a launch too.
func TestTheCodexResumePickerGetsTheFlag(t *testing.T) {
	installFakeCodex(t, helpWithDaemon)
	config := AgentConfigs[AgentCodex]

	got := config.CommandLine(config.ResumeFlag)

	if want := "codex resume --no-daemon"; got != want {
		t.Errorf("picker command = %q, want %q", got, want)
	}
}

// Already among the arguments: passing it twice makes codex refuse to start.
func TestAFlagAlreadyGivenIsNotRepeated(t *testing.T) {
	installFakeCodex(t, helpWithDaemon)

	got := AgentConfigs[AgentCodex].CommandLine(codexBypassFlag, "--no-daemon")

	if count := strings.Count(got, "--no-daemon"); count != 1 {
		t.Errorf("--no-daemon appears %d times: %q", count, got)
	}
}

// With the daemon chosen in the settings, the flag is left out and codex is
// not even asked.
func TestChoosingTheDaemonLeavesTheFlagOut(t *testing.T) {
	fake := installFakeCodex(t, helpWithDaemon)
	SetCodexUseDaemon(true)
	inst := codexYoloInstance()
	config := inst.GetAgentConfig()

	got := config.CommandLine(inst.startArgs(config, "")...)

	if want := "codex " + codexBypassFlag; got != want {
		t.Errorf("start command = %q, want %q", got, want)
	}
	if fake.asked != 0 {
		t.Errorf("codex --help ran %d times with the daemon chosen", fake.asked)
	}
}

// An older codex does not know the flag and would exit with "unexpected
// argument", so it is not given it.
func TestAnOlderCodexIsNotGivenTheFlag(t *testing.T) {
	installFakeCodex(t, helpWithoutDaemon)

	got := AgentConfigs[AgentCodex].CommandLine(codexBypassFlag)

	if want := "codex " + codexBypassFlag; got != want {
		t.Errorf("start command = %q, want %q", got, want)
	}
}

// A probe that fails must not stop the start: it goes ahead without the flag,
// and the next start asks again rather than trusting the failure.
func TestAFailedProbeStartsCodexWithoutTheFlag(t *testing.T) {
	fake := installFakeCodex(t, helpWithDaemon)
	fake.helpErr = errors.New("timed out")
	config := AgentConfigs[AgentCodex]

	if got := config.CommandLine(); got != "codex" {
		t.Errorf("the flag was added on a failed probe: %q", got)
	}

	fake.helpErr = nil
	if got := config.CommandLine(); got != "codex --no-daemon" {
		t.Errorf("a failed probe was cached; after recovery the command is %q", got)
	}
}

// Codex that is not installed is not probed and gets nothing added.
func TestAMissingCodexIsNotProbed(t *testing.T) {
	fake := installFakeCodex(t, helpWithDaemon)
	resolveAgentCommand = func(string) (string, error) { return "", errors.New("not found") }

	if got := AgentConfigs[AgentCodex].CommandLine(); got != "codex" {
		t.Errorf("command = %q, want plain codex", got)
	}
	if fake.asked != 0 {
		t.Errorf("a missing codex was run %d times", fake.asked)
	}
}

// Asked once, not on every start: several Codex tabs opened together probe
// once. Another codex on PATH is another answer.
func TestTheAnswerIsCachedPerBinary(t *testing.T) {
	fake := installFakeCodex(t, helpWithDaemon)
	config := AgentConfigs[AgentCodex]
	for range 3 {
		config.CommandLine()
	}
	if fake.asked != 1 {
		t.Errorf("codex --help ran %d times, want once", fake.asked)
	}

	resolveAgentCommand = func(command string) (string, error) {
		return "/other/node/bin/" + command, nil
	}
	fake.help = helpWithoutDaemon
	if got := config.CommandLine(); got != "codex" {
		t.Errorf("one binary's answer was used for another: %q", got)
	}
}

// Other agents have no background server, and are neither probed nor changed.
func TestOtherAgentsAreLeftAlone(t *testing.T) {
	fake := installFakeCodex(t, helpWithDaemon)

	got := AgentConfigs[AgentClaude].CommandLine("--dangerously-skip-permissions")

	if want := "claude --dangerously-skip-permissions"; got != want {
		t.Errorf("claude command = %q, want %q", got, want)
	}
	if fake.asked != 0 {
		t.Errorf("an agent without a daemon was probed %d times", fake.asked)
	}
}

// Every way an agent pane is started builds its command in CommandLine — start,
// restore, restart of the main window and of a tab, new tabs, the resume
// picker and the YOLO toggle. A path that joined its own command would start
// Codex on its daemon again, and only for that path.
func TestEveryAgentLaunchGoesThroughCommandLine(t *testing.T) {
	// Building a command from the bare binary name is what a bypassing path
	// looks like: `config.Command + ...`, `agentCmd = config.Command`, or the
	// name put first in an argument list.
	bypass := regexp.MustCompile(`\.Command \+|(agentCmd|cmd|startCmd|resumeCmd) :?= config\.Command\s*$|append\(\w+, config\.Command\)`)
	files := map[string]int{
		"instance.go":               9,
		"../main.go":                1,
		"../ui/handlers_session.go": 2,
		"../ui/handlers_dialogs.go": 3,
	}
	for file, launches := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		for number, line := range strings.Split(source, "\n") {
			if bypass.MatchString(line) {
				t.Errorf("%s:%d builds an agent command without CommandLine: %s",
					file, number+1, strings.TrimSpace(line))
			}
		}
		if got := strings.Count(source, ".CommandLine("); got < launches {
			t.Errorf("%s: CommandLine is used by %d launch paths, want %d", file, got, launches)
		}
	}
}

// Codex on this computer is probed for real, through the codex on PATH.
func TestALocalCodexIsProbed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in codex is a shell script")
	}
	installFakeCodex(t, "")
	agentHelpText, resolveAgentCommand = realAgentHelpText, realResolveAgentCommand

	dir := t.TempDir()
	script := "#!/bin/sh\ncat <<'EOF'\n" + helpWithDaemon + "EOF\n"
	if err := os.WriteFile(filepath.Join(dir, "codex"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if got := AgentConfigs[AgentCodex].CommandLine(); got != "codex --no-daemon" {
		t.Errorf("local command = %q, want codex --no-daemon", got)
	}
}

var realAgentHelpText, realResolveAgentCommand = agentHelpText, resolveAgentCommand

// The flag is matched as a word, not as part of a longer one.
func TestHelpListsFlagMatchesWholeWords(t *testing.T) {
	if !helpListsFlag(helpWithDaemon, "--no-daemon") {
		t.Error("the flag was not found in help that lists it")
	}
	if helpListsFlag("      --no-daemon-restart\n", "--no-daemon") {
		t.Error("a longer flag was taken for this one")
	}
	if !helpListsFlag("  --no-daemon\r\n", "--no-daemon") {
		t.Error("help with Windows line endings was not read")
	}
}

// The setting is read from config.json: missing file or field means no daemon,
// and only an explicit true turns it on.
func TestTheDaemonSettingIsReadFromTheConfigFile(t *testing.T) {
	installFakeCodex(t, "")
	path := filepath.Join(t.TempDir(), "config.json")
	appConfigPath = func() (string, error) { return path, nil }

	cases := []struct {
		name, content string
		want          bool
	}{
		{"no file", "", false},
		{"no field", `{}`, false},
		{"broken file", `{"codex_use_daemon":`, false},
		{"off", `{"codex_use_daemon":false}`, false},
		{"on", `{"codex_use_daemon":true}`, true},
	}
	for _, c := range cases {
		os.Remove(path)
		if c.content != "" {
			if err := os.WriteFile(path, []byte(c.content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if got := readAppConfig().CodexUseDaemon; got != c.want {
			t.Errorf("%s: codex_use_daemon = %v, want %v", c.name, got, c.want)
		}
	}
}
