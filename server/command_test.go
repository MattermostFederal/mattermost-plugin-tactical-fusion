package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

func TestGetCommandIsAutocompletable(t *testing.T) {
	command := getCommand()

	if command.Trigger != commandTrigger {
		t.Fatalf("Trigger = %q, want %q", command.Trigger, commandTrigger)
	}
	if !command.AutoComplete {
		t.Fatal("AutoComplete is off, so the command never appears in the slash command menu")
	}
	if command.DisplayName == "" || command.AutoCompleteDesc == "" {
		t.Fatalf("command has no display name or description: %+v", command)
	}
	if command.AutocompleteData == nil {
		t.Fatal("AutocompleteData is nil, so no subcommand is suggested")
	}
	if command.AutocompleteData.Trigger != commandTrigger {
		t.Fatalf("autocomplete trigger = %q, want %q", command.AutocompleteData.Trigger, commandTrigger)
	}
}

// The three places a subcommand has to be named are easy to let drift apart,
// and drift is invisible: a subcommand missing from the autocomplete data still
// works when typed in full, and one missing from subcommandList is simply never
// mentioned to anybody who asks what the command can do.
//
// The dispatch switch is the source of truth, read out of the source rather than
// restated here, because a list written in this file would just be a fourth
// place to forget.
func TestEverySubcommandIsAdvertised(t *testing.T) {
	dispatched := dispatchedSubcommands(t)
	if len(dispatched) == 0 {
		t.Fatal("parsed no subcommands out of ExecuteCommand; the parser is looking in the wrong place")
	}

	advertised := map[string]bool{}
	for _, sub := range getCommand().AutocompleteData.SubCommands {
		if advertised[sub.Trigger] {
			t.Errorf("autocomplete lists %q more than once", sub.Trigger)
		}
		advertised[sub.Trigger] = true
	}

	listed := map[string]bool{}
	for name := range strings.SplitSeq(subcommandList, ", ") {
		listed[strings.TrimSpace(name)] = true
	}

	for name := range dispatched {
		if !advertised[name] {
			t.Errorf("ExecuteCommand handles %q but the autocomplete data does not offer it", name)
		}
		if !listed[name] {
			t.Errorf("ExecuteCommand handles %q but subcommandList does not mention it", name)
		}
	}

	for name := range advertised {
		if !dispatched[name] {
			t.Errorf("autocomplete offers %q but ExecuteCommand does not handle it", name)
		}
	}

	for name := range listed {
		if !dispatched[name] {
			t.Errorf("subcommandList mentions %q but ExecuteCommand does not handle it", name)
		}
	}
}

// dispatchedSubcommands returns the case values of the switch in ExecuteCommand.
func dispatchedSubcommands(t *testing.T) map[string]bool {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed; cannot locate command.go")
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filepath.Join(filepath.Dir(thisFile), "command.go"), nil, 0)
	if err != nil {
		t.Fatalf("failed to parse command.go: %v", err)
	}

	found := map[string]bool{}
	for _, decl := range file.Decls {
		fn, isFunc := decl.(*ast.FuncDecl)
		if !isFunc || fn.Name.Name != "ExecuteCommand" {
			continue
		}

		ast.Inspect(fn, func(n ast.Node) bool {
			clause, isClause := n.(*ast.CaseClause)
			if !isClause {
				return true
			}
			for _, expr := range clause.List {
				lit, isLit := expr.(*ast.BasicLit)
				if !isLit || lit.Kind != token.STRING {
					continue
				}
				if value, err := strconv.Unquote(lit.Value); err == nil {
					found[value] = true
				}
			}
			return true
		})
	}

	return found
}

// A bare command is not a failure, so it carries no code: it is the answer to
// "what can this do".
func TestBareCommandListsSubcommands(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	response, appErr := p.ExecuteCommand(&plugin.Context{}, &model.CommandArgs{
		Command: "/tactical-fusion",
	})
	if appErr != nil {
		t.Fatalf("ExecuteCommand returned an error: %v", appErr)
	}

	if !strings.Contains(response.Text, subcommandList) {
		t.Fatalf("bare command output does not list the subcommands: %q", response.Text)
	}
	if response.ResponseType != model.CommandResponseTypeEphemeral {
		t.Fatalf("ResponseType = %q, want ephemeral", response.ResponseType)
	}
}

// argumentText recovers the text after the subcommand by finding it rather than
// by trimming a fixed prefix, because dispatch splits on Fields and tolerates
// repeated spaces a prefix trim would not. Getting that wrong made
// "/tactical-fusion  check X" echo the whole command line back as the text
// under test.
func TestArgumentText(t *testing.T) {
	for _, tc := range []struct {
		name       string
		command    string
		subcommand string
		want       string
	}{
		{"the ordinary shape", "/tactical-fusion check 34.0561, -118.2500", "check", "34.0561, -118.2500"},
		{"repeated spaces before the subcommand", "/tactical-fusion  check  X", "check", "X"},
		{"spacing inside the argument is kept", "/tactical-fusion check a  b", "check", "a  b"},
		{"nothing after the subcommand", "/tactical-fusion check", "check", ""},
		{"only whitespace after it", "/tactical-fusion check   ", "check", ""},

		// A subcommand the line does not contain yields nothing rather than the
		// whole line, which is what the prefix-trim version got wrong.
		{"a subcommand that is not there", "/tactical-fusion examples", "check", ""},
		{"an empty command", "", "check", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := argumentText(tc.command, tc.subcommand); got != tc.want {
				t.Fatalf("argumentText(%q, %q) = %q, want %q",
					tc.command, tc.subcommand, got, tc.want)
			}
		})
	}
}
