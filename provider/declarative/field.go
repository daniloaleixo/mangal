package declarative

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Extract returns the value described by the spec, read from the given element.
//
// The rule, in order: resolve Selector if set (otherwise use the element
// itself), read Attr if set (otherwise the text), apply Regex if set, then
// trim surrounding whitespace.
func (f *FieldSpec) Extract(s *goquery.Selection) string {
	if f == nil {
		return ""
	}

	target := s
	if f.Selector != "" {
		target = s.Find(f.Selector).First()
	}

	var raw string
	if f.Attr != "" {
		raw = target.AttrOr(f.Attr, "")
	} else {
		raw = target.Text()
	}

	if f.regex != nil {
		if f.Replace != nil {
			raw = f.regex.ReplaceAllString(raw, *f.Replace)
		} else {
			match := f.regex.FindStringSubmatch(raw)
			switch {
			case match == nil:
				raw = ""
			case len(match) > 1:
				raw = match[1]
			default:
				raw = match[0]
			}
		}
	}

	return strings.TrimSpace(raw)
}

// compile prepares the regex and validates the field. Called during parsing so
// that a bad pattern fails at load time rather than mid-download.
func (f *FieldSpec) compile() error {
	if f == nil {
		return nil
	}

	if f.Regex == "" {
		if f.Replace != nil {
			return errors.New("replace requires regex")
		}
		return nil
	}

	re, err := regexp.Compile(f.Regex)
	if err != nil {
		return fmt.Errorf("invalid regex %q: %w", f.Regex, err)
	}

	f.regex = re
	return nil
}
