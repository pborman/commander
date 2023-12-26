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

// Package commander is yet another commander packge that facilitates a sub
// command structure.  It uses the github.com/pborman/flags package to specify
// the flags for a given command.  Since the flags package is a wrapper around
// the standard flag package.
//
// A Command is specified by the Command structure.  The simplest declaration is
//
//	var mycmd = &commander.Command{
//		Name: "mycmd",
//		Func: func(context.Context, *Command, []string, ...any) error {
//			fmt.Print("Hello World\n")
//			return nikl
//		},
//	}
//
// The simplest decleration of a command that has sub commands is:
//
//	var cmd = &commander.Command{
//		Name:        "cmd",
//		SubCommands: []*commander.Command{mycmd},
//	}
//
// Each command can have a set of flags associated with it.  Flags are declared as a
// structure as defined by the github.com/pborman/flags.  A simple example is:
//
//	var options = &struct{
//		N    int    `flag:"-n=N          number of itterations"
//		Name string `flag:"--name=NAME   set the projects name"
//	} {
//		N: 1, // -n defaults to 1.
//	}
//
// The flags are specified by either the Defaults or the Flags field.
// If the Defaults field is set to the Flags field will be automatically set to
// a copy of the flags.  This is useful if the command might be executed more
// than once as each invocation will have a fresh set of flags.  If the Flags
// field is set no copies are made and the values will persist between invocations.
//
// For example:
//
//	var cmd = &commander.Command{
//		Name:        "cmd",
//		Defaults:    options,
//		SubCommands: []*commander.Command{mycmd},
//	}
//
// The commander package has a predefined help command:
//
//	var HelpCmd = &Command{
//		Name: "help",
//		Help: "display help",
//		Func: Help,
//	}
//
// The Help command utilizes three optional fields:
//
//	Help - A single line description of the command
//	Description - A long description of the command
//	Parameters - What parameters the command takes.
//
// As an example:
//
//	var HelpCmd = &Command{
//		Name:        "help",
//		Help:        "display help",
//		Description: `
//	}
//
//	The help command displays help for the program or sub command.
//	Example usage:
//
//		help
//		help sub
//		help sub subsub
//	`,
//		Parameters: "[cmd [cmd ...]",
//	}
//
// There are also optional fields to help with parsing the command.
//
// The MinArgs and MaxArgs fields specify the minimum and maximum number of
// position parameters for the command.  If MaxArgs is 0 there is no upper limit.
// If MaxArgs is set to commander.NoArgs then the command takes no positional parameters.
//
// The Stderr field specifies where commandeer should send output (usage or help).
// If Stderr is not specified it defaults to os.Stderr.  All sub commands that do
// not specify Stderr will inherit the main command's Stderr.
//
// OnError, when specified, is set to a function to be called when a usage error is encountered.
// There are two pre-defined OnError functions:
//
// ExitOnError - Display the message on Stderr and call os.Exit(1)
// ContinueOnError - Display the message on Stderr and return nil
//
// If OnError is nil, the default, then the error is returned.
package commander

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/pborman/flags"
	"github.com/pborman/indent"
)

// If MaxArgs is set to NoArgs then the command takes no arguments.
const NoArgs = -1

// A Command can either be a function and/or a list of subcommands.  A Command
// normally only declares Func or SubCommands.  If they are both set only Func
// is called.  Func may call c.RunSubcommands to execute a sub command.
//
// Flags for the command are a structure as defined by the
// github.com/pborman/flags package.  Use the Flags field to see the value of a
// flags.  If the flags are provided through the Default field instead of the
// Flags field then Flags will contain a copy of the flags.  Using Defaults
// makes it possible to run the command multiple times with the same set of
// flags.
//
// Currently an individual Command cannot be run concurrently with itself.
//
// To run the command c, call c.Run.  The arguments passed to c.Run must not
// include the command name.
type Command struct {
	parent      *Command
	Name        string // Name of this command
	Help        string // Short description of this command
	Description string // Long description displayed by help
	Parameters  string // Parameters to go at the end of the usage line
	MinArgs     int    // The command must have at least this many arguments
	MaxArgs     int    // Maximum number of arguments.  0 means no limit
	Defaults    any    // An options struct as defined by the flags package
	Flags       any    // See above for Defaults vs Flags
	Func        func(context.Context, *Command, []string, ...any) error
	SubCommands []*Command // Sub-Commands -- Ignored if Func is set

	// Errors are displayed to Stderr (defaults to os.Stderr).
	// If not nil, OnError is called when there is a usage error
	// running a command.  If these values are nil then
	// their parent's values are used.
	Stderr  io.Writer
	OnError func(*Command, []string, []any, error) error
}

// Exit can be overriden by tests.
var Exit = os.Exit

// ExitOnError is an OnError func that displays the error and exits
// with a return code of 1.
func ExitOnError(c *Command, _ []string, _ []any, err error) error {
	c.printf("%v\n", err)
	Exit(1)
	return nil
}

