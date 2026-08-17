// Package catalog loads and validates the config that describes what may be
// benched: subjects, agent tools, models and repositories.
//
// It exists so that ecosystem facts — CLI flags, config paths, model
// availability, auth modes — are data rather than code. Those facts change
// without warning and they are not the engine's business. The test that the
// separation holds is that adding an agent tool, a model or a competitor is a
// new JSON file and nothing else.
//
// It is pure: it takes a directory of files and returns data. It runs nothing.
package catalog

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Kind is what a subject is: the treatment under test.
type Kind string

const (
	// Baseline is the arm with no code-intelligence treatment at all.
	Baseline Kind = "baseline"
	// Sense is Sense itself, at some version.
	Sense Kind = "sense"
	// Competitor is somebody else's tool.
	Competitor Kind = "competitor"
)

// Subject is a code-intelligence treatment.
//
// Install, setup and cleanup commands are deliberately absent. Cycle 08 is the
// first thing that runs them, and a schema for commands nobody executes is
// written from imagination. They arrive with the code that runs them.
//
// The same rule trimmed this struct on review. A first draft carried version,
// executor, config paths, model aliases, a retired flag, frameworks and two
// multi-flag capability structs — thirteen fields nothing read. Because decode
// refuses unknown fields, a guessed field is load-bearing the day it ships: it
// cannot be dropped without editing every config file. Fields arrive with their
// consumer.
type Subject struct {
	ID   string `json:"id"`
	Kind Kind   `json:"kind"`
	// NeedsMCP says this treatment reaches the agent through an MCP server, so
	// it can only be driven by a tool that speaks one.
	NeedsMCP bool `json:"needs_mcp"`
	// NeedsIsolatedConfig says this treatment writes agent configuration, so it
	// may only run somewhere that configuration cannot escape onto the host.
	NeedsIsolatedConfig bool `json:"needs_isolated_config"`
	// Executor is where this subject runs.
	Executor string `json:"executor"`
	// Agents are the agent tool ids this subject can be driven through.
	Agents []string `json:"agents"`
}

