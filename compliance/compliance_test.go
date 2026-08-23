package compliance

import (
	_ "embed"
	stdjson "encoding/json"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/agentable/go-jsonpath"
)

// The CTS (Compliance Test Suite) is maintained as a git submodule at:
// .references/jsonpath-compliance-test-suite
//
// To update the CTS to the latest version:
//   cd .references/jsonpath-compliance-test-suite
//   git pull origin main
//   cd ../..
//   cp .references/jsonpath-compliance-test-suite/cts.json compliance/testdata/cts.json
//   git add compliance/testdata/cts.json
//   git commit -m "chore: update JSONPath CTS to latest version"

//go:embed testdata/cts.json
var ctsJSON []byte

// ctsFile represents the structure of the CTS JSON file.
type ctsFile struct {
	Description string     `json:"description"`
	Tests       []testCase `json:"tests"`
}

// testCase represents a single test case from the CTS.
type testCase struct {
	Name            string     `json:"name"`
	Selector        string     `json:"selector"`
	Document        any        `json:"document"`
	Result          []any      `json:"result"`
	Results         [][]any    `json:"results"`
	ResultPaths     []string   `json:"result_paths"`
	ResultsPaths    [][]string `json:"results_paths"`
	InvalidSelector bool       `json:"invalid_selector"`
	Tags            []string   `json:"tags"`
}

func matchesExpected(tc *testCase, values, locatedValues []any, paths []string) bool {
	if !cmp.Equal(values, locatedValues) {
		return false
	}

	if tc.Results == nil {
		return cmp.Equal(tc.Result, values) && slices.Equal(tc.ResultPaths, paths)
	}

	if len(tc.Results) != len(tc.ResultsPaths) {
		return false
	}
	for i := range tc.Results {
		if cmp.Equal(tc.Results[i], values) && slices.Equal(tc.ResultsPaths[i], paths) {
			return true
		}
	}
	return false
}

func TestMatchesExpectedRejectsCrossedOutcome(t *testing.T) {
	t.Parallel()

	tc := testCase{
		Results: [][]any{
			{"A", "B"},
			{"B", "A"},
		},
		ResultsPaths: [][]string{
			{"$['a']", "$['b']"},
			{"$['b']", "$['a']"},
		},
	}

	require.True(t, matchesExpected(&tc,
		[]any{"A", "B"},
		[]any{"A", "B"},
		[]string{"$['a']", "$['b']"},
	))
	require.True(t, matchesExpected(&tc,
		[]any{"B", "A"},
		[]any{"B", "A"},
		[]string{"$['b']", "$['a']"},
	))
	require.False(t, matchesExpected(&tc,
		[]any{"A", "B"},
		[]any{"A", "B"},
		[]string{"$['b']", "$['a']"},
	))
}

func TestMatchesExpectedRejectsLocatedValueMismatch(t *testing.T) {
	t.Parallel()

	tc := testCase{
		Result:      []any{"A"},
		ResultPaths: []string{"$['a']"},
	}

	require.False(t, matchesExpected(&tc,
		[]any{"A"},
		[]any{"B"},
		[]string{"$['a']"},
	))
}

func TestCompliance(t *testing.T) {
	t.Parallel()

	var suite ctsFile
	preserveNumbers := json.WithUnmarshalers(json.UnmarshalFromFunc(
		func(decoder *jsontext.Decoder, value *any) error {
			if decoder.PeekKind() != '0' {
				return errors.ErrUnsupported
			}
			raw, err := decoder.ReadValue()
			if err != nil {
				return err
			}
			*value = stdjson.Number(raw)
			return nil
		},
	))
	require.NoError(t, json.Unmarshal(ctsJSON, &suite, preserveNumbers))

	for _, tc := range suite.Tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			// Invalid selector tests
			if tc.InvalidSelector {
				_, err := jsonpath.Parse(tc.Selector)
				require.Error(t, err, "expected parse error for invalid selector")
				return
			}

			// Valid selector tests
			path, err := jsonpath.Parse(tc.Selector)
			require.NoError(t, err, "failed to parse valid selector")

			got := path.Select(tc.Document)
			located := path.SelectLocated(tc.Document)
			locatedValues := make([]any, len(located))
			gotPaths := make([]string, len(located))
			for i := range located {
				locatedValues[i] = located[i].Value
				gotPaths[i] = located[i].Path.String()
			}

			require.True(t, matchesExpected(&tc, []any(got), locatedValues, gotPaths),
				"result/path outcome is not accepted:\nvalues: %v\npaths: %v", got, gotPaths)
		})
	}
}
