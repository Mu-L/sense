package cli

import (
	"bytes"
	"strings"
	"testing"
)

// The glossary defines the words the flow deliberately does not print. Every
// one of them is a word somebody meets the moment they open a plan file, an
// artifact or a verdict, so the page is what makes those readable rather than a
// rename that would make the code worse.
func TestTheGlossaryDefinesTheWordsTheFlowDoesNotPrint(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := helpCmd([]string{"concepts"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}

	page := stdout.String()
	for _, want := range []string{"cell", "arm", "sound", "burned", "gold row", "discriminator", "park"} {
		if !strings.Contains(page, want) {
			t.Errorf("the glossary does not define %q:\n%s", want, page)
		}
	}
	// Every word carries a meaning, not just a listing.
	for _, c := range concepts {
		if c.word == "" || c.means == "" {
			t.Errorf("a concept is missing a word or its meaning: %+v", c)
		}
		if !strings.Contains(page, c.means) {
			t.Errorf("the page lists %q without what it means", c.word)
		}
	}
	if !strings.Contains(page, "sense-lab why") {
		t.Errorf("the glossary does not say where the record itself is:\n%s", page)
	}
}

// The words a repository's own files use — the judge and the driver a bench
// declares, the attempts a run tree counts — are defined too. They are the ones
// somebody meets while editing rather than while watching.
func TestTheGlossaryCoversTheWordsTheRunTreeUses(t *testing.T) {
	page := glossary()
	for _, word := range []string{"the judge", "the driver", "attempt"} {
		if !strings.Contains(page, word) {
			t.Errorf("the glossary does not define %q", word)
		}
	}
}

// help with no topic is the usage, which is what somebody typing `sense-lab`
// with nothing else has already been given.
func TestHelpWithNoTopicIsTheUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := helpCmd(nil, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "sense-lab — the bench instrument") {
		t.Errorf("stdout = %q, want the usage", stdout.String())
	}
}

// A topic that does not exist says which one does, rather than printing the
// usage and leaving the reader to guess what they got wrong.
func TestAnUnknownTopicNamesTheOneThatExists(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := helpCmd([]string{"everything"}, &stdout, &stderr)

	if code != exitUsage {
		t.Errorf("exit %d, want a usage error", code)
	}
	if !strings.Contains(stderr.String(), helpTopics) {
		t.Errorf("stderr = %q, want it to name the topic that exists", stderr.String())
	}
}

func TestHelpRefusesAFlagItDoesNotHave(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := helpCmd([]string{"-nonesuch"}, &stdout, &stderr); code != exitUsage {
		t.Errorf("exit %d, want a usage error", code)
	}
}

// The four commands the flow is walked with are on the usage, and the two that
// were deleted are not.
func TestTheUsageNamesTheFlowAndNotWhatWasDeleted(t *testing.T) {
	var stdout, stderr bytes.Buffer
	Run(nil, &stdout, &stderr)

	page := stderr.String()
	for _, want := range []string{"next", "pay", "why", "status"} {
		if !strings.Contains(page, "  "+want+" ") {
			t.Errorf("the usage does not name %q:\n%s", want, page)
		}
	}
	for _, gone := range []string{"  repo ", "  run "} {
		if strings.Contains(page, gone) {
			t.Errorf("the usage still names %q, which this binary does not have:\n%s", gone, page)
		}
	}
}
