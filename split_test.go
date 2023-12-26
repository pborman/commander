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
	"fmt"
	"testing"
)

func TestSplit(t *testing.T) {
	args := []string{"a", ";b", "c;", "d;e", ";f;g;", ";", "h"}
	for _, tt := range []struct {
		name    string
		options int
		want    [][]string
	}{{
		name: "strict",
		want: [][]string{{"a", ";b", "c;", "d;e", ";f;g;"}, {"h"}},
	}, {
		name:    "trailing",
		options: TrailingDelim,
		want:    [][]string{{"a", ";b", "c"}, {"d;e", ";f;g"}, {"h"}},
	}, {
		name:    "preededing",
		options: PreceedingDelim,
		want:    [][]string{{"a"}, {"b", "c;", "d;e"}, {"f;g;"}, {"h"}},
	}, {
		name:    "trailing&preededing",
		options: PreceedingDelim | TrailingDelim,
		want:    [][]string{{"a"}, {"b", "c"}, {"d;e"}, {"f;g"}, {"h"}},
	}, {
		name:    "any",
		options: AnyDelim,
		want:    [][]string{{"a"}, {"b"}, {"c"}, {"d"}, {"e"}, {"f"}, {"g"}, {"h"}},
	}} {
		got := SplitCommand(args, ";", tt.options)
		gots := fmt.Sprintf("%q", got)
		wants := fmt.Sprintf("%q", tt.want)

		if gots != wants {
			t.Errorf("%s: got\n%s\nwant:\n%s", tt.name, gots, wants)
		}
	}
}
