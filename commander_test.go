// Copyright 2023 Paul Borman
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and

package commander

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"
	"testing"

	"github.com/pborman/check"
	"github.com/pborman/flags"
)

var output bytes.Buffer

func printf(format string, v ...any) {
	fmt.Fprintf(&output, format, v...)
}

type exitStr struct {
	msg string
}

func init() {
	Exit = func(x int) { panic(exitStr{fmt.Sprintf("Exit(%d)", x)}) }
	flags.NewFlagSet = func(name string) flags.FlagSet { return flag.NewFlagSet(name, flag.ContinueOnError) }
	stderr = &output
}

// Below is the help for the commands declared globally in this test:

//	Usage: main [--name=NAME] subcommand [...]
//	    --name=NAME    add the name [foo]
//
//	Available sub commands:
//	  bar [--name=BAR_NAME] [--value=V] ...
//	    execute bar and sub commands
//
//	  foo [-n=VALUE] [--name=VALUE] arg0
//	    execute the foo command
//
//	  help ...
//	    display help

//	Usage: bar [--name=BAR_NAME] [--value=V] subcommand [...]
//	    --name=BAR_NAME    name of bar
//	    --value=V          set the value of v [17]
//
//	Available sub commands:
//	  subbar [--name=BAR_NAME] ...
//	    this is the subbar function

//	Usage: subbar [--name=BAR_NAME] ...
//	    --name=BAR_NAME    name of subbar [myname]

//	Usage: foo [-n=VALUE] [--name=VALUE] arg0
//	     -n=VALUE        [42]
//	    --name=VALUE

//	Usage: help ...

type fooFlags struct {
	N    int
	Name string
}

var fooCommand = &Command{
	Name:        "foo",
	Help:        "execute the foo command",
	Description: "description of foo",
	MaxArgs:     1,
	MinArgs:     1,
	Defaults:    &fooFlags{N: 42},
	Func:        fooFunc,
}

func fooFunc(ctx context.Context, c *Command, args []string, _ ...any) error {
	// There is exactly 1 argument
	if args[0] == "fatal" {
		return errors.New("fatal error")
	}
	opts := c.Flags.(*fooFlags)
	printf("Foo: %q\n", args[0])
	printf("N: %d\n", opts.N)
	return nil
}

type barFlags struct {
	Name  string `flag:"--name=BAR_NAME name of bar"`
	Value int    `flag:"--value=V set the value of v"`
}

var barCommand = &Command{
	Name:        "bar",
	Help:        "execute bar and sub commands",
	Flags:       &barFlags{Value: 17},
	Func:        barFunc,
	Parameters:  "WORD ...",
	SubCommands: []*Command{subbarCommand},
}

func barFunc(ctx context.Context, c *Command, args []string, extra ...any) error {
	printf("Bar: %q\n", args)
	opts := c.Flags.(*barFlags)
	printf("Name: %s\n", opts.Name)
	printf("Value: %d\n", opts.Value)
	if len(args) > 0 {
		return c.RunSubcommands(ctx, args, extra...)
	}
	return nil
}

type subbarFlags struct {
	Name string `flag:"--name=BAR_NAME name of subbar"`
}

var subbarCommand = &Command{
	Name:     "subbar",
	Help:     "this is the subbar function",
	Defaults: &subbarFlags{Name: "myname"},
	Func:     subbarFunc,
}

func subbarFunc(ctx context.Context, c *Command, args []string, _ ...any) error {
	printf("SubBar: %q\n", args)
	printf("Name: %s\n", c.Lookup("", "name"))
	printf("Value: %d\n", c.Lookup("", "value"))
	printf("Bar Name: %s\n", c.Lookup("bar", "name"))
	printf("Top Name: %s\n", c.Lookup("main", "name"))
	if v := c.Lookup("", "unknown"); v != nil {
		printf("Lookup of unknown got %q, want nil", v)
	}
	return nil
}

type mainFlags struct {
	Name string `flag:"--name=NAME add the name"`
}

var mainCommand = &Command{
	Name: "main",
	Help: `
The main program provides an example of
using the commander package.
`,
	Flags: &mainFlags{},
	Description: `
This is the description of the main command.
It has multiple lines.
`,
	SubCommands: []*Command{barCommand, fooCommand, HelpCmd},
}

func TestMainFlags(t *testing.T) {
	ctx := context.Background()
	opts := mainCommand.Flags.(*mainFlags)

	for _, tt := range []struct {
		cmd  []string
		want string
	}{
		{cmd: []string{"bar"}},
		{cmd: []string{"--name", "foo", "bar"}, want: "foo"},
		// flags are sticky from run to run
		{cmd: []string{"bar"}, want: "foo"},
	} {
		t.Run(strings.Join(tt.cmd, " "), func(t *testing.T) {
			t.Logf("Command: %q\n", tt.cmd)

			mainCommand.Run(ctx, tt.cmd)

			lname := mainCommand.Lookup("", "name")
			oname := opts.Name
			if lname != tt.want {
				t.Errorf("Lookup got %q, want %q\n", lname, tt.want)
			}
			if oname != tt.want {
				t.Errorf("Options got %q, want %q\n", oname, tt.want)
			}
		})
	}
}

