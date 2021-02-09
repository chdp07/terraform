package command

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/terraform/internal/moduletest"
	"github.com/hashicorp/terraform/internal/terminal"
	"github.com/hashicorp/terraform/tfdiags"
)

// TestCommand is the implementation of "terraform test".
type TestCommand struct {
	Meta
}

func (c *TestCommand) Run(rawArgs []string) int {
	args, diags := parseTestCommandArgs(rawArgs)
	view := c.View(args.Output)
	if diags.HasErrors() {
		view.Diagnostics(diags)
		return 1
	}

	diags = diags.Append(tfdiags.Sourceless(
		tfdiags.Warning,
		`The "terraform test" command is experimental`,
		"We'd like to invite adventurous module authors to write integration tests for their modules using this command, but all of the behaviors of this command are currently experimental and may change based on feedback.\n\nFor more information on the testing experiment, including ongoing research goals and avenues for feedback, see:\n    https://www.terraform.io/docs/language/modules/testing-experiment.html",
	))

	results, moreDiags := c.run(args)
	diags = diags.Append(moreDiags)
	if diags.HasErrors() {
		view.Diagnostics(diags)
		return 1
	}

	view.Results(results)
	view.Diagnostics(diags)

	if diags.HasErrors() {
		return 1
	}
	return 0
}

func (c *TestCommand) run(args testCommandArgs) (results map[string]*moduletest.Suite, diags tfdiags.Diagnostics) {
	suiteNames, err := c.collectSuiteNames()
	if err != nil {
		diags = diags.Append(tfdiags.Sourceless(
			tfdiags.Error,
			"Error while searching for test configurations",
			fmt.Sprintf("While attempting to scan the 'tests' subdirectory for potential test configurations, Terraform encountered an error: %s.", err),
		))
		return nil, diags
	}

	ret := make(map[string]*moduletest.Suite, len(suiteNames))
	for _, suiteName := range suiteNames {
		suite, moreDiags := c.runSuite(suiteName)
		diags = diags.Append(moreDiags)
		ret[suiteName] = suite
	}

	return ret, diags
}

func (c *TestCommand) runSuite(suiteName string) (*moduletest.Suite, tfdiags.Diagnostics) {
	var diags tfdiags.Diagnostics
	ret := moduletest.Suite{
		Name:       suiteName,
		Components: map[string]*moduletest.Component{},
	}

	return &ret, diags
}

func (c *TestCommand) collectSuiteNames() ([]string, error) {
	items, err := ioutil.ReadDir("tests")
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	ret := make([]string, 0, len(items))
	for _, item := range items {
		if !item.IsDir() {
			continue
		}
		name := item.Name()
		suitePath := filepath.Join("tests", name)
		tfFiles, err := filepath.Glob(filepath.Join(suitePath, "*.tf"))
		if err != nil {
			// We'll just ignore it and treat it like a dir with no .tf files
			tfFiles = nil
		}
		tfJSONFiles, err := filepath.Glob(filepath.Join(suitePath, "*.tf.json"))
		if err != nil {
			// We'll just ignore it and treat it like a dir with no .tf.json files
			tfJSONFiles = nil
		}
		if (len(tfFiles) + len(tfJSONFiles)) == 0 {
			// Not a test suite, then.
			continue
		}
		ret = append(ret, name)
	}

	return ret, nil
}

func (c *TestCommand) View(opts struct {
	DisableColor bool
	JUnitXMLFile string
}) testCommandView {
	// The abstractions here aren't quite lining up yet because we've
	// yet to refactor to split the view-related concerns out of
	// c.Meta. Therefore the color setting ends up being a mutation
	// of the command itself rather than just a field inside the view.
	// This will ultimately make c.Meta.showDiagnostics produce color output
	// when appropriate, and c.Meta.Colorize() turn on or off its color
	// mode as needed.
	// Hopefully eventually we can fix this up and make color selection
	// a concern purely for the view layer.
	c.Meta.Color = !opts.DisableColor
	c.Meta.process(nil)

	showDiags := c.showDiagnostics
	streams := c.Streams

	return &testCommandViewHuman{
		streams:         streams,
		showDiagnostics: showDiags,
		junitXMLFile:    opts.JUnitXMLFile,
	}
}

