package command

import (
	"testing"

	"github.com/hashicorp/terraform/internal/moduletest"
	"github.com/hashicorp/terraform/internal/terminal"
)

func TestTestCommandViewHuman(t *testing.T) {
	streams, close := terminal.StreamsForTesting(t)
	view := &testCommandViewHuman{
		streams: streams,
		showDiagnostics: func(vals ...interface{}) {
			// We're not actually testing this part for now, so this
			// is just a no-op. Once we get the diagnostic printing
			// into the view layer rather than the Meta layer we can
			// test this part too.
		},
		junitXMLFile: "",
	}

	results := map[string]*moduletest.Suite{}
	view.Results(results)

	output := close(t)
	gotOutput := output.All()
	wantOutput := `(map[string]*moduletest.Suite) {
}
`
	if gotOutput != wantOutput {
		t.Errorf("wrong output\ngot:\n%s\nwant:\n%s", gotOutput, wantOutput)
	}
}