// Agent is the harness that runs a model: the CLI, and how to talk to it.
//
// Everything about HOW a model is invoked lives here. The moment a model file
// knows a CLI flag, the separation is gone and the matrix stops being data.
type Agent struct {
	ID     string `json:"id"`
	Binary string `json:"binary"`
	// ModelFlag is the flag that selects a model, e.g. "--model".
	ModelFlag string `json:"model_flag"`
	// ConfigDirs are the directories this tool keeps per-user state in, under
	// HOME. Config rather than a constant for the same reason the binary is:
	// the contamination checks look inside them, and a directory compiled in
	// would have those checks looking in the right place for one tool and the
	// wrong place for every other.
	//
	// A LIST, because opencode keeps two — `.config/opencode` and
	// `.local/share/opencode` — and a single field would check one and call the
	// arm clean, which is half the configured surface silently missing.
	ConfigDirs []string `json:"config_dirs"`
	// ConfigDirVar is the environment variable that points the tool at a config
	// directory, e.g. "CLAUDE_CONFIG_DIR". It is how a run is handed its own
	// credential, so it is the door authentication comes through — per tool,
	// because no two tools spell it alike and a tool that has none takes none.
	ConfigDirVar string `json:"config_dir_var"`
	// KeychainService is the item the tool keeps its login under in the platform
	// credential store, read once by the attended parent so a run never has to
	// reach a keychain of its own. Empty means this tool has no such store, and
	// then the config directory is the only source.
	KeychainService string `json:"keychain_service"`
	// CredentialKey is the object the credential lives under inside that store's
	// document. Config rather than a constant for the same reason as the rest:
	// a key compiled in would be right for one tool and read nothing for the
	// next, and a run with no credential looks exactly like a model with
	// nothing to say.
	CredentialKey string `json:"credential_key"`
	// HeadlessArgs drive the tool with no terminal attached. They live here
	// rather than in code because they are ecosystem facts: they change when
	// somebody else ships a release, and no two tools spell them alike.
	HeadlessArgs []string `json:"headless_args"`
	// JudgeArgs drive the tool TOOL-LESS and single-turn, for grading.
	//
	// They are per tool rather than global because no two tools spell it alike,
	// and because a tool that cannot guarantee tool-less and single-turn falls
	// back to a direct API call — which is a decision recorded against that
	// tool, in its README, not one taken once for all of them.
	//
	// A judge with tools is a different instrument from one without: it may
	// verify claims itself, whether it does varies run to run, and which
	// instrument graded a run would not be visible in the number.
	JudgeArgs []string `json:"judge_args"`
	// WallNoteFlag and WallNote tell an arm how long it has.
	//
	// The CLI has no wall-clock parameter, so the number reaches the agent as a
	// system prompt. It enforces nothing — the supervisor's kill is still the
	// hard stop — and it exists so an arm can spend its clock deliberately
	// instead of being cut mid-answer.
	//
	// This is not a nicety. It was measured absent on 2026-08-17, replaying the
	// banked mastodon cell: both arms ran to their ceiling and were cut, 82 and
	// 72 turns, and the scorer refused both captures as incomplete. The banked
	// arms, which had the note, finished with time in hand.
	//
	// WallNote carries {{seconds}}, replaced with the arm's own wall. Each arm
	// gets its own number, which is the whole point: the two walls differ.
	//
	// The WORDING is load-bearing and is copied from the instrument this one
	// replaces, which recorded what a rewrite costs. An earlier phrasing offered
	// "if you run short, say where you stopped", and both arms took the exit
	// immediately: the sense arm used 143s of 480 and wrote that it had not
	// finished with 337 seconds still in hand. Reword this and re-measure, or do
	// not reword it.
	WallNoteFlag string `json:"wall_note_flag"`
	WallNote     string `json:"wall_note"`
	// Env is added to the session environment, as KEY=VALUE. Same reason.
	Env []string `json:"env"`
	// SupportsMCP bounds which subjects this tool can carry.
	SupportsMCP bool `json:"supports_mcp"`
	// AuthModes are how this tool can be authenticated, e.g. subscription, api_key.
	AuthModes []string `json:"auth_modes"`
}

// Model is a set of FACTS about a model. Never mechanics for invoking one:
// those belong to the agent tool that drives it.
type Model struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	// Aliases are other ids the same model answers to.
	Aliases []string `json:"aliases"`
	// AvailableUnder lists the auth modes that can reach this model.
	AvailableUnder []string `json:"available_under"`
	// Agents are the agent tool ids that can drive this model.
	Agents []string `json:"agents"`
}

// Executor is where and how a run happens. Two facts about it matter before
// anything spawns, and both are load-bearing:
//
//   - which auth modes survive into the session. A container that never
//     receives credentials cannot reach a model that needs them, and that is a
//     planning-time fact rather than a run-time surprise
//   - whether it isolates global config, which is what a subject that writes
//     agent configuration depends on
//
// Cycle 03 implements isolated-home and cycle 08 the container. Declaring them
// as data now is what lets the planner refuse an impossible combination before
// either exists.
// Nothing validates an executor beyond its id, which is claimed at decode time.
// An executor preserving no auth mode at all is legal and deliberate: the
// container never receives credentials.
type Executor struct {
	ID string `json:"id"`
	// PreservesAuth lists the auth modes that reach a session run this way.
	PreservesAuth []string `json:"preserves_auth"`
	// IsolatesGlobalConfig says the session gets its own agent configuration
	// rather than the host's.
	IsolatesGlobalConfig bool `json:"isolates_global_config"`
}

