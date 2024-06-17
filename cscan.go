// Package cscan scans C input for include directives
package cscan

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type IncludeKind int

const (
	IncludeAngled = IncludeKind(tokAString)    // #include <assert.h>
	IncludeQuoted = IncludeKind(tokQString)    // #include "assert.h"
	IncludeMacro  = IncludeKind(tokIdentifier) // #include MACRO
)

type IncludeDirective struct {
	Path string

	Kind        IncludeKind
	IncludeNext bool // #include_next or #include?
	Line        int
	Pos, End    int // input[Pos:End] covers the directive including the newline

	NlPad string // should you decide to rewrite this include directive,
	// append NlPad to ensure that the number of lines doesn't change
}

func NewScanner(input string, fn ErrCallback) *Scanner {
	return &Scanner{input: input, softEnd: nextLineContOrEnd(input, 0), line: 1, errCallback: fn}
}

type ErrCallback func(*Error)

type Error struct {
	input string
	pos   int
	line  lineNum
	msg   string
}

type lineNum int

func (err *Error) Error() string {
	return fmt.Sprintf("%d:%d: error: %s", err.line, err.column(), err.msg)
}

func (err *Error) Quote() string {
	quote, rel := err.quoteInput()
	pos := utf8.RuneCountInString(quote[:rel])
	return quote + "\n" + strings.Repeat(" ", pos) + "^"
}

func (err *Error) column() int {
	quote, rel := err.quoteInput()
	return 1 + utf8.RuneCountInString(quote[:rel])
}

func (err *Error) quoteInput() (string, int) {
	pos := 1 + strings.LastIndexAny(err.input[:err.pos], "\r\n")
	quote := err.input[pos:]
	if endPos := strings.IndexAny(quote, "\r\n"); endPos != -1 {
		quote = quote[:endPos]
	}
	return quote, err.pos - pos
}

type Scanner struct {
	input   string
	pos     int
	softEnd int // pos of closest \line continuation after pos or len(input)

	line        lineNum
	prevSoftEnd int // tells whether we need to removeLineCont on the token text;
	// it is a mere optimisation, it should be safe to remove
	// regardless, which holds as long as quoted string tokens include
	// terminating " character for inputs such as "foo\\"

	errCallback ErrCallback
}

func (s *Scanner) NextInclude() *IncludeDirective {
	return nextInclude(s)
}

//go:generate re2go -W -Werror --no-debug-info --no-generation-date -o scanner.go scanner.re
//go:generate gofmt -w scanner.go

func (s *Scanner) reportError(msg string, pos int, line lineNum) {
	if s.errCallback == nil {
		return
	}
	s.errCallback(&Error{input: s.input, line: line, pos: pos, msg: msg})
}

const (
	errUnterminatedAString = "missing terminating '>' character"
	errUnterminatedCharLit = "missing terminating ' character"
	errUnterminatedComment = "unterminated /* comment"
	errUnterminatedQString = `missing terminating '"' character`
	errExpectedFilename    = `expected "FILENAME" or <FILENAME>`
	errEmptyFilename       = "empty filename"
)

func nextLineContOrEnd(input string, pos int) int {
	for {
		if pos >= len(input) {
			return len(input)
		}
		if input[pos] == '\\' && skipLineCont(input, pos) != pos {
			return pos
		}
		pos++
	}
}

func skipLineCont(input string, pos int) int {
	if pos >= len(input) {
		return len(input)
	}
	if input[pos] != '\\' {
		return pos
	}
	pp := pos + 1
	for {
		if pp >= len(input) {
			return pos
		}
		switch input[pp] {
		case ' ', '\t', '\v':
			pp++
		case '\r', '\n':
			return skipNewline(input, pp)
		default:
			return pos
		}
	}
}

func skipNewline(input string, pos int) int {
	if pos >= len(input) {
		return len(input)
	}
	switch input[pos] {
	case '\r':
		if pos+1 >= len(input) || input[pos+1] != '\n' {
			return pos + 1
		}
		return pos + 2
	case '\n':
		if pos+1 >= len(input) || input[pos+1] != '\r' {
			return pos + 1
		}
		return pos + 2
	default:
		return pos
	}
}

func removeLineCont(input string) string {
	var chunks []string
	for pos := 0; pos != len(input); {
		end := nextLineContOrEnd(input, pos)
		chunks = append(chunks, input[pos:end])
		pos = skipLineCont(input, end)
	}
	return strings.Join(chunks, "")
}

func countNewlines(input string) (int, string) {
	count, newline := 0, ""
	for pos := 0; pos < len(input); {
		switch input[pos] {
		case '\r', '\n':
			next := skipNewline(input, pos)
			newline = input[pos:next]
			count++
			pos = next
		default:
			pos++
		}
	}
	return count, newline
}

type tokenKind int

const (
	tokUnspec tokenKind = iota
	tokIdentifier
	tokHash
	tokInclude
	tokIncludeNext
	tokQString
	tokAString

	tokUnterminatedQString
	tokUnterminatedAString
)

type token struct {
	kind        tokenKind
	pos, end    int
	line        lineNum
	prevSoftEnd int
}

func (t *token) string(s *Scanner) string {
	tok := s.input[t.pos:t.end]
	if t.pos <= t.prevSoftEnd {
		return removeLineCont(tok)
	}
	return tok
}

func (t *token) endLine(s *Scanner) lineNum {
	linesDelta, _ := countNewlines(s.input[t.pos:t.end])
	return t.line + lineNum(linesDelta)
}

type tokenBag struct {
	count  int
	tokens [4]token
}

func (tbag *tokenBag) reset() {
	tbag.count = 0
}

func (tbag *tokenBag) push(k tokenKind, pos, end int, line lineNum, s *Scanner) {
	if tbag.count < len(tbag.tokens) {
		tbag.tokens[tbag.count] = token{
			kind: k, pos: pos, end: end, line: line, prevSoftEnd: s.prevSoftEnd,
		}
	}
	tbag.count++
}

func (tbag *tokenBag) isIncludeDirective() bool {
	if tbag.count >= 2 && tbag.tokens[0].kind == tokHash {
		return tbag.tokens[1].kind == tokInclude || tbag.tokens[1].kind == tokIncludeNext
	}
	return false
}

func (tbag *tokenBag) handleIncludeDirective(endPos int, s *Scanner) *IncludeDirective {
	if !tbag.isIncludeDirective() {
		return nil
	}
	hashTok, includeTok := tbag.tokens[0], tbag.tokens[1]
	if tbag.count == 2 {
		s.reportError(errExpectedFilename, includeTok.end, includeTok.endLine(s))
		return nil
	}
	argTok := tbag.tokens[2]
	path := argTok.string(s)
	switch argTok.kind {
	case tokIdentifier:
	case tokQString, tokAString:
		path = path[1 : len(path)-1]
	case tokUnterminatedQString:
		argTok.kind, path = tokQString, path[1:]
	case tokUnterminatedAString:
		argTok.kind, path = tokAString, path[1:]
	default:
		s.reportError(errExpectedFilename, argTok.pos, argTok.line)
		return nil
	}
	if len(path) == 0 {
		s.reportError(errEmptyFilename, argTok.pos, argTok.line)
		return nil
	}
	nlCount, newline := countNewlines(s.input[hashTok.pos:endPos])
	return &IncludeDirective{
		Path:        path,
		Kind:        IncludeKind(argTok.kind),
		IncludeNext: includeTok.kind == tokIncludeNext,
		Line:        int(hashTok.line),
		Pos:         hashTok.pos,
		End:         endPos,
		NlPad:       strings.Repeat(newline, nlCount),
	}
}
