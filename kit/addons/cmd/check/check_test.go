package main

import (
	"bytes"
	"strings"
	"testing"
)

func check(args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestTags(t *testing.T) {
	cases := []struct{ args []string; want string }{
		{[]string{"-known", "vale", ""}, ""},
		{[]string{"-known", "vale", "vale"}, "addon_vale"},
		{[]string{"-known", "vale,voice", "vale,voice"}, "addon_vale,addon_voice"},
		{[]string{"-known", "vale,voice", " voice , vale ,vale"}, "addon_voice,addon_vale"}, // trimmed, deduplicated, order kept
		{[]string{"-known", "vale"}, ""},                                                        // no list: no add-ons
	}
	for _, c := range cases {
		code, out, errOut := check(c.args...)
		if code != 0 || strings.TrimSpace(out) != c.want || errOut != "" {
			t.Errorf("%q: code %d, out %q, err %q; want %q", c.args, code, out, errOut, c.want)
		}
	}
}

func TestUnknown(t *testing.T) {
	for _, list := range []string{"voice", "vale,nope", "Vale", "va le", "../x", "vale;rm", "a,,b"} {
		code, out, errOut := check("-known", "vale", list)
		if code == 0 || out != "" {
			t.Errorf("%q: code %d, out %q; want failure and no tags", list, code, out)
		}
		if !strings.Contains(errOut, "known add-ons: vale") {
			t.Errorf("%q: error %q does not list the known add-ons", list, errOut)
		}
	}
}
