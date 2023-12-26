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
	"strings"
)

// The following are options to the SplitCommand function.  They determine how
// to split the command.  Assuming the deliminter is ';' then command1 and
// command2:
//
//	StrictDelim - command1 ; command2 (always works)
//	TrailingDelim - command1; command2
//	PreceedingDelim - command1 ;command2
//	AnyDelim - command1;command2
const (
	StrictDelim   = 0
	TrailingDelim = 1 << iota
	PreceedingDelim
	AnyDelim
)

func SplitCommand(args []string, delim string, options int) [][]string {
	var words []string
	if options != StrictDelim {
		for _, arg := range args {
			if arg == delim {
				words = append(words, arg)
				continue
			}
			if (options & AnyDelim) != 0 {
				for _, part := range strings.Split(arg, delim) {
					if len(part) > 0 {
						words = append(words, part, delim)
					} else {
						words = append(words, delim)
					}
				}
				continue
			}
			if (options & PreceedingDelim) != 0 {
				if strings.HasPrefix(arg, delim) {
					words = append(words, delim)
					arg = strings.TrimPrefix(arg, delim)
				}
			}
			if (options & TrailingDelim) != 0 {
				if strings.HasSuffix(arg, delim) {
					words = append(words, strings.TrimSuffix(arg, delim), delim)
					continue
				}
			}
			words = append(words, arg)
		}
		args = words
	}
	var cmds [][]string
	start := 0
	for i, arg := range args {
		if arg == delim {
			if i > start {
				cmds = append(cmds, args[start:i])
			}
			start = i + 1
			continue
		}
	}
	if start < len(args) {
		cmds = append(cmds, args[start:])
	}
	return cmds
}