// ContinueOnError is on OnError func that displays the error and
// returns no error.
func ContinueOnError(c *Command, _ []string, _ []any, err error) error {
	c.printf("%v\n", err)
	return nil
}

// A UsageError is returned when there is an error in usage.
type UsageError struct {
	C   *Command
	Err error
}

// Implements the error interface.
func (u *UsageError) Error() string {
	if u.Err != nil {
		return fmt.Sprintf("%s: %s", u.C.Command(), u.Err)
	}
	return fmt.Sprintf("%s: incorrect usage", u.C.Command())
}

// Command returns the possibly multi-part command name for c.
func (c *Command) Command() string {
	if c.parent != nil {
		return c.parent.Command() + " " + c.Name
	}
	return c.Name
}

// Tests can override this
var stderr io.Writer = os.Stderr

func (c *Command) printf(format string, v ...any) {
	fmt.Fprintf(c.stderr(), format, v...)
}

func (c *Command) subCommands() []string {
	var cmds []string
	for _, sc := range c.SubCommands {
		cmds = append(cmds, sc.Name)
	}
	sort.Strings(cmds)
	return cmds
}

// Run runs the command with the provided arguments after parsing any flags.
// The command name itself is not part of the arguments.  If c does not have a
// Func defined then the first argument is used to find the subcommand listed in
// SubCommands.  The subcommand's Run method is then called with the arguments
// following the subcommand.  An error is returned if the command could not be
// run or the command failed.
//
// If the command has both Func and SubCommands then Func is called if there
// are no positional parameters otherwise the first argument is used to find
// the sub command listed in SubCommands.
func (c *Command) Run(ctx context.Context, args []string, extra ...any) (err error) {
	defer func() {
		if c.onError(err) == nil {
			return
		}
		err = c.onError(err)(c, args, extra, err)
	}()
	args, err = c.parse(args)
	if err != nil {
		c.printf("%v\n", err)
		if ue, ok := err.(*UsageError); ok {
			Help(ctx, ue.C, nil)
		}
		return err
	}
	if c.SubCommands != nil && len(args) > 0 {
		return c.runsub(ctx, args, extra...)
	}
	if c.Func != nil {
		return c.Func(ctx, c, args, extra...)
	}
	return nil
}

// RunSubcommands is similar to Run excpet it ignores c.Func and just runs sub
// commands.
func (c *Command) RunSubcommands(ctx context.Context, args []string, extra ...any) (err error) {
	defer func() {
		if c.onError(err) == nil {
			return
		}
		err = c.onError(err)(c, args, extra, err)
	}()
	args, err = c.parse(args)
	if err != nil {
		c.printf("%v\n", err)
		if ue, ok := err.(*UsageError); ok {
			Help(ctx, ue.C, nil)
		}
		return err
	}
	return c.runsub(ctx, args, extra...)
}

func (c *Command) runsub(ctx context.Context, args []string, extra ...any) (err error) {
	if len(args) < 1 {
		return &UsageError{
			C:   c,
			Err: fmt.Errorf("sub command required {%s}", strings.Join(c.subCommands(), ", ")),
		}
	}
	cmd := args[0]
	args = args[1:]
	for _, sc := range c.SubCommands {
		if sc.Name == cmd {
			sc.parent = c
			return sc.Run(ctx, args, extra...)
		}
	}
	return &UsageError{
		C:   c,
		Err: fmt.Errorf("%s: unknown command", cmd),
	}
}

func (c *Command) parse(args []string) ([]string, error) {
	var set flags.FlagSet
	if c.Defaults != nil {
		c.Flags, set = flags.RegisterNew(c.Command(), c.Defaults)
	} else if c.Flags != nil {
		set = flags.NewFlagSet(c.Name)
		flags.RegisterSet(c.Command(), c.Flags, set)
	}
	var buf bytes.Buffer
	oStderr := c.Stderr
	defer func() { c.Stderr = oStderr }()
	c.Stderr = &buf

	if set != nil {
		w := c.stderr()
		set.SetOutput(w)
		if err := set.Parse(args); err != nil {
			flags.Help(w, c.Name, c.parameters(), c.Flags)
			return args, &UsageError{C: c, Err: err}
		}
		args = set.Args()
	}
	if c.MaxArgs == NoArgs && len(args) != 0 {
		return args, &UsageError{
			C:   c,
			Err: errors.New("takes no arguments"),
		}
	}
	if len(args) < c.MinArgs {
		return args, &UsageError{
			C:   c,
			Err: fmt.Errorf("requires at least %d arguments", c.MinArgs),
		}
	}
	if c.MaxArgs > 0 && len(args) > c.MaxArgs {
		return args, &UsageError{
			C:   c,
			Err: fmt.Errorf("takes no more than %d arguments", c.MaxArgs),
		}
	}
	return args, nil
}