func TestDefaulted(t *testing.T) {
	ctx := context.Background()

	for _, tt := range []struct {
		cmd  []string
		want string
	}{
		{cmd: []string{"bar"}, want: "foo"},
	} {
		t.Run(strings.Join(tt.cmd, " "), func(t *testing.T) {
			t.Logf("Command: %q\n", tt.cmd)

			if err := mainCommand.Run(ctx, tt.cmd); err != nil {
				t.Errorf("failed: %v", err)
				return
			}
			opts := mainCommand.Flags.(*mainFlags)

			lname := mainCommand.Lookup("", "name")
			oname := opts.Name
			if lname != tt.want {
				t.Errorf("Lookup got %q, want %q\n", lname, tt.want)
			}
			if oname != tt.want {
				t.Errorf("Options got %q, want %q\n", oname, tt.want)
			}
		})
	}
}

func TestExitOnError(t *testing.T) {
	ctx := context.Background()
	mainCommand.OnError = ExitOnError
	output.Reset()
	mainCommand.Stderr = &output
	defer func() {
		mainCommand.OnError = nil
		got := output.String()
		want := "main: bob: unknown command\n"
		if got != want {
			t.Errorf("got output %q, want %q", got, want)
		}
		if p := recover(); p != nil {
			if e, ok := p.(exitStr); ok {
				if e.msg != "Exit(1)" {
					t.Errorf("Got %s, want Exit(1)", e.msg)
				}
				return
			}
			panic(p)
		}
		t.Errorf("Did not get Exit(1)")
	}()
	err := mainCommand.Run(ctx, []string{"bob"})
	t.Errorf("Unexpected return from Run: %v", err)
}

func TestContinueOnError(t *testing.T) {
	ctx := context.Background()
	mainCommand.OnError = ContinueOnError
	defer func() { mainCommand.OnError = nil }()
	output.Reset()
	mainCommand.Stderr = &output
	returned := false
	defer func() {
		if !returned {
			t.Errorf("run did not return")
		}
	}()
	err := mainCommand.Run(ctx, []string{"bob"})
	returned = true
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	got := output.String()
	want := "main: bob: unknown command\n"
	if got != want {
		t.Errorf("Got %q, want %q", got, want)
	}
}

func TestUsageError(t *testing.T) {
	ue := &UsageError{
		C:   &Command{Name: "UE"},
		Err: errors.New("xyzzy"),
	}
	if got, want := ue.Error(), "UE: xyzzy"; got != want {
		t.Errorf("Got usage %q, want %q", got, want)
	}
	ue.Err = nil
	if got, want := ue.Error(), "UE: incorrect usage"; got != want {
		t.Errorf("Got default usage %q, want %q", got, want)
	}
}

func TestSubCommands(t *testing.T) {
	sc := mainCommand.subCommands()
	got := fmt.Sprintf("%q", sc)
	want := `["bar" "foo" "help"]`
	if got != want {
		t.Errorf("Got %q, want %q", got, want)
	}
	err := mainCommand.runsub(nil, nil)
	want = "main: sub command required {bar, foo, help}"
	if err == nil {
		t.Errorf("Did not get error %q", want)
	} else if got := err.Error(); got != want {
		t.Errorf("Got error %q, want %q", got, want)
	}
	output.Reset()
	mainCommand.Run(nil, []string{"bar", "subbar"})
	got = output.String()
	want = `
SubBar: []
Name: myname
Value: 17
Bar Name: 
Top Name: foo
`[1:]
	if got != want {
		t.Errorf("Got output:\n%s\nWant:\n%s", got, want)
	}
	// Forse a parse error when RubSubcommands is called
	output.Reset()
	barCommand.OnError = func(_ *Command, _ []string, _ []any, err error) error { return err }
	defer func() { barCommand.OnError = nil }()
	err = mainCommand.Run(nil, []string{"bar", "-f", "subbar"})
	want = "main bar: flag provided but not defined: -f"
	if err == nil {
		t.Errorf("Did not get error %q", want)
	} else if got := err.Error(); got != want {
		t.Errorf("Got error %q, want %q", got, want)
	}
	got = output.String()
	want = `
main bar: flag provided but not defined: -f
Usage: main [--name=NAME] subcommand [...]
    This is the description of the main command.
    It has multiple lines.

    --name=NAME    add the name [foo]

Available sub commands:
  bar [--name=BAR_NAME] [--value=V] WORD ...
    execute bar and sub commands

  foo [-n=VALUE] [--name=VALUE] arg0
    description of foo

  help ...
    display help
`[1:]
	if got != want {
		t.Errorf("Got output:\n%s\nWant:\n%s", got, want)
	}

}

