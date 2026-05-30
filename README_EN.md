<p align="center">
  <strong>🌐 Language / 语言</strong>
  &nbsp;&nbsp;|&nbsp;&nbsp;
  <a href="README.md">🇨🇳 中文</a>
  &nbsp;&nbsp;|&nbsp;&nbsp;
  <strong>🇬🇧 English</strong>
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License: MIT"></a>
  <img src="https://img.shields.io/badge/version-v1.1-brightgreen.svg" alt="Version v1.1">
  <img src="https://img.shields.io/badge/go-1.26-blue.svg" alt="Go 1.26">
  <img src="https://img.shields.io/badge/platform-windows%20%7C%20linux-lightgrey.svg" alt="Platform">
</p>

---

# code-detector

**code-detector** is a cross-platform, multi-language function scanner written in Go. It recursively scans a project directory, automatically identifies function/method definitions in source files, records their line ranges, function bodies, call dependencies, and stores the results in a SQLite database for further analysis.

## Features
- Detect all functions and global variables in source code and register them into the database
- **Advantage**: Review project robustness from a function-level perspective — evaluate function quality and detect duplicate/reinvented functionality
- Eliminate irrelevant context interference, providing excellent support for code agents

Current version: **v1.1**

---

## Supported Languages & File Extensions

| Language | Internal Name | File Extensions | Parser |
|----------|---------------|-----------------|--------|
| **Go** | `go` | `.go` | tree-sitter AST |
| **Python** | `python` | `.py` | tree-sitter AST |
| **Java** | `java` | `.java` | tree-sitter AST |
| **Kotlin** | `kotlin` | `.kt`, `.kts` | tree-sitter AST |
| **Swift** | `swift` | `.swift` | tree-sitter AST |
| **JavaScript** | `javascript` | `.js`, `.jsx`, `.mjs` | tree-sitter AST |
| **TypeScript** | `typescript` | `.ts`, `.tsx` | tree-sitter AST |
| **PHP** | `php` | `.php` | tree-sitter AST |
| **Lua** | `lua` | `.lua` | tree-sitter AST |
| **Scala** | `scala` | `.scala` | tree-sitter AST |
| **C#** | `csharp` | `.cs` | tree-sitter AST |
| **C/C++** | `cpp` | `.cpp`, `.cxx`, `.cc`, `.c`, `.h`, `.hpp` | tree-sitter AST |
| **Rust** | `rust` | `.rs` | tree-sitter AST |
| **Ruby** | `ruby` | `.rb` | tree-sitter AST |
| **Custom languages** | — | Extendable via `config.yaml` | Generic regex parser |

> You can register custom regex parsing rules for any file extension via the `config.yaml` configuration file, with support for single-line and block comment filtering.

---

## Usage

### Basic Usage

```cmd
code-detector [options] <project_root>
```

When `<project_root>` is omitted, the current working directory is scanned.

### Examples

**Scan all supported languages:**

```cmd
code-detector -verbose ./myproject
```

**Scan specific languages (comma-separated):**

```cmd
code-detector -lang go,java,python ./myproject
```

**Specify language names:**

```cmd
code-detector -lang go,py,js,ts,rs -verbose ./src
```

The `-lang` flag accepts internal language names or file extensions, e.g. `go` / `py` / `java` / `js` / `ts` / `cs` / `cpp` / `rs` / `rb` / `kt` / `swift` / `php` / `lua` / `scala`.

**Exclude directories and set concurrency:**

```cmd
code-detector -skip-dirs .git,bin,obj,node_modules -workers 8 ./myproject
```

**Build call graph and print statistics:**

```cmd
code-detector -graph ./myproject
```

**Incremental scan (re-parse only changed files):**

```cmd
code-detector -incremental ./myproject
```

**One-click scan (skip all test folders):**

```cmd
scan.bat
```

A `scan.bat` is provided in the project root. Double-click it to run code-detector with `-skip-dirs testdata,testdata_extreme,tests,test,__tests__,node_modules,mock,mocks`, automatically skipping common test/temp directories — ideal for quick daily scans.

**Debug mode (view parser skip details):**

```cmd
code-detector -debug ./myproject
```

