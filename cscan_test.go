package cscan

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRemoveLineCont(t *testing.T) {
	for _, test := range []struct{ input, res string }{
		{},
		{"foo", "foo"},
		{"foo\\\nbar", "foobar"},
		{"foo\\ \nbar", "foobar"},
		{"foo\\bar", "foo\\bar"},
		{"foo\\", "foo\\"},
		{"foo\\\n", "foo"},
	} {
		t.Run(test.input, func(t *testing.T) {
			assert.Equal(t, test.res, removeLineCont(test.input))
		})
	}
}

func TestScanner(t *testing.T) {
	const (
		killer = "/*\n\n*/ # /*\n*/i\\\nn\\\nc\\\nl\\\nu\\\nd\\\ne<\\\ns\\\nt\\\nd\\\ni\\\no\\\n.\\\nh> /*\n*/"
	)
	type include struct {
		path, fullText string
		kind           IncludeKind
		includeNext    bool
		line           int
	}
	type err struct {
		msg  string
		line lineNum
		col  int
	}
	for _, test := range []struct {
		label, input string
		res          []include
		errors       []err
	}{
		{"empty", "", nil, nil},
		{"angled_basic", "#include <stdio.h>\n", []include{
			{path: "stdio.h", kind: IncludeAngled, line: 1, fullText: "#include <stdio.h>\n"},
		}, nil},
		{"angled_next_basic", "#include_next <stdio.h>\n", []include{
			{path: "stdio.h", kind: IncludeAngled, includeNext: true, line: 1, fullText: "#include_next <stdio.h>\n"},
		}, nil},
		{"angled_escape", "#include <stdio.h\\> >\n", []include{
			{path: "stdio.h\\> ", kind: IncludeAngled, line: 1, fullText: "#include <stdio.h\\> >\n"},
		}, nil},
		{"quoted_basic", `#include "stdio.h"\n`, []include{
			{path: "stdio.h", kind: IncludeQuoted, line: 1, fullText: `#include "stdio.h"\n`},
		}, nil},
		{"quoted_escape", `#include "stdio\\"h"\n`, []include{
			{path: `stdio\\"h`, kind: IncludeQuoted, line: 1, fullText: `#include "stdio\\"h"\n`},
		}, nil},
		{"quoted_escape2", `#include "stdio\\\\\\"h"\n`, []include{
			{path: `stdio\\\\\\"h`, kind: IncludeQuoted, line: 1, fullText: `#include "stdio\\\\\\"h"\n`},
		}, nil},
		{"macro_basic", "#include NAME\n", []include{
			{path: "NAME", kind: IncludeMacro, line: 1, fullText: "#include NAME\n"},
		}, nil},
		{"multi_basic", "#include <stddef.h>\n#include <stdio.h>\n", []include{
			{path: "stddef.h", kind: IncludeAngled, line: 1, fullText: "#include <stddef.h>\n"},
			{path: "stdio.h", kind: IncludeAngled, line: 2, fullText: "#include <stdio.h>\n"},
		}, nil},
		{"angled_at_eof", "\n#include <stdio.h>", []include{
			{path: "stdio.h", kind: IncludeAngled, line: 2, fullText: "#include <stdio.h>"},
		}, nil},
		{"cpp_comment", "//#include<stdio.h>\n", nil, nil},
		{"cpp_comment_at_eof", "//#include<stdio.h>", nil, nil},
		{"comment", "/*\n#include<stdio.h>\n*/", nil, nil},
		{"comment2", "/*****\n#include<stdio.h>\n*/", nil, nil},
		{"comment_unterm", "/*\n#include<stdio.h>\n*", nil, []err{
			{msg: errUnterminatedComment, line: 1, col: 1},
		}},
		{"comment_bumps_lineno", "/* :)\n*/ #include <stdio.h>\n", []include{
			{path: "stdio.h", kind: IncludeAngled, line: 2, fullText: "#include <stdio.h>\n"},
		}, nil},
		{"line_cont", "#in\\\nclude <std\\\nio.h>\n", []include{
			{path: "stdio.h", kind: IncludeAngled, line: 1, fullText: "#in\\\nclude <std\\\nio.h>\n"},
		}, nil},
		{"angled_unterm", "#include <stdio.h\n", []include{
			{path: "stdio.h", kind: IncludeAngled, line: 1, fullText: "#include <stdio.h\n"},
		}, []err{
			{msg: errUnterminatedAString, col: 10, line: 1},
		}},
		{"angled_unterm2", "#include <stdio.h\\>\n", []include{
			{path: "stdio.h\\>", kind: IncludeAngled, line: 1, fullText: "#include <stdio.h\\>\n"},
		}, []err{
			{msg: errUnterminatedAString, col: 10, line: 1},
		}},
		{"angled_unterm_at_eof", "#include <stdio.h", []include{
			{path: "stdio.h", kind: IncludeAngled, line: 1, fullText: "#include <stdio.h"},
		}, []err{
			{msg: errUnterminatedAString, col: 10, line: 1},
		}},
		{"angled_unterm_multi", "#include <stdio.h\n #include <stddef.h>", []include{
			{path: "stdio.h", kind: IncludeAngled, line: 1, fullText: "#include <stdio.h\n"},
			{path: "stddef.h", kind: IncludeAngled, line: 2, fullText: "#include <stddef.h>"},
		}, []err{
			{msg: errUnterminatedAString, col: 10, line: 1},
		}},
		{"angled_unterm_multi_line_cont", "#include <stdio.h\\\n\n #include <stddef.h>", []include{
			{path: "stdio.h", kind: IncludeAngled, line: 1, fullText: "#include <stdio.h\\\n\n"},
			{path: "stddef.h", kind: IncludeAngled, line: 3, fullText: "#include <stddef.h>"},
		}, []err{
			{msg: errUnterminatedAString, col: 10, line: 1},
		}},
		{"quoted_unterm", "#include \"stdio.h\n", []include{
			{path: "stdio.h", kind: IncludeQuoted, line: 1, fullText: "#include \"stdio.h\n"},
		}, []err{
			{msg: errUnterminatedQString, col: 10, line: 1},
		}},
		{"quoted_unterm2", "#include \"stdio.h\\\"\n", []include{
			{path: "stdio.h\\\"", kind: IncludeQuoted, line: 1, fullText: "#include \"stdio.h\\\"\n"},
		}, []err{
			{msg: errUnterminatedQString, col: 10, line: 1},
		}},
		{"quoted_unterm_at_eof", `#include "stdio.h`, []include{
			{path: "stdio.h", kind: IncludeQuoted, line: 1, fullText: `#include "stdio.h`},
		}, []err{
			{msg: errUnterminatedQString, col: 10, line: 1},
		}},
		{"quoted_unterm_multi", "#include \"stdio.h\n #include \"stddef.h\"", []include{
			{path: "stdio.h", kind: IncludeQuoted, line: 1, fullText: "#include \"stdio.h\n"},
			{path: "stddef.h", kind: IncludeQuoted, line: 2, fullText: "#include \"stddef.h\""},
		}, []err{
			{msg: errUnterminatedQString, col: 10, line: 1},
		}},
		{"char_lit", " # include 's'\n", nil, []err{
			{msg: errExpectedFilename, line: 1, col: 12},
		}},
		{"char_lit_unterm", "#include 's\n", nil, []err{
			{msg: errUnterminatedCharLit, line: 1, col: 10},
			{msg: errExpectedFilename, line: 1, col: 10},
		}},
		{"char_lit_unterm_at_eof", "#include 's", nil, []err{
			{msg: errUnterminatedCharLit, line: 1, col: 10},
			{msg: errExpectedFilename, line: 1, col: 10},
		}},
		{"char_lit_unterm_multi", "#include 's\n#include <stdio.h>", []include{
			{path: "stdio.h", kind: IncludeAngled, line: 2, fullText: "#include <stdio.h>"},
		}, []err{
			{msg: errUnterminatedCharLit, line: 1, col: 10},
			{msg: errExpectedFilename, line: 1, col: 10},
		}},
		{"include_no_filename", " # include \n", nil, []err{
			{msg: errExpectedFilename, line: 1, col: 11},
		}},
		{"include_empty_filename", " # include <>\n", nil, []err{
			{msg: errEmptyFilename, line: 1, col: 12},
		}},
		{"include_empty_filename2", " # include <\n", nil, []err{
			{msg: errUnterminatedAString, line: 1, col: 12},
			{msg: errEmptyFilename, line: 1, col: 12},
		}},
		{"include_empty_filename3", " # include \"\n", nil, []err{
			{msg: errUnterminatedQString, line: 1, col: 12},
			{msg: errEmptyFilename, line: 1, col: 12},
		}},
		{"killer", killer, []include{
			{path: "stdio.h", kind: IncludeAngled, line: 3, fullText: killer[7:]},
		}, nil},
	} {
		t.Run(test.label, func(t *testing.T) {
			var errors []err
			s := NewScanner(test.input, func(e *Error) {
				errors = append(errors, err{msg: e.msg, line: e.line, col: e.column()})
			})
			var res []include
			for {
				i := s.NextInclude()
				if i == nil {
					break
				}
				res = append(res, include{
					path: i.Path, kind: i.Kind, includeNext: i.IncludeNext,
					line: i.Line, fullText: test.input[i.Pos:i.End],
				})
			}
			assert.Equal(t, test.res, res)
			assert.Equal(t, test.errors, errors)
		})
	}
}