func (c *TestCommand) Help() string {
	helpText := `
Usage: terraform test [options]

  This is an experimental command to help with automated integration
  testing of shared modules. The usage and behavior of this command is
  likely to change in breaking ways in subsequent releases, as we
  are currently using this command primarily for research purposes.

  In its current experimental form, "test" will look under the current
  working directory for a subdirectory called "tests", and then within
  that directory search for one or more subdirectories that contain
  ".tf" or ".tf.json" files. For any that it finds, it will perform
  Terraform operations similar to the following sequence of commands
  in each of those directories:
      terraform validate
      terraform apply
      terraform destroy

  The test configurations should not declare any input variables and
  should at least contain a call to the module being tested, which
  will always be available at the path ../.. due to the expected
  filesystem layout.

  The tests are considered to be successful if all of the above steps
  succeed.

  Test configurations may optionally include uses of the special
  built-in test provider terraform.io/builtin/test, which allows
  writing explicit test assertions which must also all pass in order
  for the test run to be considered successful.

  This initial implementation is intended as a minimally-viable
  product to use for further research and experimentation, and in
  particular it currently lacks the following capabilities that we
  expect to consider in later iterations, based on feedback:
    - Testing of subsequent updates to existing infrastructure,
      where currently it only supports initial creation and
      then destruction.
    - Testing top-level modules that are intended to be used for
      "real" environments, which typically have hard-coded values
      that don't permit creating a separate "copy" for testing.
    - Some sort of support for unit test runs that don't interact
      with remote systems at all, e.g. for use in checking pull
      requests from untrusted contributors.

  In the meantime, we'd like to hear feedback from module authors
  who have tried writing some experimental tests for their modules
  about what sorts of tests you were able to write, what sorts of
  tests you weren't able to write, and any tests that you were
  able to write but that were difficult to model in some way.

Options:

  -junit-xml=FILE  In addition to the usual output, also write test
                   results to the given file path in JUnit XML format.
                   This format is commonly supported by CI systems, and
                   they typically expect to be given a filename to search
                   for in the test workspace after the test run finishes.
`
	return strings.TrimSpace(helpText)
}

func (c *TestCommand) Synopsis() string {
	return "Experimental support for module integration testing"
}

// testCommandArgs represents the command line arguments for "terraform test".
type testCommandArgs struct {
	Output struct {
		// DisableColor can be set to true to force the human-oriented output
		// to not include any terminal formatting codes.
		DisableColor bool

		// If not an empty string, JUnitXMLFile gives a filename where JUnit-style
		// XML test result output should be written, in addition to the normal
		// output printed to the standard output and error streams.
		// (The typical usage pattern for tools that can consume this file format
		// is to configure them to look for a separate test result file on disk
		// after running the tests.)
		JUnitXMLFile string
	}
}

func parseTestCommandArgs(args []string) (testCommandArgs, tfdiags.Diagnostics) {
	var ret testCommandArgs
	var diags tfdiags.Diagnostics

	// NOTE: parseTestCommandArgs should still return at least a partial
	// testCommandArgs even on error, containing enough information for the
	// command to report error diagnostics in a suitable way.

	f := flag.NewFlagSet("test", flag.ContinueOnError)
	f.SetOutput(ioutil.Discard)
	f.Usage = func() {}
	f.StringVar(&ret.Output.JUnitXMLFile, "junit-xml", "", "Write a JUnit XML file describing the results")
	f.BoolVar(&ret.Output.DisableColor, "no-color", false, "Disable terminal formatting sequences")

	err := f.Parse(args)
	if err != nil {
		diags = diags.Append(err)
		return ret, diags
	}

	// We'll now discard all of the arguments that the flag package handled,
	// and focus only on the positional arguments for the rest of the function.
	args = f.Args()

	if len(args) != 0 {
		diags = diags.Append(tfdiags.Sourceless(
			tfdiags.Error,
			"Invalid command arguments",
			"The test command doesn't expect any positional command-line arguments.",
		))
		return ret, diags
	}

	return ret, diags
}

type testCommandView interface {
	Results(map[string]*moduletest.Suite) tfdiags.Diagnostics

	// Diagnostics is for reporting warnings or errors that occurred with the
	// mechanics of running tests. For this command in particular, some
	// errors are considered to be test failures rather than mechanism failures,
	// and so those will be reported via Results rather than via Diagnostics.
	Diagnostics(tfdiags.Diagnostics)
}

type testCommandViewHuman struct {
	streams *terminal.Streams

	// For now we're awkwardly grasping the showDiagnostics method from
	// Meta because we're not yet done refactoring things to separate
	// view from controller. Hopefully before too long this'll be done
	// a different way. Since this bypasses the streams we've captured
	// above, the diagnostics printing isn't currently included in the
	// output that can be captured by mocking those streams.
	showDiagnostics func(vals ...interface{})

	// If junitXMLFile is not empty then results will be written to
	// the given file path in addition to the usual output.
	junitXMLFile string
}

func (v *testCommandViewHuman) Results(results map[string]*moduletest.Suite) tfdiags.Diagnostics {
	// TODO: Something more appropriate than this
	v.streams.Stdout.File.WriteString(spew.Sdump(results))

	if v.junitXMLFile != "" {
		// TODO: Also write JUnit XML to the given file
	}

	return nil
}

func (v *testCommandViewHuman) Diagnostics(diags tfdiags.Diagnostics) {
	if len(diags) == 0 {
		return
	}
	v.showDiagnostics(diags)
}