> `-debug` outputs parser-level diagnostic info (e.g. which function was skipped due to brace mismatch and its location). Use `-verbose` for normal detailed output.

**Specify output database path:**

```cmd
code-detector -db ./output/my_scan.db -verbose ./myproject
```

---

## Command-Line Options

| Option | Description |
|--------|-------------|
| `-lang <list>` | Languages to scan, comma-separated (e.g. `go,py,java`). Omit to scan all supported languages |
| `-db <path>` | SQLite database output path (default `scaned_db/scan_result.db`) |
| `-config <path>` | Configuration file path (default `config.yaml`) |
| `-max-size <N>` | Maximum file size in bytes (default 1MB); files larger than this are skipped |
| `-skip-dirs <list>` | Additional subdirectory names to skip, comma-separated (default skips `.git`, `node_modules`, etc.) |
| `-workers <N>` | Number of concurrent worker goroutines (default equals CPU logical cores) |
| `-verbose` | Enable verbose logging, showing scan progress, registration info, and parser skip records |
| `-debug` | Same as `-verbose`, plus parser-level debug output (e.g. brace mismatch skip details). Use when reporting parser bugs |
| `-graph` | Build a call graph after scanning and print a statistical summary |
| `-incremental` | Incremental scan mode: only re-parse files whose mtime has changed |
| `-format` | Output format: `text` (default) or `json` (query mode only) | `-format json` |
| `-v` | Show version number |

---

## Usage in CMD

Run the executable directly in the terminal:

```cmd
code-detector -lang go,python -verbose D:\projects\myapp
```

Scroll output logs will be displayed, and the program exits automatically when done. If run by double-clicking (non-interactive terminal), the program will not pause for key input.

## Usage in PowerShell

```powershell
.\code-detector.exe -lang go,js,ts -graph .\myproject
```

With full path:

```powershell
& "D:\tools\code-detector.exe" -lang cpp,cs -workers 8 -verbose "D:\projects\myapp"
```

---

## Output Description

Scan results are stored by default in `scaned_db/scan_result.db` (SQLite database), containing **6 tables**. Below is the complete field documentation:

---

### `scan_sessions` — Scan Sessions Table

| DB Field | Description |
|----------|-------------|
| `id` | Session ID, auto-increment primary key |
| `project_root` | Project root directory being scanned |
| `scan_time` | Scan start timestamp |
| `duration_ms` | Total scan duration in milliseconds |
| `file_count` | Total number of files scanned |
| `func_count` | Total number of functions/methods found |
| `var_count` | Total number of global variables/constants found |

---

### `functions` — Functions Table

