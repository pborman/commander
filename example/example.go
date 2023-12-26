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

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pborman/commander"
)

var options = struct {
	Title   string `flag:"--title=TITLE set the title"`
	N       int    `flag:"-n=N run N times"`
	Verbose bool   `flag:"-v be verbose"`
}{
	Title: "Main",
	N:     1,
}

var mainCmd = commander.Command{
	Name:  "main",
	Flags: &options,
	SubCommands: []*commander.Command{
		&listCmd,
		&deepCmd,
		&helpCmd,
	},
	// Call Help if there are no positional arguments.
	Func: commander.Help,
}

var listCmd = commander.Command{
	Name:    "list",
	Help:    "show a list",
	MinArgs: 1,
	MaxArgs: 1,
	Defaults: &struct {
		Title string `flag:"--title=TITLE set the title of the list"`
	}{
		Title: "Local",
	},
	Func: list,
}

func list(ctx context.Context, c *commander.Command, args []string, _ ...any) error {
	myTitle := c.Lookup("", "title")
	mainTitle := c.Lookup("main", "title")
	if args[0] == "error" {
		return fmt.Errorf("%s:%s: has an error", mainTitle, myTitle)
	}
	fmt.Printf("List of %s:%s\n", mainTitle, myTitle)
	for i := 0; i < options.N; i++ {
		fmt.Printf("  %s\n", args[0])
	}
	return nil
}

var deepCmd = commander.Command{
	Name: "deep",
	Description: `
A very deep subject to go into.
`,
	Help: "multi-level command",
	Defaults: &struct {
		Duration time.Duration `flag:"--duration=D ponder for a duration of D"`
	}{},
	SubCommands: []*commander.Command{
		&seaCmd,
		&thoughtCmd,
	},
}

var seaCmd = commander.Command{
	Name:    "sea",
	MaxArgs: commander.NoArgs,
	Func: func(ctx context.Context, c *commander.Command, args []string, _ ...any) error {
		fmt.Printf("The deep blue sea\n")
		return nil
	},
}

var thoughtCmd = commander.Command{
	Name: "thought",
	Description: `
Travel to PLANET and ponder the question
of life, the universe, and everything.
Provide the answer to the question to the mice.
What the acutal question is is unknown."
`,
	Parameters: "[who] [what] [when]",
	Defaults: &struct {
		Verbose bool   `flag:"-v be verbose"`
		Plant   string `flag:"--planet=PLANET go to PLANET"`
	}{},
	Func: func(ctx context.Context, c *commander.Command, args []string, _ ...any) error {
		fmt.Printf("Having deep thoughts %q\n", args)
		return nil
	},
}

var helpCmd = commander.Command{
	Name:       "help",
	Help:       "give usage help",
	Parameters: "[cmd [cmd [...]]]",
	Func:       commander.Help,
	Description: `
If there are no arguments, describe the usage of the command.
Otherwise describe the usage of the specified sub command.
Multiple arugments will descend the command tree.
`,
}

func main() {
	if err := mainCmd.Run(context.Background(), os.Args[1:]); err != nil {
		if _, ok := err.(*commander.UsageError); !ok {
			fmt.Fprintf(os.Stderr, "Command failed: %v\n", err)
		}
		os.Exit(1)
	}
}
