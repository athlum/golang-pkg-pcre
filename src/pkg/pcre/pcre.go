// Copyright (c) 2011 Florian Weimer. All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
// * Redistributions of source code must retain the above copyright
//   notice, this list of conditions and the following disclaimer.
//
// * Redistributions in binary form must reproduce the above copyright
//   notice, this list of conditions and the following disclaimer in the
//   documentation and/or other materials provided with the distribution.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

// This package provides access to the Perl Compatible Regular
// Expresion library, PCRE.
//
// It implements two main types, Regexp and Matcher.  Regexp objects
// store a compiled regular expression.  They are immutable.
// Compilation of regular expressions using Compile or MustCompile is
// slightly expensive, so these objects should be kept and reused,
// instead of compiling them from scratch for each matching attempt.
//
// Matcher objects keeps the results of a match against a []byte or
// string subject.  The Group and GroupString functions provide access
// to capture groups; both versions work no matter if the subject was a
// []byte or string, but the version with the matching type is slightly
// more efficient.
//
// Matcher objects contain some temporary space and refer the original
// subject.  They are mutable and can be reused (using Match,
// MatchString, Reset or ResetString).
//
// For details on the regular expression language implemented by this
// package and the flags defined below, see the PCRE documentation.
package pcre

/*
#cgo LDFLAGS: -lpcre
#cgo CFLAGS: -I/opt/local/include
#include <pcre.h>
#include <string.h>
*/
import "C"

import (
	"github.com/pkg/errors"
	"strconv"
	"unsafe"
)

// Flags for Compile and Match functions.
const (
	ANCHORED        = C.PCRE_ANCHORED
	BSR_ANYCRLF     = C.PCRE_BSR_ANYCRLF
	BSR_UNICODE     = C.PCRE_BSR_UNICODE
	NEWLINE_ANY     = C.PCRE_NEWLINE_ANY
	NEWLINE_ANYCRLF = C.PCRE_NEWLINE_ANYCRLF
	NEWLINE_CR      = C.PCRE_NEWLINE_CR
	NEWLINE_CRLF    = C.PCRE_NEWLINE_CRLF
	NEWLINE_LF      = C.PCRE_NEWLINE_LF
	NO_UTF8_CHECK   = C.PCRE_NO_UTF8_CHECK
)

// Flags for Compile functions
const (
	CASELESS          = C.PCRE_CASELESS
	DOLLAR_ENDONLY    = C.PCRE_DOLLAR_ENDONLY
	DOTALL            = C.PCRE_DOTALL
	DUPNAMES          = C.PCRE_DUPNAMES
	EXTENDED          = C.PCRE_EXTENDED
	EXTRA             = C.PCRE_EXTRA
	FIRSTLINE         = C.PCRE_FIRSTLINE
	JAVASCRIPT_COMPAT = C.PCRE_JAVASCRIPT_COMPAT
	MULTILINE         = C.PCRE_MULTILINE
	NO_AUTO_CAPTURE   = C.PCRE_NO_AUTO_CAPTURE
	UNGREEDY          = C.PCRE_UNGREEDY
	UTF8              = C.PCRE_UTF8
)

// Flags for Match functions
const (
	NOTBOL            = C.PCRE_NOTBOL
	NOTEOL            = C.PCRE_NOTEOL
	NOTEMPTY          = C.PCRE_NOTEMPTY
	NOTEMPTY_ATSTART  = C.PCRE_NOTEMPTY_ATSTART
	NO_START_OPTIMIZE = C.PCRE_NO_START_OPTIMIZE
	PARTIAL_HARD      = C.PCRE_PARTIAL_HARD
	PARTIAL_SOFT      = C.PCRE_PARTIAL_SOFT
)

var (
	PCRE_ERROR_NOMATCH    = errors.New("PCRE_ERROR_NOMATCH")
	PCRE_ERROR_MATCHLIMIT = errors.New("PCRE_ERROR_MATCHLIMIT")
)

// A reference to a compiled regular expression.
// Use Compile or MustCompile to create such objects.
type Regexp struct {
	ptr []byte
}

