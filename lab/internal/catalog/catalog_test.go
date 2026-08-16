package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// good returns a minimal catalog directory that loads clean. Tests break one
// thing in it at a time, so each failure has exactly one cause.
func good(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeSubject(t, dir, "baseline", Subject{
		ID: "baseline", Kind: Baseline, Agents: []string{"tool"},
	})
	writeAgent(t, dir, "tool", Agent{
		ID: "tool", Binary: "toolbin", ModelFlag: "--model",
		HeadlessArgs: []string{"-p"}, AuthModes: []string{"api_key"},
		SupportsMCP: true,
	})
	writeModel(t, dir, Model{
		ID: "m1", Provider: "acme", AvailableUnder: []string{"api_key"}, Agents: []string{"tool"},
	})
	writeRepo(t, dir, Repo{
		ID: "r1", URL: "https://example.test/r1.git", Commit: "abc123", Languages: []string{"go"},
	})
	return dir
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeSubject(t *testing.T, dir, id string, s Subject) {
	writeJSON(t, filepath.Join(dir, "subjects", id, "subject.json"), s)
}
func writeAgent(t *testing.T, dir, id string, a Agent) {
	writeJSON(t, filepath.Join(dir, "agents", id, "agent.json"), a)
}
func writeModel(t *testing.T, dir string, m Model) {
	writeJSON(t, filepath.Join(dir, "models", strings.NewReplacer("/", "_", ":", "_").Replace(m.ID)+".json"), m)
}
func writeRepo(t *testing.T, dir string, r Repo) {
	writeJSON(t, filepath.Join(dir, "repos", r.ID+".json"), r)
}

func loadErr(t *testing.T, dir string) string {
	t.Helper()
	_, err := Load(dir)
	if err == nil {
		t.Fatal("Load accepted a catalog it should have refused")
	}
	return err.Error()
}

func TestAWellFormedCatalogLoads(t *testing.T) {
	c, err := Load(good(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if c.Subjects["baseline"].Kind != Baseline {
		t.Errorf("subject kind = %q", c.Subjects["baseline"].Kind)
	}
	if c.Agents["tool"].Binary != "toolbin" {
		t.Errorf("agent binary = %q", c.Agents["tool"].Binary)
	}
	if c.Models["m1"].Provider != "acme" {
		t.Errorf("model provider = %q", c.Models["m1"].Provider)
	}
	if c.Repos["r1"].Commit != "abc123" {
		t.Errorf("repo commit = %q", c.Repos["r1"].Commit)
	}
}

// A malformed config must fail `make test`, not a run. Everything below is
// caught at load time, before anything is spawned or spent.
func TestAMalformedCatalogIsRefusedAtLoadTime(t *testing.T) {
	for _, tc := range []struct {
		name   string
		breaks func(t *testing.T, dir string)
		want   string
	}{
		{
			name: "a subject naming an agent that does not exist",
			breaks: func(t *testing.T, dir string) {
				writeSubject(t, dir, "baseline", Subject{ID: "baseline", Kind: Baseline, Agents: []string{"ghost"}})
			},
			want: `names agent "ghost", which no agent file declares`,
		},
		{
			name: "a model naming an agent that does not exist",
			breaks: func(t *testing.T, dir string) {
				writeModel(t, dir, Model{ID: "m1", Provider: "acme", AvailableUnder: []string{"api_key"}, Agents: []string{"ghost"}})
			},
			want: `names agent "ghost"`,
		},
		{
			name: "a subject with a kind that is not one of the three",
			breaks: func(t *testing.T, dir string) {
				writeSubject(t, dir, "baseline", Subject{ID: "baseline", Kind: "helper", Agents: []string{"tool"}})
			},
			want: `kind "helper" is not baseline, sense or competitor`,
		},
		{
			name: "a subject nothing can drive",
			breaks: func(t *testing.T, dir string) {
				writeSubject(t, dir, "baseline", Subject{ID: "baseline", Kind: Baseline})
			},
			want: "names no agent tools",
		},
		{
			name: "a repo with no pinned commit",
			breaks: func(t *testing.T, dir string) {
				writeRepo(t, dir, Repo{ID: "r1", URL: "https://example.test/r1.git", Languages: []string{"go"}})
			},
			want: "no pinned commit",
		},
		{
			name: "an agent that cannot be driven headlessly",
			breaks: func(t *testing.T, dir string) {
				writeAgent(t, dir, "tool", Agent{ID: "tool", Binary: "toolbin", ModelFlag: "--model", AuthModes: []string{"api_key"}})
			},
			want: "no headless args",
		},
		{
			name: "an agent with no way to select a model",
			breaks: func(t *testing.T, dir string) {
				writeAgent(t, dir, "tool", Agent{ID: "tool", Binary: "toolbin", HeadlessArgs: []string{"-p"}, AuthModes: []string{"api_key"}})
			},
			want: "no model flag",
		},
		{
			name: "an unknown field, which is a setting that looks applied and is not",
			breaks: func(t *testing.T, dir string) {
				path := filepath.Join(dir, "repos", "r1.json")
				if err := os.WriteFile(path, []byte(`{"id":"r1","url":"u","commit":"c","languages":["go"],"cohort":"big"}`), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want: "unknown field",
		},
		{
			name: "a file that is not JSON at all",
			breaks: func(t *testing.T, dir string) {
				if err := os.WriteFile(filepath.Join(dir, "repos", "r1.json"), []byte("{oops"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want: "r1.json",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := good(t)
			tc.breaks(t, dir)

			if got := loadErr(t, dir); !strings.Contains(got, tc.want) {
				t.Errorf("error = %q, want it to contain %q", got, tc.want)
			}
		})
	}
}

// The failure this whole package exists to prevent, in its measured form: an
// arm whose model resolved to nothing, so every run came back empty with zero
// tokens and exit 1 — and it failed as a bad result rather than as a crash.
// A model no agent tool can authenticate to is that same shape, and it is
// catchable before any money is spent.
func TestAModelNoAgentCanReachIsRefused(t *testing.T) {
	dir := good(t)
	// The agent authenticates by subscription only; the model needs an api key.
	writeAgent(t, dir, "tool", Agent{
		ID: "tool", Binary: "toolbin", ModelFlag: "--model",
		HeadlessArgs: []string{"-p"}, AuthModes: []string{"subscription"},
		SupportsMCP: true,
	})

	got := loadErr(t, dir)

	if !strings.Contains(got, "nothing could reach it") {
		t.Errorf("error = %q, want it to say the model is unreachable", got)
	}
}

// A subject that registers an MCP server cannot run through a tool that does
// not speak MCP. Catching it here costs nothing; catching it at run time costs
// the run.
func TestASubjectNeedingMCPCannotUseAToolWithoutIt(t *testing.T) {
	dir := good(t)
	writeSubject(t, dir, "sense", Subject{
		ID: "sense", Kind: Sense, Agents: []string{"tool"},
		NeedsMCP: true,
	})
	writeAgent(t, dir, "tool", Agent{
		ID: "tool", Binary: "toolbin", ModelFlag: "--model",
		HeadlessArgs: []string{"-p"}, AuthModes: []string{"api_key"},
		SupportsMCP: false,
	})

	got := loadErr(t, dir)

	if !strings.Contains(got, "needs MCP but agent tool does not support it") {
		t.Errorf("error = %q, want it to name the mismatch", got)
	}
}

// A config directory is edited by hand. Reporting one problem at a time turns a
// five-minute fix into an afternoon, so every problem is reported at once.
func TestEveryProblemIsReportedNotJustTheFirst(t *testing.T) {
	dir := good(t)
	writeRepo(t, dir, Repo{ID: "r1", Languages: []string{"go"}})

	got := loadErr(t, dir)

	for _, want := range []string{"no url", "no pinned commit"} {
		if !strings.Contains(got, want) {
			t.Errorf("error is missing %q:\n%s", want, got)
		}
	}
}

// "You have not pointed me at a catalog" and "your catalog is wrong" are
// different problems for the person reading the message.
func TestAMissingConfigDirectoryIsItsOwnError(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope"))

	if err == nil {
		t.Fatal("Load succeeded with no config directory")
	}
	if !strings.Contains(err.Error(), "no config directory") {
		t.Errorf("error = %q, want it to say the directory is missing", err)
	}
}

func TestAConfigDirectoryMissingOneKindIsAnError(t *testing.T) {
	dir := good(t)
	if err := os.RemoveAll(filepath.Join(dir, "models")); err != nil {
		t.Fatal(err)
	}

	if got := loadErr(t, dir); !strings.Contains(got, "models") {
		t.Errorf("error = %q, want it to name the missing kind", got)
	}
}

func TestIDsAreSortedSoOutputIsStable(t *testing.T) {
	got := IDs(map[string]int{"c": 1, "a": 1, "b": 1})

	want := []string{"a", "b", "c"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("IDs = %v, want %v", got, want)
		}
	}
}

// A config file that cannot be read at all is a different failure from one
// that is wrong, and the message has to say which: "your catalog is malformed"
// sends someone to a linter, "I could not open this file" sends them to
// permissions.
func TestAConfigFileThatCannotBeReadNamesThePath(t *testing.T) {
	for _, tc := range []struct {
		name   string
		breaks func(t *testing.T, dir string)
		want   string
	}{
		{
			name: "a subject directory with no subject.json",
			breaks: func(t *testing.T, dir string) {
				if err := os.MkdirAll(filepath.Join(dir, "subjects", "empty"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			want: filepath.Join("subjects", "empty", "subject.json"),
		},
		{
			name: "an agent directory with no agent.json",
			breaks: func(t *testing.T, dir string) {
				if err := os.MkdirAll(filepath.Join(dir, "agents", "empty"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			want: filepath.Join("agents", "empty", "agent.json"),
		},
		{
			name: "a model file that cannot be opened",
			breaks: func(t *testing.T, dir string) {
				p := filepath.Join(dir, "models", "locked.json")
				if err := os.WriteFile(p, []byte("{}"), 0o000); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(p, 0o600) })
			},
			want: "locked.json",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := good(t)
			tc.breaks(t, dir)

			if got := loadErr(t, dir); !strings.Contains(got, tc.want) {
				t.Errorf("error = %q, want it to name %q", got, tc.want)
			}
		})
	}
}

// Directories under models/ and repos/ are not config files. A catalog that
// tripped over one would be a catalog nobody could organise.
func TestDirectoriesAndNonJSONFilesAreSkipped(t *testing.T) {
	dir := good(t)
	if err := os.MkdirAll(filepath.Join(dir, "models", "archive"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "repos", "NOTES.md"), []byte("# notes"), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.Models) != 1 || len(c.Repos) != 1 {
		t.Errorf("got %d models and %d repos, want 1 and 1", len(c.Models), len(c.Repos))
	}
}

// An entry with no id is refused, and the message NAMES THE FILE. Reported
// without a path it is an anonymous failure: with a dozen subjects declared,
// "a subject has no id" tells the reader nothing about which one to open.
func TestAConfigWithNoIDNamesTheFileItIsIn(t *testing.T) {
	for _, tc := range []struct {
		name  string
		write func(t *testing.T, dir string)
		file  string
	}{
		{"subject", func(t *testing.T, dir string) { writeSubject(t, dir, "anon", Subject{Kind: Baseline}) },
			filepath.Join("subjects", "anon", "subject.json")},
		{"agent", func(t *testing.T, dir string) { writeAgent(t, dir, "anon", Agent{Binary: "b"}) },
			filepath.Join("agents", "anon", "agent.json")},
		{"model", func(t *testing.T, dir string) {
			writeJSON(t, filepath.Join(dir, "models", "anon.json"), Model{Provider: "p"})
		}, filepath.Join("models", "anon.json")},
		{"repo", func(t *testing.T, dir string) {
			writeJSON(t, filepath.Join(dir, "repos", "anon.json"), Repo{URL: "u"})
		}, filepath.Join("repos", "anon.json")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := good(t)
			tc.write(t, dir)

			got := loadErr(t, dir)
			if !strings.Contains(got, "no id") {
				t.Errorf("error = %q, want it to say the entry has no id", got)
			}
			if !strings.Contains(got, tc.file) {
				t.Errorf("error = %q, want it to name %s", got, tc.file)
			}
		})
	}
}

func TestAgentAndModelFieldsThatWouldStopARunAreRequired(t *testing.T) {
	for _, tc := range []struct {
		name  string
		write func(t *testing.T, dir string)
		want  string
	}{
		{"an agent with no binary", func(t *testing.T, dir string) {
			writeAgent(t, dir, "tool", Agent{ID: "tool", ModelFlag: "-m", HeadlessArgs: []string{"-p"}, AuthModes: []string{"api_key"}})
		}, "no binary to spawn"},
		{"an agent with no auth modes", func(t *testing.T, dir string) {
			writeAgent(t, dir, "tool", Agent{ID: "tool", Binary: "b", ModelFlag: "-m", HeadlessArgs: []string{"-p"}})
		}, "no auth modes"},
		{"a model with no provider", func(t *testing.T, dir string) {
			writeModel(t, dir, Model{ID: "m1", AvailableUnder: []string{"api_key"}, Agents: []string{"tool"}})
		}, "no provider"},
		{"a model nothing can drive", func(t *testing.T, dir string) {
			writeModel(t, dir, Model{ID: "m1", Provider: "acme", AvailableUnder: []string{"api_key"}})
		}, "names no agent tools"},
		{"a repo with no languages", func(t *testing.T, dir string) {
			writeRepo(t, dir, Repo{ID: "r1", URL: "u", Commit: "c"})
		}, "no languages"},
		{"a subject with no kind", func(t *testing.T, dir string) {
			writeSubject(t, dir, "baseline", Subject{ID: "baseline", Agents: []string{"tool"}})
		}, "no kind"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := good(t)
			tc.write(t, dir)

			if got := loadErr(t, dir); !strings.Contains(got, tc.want) {
				t.Errorf("error = %q, want %q", got, tc.want)
			}
		})
	}
}

// Malformed JSON is refused for every kind, not just the one that happened to
// get a test. Each kind has its own decode path, so each needs its own case.
func TestMalformedJSONIsRefusedForEveryKind(t *testing.T) {
	for _, tc := range []struct{ name, path string }{
		{"subject", filepath.Join("subjects", "baseline", "subject.json")},
		{"agent", filepath.Join("agents", "tool", "agent.json")},
		{"model", filepath.Join("models", "m1.json")},
		{"repo", filepath.Join("repos", "r1.json")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := good(t)
			if err := os.WriteFile(filepath.Join(dir, tc.path), []byte(`{"id": }`), 0o644); err != nil {
				t.Fatal(err)
			}

			if got := loadErr(t, dir); !strings.Contains(got, filepath.Base(tc.path)) {
				t.Errorf("error = %q, want it to name the file", got)
			}
		})
	}
}

// A README beside a subject is expected; a loose file where a subject
// directory should be is not a subject. Tripping over one would mean the human
// half of the config could not live next to the machine half.
func TestLooseFilesBesideSubjectDirectoriesAreSkipped(t *testing.T) {
	dir := good(t)
	if err := os.WriteFile(filepath.Join(dir, "subjects", "README.md"), []byte("# subjects"), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.Subjects) != 1 {
		t.Errorf("got %d subjects, want 1", len(c.Subjects))
	}
}

// A config directory missing subjects/ or agents/ entirely is an error naming
// the kind, not a catalog that silently has none of them.
func TestAMissingSubjectsOrAgentsDirectoryIsAnError(t *testing.T) {
	for _, kind := range []string{"subjects", "agents"} {
		t.Run(kind, func(t *testing.T) {
			dir := good(t)
			if err := os.RemoveAll(filepath.Join(dir, kind)); err != nil {
				t.Fatal(err)
			}

			if got := loadErr(t, dir); !strings.Contains(got, kind) {
				t.Errorf("error = %q, want it to name %q", got, kind)
			}
		})
	}
}

// Two files claiming one id lets the last one read win and silently discards
// the other — the same "setting that looks applied and is not" that unknown
// fields are refused for. Ids and filenames are independent by design, so
// nothing else would notice.
func TestTwoFilesClaimingOneIDAreRefused(t *testing.T) {
	for _, tc := range []struct {
		name  string
		write func(t *testing.T, dir string)
		want  string
	}{
		{"two models", func(t *testing.T, dir string) {
			writeJSON(t, filepath.Join(dir, "models", "shadow.json"),
				Model{ID: "m1", Provider: "other", AvailableUnder: []string{"api_key"}, Agents: []string{"tool"}})
		}, `model id "m1" is already claimed by`},
		{"two repos", func(t *testing.T, dir string) {
			writeJSON(t, filepath.Join(dir, "repos", "shadow.json"),
				Repo{ID: "r1", URL: "u", Commit: "c", Languages: []string{"go"}})
		}, `repo id "r1" is already claimed by`},
		{"two subjects", func(t *testing.T, dir string) {
			writeSubject(t, dir, "shadow", Subject{ID: "baseline", Kind: Baseline, Agents: []string{"tool"}})
		}, `subject id "baseline" is already claimed by`},
		{"two agents", func(t *testing.T, dir string) {
			writeAgent(t, dir, "shadow", Agent{ID: "tool", Binary: "b", ModelFlag: "-m",
				HeadlessArgs: []string{"-p"}, AuthModes: []string{"api_key"}})
		}, `agent id "tool" is already claimed by`},
		// An alias collides exactly as an id does. Which of the two files
		// trips the check depends on filename order, which is why the message
		// names both sides rather than blaming whichever was read second.
		// The alias side, where the alias is claimed second. Read order decides
		// which of the two trips, so both directions need a case.
		{"an alias claimed after the id it collides with", func(t *testing.T, dir string) {
			writeJSON(t, filepath.Join(dir, "models", "zz-later.json"),
				Model{ID: "m2", Provider: "acme", Aliases: []string{"m1"},
					AvailableUnder: []string{"api_key"}, Agents: []string{"tool"}})
		}, `model alias id "m1" is already claimed by`},
		{"an alias colliding with a model id", func(t *testing.T, dir string) {
			writeJSON(t, filepath.Join(dir, "models", "aliased.json"),
				Model{ID: "m2", Provider: "acme", Aliases: []string{"m1"},
					AvailableUnder: []string{"api_key"}, Agents: []string{"tool"}})
		}, `"m1" is already claimed by`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := good(t)
			tc.write(t, dir)

			got := loadErr(t, dir)
			if !strings.Contains(got, tc.want) {
				t.Errorf("error = %q, want %q", got, tc.want)
			}
			// Both files, always: the one that tripped the check is often the
			// innocent one, and which that is depends on filename order.
			if strings.Count(got, ".json") < 2 {
				t.Errorf("error = %q, want it to name both files", got)
			}
		})
	}
}

// An alias is a name the same model answers to, and it RESOLVES. The measured
// failure was a model named one way and offered another; a field recording that
// confusion with no lookup behind it is how a reader concludes it is handled.
func TestAModelResolvesByItsAliases(t *testing.T) {
	dir := good(t)
	writeModel(t, dir, Model{
		ID: "ollama-cloud/mistral-large-3:675b", Provider: "ollama-cloud",
		Aliases:        []string{"mistral-large-3:675b"},
		AvailableUnder: []string{"api_key"}, Agents: []string{"tool"},
	})

	c, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	byAlias, ok := c.Model("mistral-large-3:675b")
	if !ok {
		t.Fatal("the alias does not resolve; the field records the confusion without fixing it")
	}
	if byAlias.ID != "ollama-cloud/mistral-large-3:675b" {
		t.Errorf("the alias resolved to %q, want the full id", byAlias.ID)
	}

	// An alias resolves, and it is NOT a second model. A lookup index
	// masquerading as the set makes the catalog list a model that does not
	// exist, offers two spellings of one arm to whoever just mistyped one, and
	// makes the validator report a single fault once per alias.
	if _, listed := c.Models["mistral-large-3:675b"]; listed {
		t.Error("the alias is in the model set, so the catalog lists one model twice")
	}
	if len(c.Models) != 2 {
		t.Errorf("the catalog holds %d models, want 2 — one per model file", len(c.Models))
	}
}

// One model with one fault reports one line, however many aliases it has.
// Ranging over an index instead of the set reports it once per alias, which
// works directly against reporting every problem in a readable list.
func TestAnAliasedModelWithAFaultIsReportedOnce(t *testing.T) {
	dir := good(t)
	writeModel(t, dir, Model{
		ID: "m2", Aliases: []string{"m2-alt", "m2-old"},
		AvailableUnder: []string{"api_key"}, Agents: []string{"tool"},
	})

	got := loadErr(t, dir)

	if n := strings.Count(got, "model m2: no provider"); n != 1 {
		t.Errorf("the fault is reported %d times, want once:\n%s", n, got)
	}
}

// An agent whose config declares no MCP support cannot carry a subject that
// needs one, and the reverse pairing must stay legal.
func TestAnAgentWithoutMCPMayStillCarryASubjectThatDoesNotNeedIt(t *testing.T) {
	dir := good(t)
	writeAgent(t, dir, "tool", Agent{
		ID: "tool", Binary: "b", ModelFlag: "-m",
		HeadlessArgs: []string{"-p"}, AuthModes: []string{"api_key"}, SupportsMCP: false,
	})

	if _, err := Load(dir); err != nil {
		t.Errorf("a baseline subject was refused an agent with no MCP: %v", err)
	}
}
