# gocscan: scan C files for include directives

Create a scanner object via `cscan.NewScanner`. Call `NextInclude` method to enumerate include directives.

`IncludeDirective` object captures the included file path as well as include kind (`""` or `<>`).
The object also records directive's offsets in the input so that it is easy to rewrite the directive if needed.

The module aims to handle most reasonable C code found in the wild without issues.
We don't support trigraphs but we handle line contunations properly.