// Number of bytes in the compiled pattern
func pcresize(ptr *C.pcre) (size C.size_t) {
	C.pcre_fullinfo(ptr, nil, C.PCRE_INFO_SIZE, unsafe.Pointer(&size))
	return
}

// Number of capture groups
func pcregroups(ptr *C.pcre) (count C.int) {
	C.pcre_fullinfo(ptr, nil,
		C.PCRE_INFO_CAPTURECOUNT, unsafe.Pointer(&count))
	return
}

func pcrenamedgroups(ptr *C.pcre) (count C.int) {
	C.pcre_fullinfo(ptr, nil,
		C.PCRE_INFO_NAMECOUNT, unsafe.Pointer(&count))
	return
}

func pcrenamedgroupsentrysize(ptr *C.pcre) (size C.int) {
	C.pcre_fullinfo(ptr, nil,
		C.PCRE_INFO_NAMEENTRYSIZE, unsafe.Pointer(&size))
	return
}

func pcrenamedgroupslist(ptr *C.pcre) (cp *C.char) {
	C.pcre_fullinfo(ptr, nil,
		C.PCRE_INFO_NAMETABLE, unsafe.Pointer(&cp))
	return
}

// Move pattern to the Go heap so that we do not have to use a
// finalizer.  PCRE patterns are fully relocatable. (We do not use
// custom character tables.)
func toheap(ptr *C.pcre) (re Regexp) {
	defer C.free(unsafe.Pointer(ptr))
	size := pcresize(ptr)
	re.ptr = make([]byte, size)
	C.memcpy(unsafe.Pointer(&re.ptr[0]), unsafe.Pointer(ptr), size)
	return
}

// Try to compile the pattern.  If an error occurs, the second return
// value is non-nil.
func Compile(pattern string, flags int) (Regexp, *CompileError) {
	pattern1 := C.CString(pattern)
	defer C.free(unsafe.Pointer(pattern1))
	if clen := int(C.strlen(pattern1)); clen != len(pattern) {
		return Regexp{}, &CompileError{
			Pattern: pattern,
			Message: "NUL byte in pattern",
			Offset:  clen,
		}
	}
	var errptr *C.char
	var erroffset C.int
	ptr := C.pcre_compile(pattern1, C.int(flags), &errptr, &erroffset, nil)
	if ptr == nil {
		return Regexp{}, &CompileError{
			Pattern: pattern,
			Message: C.GoString(errptr),
			Offset:  int(erroffset),
		}
	}
	return toheap(ptr), nil
}

// Compile the pattern.  If compilation fails, panic.
func MustCompile(pattern string, flags int) (re Regexp) {
	re, err := Compile(pattern, flags)
	if err != nil {
		panic(err)
	}
	return
}

// Returns the number of capture groups in the compiled pattern.
func (re Regexp) Groups() int {
	if re.ptr == nil {
		panic("Regexp.Groups: uninitialized")
	}
	return int(pcregroups((*C.pcre)(unsafe.Pointer(&re.ptr[0]))))
}

func (re Regexp) NamedGroups() map[string]int {
	length := int(pcrenamedgroups((*C.pcre)(unsafe.Pointer(&re.ptr[0]))))
	entrySize := int(pcrenamedgroupsentrysize((*C.pcre)(unsafe.Pointer(&re.ptr[0]))))
	pc := pcrenamedgroupslist((*C.pcre)(unsafe.Pointer(&re.ptr[0])))

	groups := make(map[string]int)
	for i := 0; i < length; i += 1 {
		g := (*[1 << 30]byte)(unsafe.Pointer(pc))[i*entrySize : (i+1)*entrySize]
		s := len(g)
		for ii, v := range g[2:s] {
			if int(v) == 0 {
				s = ii + 2
				break
			}
		}
		groups[string(g[2:s])] = int(g[1])
	}
	return groups
}

// Matcher objects provide a place for storing match results.
// They can be created by the Matcher and MatcherString functions,
// or they can be initialized with Reset or ResetString.
type Matcher struct {
	re       Regexp
	groups   int
	ovector  []C.int // scratch space for capture offsets
	matches  bool    // last match was successful
	subjects string  // one of these fields is set to record the subject,
	subjectb []byte  // so that Group/GroupString can return slices
}