func TestParseError(t *testing.T) {
	var buf bytes.Buffer
	cmd := &Command{
		Name: "foo",
		Flags: &struct {
			Flag string
		}{},
		Stderr: &buf,
	}
	Exit = func(_ int) { t.Errorf("Exit called") }
	if err := cmd.Run(nil, nil); err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	want := "foo: flag provided but not defined: -x"
	if err := cmd.Run(nil, []string{"-x"}); err != nil {
		if err.Error() != want {
			t.Errorf("Got error %q, want %q", err.Error(), want)
		}
	} else {
		t.Errorf("Did not get expected error %q", want)
	}
	got := buf.String()
	want = "flag provided but not defined"
	if !strings.Contains(got, want) {
		t.Errorf("Output %q does not contain %q", got, want)
	}
}

func TestArgs(t *testing.T) {
	cmd := &Command{
		Name:    "test",
		MaxArgs: NoArgs,
	}
	err := cmd.Run(nil, []string{"arg"})
	want := "test: takes no arguments"
	if err == nil {
		t.Errorf("Did not get error %s", want)
	} else if got := err.Error(); got != want {
		t.Errorf("got error %s, want %s", got, want)
	}
	cmd.MinArgs = 1
	cmd.MaxArgs = 2
	err = cmd.Run(nil, nil)
	want = "test: requires at least 1 arguments"
	if err == nil {
		t.Errorf("Did not get error %s", want)
	} else if got := err.Error(); got != want {
		t.Errorf("got error %s, want %s", got, want)
	}

	err = cmd.Run(nil, []string{"1", "2", "3"})
	want = "test: takes no more than 2 arguments"
	if err == nil {
		t.Errorf("Did not get error %s", want)
	} else if got := err.Error(); got != want {
		t.Errorf("got error %s, want %s", got, want)
	}
}

func TestPrintf(t *testing.T) {
	output.Reset()
	(&Command{}).printf("hello")
	got := output.String()
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestUsage(t *testing.T) {
	output.Reset()
	mainCommand.PrintUsage(&output)
	want := `
Usage: main [--name=NAME] subcommand ...
  --name=NAME    add the name [foo]
Known sub commands:

   bar  execute bar and sub commands
   foo  execute the foo command
   help  display help
`[1:]
	got := output.String()
	output.Reset()
	if got != want {
		t.Errorf("Got:\n%s\nWant:\n%s", got, want)
	}

	fooCommand.PrintUsage(&output)
	want = `
Usage: foo [-n=VALUE] [--name=VALUE]
   -n=VALUE        [42]
  --name=VALUE
`[1:]
	got = output.String()
	output.Reset()
	if got != want {
		t.Errorf("Got:\n%s\nWant:\n%s", got, want)
	}

	Help(nil, &Command{
		Name:        "program",
		SubCommands: []*Command{fooCommand, barCommand},
	}, nil, nil)
	want = `
Usage: program subcommand [...]

Available sub commands:
  bar [--name=BAR_NAME] [--value=V] WORD ...
    execute bar and sub commands

  foo [-n=VALUE] [--name=VALUE] arg0
    description of foo
`[1:]
	got = output.String()
	if got != want {
		t.Errorf("Got:\n%s\nWant:\n%s", got, want)
	}
}

func TestHelp(t *testing.T) {
	ctx := context.Background()

	output.Reset()
	mainCommand.RunSubcommands(ctx, []string{"help"})
	got := output.String()
	if !strings.HasPrefix(got, "Usage: main [--name=NAME] subcommand [...]") {
		t.Errorf("Wrong output returned from help:\n%s", got)
	}

	output.Reset()
	defer func() { mainCommand.OnError = nil }()
	sawError := false
	mainCommand.OnError = func(_ *Command, _ []string, _ []any, err error) error { sawError = true; return err }

	mainCommand.RunSubcommands(ctx, []string{"--foo"})
	got = output.String()
	if !strings.HasPrefix(got, "main: flag provided but not defined: -foo") {
		t.Errorf("Wrong output returned from --foo:\n%s", got)
	}
	if !sawError {
		t.Errorf("OnError function was not called")
	}

	if s := check.Error(Help(ctx, mainCommand, []string{"foo"}), ""); s != "" {
		t.Error(s)
	}
	if s := check.Error(Help(ctx, fooCommand, []string{"bad"}), "foo has no subcommands"); s != "" {
		t.Error(s)
	}
	if s := check.Error(Help(ctx, mainCommand, []string{"bad"}), "main has no subcommand bad"); s != "" {
		t.Error(s)
	}
}

// RubSubCommand, findSub, Help,
