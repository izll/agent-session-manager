package session

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Codex's shared background server, and why it is off unless asked for.
//
// From 0.157 the terminal `codex` is a client of a background "app-server
// daemon" that it starts on first use and every later codex connects to. Two
// things go wrong with that under this app:
//
//   - The session's settings do not reach the daemon. The thread is created
//     with no approval or sandbox policy, so the daemon applies config.toml's
//     defaults — --dangerously-bypass-approvals-and-sandbox is accepted on the
//     command line and then ignored, and a YOLO session asks for approval in a
//     sandbox. (openai/codex #9144, #14068, #46252.)
//   - The daemon sometimes does not come up in time, and codex exits with
//     "app server did not become ready ... rerun with --no-daemon".
//
// --no-daemon runs the old way, in the pane. It is the default here; the
// codex_use_daemon setting exists for someone who wants the daemon regardless.

// appConfigFile holds the settings that belong to the machine rather than to a
// project, so they live beside projects.json and not in a sessions.json.
const appConfigFile = "config.json"

// appConfig is the shape of appConfigFile. A missing field is its zero value,
// so every setting's default has to be false.
type appConfig struct {
	// CodexUseDaemon lets Codex run on its shared background server. Off by
	// default: in that mode Codex ignores the YOLO flag (see above).
	CodexUseDaemon bool `json:"codex_use_daemon,omitempty"`
}

var (
	codexDaemonSettingOnce sync.Once
	codexUseDaemon         atomic.Bool
)

// appConfigPath returns where appConfigFile lives. A variable so tests can
// point it at a temporary directory.
var appConfigPath = func() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "agent-session-manager", appConfigFile), nil
}

// readAppConfig reads the machine-wide settings. No file, or one that does not
// parse, means the defaults: a broken setting must not stop Codex starting.
func readAppConfig() appConfig {
	var config appConfig
	path, err := appConfigPath()
	if err != nil {
		return config
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return config
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return appConfig{}
	}
	return config
}

// CodexUseDaemon reports whether Codex may use its background server. Read
// from the config file on first use — every process that starts an agent,
// the yolo-confirm helper included, gets the same answer without being told.
func CodexUseDaemon() bool {
	codexDaemonSettingOnce.Do(func() {
		codexUseDaemon.Store(readAppConfig().CodexUseDaemon)
	})
	return codexUseDaemon.Load()
}

// SetCodexUseDaemon overrides the setting for this process.
func SetCodexUseDaemon(use bool) {
	codexDaemonSettingOnce.Do(func() {})
	codexUseDaemon.Store(use)
}

// agentFlagProbeLifetime is how long an answer about a flag is trusted.
//
// Long enough that starting a handful of tabs asks once, short enough that an
// upgrade — `npm i -g @openai/codex` while the app runs — is noticed within
// minutes rather than at the next launch.
const agentFlagProbeLifetime = 10 * time.Minute

// agentFlagProbeTimeout bounds one `<agent> --help`. It runs on the way to
// starting the agent, so a hung probe must not become a hung start.
const agentFlagProbeTimeout = 5 * time.Second

type agentFlagProbeAnswer struct {
	supported bool
	at        time.Time
}

var (
	agentFlagProbeMu    sync.Mutex
	agentFlagProbeCache = map[string]agentFlagProbeAnswer{}
)

// resolveAgentCommand finds the binary a command names. A variable so tests
// need no agent installed.
var resolveAgentCommand = exec.LookPath

// agentHelpText returns what `<command> --help` prints. A variable so tests can
// answer without an agent installed.
var agentHelpText = func(command string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), agentFlagProbeTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, command, "--help").CombinedOutput()
	return string(out), err
}

// agentSupportsFlag reports whether the installed agent lists a flag in its
// help.
//
// Read from the help rather than worked out from a version number: the help is
// what the installed binary actually accepts. Passing a flag an older codex
// does not know makes it exit with "unexpected argument" — so any doubt,
// including a probe that failed, answers no.
//
// Only a successful answer is kept. A failure is asked again next time, rather
// than keeping the flag off for the app's whole lifetime because of one slow
// start.
func agentSupportsFlag(command, flag string) bool {
	// Where the command resolves to is part of the key: switching node
	// versions swaps which codex is on PATH without anything else changing.
	path, err := resolveAgentCommand(command)
	if err != nil {
		return false
	}
	key := command + "\x00" + flag + "\x00" + path

	agentFlagProbeMu.Lock()
	cached, ok := agentFlagProbeCache[key]
	agentFlagProbeMu.Unlock()
	if ok && time.Since(cached.at) < agentFlagProbeLifetime {
		return cached.supported
	}

	help, err := agentHelpText(command)
	if err != nil {
		return false
	}
	supported := helpListsFlag(help, flag)

	agentFlagProbeMu.Lock()
	agentFlagProbeCache[key] = agentFlagProbeAnswer{supported: supported, at: time.Now()}
	agentFlagProbeMu.Unlock()
	return supported
}

// helpListsFlag reports whether help text lists a flag as a whole word, so
// --no-daemon is not found inside, say, --no-daemon-restart.
func helpListsFlag(help, flag string) bool {
	for _, field := range strings.FieldsFunc(help, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == ',' || r == '=' || r == '[' || r == ']'
	}) {
		if field == flag {
			return true
		}
	}
	return false
}

// resetAgentFlagProbes forgets every cached answer. For tests.
func resetAgentFlagProbes() {
	agentFlagProbeMu.Lock()
	agentFlagProbeCache = map[string]agentFlagProbeAnswer{}
	agentFlagProbeMu.Unlock()
}

// noDaemonArgs returns the flag that keeps an agent off its background server,
// or nothing: when the agent has no such server, when the user chose to use
// it, when the app's own arguments already carry it (a repeated flag is an
// error), or when the installed agent does not know the flag.
func (c AgentConfig) noDaemonArgs(args []string) []string {
	if c.NoDaemonFlag == "" || CodexUseDaemon() {
		return nil
	}
	for _, arg := range args {
		if arg == c.NoDaemonFlag {
			return nil
		}
	}
	if !agentSupportsFlag(c.Command, c.NoDaemonFlag) {
		return nil
	}
	return []string{c.NoDaemonFlag}
}

// CommandLine returns the shell command that starts this agent: the command,
// the app's own arguments, then whatever the installed agent needs added.
//
// Every place that launches an agent builds its command here, so none of them
// can start Codex on its background server by forgetting the flag.
//
// The added flag goes after the app's arguments rather than first: for a
// subcommand (codex resume <id>) that puts it on the subcommand, which is
// where it is sure to be read.
func (c AgentConfig) CommandLine(args ...string) string {
	parts := append([]string{c.Command}, args...)
	parts = append(parts, c.noDaemonArgs(args)...)
	return strings.Join(parts, " ")
}