// Lookup returns the value of the flag named flag.  If cmd is not empty Lookup will look for a command in the tree that is named cmd.
// For example, consider the command "foo" that has a sub command "bar":
//
//	foo --name VALUE1 bar --name VALEUE2
//
//	bar.Lookup("", "name") -> VALUE2
//	bar.Lookup("foo", "name") -> VALUE1
func (c *Command) Lookup(cmd, name string) any {
	if c == nil {
		return nil
	}
	if cmd == "" || cmd == c.Name {
		if i := flags.Lookup(c.Flags, name); i != nil {
			return i
		}
	}
	return c.parent.Lookup(cmd, name)
}

func (c *Command) findSub(name string) *Command {
	for _, sc := range c.SubCommands {
		if sc.Name == name {
			return sc
		}
	}
	return nil
}

// PrintUsage write the usage information for c to w.
func (c *Command) PrintUsage(w io.Writer) {
	opts := c.Defaults
	if opts == nil {
		opts = c.Flags
	}
	if len(c.SubCommands) > 0 {
		flags.Help(w, c.Name, "subcommand ...", opts)
		fmt.Fprintf(w, "Known sub commands:\n")
		// Find the longest name
		for i, subcmd := range c.SubCommands {
			if i == 0 {
				fmt.Fprintln(w)
			}
			fmt.Fprintf(w, "   %s  %s\n", subcmd.Name, subcmd.Help)
		}
		return
	}
	flags.Help(w, c.Name, "", opts)
}

func (c *Command) stderr() io.Writer {
	for c != nil {
		if c.Stderr != nil {
			return c.Stderr
		}
		c = c.parent
	}
	return stderr
}

func (c *Command) onError(err error) func(*Command, []string, []any, error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(*UsageError); !ok {
	}
	for c != nil {
		if c.OnError != nil {
			return c.OnError
		}
		c = c.parent
	}
	return nil
}

// HelpCmd is a sub command that calls the Help function.
var HelpCmd = &Command{
	Name: "help",
	Help: "display help",
	Func: Help,
}

// Help implements the help command.
//
//	Usage: help [subcommand [subcommand [...]]]
func Help(ctx context.Context, c *Command, args []string, extra ...any) error {
	w := c.stderr()

	if c.parent != nil {
		c = c.parent
	}

	command := c.Name
	for _, name := range args {
		if len(c.SubCommands) == 0 {
			return fmt.Errorf("%s has no subcommands", command)
		}
		if c = c.findSub(name); c == nil {
			return fmt.Errorf("%s has no subcommand %s", command, name)
		}
		command += " " + name
	}
	if len(c.SubCommands) == 0 {
		c.printf("Usage: %s\n", flags.UsageLine(c.Name, c.parameters(), c.getFlags()))
		if d := c.description(); d != "" {
			c.printf("%s\n", indent.String("    ", d))
			if c.getFlags() != nil {
				c.printf("\n")
			}
		}
		flags.Help(indent.NewWriter(w, "  "), "", "", c.getFlags())
		return nil
	}
	c.printf("Usage: %s\n", flags.UsageLine(c.Name, "subcommand [...]", c.getFlags()))
	if d := c.description(); d != "" {
		c.printf("%s\n", indent.String("    ", d))
		if c.getFlags() != nil {
			c.printf("\n")
		}
	}
	flags.Help(indent.NewWriter(w, "  "), "", "", c.getFlags())
	sc := c.SubCommands
	sort.Slice(sc, func(i, j int) bool { return sc[i].Name < sc[j].Name })
	c.printf("\nAvailable sub commands:")
	for _, sc := range c.SubCommands {
		parameters := sc.parameters()
		if parameters == "" && len(sc.SubCommands) > 0 {
			parameters = "subcommand [...]"
		}
		c.printf("\n%s\n", indent.String("  ", flags.UsageLine(sc.Name, parameters, sc.getFlags())))
		if d := sc.description(); d != "" {
			c.printf("%s\n", indent.String("    ", d))
		} else if sc.Help != "" {
			c.printf("%s\n", indent.String("    ", sc.Help))
		}
	}
	return nil
}

type helper struct {
	c *Command
}

func (c *Command) description() string {
	return strings.TrimSpace(c.Description)
}

func (c *Command) getFlags() any {
	if c.Flags != nil {
		return c.Flags
	}
	return c.Defaults
}

func (c *Command) parameters() string {
	if c.Parameters != "" {
		return c.Parameters
	}
	if c.MaxArgs == NoArgs {
		return ""
	}
	var b strings.Builder
	for i := 0; i < c.MinArgs; i++ {
		fmt.Fprintf(&b, " arg%d", i)
	}
	if c.MaxArgs == 0 || c.MaxArgs < c.MinArgs {
		fmt.Fprintf(&b, " ...")
	}
	return b.String()[1:]
}

func (h *helper) Set(s string) {}
func (h *helper) Value() any   { return nil }
