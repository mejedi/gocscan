package cscan

func nextInclude(s *Scanner) *IncludeDirective {
	var tbag tokenBag

	buf := s.input[:s.softEnd]
	pos := s.pos

	/*!re2c

	// UNREASONABLY PERFORMANT C SCANNER
	// Re2c docs suggest sentinel as the most efficient method to
	// handle end of input. Keep in mind though that Golang peforms
	// implicit bounds check on every slice access anyway. Instead of a
	// sentinel, it is more efficient to use EXPLICIT bounds check to
	// detect end of input, as it inhibits generation of implicit bounds
	// check.
	//
	// Further note irt sentinel and YYLESSTHAN. Once YYPEEK detects EOF
	// it still needs to return something, we go with 0 (must match
	// re2c:eof). A subsequent YYLESSTHAN disambiguates EOF vs. \0
	// occuring in the input.

	re2c:define:YYCTYPE = byte;
	re2c:eof = 0;
	re2c:define:YYPEEK = "0; if pos<0||pos>=len(buf){s,buf,pos,yych=yypeek(s,len(buf))}else{yych=buf[pos]}";
	re2c:define:YYLESSTHAN = "pos < 0 || pos >= len(buf)";
	re2c:define:YYSKIP = "pos++";
	re2c:yyfill:enable = 0;

	any     = [\000-\377];
	nlchar = [\r\n];
	Newline = "\r\n" | "\n\r" | "\n" | "\r";
	*/

acceptToken:
	tokBegin, tokLine := pos, s.line

	/*!re2c

	"//" (any \ nlchar)*  { goto acceptToken }
	"/*"                   { goto comment  }
	[ \t\f\v]              { goto acceptToken }
	"#"                    { tbag.push(tokHash, tokBegin, pos, tokLine, s); goto acceptToken }

	"\"" { goto quotedString }
	"'"  { goto charLit }

	"<" {
		if tbag.count == 2 && tbag.isIncludeDirective() {
			goto angledString
		}
		tbag.push(tokUnspec, tokBegin, pos, tokLine, s)
		goto acceptToken
	}

	[a-zA-Z_][a-zA-Z_0-9]* {
		tbag.push(tokIdentifier, tokBegin, pos, tokLine, s)
		if tbag.count == 2 {
			switch tbag.tokens[1].string(s) {
			case "include":
				tbag.tokens[1].kind = tokInclude
			case "include_next":
				tbag.tokens[1].kind = tokIncludeNext
			}
		}
		goto acceptToken
	}

	Newline {
		s.line++
		if includeDir := tbag.handleIncludeDirective(pos, s); includeDir != nil {
			s.softEnd, s.pos = len(buf), pos
			return includeDir
		}
		tbag.reset()
		goto acceptToken
	}

	$ {
		s.softEnd, s.pos = len(buf), pos
		return tbag.handleIncludeDirective(pos, s)
	}

	any { tbag.push(tokUnspec, tokBegin, pos, tokLine, s); goto acceptToken }

	*/

comment:
	/*!re2c

	[*][/]  { goto acceptToken }
	Newline { s.line++; goto comment }
	any     { goto comment }

	$ {
		s.reportError(errUnterminatedComment, tokBegin, tokLine)
		goto acceptToken
	}

	*/

quotedString:
	/*!re2c

	["] {
		tbag.push(tokQString, tokBegin, pos, tokLine, s)
		goto acceptToken
	}

	[\\]["] { goto quotedString }

	nlchar {
		s.reportError(errUnterminatedQString, tokBegin, tokLine)
		tbag.push(tokUnterminatedQString, tokBegin, pos-1, tokLine, s)
		pos--; goto acceptToken
	}

	any { goto quotedString }

	$ {
		s.reportError(errUnterminatedQString, tokBegin, tokLine)
		tbag.push(tokUnterminatedQString, tokBegin, pos, tokLine, s)
		goto acceptToken
	}

	*/

charLit:
	/*!re2c

	['] {
		tbag.push(tokUnspec, tokBegin, pos, tokLine, s)
		goto acceptToken
	}

	[\\]['] { goto charLit }

	nlchar {
		s.reportError(errUnterminatedCharLit, tokBegin, tokLine)
		tbag.push(tokUnspec, tokBegin, pos-1, tokLine, s)
		pos--
		goto acceptToken
	}

	any { goto charLit }

	$ {
		s.reportError(errUnterminatedCharLit, tokBegin, tokLine)
		tbag.push(tokUnspec, tokBegin, pos, tokLine, s)
		goto acceptToken
	}

	*/

angledString:
	/*!re2c

	[>] {
		tbag.push(tokAString, tokBegin, pos, tokLine, s)
		goto acceptToken
	}

	[\\][>] { goto angledString }

	nlchar {
		s.reportError(errUnterminatedAString, tokBegin, tokLine)
		tbag.push(tokUnterminatedAString, tokBegin, pos-1, tokLine, s)
		pos--; goto acceptToken
	}

	any { goto angledString }

	$ {
		s.reportError(errUnterminatedAString, tokBegin, tokLine)
		tbag.push(tokUnterminatedAString, tokBegin, pos, tokLine, s)
		goto acceptToken
	}

	*/
}

//go:noinline
func yypeek(s *Scanner, softEnd int) (*Scanner, string, int, byte) {
	// Note: no callee-save registers, roundtripping Scanner to avoid
	// spills in caller
	s.prevSoftEnd = softEnd
	for {
		pos := skipLineCont(s.input, softEnd)
		if pos == softEnd {
			break
		}
		s.line++
		softEnd = nextLineContOrEnd(s.input, pos)
		if pos != softEnd {
			return s, s.input[:softEnd], pos, s.input[pos]
		}
	}
	return s, s.input, len(s.input), 0
}