// Returns a new matcher object, with the byte array slice as a
// subject.
func (re Regexp) Matcher(subject []byte, flags int) (m *Matcher, err error) {
	m = new(Matcher)
	err = m.Reset(re, subject, flags)
	return
}

// Returns a new matcher object, with the specified subject string.
func (re Regexp) MatcherString(subject string, flags int) (m *Matcher, err error) {
	m = new(Matcher)
	err = m.ResetString(re, subject, flags)
	return
}

// Switches the matcher object to the specified pattern and subject.
func (m *Matcher) Reset(re Regexp, subject []byte, flags int) error {
	if re.ptr == nil {
		panic("Regexp.Matcher: uninitialized")
	}
	m.init(re)
	_, err := m.Match(subject, flags)
	return err
}

// Switches the matcher object to the specified pattern and subject
// string.
func (m *Matcher) ResetString(re Regexp, subject string, flags int) error {
	if re.ptr == nil {
		panic("Regexp.Matcher: uninitialized")
	}
	m.init(re)
	_, err := m.MatchString(subject, flags)
	return err
}

func (m *Matcher) init(re Regexp) {
	m.matches = false
	if m.re.ptr != nil && &m.re.ptr[0] == &re.ptr[0] {
		// Skip group count extraction if the matcher has
		// already been initialized with the same regular
		// expression.
		return
	}
	m.re = re
	m.groups = re.Groups()
	if ovectorlen := 3 * (1 + m.groups); len(m.ovector) < ovectorlen {
		m.ovector = make([]C.int, ovectorlen)
	}
}

var nullbyte = []byte{0}

// Tries to match the speficied byte array slice to the current
// pattern.  Returns true if the match succeeds.
func (m *Matcher) Match(subject []byte, flags int) (bool, error) {
	if m.re.ptr == nil {
		panic("Matcher.Match: uninitialized")
	}
	length := len(subject)
	m.subjects = ""
	m.subjectb = subject
	if length == 0 {
		subject = nullbyte // make first character adressable
	}
	subjectptr := (*C.char)(unsafe.Pointer(&subject[0]))
	return m.match(subjectptr, length, flags)
}

// Tries to match the speficied subject string to the current pattern.
// Returns true if the match succeeds.
func (m *Matcher) MatchString(subject string, flags int) (bool, error) {
	if m.re.ptr == nil {
		panic("Matcher.Match: uninitialized")
	}
	length := len(subject)
	m.subjects = subject
	m.subjectb = nil
	if length == 0 {
		subject = "\000" // make first character addressable
	}
	// The following is a non-portable kludge to avoid a copy
	subjectptr := *(**C.char)(unsafe.Pointer(&subject))
	return m.match(subjectptr, length, flags)
}

func (m *Matcher) match(subjectptr *C.char, length, flags int) (bool, error) {
	rc := C.pcre_exec((*C.pcre)(unsafe.Pointer(&m.re.ptr[0])), nil,
		subjectptr, C.int(length),
		0, C.int(flags), &m.ovector[0], C.int(len(m.ovector)))
	switch {
	case rc >= 0:
		m.matches = true
		return true, nil
	case rc == C.PCRE_ERROR_NOMATCH:
		m.matches = false
		return false, PCRE_ERROR_NOMATCH
	case rc == C.PCRE_ERROR_MATCHLIMIT:
		m.matches = false
		return false, PCRE_ERROR_MATCHLIMIT
	case rc == C.PCRE_ERROR_BADOPTION:
		panic("PCRE.Match: invalid option flag")
	}
	panic("unexepected return code from pcre_exec: " +
		strconv.Itoa(int(rc)))
}

// Returns true if a previous call to Matcher, MatcherString, Reset,
// ResetString, Match or MatchString succeeded.
func (m *Matcher) Matches() bool {
	return m.matches
}

// Returns the number of groups in the current pattern.
func (m *Matcher) Groups() int {
	return m.groups
}

