# code-detector

**code-detector** is a cross-platform, multi-language function scanner written in Go. It recursively scans a project directory, automatically identifies function/method definitions in source files, records their line ranges, function bodies, call dependencies, and stores the results in a SQLite database for further analysis.

## Features
- Detect all functions and global variables in source code and register them into the database
- **Advantage**: Review project robustness from a function-level perspective — evaluate function quality and detect duplicate/reinvented functionality
- Eliminate irrelevant context interference, providing excellent support for code agents

Current version: **v0.5**

---

## Supported Languages & File Extensions

| Language | Internal Name | File Extensions | Parser |
|----------|---------------|-----------------|--------|
| **Go** | `go` | `.go` | Dedicated parser |
| **Python** | `python` | `.py` | Dedicated parser |
| **Java** | `java` | `.java` | Dedicated parser |
| **Kotlin** | `kotlin` | `.kt`, `.kts` | Dedicated parser (reuses Java parser) |
| **JavaScript** | `javascript` | `.js`, `.jsx`, `.mjs` | Dedicated parser |
| **TypeScript** | `typescript` | `.ts`, `.tsx` | Dedicated parser (reuses JS parser) |
| **C#** | `csharp` | `.cs` | Dedicated parser |
| **C/C++** | `cpp` | `.cpp`, `.cxx`, `.cc`, `.c`, `.h`, `.hpp` | Dedicated parser |
| **Rust** | `rust` | `.rs` | Dedicated parser |
| **Ruby** | `ruby` | `.rb` | Dedicated parser |
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

The `-lang` flag accepts internal language names or file extensions, e.g. `go` / `py` / `java` / `js` / `ts` / `cs` / `cpp` / `rs` / `rb` / `kt`.

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
| `-verbose` | Enable verbose logging, showing scan progress and registration info |
| `-graph` | Build a call graph after scanning and print a statistical summary |
| `-incremental` | Incremental scan mode: only re-parse files whose mtime has changed |
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

When the `-graph` option is enabled, a call graph statistical summary is printed to the terminal.

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