// Repo is the code under study, at a pinned commit.
//
// A vertical is a query over this metadata rather than a directory tree, which
// is why languages, frameworks and stack live here. `cohort` is deliberately
// absent: nothing selects on it yet.
type Repo struct {
	ID        string   `json:"id"`
	URL       string   `json:"url"`
	Commit    string   `json:"commit"`
	Languages []string `json:"languages"`
	Stack     string   `json:"stack"`
}

// ErrNoConfig is returned when the config directory is not there at all. It is
// its own error because "you have not pointed me at a catalog" and "your
// catalog is wrong" are different problems for the person reading the message.
var ErrNoConfig = errors.New("no config directory")

// Catalog is everything the config directory describes.
type Catalog struct {
	// claimed maps every id and alias to the file that took it, so a collision
	// can name both sides.
	claimed  map[string]string
	Subjects map[string]Subject
	Agents   map[string]Agent
	// Models is the set of models, one entry per model file. Aliases are NOT
	// in here: a lookup index masquerading as the set makes the catalog list a
	// model that does not exist and makes the validator report one fault twice.
	Models map[string]Model
	// byName resolves a model by its id OR any of its aliases.
	byName    map[string]Model
	Repos     map[string]Repo
	Executors map[string]Executor
}

// Load reads a config directory and validates it.
//
// Validation depth follows consumers: identifiers resolve, referenced ids
// exist, required fields are present. Full schemas for every field wait until
// something depends on those fields, because a rule for a field nobody reads is
// a rule written from imagination.
func Load(dir string) (*Catalog, error) {
	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNoConfig, err)
	}
	c := &Catalog{
		claimed:   map[string]string{},
		Subjects:  map[string]Subject{},
		Agents:    map[string]Agent{},
		Models:    map[string]Model{},
		byName:    map[string]Model{},
		Repos:     map[string]Repo{},
		Executors: map[string]Executor{},
	}
	if err := loadInto(dir, c); err != nil {
		return nil, err
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// loadInto reads all four kinds of file. Subjects and agents are directories
// with a JSON file inside, because each carries a README beside it; models and
// repos are plain files.
func loadInto(dir string, c *Catalog) error {
	if err := readNested(filepath.Join(dir, "subjects"), "subject.json", c.addSubject); err != nil {
		return err
	}
	if err := readNested(filepath.Join(dir, "agents"), "agent.json", c.addAgent); err != nil {
		return err
	}
	if err := readFlat(filepath.Join(dir, "models"), c.addModel); err != nil {
		return err
	}
	if err := readFlat(filepath.Join(dir, "repos"), c.addRepo); err != nil {
		return err
	}
	return readFlat(filepath.Join(dir, "executors"), c.addExecutor)
}

func (c *Catalog) addSubject(path string, b []byte) error {
	var s Subject
	if err := decode(path, b, &s); err != nil {
		return err
	}
	if err := c.claim(path, "subject", s.ID); err != nil {
		return err
	}
	c.Subjects[s.ID] = s
	return nil
}

func (c *Catalog) addAgent(path string, b []byte) error {
	var a Agent
	if err := decode(path, b, &a); err != nil {
		return err
	}
	if err := c.claim(path, "agent", a.ID); err != nil {
		return err
	}
	c.Agents[a.ID] = a
	return nil
}

func (c *Catalog) addModel(path string, b []byte) error {
	var m Model
	if err := decode(path, b, &m); err != nil {
		return err
	}
	if err := c.claim(path, "model", m.ID); err != nil {
		return err
	}
	c.Models[m.ID] = m
	c.byName[m.ID] = m
	// An alias is a name the same model answers to. They are indexed rather
	// than merely recorded, because the failure this package exists to prevent
	// was exactly a model named one way and offered another: a bare
	// `mistral-large-3:cloud` that resolved to nothing. A field documenting
	// that confusion with no lookup behind it is how a reader concludes it is
	// handled.
	for _, alias := range m.Aliases {
		if err := c.claim(path, "model alias", alias); err != nil {
			return err
		}
		c.byName[alias] = m
	}
	return nil
}

// Model resolves a model by its id or any of its aliases.
//
// The alias lookup exists because the failure this package is named for was a
// model called one way and offered another. It is a separate index rather than
// extra entries in Models, so the catalog never lists a model twice and the
// validator never reports one fault once per alias.
// It consults the set as well as the index, so a Catalog built as a literal —
// which every caller outside this package does in its tests — resolves ids even
// though it has no alias index. A method that silently finds nothing on a
// hand-built value is a trap.
func (c *Catalog) Model(name string) (Model, bool) {
	if m, ok := c.byName[name]; ok {
		return m, true
	}
	m, ok := c.Models[name]
	return m, ok
}

func (c *Catalog) addRepo(path string, b []byte) error {
	var r Repo
	if err := decode(path, b, &r); err != nil {
		return err
	}
	if err := c.claim(path, "repo", r.ID); err != nil {
		return err
	}
	c.Repos[r.ID] = r
	return nil
}

func (c *Catalog) addExecutor(path string, b []byte) error {
	var e Executor
	if err := decode(path, b, &e); err != nil {
		return err
	}
	if err := c.claim(path, "executor", e.ID); err != nil {
		return err
	}
	c.Executors[e.ID] = e
	return nil
}

// claim reserves an id, refusing an empty one and refusing to overwrite an id
// something else already took.
//
// Both failures are silent otherwise. An entry with no id becomes an anonymous
// validation error nobody can locate; a duplicate lets the last file read win
// and quietly discards the other, which is precisely the "setting that looks
// applied and is not" that DisallowUnknownFields below exists to stop. Ids and
// filenames are independent here by design — `models/glm-5.2_cloud.json` holds
// id `glm-5.2:cloud` — so nothing else would notice.
//
// The message names BOTH files, because which one is read first depends on
// filename order and the one that trips the check is often the innocent one.
func (c *Catalog) claim(path, kind, id string) error {
	if id == "" {
		return fmt.Errorf("%s: no id", path)
	}
	if first, dup := c.claimed[id]; dup {
		return fmt.Errorf("%s: %s id %q is already claimed by %s", path, kind, id, first)
	}
	c.claimed[id] = path
	return nil
}

// decode parses one config file. Unknown fields are refused: a typo'd key that
// parses silently is a setting that looks applied and is not, which is the
// failure this whole package exists to prevent.
func decode(path string, b []byte, into any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// readNested reads <dir>/<id>/<name>, which is the shape for config that has a
// README beside it.
func readNested(dir, name string, add func(string, []byte) error) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read %s: %w", dir, err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name(), name)
		b, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if err := add(path, b); err != nil {
			return err
		}
	}
	return nil
}

// readFlat reads every .json file directly in dir.
func readFlat(dir string, add func(string, []byte) error) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if err := add(path, b); err != nil {
			return err
		}
	}
	return nil
}

// IDs returns the ids of a catalog map, sorted, so output is stable.
func IDs[T any](m map[string]T) []string {
	return slices.Sorted(maps.Keys(m))
}

// wallNoteSeconds is the placeholder WallNote carries for the arm's own wall.
const wallNoteSeconds = "{{seconds}}"

// WallNoteArgs renders this tool's wall note for an arm that has this long,
// ready to append to the arm's arguments.
//
// It returns nothing when the tool declares no note, so a tool that has no way
// to carry one runs exactly as it did before rather than being handed an empty
// flag. Seconds are truncated rather than rounded: a note claiming one second
// more than the supervisor allows is a note that lies in the direction that
// costs the arm its answer.
func (a Agent) WallNoteArgs(wall time.Duration) []string {
	if a.WallNoteFlag == "" || a.WallNote == "" {
		return nil
	}
	seconds := strconv.Itoa(int(wall.Seconds()))
	return []string{a.WallNoteFlag, strings.ReplaceAll(a.WallNote, wallNoteSeconds, seconds)}
}
