package mine_test

import (
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/mine"
)

// The capture is what the agent actually asked and what Sense actually
// answered, so the parser is where a wrong answer about a run comes from.
func TestACallIsPairedWithItsOwnReply(t *testing.T) {
	log := strings.Join([]string{
		`{"dir":"c2s","msg":{"id":4,"method":"tools/call","params":{"name":"sense_blast","arguments":{"symbol":"Plan","max_hops":3}}}}`,
		`{"dir":"c2s","msg":{"id":5,"method":"tools/call","params":{"name":"sense_graph","arguments":{"symbol":"Guard"}}}}`,
		`{"dir":"s2c","msg":{"id":5,"result":{"content":[{"type":"text","text":"{\"called_by\":[{\"ref\":\"src/Guard.cs:12\"}]}"}]}}}`,
		`{"dir":"s2c","msg":{"id":4,"result":{"content":[{"type":"text","text":"{\"direct_callers\":[{\"file\":\"src/Plan.cs\"}]}"}]}}}`,
	}, "\n")

	calls := mine.Capture([]byte(log))
	if len(calls) != 2 {
		t.Fatalf("read %d calls, want 2", len(calls))
	}
	// Replies arrive out of order, which is what a concurrent client does. The
	// pairing is by id, so the graph reply must not land on the blast call.
	byTool := map[string]mine.Call{}
	for _, c := range calls {
		byTool[c.Tool] = c
	}
	if got := byTool["sense_graph"].Returned; len(got) != 1 || got[0] != "src/Guard.cs" {
		t.Errorf("the graph call returned %v, want the file from its own reply", got)
	}
	if got := byTool["sense_blast"].Returned; len(got) != 1 || got[0] != "src/Plan.cs" {
		t.Errorf("the blast call returned %v, want the file from its own reply", got)
	}
}

// A ref carries a line and a file does not, and both are locations. Reading only
// one of them would make a whole tool's replies look empty.
func TestBothRefAndFileAreReadAsLocationsAndALineIsNotPartOfThePath(t *testing.T) {
	log := `{"dir":"c2s","msg":{"id":1,"method":"tools/call","params":{"name":"sense_blast","arguments":{"symbol":"Plan"}}}}` + "\n" +
		`{"dir":"s2c","msg":{"id":1,"result":{"content":[{"type":"text","text":"{\"a\":{\"ref\":\"src/A.cs:44\"},\"b\":[{\"file\":\"src/B.cs\"},{\"ref\":\"src/A.cs:91\"}]}"}]}}}`

	calls := mine.Capture([]byte(log))
	if len(calls) != 1 {
		t.Fatalf("read %d calls, want 1", len(calls))
	}
	got := calls[0].Returned
	if len(got) != 2 || got[0] != "src/A.cs" || got[1] != "src/B.cs" {
		t.Errorf("returned %v, want the two distinct files with no line numbers", got)
	}
}

// The capture is telemetry written beside a paid run. Refusing to mine the run
// because one frame was truncated would throw away everything the run is worth.
func TestATruncatedOrUnpairedFrameDoesNotLoseTheRestOfTheRun(t *testing.T) {
	log := strings.Join([]string{
		`{"dir":"s2c","msg":{"id":99,"result":{"content":[]}}}`,
		`{"dir":"c2s","msg":{"id":1,"method":"tools/call","params":{"name":"sense_blast","argum`,
		`not json at all`,
		``,
		`{"dir":"c2s","msg":{"id":2,"method":"tools/call","params":{"name":"sense_blast","arguments":{"symbol":"Plan"}}}}`,
		`{"dir":"s2c","msg":{"id":2,"result":{"content":[{"type":"text","text":"{\"direct_callers\":[{\"file\":\"src/Plan.cs\"}]}"}]}}}`,
	}, "\n")

	calls := mine.Capture([]byte(log))
	if len(calls) != 1 {
		t.Fatalf("read %d calls, want the one complete pair: %+v", len(calls), calls)
	}
	if calls[0].Key != "Plan" {
		t.Errorf("the surviving call is keyed %q", calls[0].Key)
	}
}

// A reply the parser cannot read is a call that returned nothing it can see,
// which is exactly what the empty-return detector is for. Silently dropping the
// call would hide the surface that produced it.
func TestAReplyThatIsNotReadableJsonIsACallWithNoLocations(t *testing.T) {
	for _, reply := range []string{
		`{"content":[{"type":"text","text":"the index is not built"}]}`,
		`{"content":[{"type":"image","data":"..."}]}`,
		`{"isError":true}`,
	} {
		log := `{"dir":"c2s","msg":{"id":1,"method":"tools/call","params":{"name":"sense_graph","arguments":{"symbol":"ClaimsMap"}}}}` + "\n" +
			`{"dir":"s2c","msg":{"id":1,"result":` + reply + `}}`
		calls := mine.Capture([]byte(log))
		if len(calls) != 1 {
			t.Fatalf("reply %s: read %d calls, want 1", reply, len(calls))
		}
		if len(calls[0].Returned) != 0 {
			t.Errorf("reply %s: returned %v", reply, calls[0].Returned)
		}
	}
}

// A call groups on what it was about. Options change the answer and are kept,
// but they do not make it a different question.
func TestACallIsKeyedOnItsSymbolOrItsQuery(t *testing.T) {
	log := strings.Join([]string{
		`{"dir":"c2s","msg":{"id":1,"method":"tools/call","params":{"name":"sense_blast","arguments":{"symbol":"Plan","max_hops":4}}}}`,
		`{"dir":"s2c","msg":{"id":1,"result":{"content":[]}}}`,
		`{"dir":"c2s","msg":{"id":2,"method":"tools/call","params":{"name":"sense_search","arguments":{"query":"how does billing work"}}}}`,
		`{"dir":"s2c","msg":{"id":2,"result":{"content":[]}}}`,
		`{"dir":"c2s","msg":{"id":3,"method":"tools/call","params":{"name":"sense_status","arguments":{}}}}`,
		`{"dir":"s2c","msg":{"id":3,"result":{"content":[]}}}`,
	}, "\n")

	calls := mine.Capture([]byte(log))
	if len(calls) != 3 {
		t.Fatalf("read %d calls, want 3", len(calls))
	}
	want := []string{"Plan", "how does billing work", "{}"}
	for i, w := range want {
		if calls[i].Key != w {
			t.Errorf("call %d keyed %q, want %q", i, calls[i].Key, w)
		}
	}
	if !strings.Contains(calls[0].Args, "max_hops") {
		t.Errorf("the arguments were not kept verbatim: %q", calls[0].Args)
	}
}

// Gold with no file cannot be missed by a resolver, and treating it as missed
// would report a finding about the gold rather than about the product.
func TestAGoldRowWithNoFileIsNotAResolverMiss(t *testing.T) {
	run, err := mine.Complete("r1", "completed",
		[]mine.Call{{Tool: "sense_blast", Key: "Plan", Args: `{"symbol":"Plan"}`, Returned: []string{"src/Plan.cs"}}},
		[]mine.Cited{{ID: "g:nowhere", Group: "guards", Path: "", Discriminator: true}})
	if err != nil {
		t.Fatal(err)
	}
	found := mine.CitedNotReturned([]mine.Completed{run})
	if len(found) != 1 {
		t.Fatalf("a gold row with no file produced %d findings, want it reported once", len(found))
	}
	if !strings.Contains(found[0].Detail, "guards") {
		t.Errorf("the finding does not name the group it came from: %q", found[0].Detail)
	}
}
