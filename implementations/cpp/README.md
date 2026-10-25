# C++ sample parser

An independent C++17 implementation of the [dotReg format](../../docs/config-format.md), using only the standard library. The header-only `dotreg.hpp` provides syntax processing and common string decoding. It covers required file properties, heading resolution, decoded key element boundaries, opaque type annotations, repeated values, UTF-8 validation and all three value forms. It does not supply a type conversion profile.

## Build and check

From the repository root, use a C++17 compiler:

```sh
c++ -std=c++17 -O2 -Wall -Wextra -pedantic implementations/cpp/main.cpp -o dotreg-cpp
c++ -std=c++17 -O2 -Wall -Wextra -pedantic implementations/cpp/tests.cpp -o dotreg-cpp-tests
./dotreg-cpp-tests
```

Alternatively:

```sh
cmake -S implementations/cpp -B implementations/cpp/build -DCMAKE_BUILD_TYPE=Release
cmake --build implementations/cpp/build --config Release
ctest --test-dir implementations/cpp/build -C Release --output-on-failure
```

Use `dotreg-cpp --mode strings settings.reg`, or omit the file argument to read standard input. `strings` is the default mode. `--mode raw` emits syntax entries without decoding values. Successful output is one JSON array; string mode adds `decoded` to each value. Invalid input returns a nonzero exit status, emits a safe line-numbered diagnostic to standard error and emits no JSON. The CLI collects syntax entries first, then decodes values and buffers its JSON until the complete input succeeds. Include `dotreg.hpp` and use `parse_each` for incremental consumption.

The executable can also run the shared [conformance runner](../conformance/README.md):

```sh
node implementations/conformance/run.mjs --command '["./dotreg-cpp"]'
```

## Library API and retention

```cpp
#include "dotreg.hpp"
#include <fstream>

std::ifstream source("settings.reg", std::ios::binary);
dotreg::parse_each(source, [](dotreg::Entry entry) {
    if (entry.kind == "value") {
        auto text = dotreg::decode_string(entry);
        // Consume text and entry.unset here.
    }
});
```

Open external streams in binary mode to preserve LF and CRLF exactly. The command configures Windows standard input and output in binary mode and opens optional files accordingly. Source streams are consumed once; each parser call starts with fresh scopes and occurrence state.

The visitor receives an independently owned entry. Changing its path cannot modify the shared heading nodes. Recipient exceptions propagate unchanged, immediately after the current complete entry; later entries are not parsed. Source errors are replaced with safe `ParseError` diagnostics without underlying exception text. Null source pointers and empty visitors are rejected. `parse(std::istream&)` collects entries and returns no partial collection on failure. Side effects already performed by a visitor are not rolled back.

The implementation scans each physical line without regular expressions, preserves original source endings and delays validation of a non-pipe lookahead until its normal turn. Streaming retention follows active headings, the current expanded path, one current complete block and one lookahead line, plus the supplied stream's buffering. Shared immutable heading nodes retain only their written suffix and parent. Previous values are not retained by the parser. Triple-quoted margin removal temporarily holds source body lines and the completed body together. A collecting caller also retains every returned entry and expanded path. No fixed physical-line, path or nesting limit is imposed; available memory is the practical bound. Heading expansion and scope release use iterative traversal.

`decode_string` is separate from syntax processing. Raw syntax can preserve an invalid quoted value for inspection, while application delivery must first pass common string decoding. Literal single-line values and pipe bodies preserve backslashes; quoted and triple-quoted forms decode them once. The native tests cover recipient exception identity, deferred invalid lookahead, mutation isolation, source failure redaction, required inputs, deep scopes, UTF-8 errors and mixed line endings.