// Returns true if the numbered capture group is present in the last
// match (performed by Matcher, MatcherString, Reset, ResetString,
// Match, or MatchString).  Group numbers start at 1.  A capture group
// can be present and match the empty string.
func (m *Matcher) Present(group int) bool {
	return m.ovector[2*group] >= 0
}

// Returns the numbered capture group of the last match (performed by
// Matcher, MatcherString, Reset, ResetString, Match, or MatchString).
// Group 0 is the part of the subject which matches the whole pattern;
// the first actual capture group is numbered 1.  Capture groups which
// are not present return a nil slice.
func (m *Matcher) Group(group int) []byte {
	start := m.ovector[2*group]
	end := m.ovector[2*group+1]
	if start >= 0 {
		if m.subjectb != nil {
			return m.subjectb[start:end]
		}
		return []byte(m.subjects[start:end])
	}
	return nil
}

// Returns the numbered capture group as a string.  Group 0 is the
// part of the subject which matches the whole pattern; the first
// actual capture group is numbered 1.  Capture groups which are not
// present return an empty string.
func (m *Matcher) GroupString(group int) string {
	start := m.ovector[2*group]
	end := m.ovector[2*group+1]
	if start >= 0 {
		if m.subjectb != nil {
			return string(m.subjectb[start:end])
		}
		return m.subjects[start:end]
	}
	return ""
}

func (m *Matcher) NamedStringMap() map[string]string {
	nm := m.re.NamedGroups()
	sm := make(map[string]string)
	for k, v := range nm {
		sm[k] = m.GroupString(v)
	}
	return sm
}

func (m *Matcher) name2index(name string) (group int) {
	if m.re.ptr == nil {
		panic("Matcher.Named: uninitialized")
	}
	name1 := C.CString(name)
	defer C.free(unsafe.Pointer(name1))
	group = int(C.pcre_get_stringnumber(
		(*C.pcre)(unsafe.Pointer(&m.re.ptr[0])), name1))
	if group < 0 {
		panic("Matcher.Named: unknown name: " + name)
	}
	return
}

// Returns the value of the named capture group.  This is a nil slice
// if the capture group is not present.  Panics if the name does not
// refer to a group.
func (m *Matcher) Named(group string) []byte {
	return m.Group(m.name2index(group))
}

// Returns the value of the named capture group, or an empty string if
// the capture group is not present.  Panics if the name does not
// refer to a group.
func (m *Matcher) NamedString(group string) string {
	return m.GroupString(m.name2index(group))
}

// Returns true if the named capture group is present.  Panics if the
// name does not refer to a group.
func (m *Matcher) NamedPresent(group string) bool {
	return m.Present(m.name2index(group))
}

// Return the start and end of the first match, or nil if no match.
// loc[0] is the start and loc[1] is the end.
func (re *Regexp) FindIndex(bytes []byte, flags int) ([]int, error) {
	m, err := re.Matcher(bytes, flags)
	if err != nil {
		return nil, err
	}
	matched, err := m.Match(bytes, flags)
	if matched {
		return []int{int(m.ovector[0]), int(m.ovector[1])}, err
	}
	return nil, err
}

// Return a copy of a byte slice with pattern matches replaced by repl.
func (re Regexp) ReplaceAll(bytes, repl []byte, flags int) ([]byte, error) {
	m, err := re.Matcher(bytes, 0)
	if err != nil {
		return nil, err
	}
	r := []byte{}
	var matched bool = true
	for matched {
		if matched, err = m.Match(bytes, flags); matched {
			r = append(append(r, bytes[:m.ovector[0]]...), repl...)
			bytes = bytes[m.ovector[1]:]
		}
	}
	if err == PCRE_ERROR_NOMATCH { //err will be NOMATCH for all time.
		err = nil
	}
	return append(r, bytes...), err
}

// A compilation error, as returned by the Compile function.  The
// offset is the byte position in the pattern string at which the
// error was detected.
type CompileError struct {
	Pattern string
	Message string
	Offset  int
}

func (e *CompileError) Error() string {
	return e.String()
}

func (e *CompileError) String() string {
	return e.Pattern + " (" + strconv.Itoa(e.Offset) + "): " + e.Message
}