| DB Field | Description |
|----------|-------------|
| `id` | Function ID, auto-increment primary key |
| `session_id` | Associated session ID, references `scan_sessions.id` |
| `name` | Function/method name |
| `package_name` | Package or namespace (e.g. Go package, Java package, C# namespace) |
| `language` | Programming language (e.g. `go`, `python`, `java`) |
| `file_path` | File path relative to the project root |
| `line_start` | Start line of the function definition |
| `line_end` | End line of the function body |
| `body` | Complete source code of the function |
| `hash` | SHA256 content hash (first 16 bytes, used for deduplication) |
| `call_count` | Total number of internal function calls (including duplicate calls) |
| `nesting_depth` | Maximum bracket nesting depth |
| `parameters` | Function parameter list string, e.g. `(a int, b string)` |
| `return_types` | Return type string, e.g. `(int, error)` |
| `receiver` | Method receiver, e.g. `(s *Server)` |
| `is_method` | Whether it is a method (`1` = method, `0` = function) |
| `visibility` | `public` or `private` (based on first letter case) |
| `cyclomatic` | McCabe cyclomatic complexity (counts if/for/switch/case/&&/||) |
| `parameter_count` | Number of function parameters |
| `return_count` | Number of return statements in the function body |
| `statement_count` | Total number of statements in the function body |
| `anonymous_funcs` | Number of anonymous functions/closures inside the function |

---

### `function_deps` — Function Dependencies Table

| DB Field | Description |
|----------|-------------|
| `id` | Dependency ID, auto-increment primary key |
| `caller_id` | Caller function ID, references `functions.id` |
| `callee_name` | Name of the called function |

---

### `global_vars` — Global Variables Table

| DB Field | Description |
|----------|-------------|
| `id` | Variable ID, auto-increment primary key |
| `session_id` | Associated session ID, references `scan_sessions.id` |
| `name` | Variable/constant name |
| `var_type` | Data type (e.g. `int`, `string`, `[]byte`) |
| `language` | Programming language |
| `package_name` | Package or namespace |
| `visibility` | `public` or `private` |
| `file_path` | File path relative to the project root |
| `line_num` | Line number where the variable is defined |
| `is_const` | Whether it is a constant (`1` = constant, `0` = variable) |
| `hash` | Content hash (used for deduplication) |

---

### `file_cache` — File Cache Table (used for incremental scanning)

| DB Field | Description |
|----------|-------------|
| `file_path` | File path (primary key) |
| `mtime` | Last modification time as Unix timestamp |
| `hash` | SHA256 hash of the file content |
| `session_id` | Associated session ID, references `scan_sessions.id` |

### `file_metrics` — File Metrics Table (AST Enhanced)

| DB Field | Description |
|----------|-------------|
| `file_path` | File path relative to the project root |
| `language` | Programming language |
| `total_lines` | Total lines in the file (max function end line) |
| `func_count` | Total number of functions/methods in the file |
| `type_count` | Number of type definitions (structs/interfaces) |
| `avg_cyclomatic` | Average cyclomatic complexity of functions in the file |
| `max_cyclomatic` | Maximum cyclomatic complexity in the file |
| `total_parameters` | Sum of all function parameters in the file |
| `max_parameters` | Maximum parameter count of a single function |
| `total_returns` | Sum of all return statements in the file |
| `total_statements` | Sum of all statements in the file |
| `total_anon_funcs` | Total number of anonymous functions/closures |
| `public_funcs` | Number of public functions |
| `private_funcs` | Number of private functions |
| `methods_count` | Number of methods in the file |

### `type_defs` — Type Definitions Table (AST Enhanced)

| DB Field | Description |
|----------|-------------|
| `name` | Type definition name |
| `kind` | Type kind: `struct` / `interface` / `alias` / `enum` |
| `language` | Programming language |
| `package_name` | Package or namespace |
| `file_path` | File path relative to the project root |
| `line_start` | Start line of the type definition |
| `line_end` | End line of the type definition |
| `body` | Complete source code of the type definition |
| `fields` | Structured field description (JSON format) |

When the `-graph` option is enabled, a call graph statistical summary is printed to the terminal.

---

## Query Mode (`-query`)

Read and analyze an existing SQLite database directly without re-scanning.

```cmd
code-detector -query <mode> [-db <database_path>] [-format text|json]
```

| Mode | Description | Example |
|------|-------------|---------|
| `summary` | Show database overview (session count, function/variable totals, language distribution) | `-query summary` |
| `functions` | List all functions (no body field, grouped by file with line range and call count) | `-query functions` |
| `func=NAME` | Show function details (supports comma-separated batch: `func=A,B,C`; includes dependencies, callers, body preview) | `-query func=main` |
| `vars` | List all global variables | `-query vars` |
| `deps` | Call statistics: hottest functions, dead code candidates, widest call branches | `-query deps` |
| `calls=NAME` | Show which functions call the specified function | `-query calls=Parse` |
| `dead` | List functions with `call_count = 0` (potential dead code) | `-query dead` |
| `missing` | List called functions that have no definition (dependency analysis) | `-query missing` |
| `top=N` | List the N largest functions by line count (risk analysis for oversized functions) | `-query top=10` |
| `deep=N` | List functions with nesting depth >= N (complexity analysis) | `-query deep=3` |
| `tree=NAME` | 🆕 Recursively extract a function and all its transitive dependencies (with function bodies) | `-query tree=main` |
| `complexity=N` | 🆕 List top N functions by cyclomatic complexity | `-query complexity=5` |
| `params=N` | 🆕 List functions with parameter count >= N | `-query params=5` |
| `anon` | 🆕 List functions containing anonymous functions/closures | `-query anon` |
| `files` | 🆕 File-level statistics: func count / complexity / params / returns / visibility | `-query files` |
| `types` | 🆕 List all type definitions (struct/interface) | `-query types` |

**Batch query example** — inspect multiple functions at once:
```cmd
code-detector -query func=main,Scan,InitDB
```

**JSON output example** — all `-query` modes support JSON format:
```cmd
code-detector -query summary -format json
code-detector -query func=main -format json
code-detector -query top=5 -format json
code-detector -query tree=printBanner -format json
```

---

## MCP Protocol Support

Starting from v0.8, **code-detector** supports [MCP (Model Context Protocol)](https://modelcontextprotocol.io/) server mode, allowing AI clients (such as Claude Desktop) to interact directly with code-detector for real-time project function data queries.

### How to Start

```cmd
code-detector -mcp [-db <database_path>]
```

Communicates via stdio using JSON-RPC messages, compatible with all standard MCP clients.

### 16 MCP Tools

| Tool Name | Description | Equivalent CLI Query |
|-----------|-------------|---------------------|
| `get_summary` | Database overview (sessions/functions/variables/language distribution) | `summary` |
| `list_functions` | List all functions (filterable by language) | `functions` |
| `get_function` | View function details (signature/complexity/dependencies) | `func=NAME` |
| `get_function_body` | Get function body source code | (New) |
| `list_variables` | List global variables | `vars` |
| `analyze_deps` | Function call relationship statistics | `deps` |
| `find_callers` | Find which functions call the specified function | `calls=NAME` |
| `find_dead_code` | Dead code detection | `dead` |
| `find_missing_deps` | Missing dependency detection | `missing` |
| `top_functions` | Top N functions sorted by line count | `top=N` |
| `deep_nesting` | Deep nesting detection | `deep=N` |
| `high_complexity` | Top N functions by cyclomatic complexity | `complexity=N` |
| `many_params` | Functions with excessive parameters | `params=N` |
| `find_anonymous` | Functions containing anonymous functions/closures | `anon` |
| `file_metrics` | File-level statistics | `files` |
| `list_types` | List type definitions | `types` |

### 6 MCP Resources

| URI | Content | Format |
|-----|---------|--------|
| `db://summary` | Database summary | JSON |
| `db://functions` | Full function list | JSON |
| `db://variables` | Global variables list | JSON |
| `db://files` | File-level statistics | JSON |
| `db://types` | Type definitions | JSON |
| `db://sessions/latest` | Latest scan session info | JSON |

### Configuration Example (Claude Desktop)

Add the following to your Claude Desktop `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "code-detector": {
      "command": "code-detector.exe",
      "args": ["-mcp", "-db", "D:\\projects\\myapp\\scaned_db\\scan_result.db"]
    }
  }
}
```

> Once configured, Claude Desktop can directly call all 16 tools and 6 resources to query project code analysis results.

---

## Safety Protection

- The program has built-in **system critical directory protection**, refusing to scan Windows system drive root, `C:\Windows`, `/etc`, `/proc`, and other system directories to prevent disk thrashing or data corruption.
- Provides a key-press wait before exit (only triggered in interactive terminals), making it convenient to view results when running from a double-click.

---

## Configuration File

The default `config.yaml` is empty (built-in parsers are used). You can customize language parsing rules in the following format:

```yaml
languages:
  - name: "my_lang"
    extensions: [".mylang"]
    function_regex: "func\\s+(?P<name>\\w+)\\s*\\("
    single_comment: ["//"]
    block_comment: [["/*", "*/"]]
```

> If a file extension is already handled by a built-in parser, the custom rules will not override it. Custom regex parsing only applies to extensions not natively supported.

---

## Building

### Build on Windows

```cmd
build.bat
```

### Build using Makefile

```cmd
make build
```

The build output is `code-detector.exe` (Windows) or `code-detector` (Linux/macOS).

---

## License

This project is open source under the [MIT License](LICENSE).

## Acknowledgements

This project uses [tree-sitter](https://tree-sitter.github.io/) — a powerful incremental parsing framework.
Its core and all language grammar parsers are released under the MIT License.

- tree-sitter © 2018 Max Brunsfeld — [MIT](https://github.com/tree-sitter/tree-sitter/blob/master/LICENSE)
- go-tree-sitter © 2019 Maxim Sukharev — [MIT](https://github.com/smacker/go-tree-sitter/blob/master/LICENSE)
