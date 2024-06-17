# gocscan: scan C files for include directives

Create a scanner object via `cscan.NewScanner`. Call `NextInclude` method to enumerate include directives.

`IncludeDirective` object captures the included file path as well as include kind (`""` or `<>`).
The object also records directive's offsets in the input so that it is easy to rewrite the directive if needed.

The module aims to handle most reasonable C code found in the wild without issues.
We don't support trigraphs but we handle line contunations properly.

## Limitations
Properly identifying include files requires a complete C preprocessor AND the full set
of preprocessor definitions used by the actual compiler. Include directives
can be wrapped in conditionals:

```c
#if CONDITION
#include "bar.h"
#endif
```
Arguments to `include` directive undergo macro-expansion:
```c
#define FOO "foo.h"
#include FOO
```

This module doesn't aim to be a full preprocessor. It doesn't understand conditionals therefore it will report extraneous `#includes`.
Still it is useful as a simple dependency scanner (as long as the codebase doesn't use `#include MACRO`).
