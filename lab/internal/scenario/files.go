package scenario

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// A scenario is three files, each with its own consumer and its own hash:
//
//	<name>.yaml         the agent reads it   editing it means RE-RUN
//	<name>.gold.yaml    the scorer reads it  editing it means RESCORE
//	<name>.rubric.yaml  the judge reads it   editing it means REJUDGE
//
// The split is the whole point. Gold correction is most of the work in this
// bench, and while gold lived inside the scenario file a corrected row looked
// like the arms had seen something different, so the honest response was a
// re-run. Now a gold fix costs a rescore and the transcripts stay valid.
const (
	goldSuffix   = ".gold.yaml"
	rubricSuffix = ".rubric.yaml"
)

// Set is a scenario and its gold and rubric, each with the hash of its own
// content. Three hashes, because they invalidate three different things.
type Set struct {
	Name string
	Path string

	Scenario   Scenario
	Gold       Gold
	Rubric     Rubric
	ScenarioID string
	GoldID     string
	RubricID   string
}

// Gold is what a good answer has to find.
type Gold struct {
	// Discriminator names the gold group carrying the margin — the one whose
	// delta is the headline number.
	//
	// It is the one deliberate addition in the split. It decides the headline
	// and it was a convention held in prose, which means it could be read two
	// ways by two readers and nothing would say so.
	Discriminator string    `yaml:"discriminator"`
	Rows          []GoldRow `yaml:"rows"`
}

// Group returns the rows of one gold group, in file order.
//
// A row with no parseable location is an error rather than a row: left alone it
// becomes a miss nothing can ever satisfy, which silently deflates the score
// and looks exactly like an arm that failed to find the place.
func (g Gold) Group(name string) ([]GoldRow, error) {
	var out []GoldRow
	for _, r := range g.Rows {
		if r.Group != name {
			continue
		}
		if r.Cite() == "" {
			return nil, fmt.Errorf("gold row %q in group %q has no path:line at the front of its relation, "+
				"so nothing could ever match it", r.ID, name)
		}
		out = append(out, r)
	}
	return out, nil
}

// Rubric is what the judge grades.
type Rubric struct {
	Audience string       `yaml:"audience"`
	Steps    []RubricStep `yaml:"steps"`
}

// RubricStep is one step's criteria. The criteria themselves are carried
// unread: what a judge does with them is cycle 04's business, and a schema for
// them written here would be written from imagination.
type RubricStep struct {
	Name     string         `yaml:"name"`
	Criteria map[string]any `yaml:"criteria"`
}

// LoadPath reads the set a scenario file belongs to, deriving its companions
// from its own name.
func LoadPath(path string) (Set, error) {
	name := strings.TrimSuffix(filepath.Base(path), ".yaml")
	return Load(filepath.Dir(path), name)
}

// Load reads a scenario and its two companions from a directory.
//
// name is the scenario's stem: Load(dir, "discourse") reads discourse.yaml,
// discourse.gold.yaml and discourse.rubric.yaml.
func Load(dir, name string) (Set, error) {
	s := Set{Name: name, Path: filepath.Join(dir, name+".yaml")}

	if err := readFile(s.Path, &s.Scenario, &s.ScenarioID); err != nil {
		return Set{}, err
	}
	if err := readFile(filepath.Join(dir, name+goldSuffix), &s.Gold, &s.GoldID); err != nil {
		return Set{}, err
	}
	if err := readFile(filepath.Join(dir, name+rubricSuffix), &s.Rubric, &s.RubricID); err != nil {
		return Set{}, err
	}
	if err := s.validate(); err != nil {
		return Set{}, err
	}
	return s, nil
}

func readFile(path string, into any, id *string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read scenario file: %w", err)
	}
	// Hashed before decoding, because the two fail differently: Hash fails when
	// the YAML is malformed, and the decode fails when it is well-formed YAML
	// that is not this KIND of file. Both are real and the messages differ.
	h, err := Hash(b)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := yaml.Unmarshal(b, into); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	*id = h
	return nil
}

// validate is the schema check: a malformed file must fail the test suite, not
// a paid run forty minutes in.
//
// It checks that the file is a scenario, a gold set or a rubric — not whether
// the gold is GOOD, which is the validator's job in a later pitch and a
// different question.
func (s Set) validate() error {
	var probs []string
	probs = append(probs, s.Scenario.problems()...)
	probs = append(probs, s.Gold.problems()...)
	probs = append(probs, s.Rubric.problems(s.Scenario)...)
	if len(probs) == 0 {
		return nil
	}
	return fmt.Errorf("scenario %s:\n  %s", s.Name, strings.Join(probs, "\n  "))
}

func (s Scenario) problems() []string {
	var p []string
	if s.Name == "" {
		p = append(p, "the scenario has no name")
	}
	if s.Repo == "" {
		p = append(p, "the scenario names no repo, so nothing knows what to run it against")
	}
	if len(s.Steps) == 0 {
		p = append(p, "no steps")
	}
	for i, step := range s.Steps {
		if strings.TrimSpace(step.Prompt) == "" {
			p = append(p, fmt.Sprintf("step %d (%q) has an empty prompt", i+1, step.Name))
		}
	}
	return p
}

func (g Gold) problems() []string {
	var p []string
	if len(g.Rows) == 0 {
		return []string{"the gold file has no rows"}
	}
	if g.Discriminator == "" {
		p = append(p, "no discriminator group named; the headline number would be a convention rather than a fact")
	}
	seen := map[string]bool{}
	var inDiscriminator int
	for _, r := range g.Rows {
		switch {
		case r.ID == "":
			p = append(p, "a gold row has no id")
		case seen[r.ID]:
			p = append(p, fmt.Sprintf("gold row %q appears twice", r.ID))
		}
		seen[r.ID] = true
		if r.Group == "" {
			p = append(p, fmt.Sprintf("gold row %q is in no group", r.ID))
		}
		if r.Group == g.Discriminator {
			inDiscriminator++
		}
	}
	if g.Discriminator != "" && inDiscriminator == 0 {
		p = append(p, fmt.Sprintf("the discriminator group %q has no rows", g.Discriminator))
	}
	return p
}

func (r Rubric) problems(s Scenario) []string {
	var p []string
	if strings.TrimSpace(r.Audience) == "" {
		p = append(p, "the rubric names no audience, so a judge does not know who the answer is for")
	}
	if len(r.Steps) == 0 {
		return append(p, "the rubric has no steps")
	}
	// A rubric that grades a different number of steps than the scenario asks
	// is grading something else, and the mismatch is invisible in a score.
	if len(r.Steps) != len(s.Steps) {
		p = append(p, fmt.Sprintf("the rubric grades %d steps but the scenario asks %d",
			len(r.Steps), len(s.Steps)))
	}
	for i, step := range r.Steps {
		if len(step.Criteria) == 0 {
			p = append(p, fmt.Sprintf("rubric step %d (%q) has no criteria", i+1, step.Name))
		}
	}
	return p
}
