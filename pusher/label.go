//    This file is part of rusage.
//
//    rusage is free software: you can redistribute it and/or modify it under
//    the terms of the GNU General Public License as published by the Free
//    Software Foundation, either version 3 of the License, or (at your option)
//    any later version.
//
//    rusage is distributed in the hope that it will be useful, but WITHOUT ANY
//    WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS
//    FOR A PARTICULAR PURPOSE.  See the GNU General Public License for more
//    details.
//
//    You should have received a copy of the GNU General Public License along
//    with rusage.  If not, see <http://www.gnu.org/licenses/>.

package pusher

import (
	"flag"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/prometheus/common/model"
)

// Labels is a set of (grouping) labels to be added to metrics.
type Labels struct {
	hasBeenReset bool
	labelSet     model.LabelSet
}

func NewLabels(ls model.LabelSet) *Labels { return &Labels{labelSet: ls} }

// Set parses a parameter into a set of (grouping) labels.  Can be repeated.
//
// Does not support `/` or `=` as label name or `,` as label name or label
// value.  Leading and trailing white space and quotes are mangled.  For the
// restriction of `/` as label value, see
// https://github.com/prometheus/pushgateway/issues/97 -- the others are simply
// due to naive parsing.
func (ll *Labels) Set(v string) error {
	if !ll.hasBeenReset {
		ll.labelSet = make(model.LabelSet)
		ll.hasBeenReset = true
	}

	for _, l := range strings.FieldsFunc(v, comma) {
		kv := strings.SplitN(l, "=", 2)
		if len(kv) != 2 {
			return fmt.Errorf("not a valid label: %q", l)
		}
		k, v := trimSpaceAndQuote(kv[0]), trimSpaceAndQuote(kv[1])
		labelName, labelValue := model.LabelName(k), model.LabelValue(v)
		if !labelName.IsValid() || strings.ContainsRune(k, '/') || !labelValue.IsValid() {
			return fmt.Errorf("not a valid label: %q", l)
		}
		ll.labelSet[labelName] = labelValue
	}
	return nil
}

func (ll *Labels) String() string {
	ss := make([]string, 0, len(ll.labelSet))
	for k, v := range ll.labelSet {
		ss = append(ss, fmt.Sprintf("%s=%q", k, v))
	}
	sort.Strings(ss)
	return strings.Join(ss, ",")
}

func (ll *Labels) LabelSet() model.LabelSet { return model.LabelSet(ll.labelSet) }

func comma(r rune) bool                 { return r == ',' }
func trimSpaceAndQuote(s string) string { return strings.TrimFunc(s, spaceOrQuote) }
func spaceOrQuote(r rune) bool          { return r == '"' || unicode.IsSpace(r) }

var (
	_ flag.Value = (*Labels)(nil) // static type assert
)
