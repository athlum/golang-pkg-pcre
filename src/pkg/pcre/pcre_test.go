// Copyright (C) 2011 Florian Weimer <fw@deneb.enyo.de>

package pcre

import (
	"testing"
)

func TestCompile(t *testing.T) {
	var check = func(p string, groups int) {
		re, err := Compile(p, 0)
		if err != nil {
			t.Error(p, err)
		}
		if g := re.Groups(); g != groups {
			t.Error(p, g)
		}
	}
	check("", 0)
	check("^", 0)
	check("^$", 0)
	check("()", 1)
	check("(())", 2)
	check("((?:))", 1)
}

func TestCompileFail(t *testing.T) {
	var check = func(p, msg string, off int) {
		_, err := Compile(p, 0)
		switch {
		case err == nil:
			t.Error(p)
		case err.Message != msg:
			t.Error(p, "Message", err.Message)
		case err.Offset != off:
			t.Error(p, "Offset", err.Offset)
		}
	}
	check("(", "missing )", 1)
	check("\\", "\\ at end of pattern", 1)
	check("abc\\", "\\ at end of pattern", 4)
	check("abc\000", "NUL byte in pattern", 3)
	check("a\000bc", "NUL byte in pattern", 1)
}

func strings(b [][]byte) (r []string) {
	r = make([]string, len(b))
	for i, v := range b {
		r[i] = string(v)
	}
	return
}

func equal(l, r []string) bool {
	if len(l) != len(r) {
		return false
	}
	for i, lv := range l {
		if lv != r[i] {
			return false
		}
	}
	return true
}

func checkmatch1(t *testing.T, dostring bool, m *Matcher,
	pattern, subject string, args ...interface{}) {
	re := MustCompile(pattern, 0)
	var (
		prefix string
		err    error
	)
	if dostring {
		if m == nil {
			m, err = re.MatcherString(subject, 0)
		} else {
			err = m.ResetString(re, subject, 0)
		}
		prefix = "string"
	} else {
		if m == nil {
			m, err = re.Matcher([]byte(subject), 0)
		} else {
			err = m.Reset(re, []byte(subject), 0)
		}
		prefix = "[]byte"
	}
	if err != nil {
		t.Error(err)
	}
	if len(args) == 0 {
		if m.Matches() {
			t.Error(prefix, pattern, subject, "!Matches")
		}
	} else {
		if !m.Matches() {
			t.Error(prefix, pattern, subject, "Matches")
			return
		}
		if m.Groups() != len(args)-1 {
			t.Error(prefix, pattern, subject, "Groups", m.Groups())
			return
		}
		for i, arg := range args {
			if s, ok := arg.(string); ok {
				if !m.Present(i) {
					t.Error(prefix, pattern, subject,
						"Present", i)

				}
				if g := string(m.Group(i)); g != s {
					t.Error(prefix, pattern, subject,
						"Group", i, g, "!=", s)
				}
				if g := m.GroupString(i); g != s {
					t.Error(prefix, pattern, subject,
						"GroupString", i, g, "!=", s)
				}
			} else {
				if m.Present(i) {
					t.Error(prefix, pattern, subject,
						"!Present", i)
				}
			}
		}
	}
}

func TestMatcher(t *testing.T) {
	var m Matcher
	check := func(pattern, subject string, args ...interface{}) {
		checkmatch1(t, false, nil, pattern, subject, args...)
		checkmatch1(t, true, nil, pattern, subject, args...)
		checkmatch1(t, false, &m, pattern, subject, args...)
		checkmatch1(t, true, &m, pattern, subject, args...)
	}

	check(`^$`, "", "")
	check(`^abc$`, "abc", "abc")
	check(`^(X)*ab(c)$`, "abc", "abc", nil, "c")
	check(`^(X)*ab()c$`, "abc", "abc", nil, "")
	check(`^.*$`, "abc", "abc")
	check(`^.*$`, "a\000c", "a\000c")
	check(`^(.*)$`, "a\000c", "a\000c", "a\000c")
}

func TestCaseless(t *testing.T) {
	m, err := MustCompile("abc", CASELESS).MatcherString("Abc", 0)
	if err != nil {
		t.Error(err)
	}
	if !m.Matches() {
		t.Error("CASELESS")
	}
	m, err = MustCompile("abc", 0).MatcherString("Abc", 0)
	if m.Matches() {
		t.Error("!CASELESS")
	}
}

func checkIndex(t *testing.T, k string, i, ii int) {
	if i != ii {
		t.Errorf("%v index %v, expected: %v", k, i, ii)
	}
}

func checkSubstring(t *testing.T, k, i, ii string) {
	if i != ii {
		t.Errorf("%v substring %v, expected: %v", k, i, ii)
	}
}

func TestNamedGroup(t *testing.T) {
	re := MustCompile(`{hostname: (?<hostname>.*), ip: (?<ip>.*), topic: (?<topic>.*)} (?<source_msg>.*)`, 0)
	for k, i := range re.NamedGroups() {
		switch k {
		case "hostname":
			checkIndex(t, k, i, 1)
		case "ip":
			checkIndex(t, k, i, 2)
		case "topic":
			checkIndex(t, k, i, 3)
		case "source_msg":
			checkIndex(t, k, i, 4)
		}
	}
	m, err := re.MatcherString(`{@timestamp=2018-07-10T11:38:42.963+08:00, message={hostname: adca-mesos-32.vm.elenet.me, ip: 10.101.64.117, topic: arch.appos_agent} {"error":"request: Post http://127.0.0.1:1988/metrics?key=docker: net/http: request canceled (Client.Timeout exceeded while awaiting headers)","indice":"appos.agent","level":"error","log_source":"appos-agent","msg":"Send stats","time":"2018-06-18T03:15:41+08:00"}}`, 0)
	if err != nil {
		t.Error(err)
	}
	for k, v := range m.NamedStringMap() {
		switch k {
		case "hostname":
			checkSubstring(t, k, v, "adca-mesos-32.vm.elenet.me")
		case "ip":
			checkSubstring(t, k, v, "10.101.64.117")
		case "topic":
			checkSubstring(t, k, v, "arch.appos_agent")
		case "source_msg":
			checkSubstring(t, k, v, `{"error":"request: Post http://127.0.0.1:1988/metrics?key=docker: net/http: request canceled (Client.Timeout exceeded while awaiting headers)","indice":"appos.agent","level":"error","log_source":"appos-agent","msg":"Send stats","time":"2018-06-18T03:15:41+08:00"}}`)
		}
	}
}

func TestNamed(t *testing.T) {
	m, err := MustCompile("(?<L>a)(?<M>X)*bc(?<DIGITS>\\d*)", 0).
		MatcherString("abc12", 0)
	if err != nil {
		t.Error(err)
	}
	if !m.Matches() {
		t.Error("Matches")
	}
	if !m.NamedPresent("L") {
		t.Error("NamedPresent(\"L\")")
	}
	if m.NamedPresent("M") {
		t.Error("NamedPresent(\"M\")")
	}
	if !m.NamedPresent("DIGITS") {
		t.Error("NamedPresent(\"DIGITS\")")
	}
	if "12" != m.NamedString("DIGITS") {
		t.Error("NamedString(\"DIGITS\")")
	}
}

func TestFindIndex(t *testing.T) {
	re := MustCompile("bcd", 0)
	i, err := re.FindIndex([]byte("abcdef"), 0)
	if err != nil {
		t.Error(err)
	}
	if i[0] != 1 {
		t.Error("FindIndex start", i[0])
	}
	if i[1] != 4 {
		t.Error("FindIndex end", i[1])
	}
}

func TestReplaceAll(t *testing.T) {
	re := MustCompile("foo", 0)
	// Don't change at ends.
	result, err := re.ReplaceAll([]byte("I like foods."), []byte("car"), 0)
	if err != nil {
		t.Error(err)
	}
	if string(result) != "I like cards." {
		t.Error("ReplaceAll", result)
	}
	// Change at ends.
	result, err = re.ReplaceAll([]byte("food fight fools foo"), []byte("car"), 0)
	if err != nil {
		t.Error(err)
	}
	if string(result) != "card fight carls car" {
		t.Error("ReplaceAll2", result)
	}
	// No changes.
	result, err = re.ReplaceAll([]byte("test no changes"), []byte("car"), 0)
	if err != nil {
		t.Error(err)
	}
	if string(result) != "test no changes" {
		t.Error("ReplaceAll2", result)
	}
}
