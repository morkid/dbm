# Contributing

Contributions are welcome! Here's how you can help.

## Reporting Issues

Open an issue on GitHub describing the bug or feature request. Include:

- A clear title and description
- Steps to reproduce (for bugs)
- Go version and environment details

## Pull Requests

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/my-feature`)
3. Write your code
4. Add or update tests as needed
5. Run the tests and verify **100% code coverage** (required, PRs below 100% will be rejected):

   ```
   go test -v -cover ./...
   ```
6. Format your code:

   ```
   gofmt -s -w .
   ```
7. Commit with a clear message following [Conventional Commits](#commit-messages) (`git commit -m "feat: ..."`)
8. Push and open a PR

## Code Style

- Follow standard Go conventions (`gofmt`)
- Keep the API surface minimal and clean
- Add comments only for exported symbols (Go doc style)
- Avoid introducing new dependencies unnecessarily
- **Do not add dependencies that require CGO** -- all dependencies must compile with `CGO_ENABLED=0`
- Avoid modify `seeder.go` , `hook.go` , `connection.go` and `driver.go`
- **Use English** for all in-code documentation, commit messages, issue reports, and PR descriptions.

## Commit Messages

This project follows [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/). Every commit message MUST use the format below:

```
<type>[optional scope][!]: <description>

[optional body]

[optional footer(s)]
```

### Allowed Types

- `feat` -- a new user-visible feature.
- `fix` -- a bug fix.
- `docs` -- documentation-only changes (README, CONTRIBUTING, in-code comments).
- `style` -- formatting changes that do not affect meaning (e.g. `gofmt -s`, whitespace).
- `refactor` -- a code change that neither fixes a bug nor adds a feature.
- `perf` -- a performance improvement.
- `test` -- adding or fixing tests.
- `build` -- build system or dependency changes (`go.mod`, driver subpackages).
- `ci` -- continuous-integration configuration changes.
- `chore` -- tooling or maintenance tasks that do not modify source or tests.
- `revert` -- revert a previous commit.

### Scope

A scope is a noun in parentheses naming the area of the codebase affected, for example `feat(connection):` or `fix(mysql):`. Use the driver name (`mysql`, `postgres`, `sqlite`, `clickhouse`, `mssql`, `cockroach`, `tidb`, etc.) for driver-specific changes, or omit the scope when a change spans the core package broadly.

### Breaking Changes

Mark a commit as a breaking change by appending `!` after the type (and scope), for example:

```
feat(api)!: rename New() to NewConnection()
```

A breaking change MUST also include a `BREAKING CHANGE:` footer that explains the impact and the migration path for users.

### Examples

```
feat: add auto-connect via variadic boolean in Register
fix(connection): resolve race in Connect() when override is false
docs(contributing): describe conventional commit rules
perf(mysql): cache prepared-statement compiled DSN
refactor(driver): consolidate extras encoder
build(postgres): bump gorm.io/driver/postgres to v1.5.0
test(sqlite): cover sqlite duplicate index path in Migrate
```

### Footers

Footers use the `token: value` or `token #value` form and are separated from the body by a blank line. Common tokens:

- `Refs:`, `Closes:`, `Fixes:` -- reference issues by number, for example `Refs: #123`.
- `BREAKING CHANGE:` -- required when `!` is used in the header; describe the impact and migration path.
- `Co-authored-by:` -- for co-authored commits.

Use imperative mood in the description ("add caching", not "added caching"), keep the subject line under 72 characters, and wrap the body at 72 characters.

## Adding a New Database Driver

Drivers live in **per-driver subpackages** under `driver/<name>/`, not as flat `driver_<name>.go` files in the package root. To add a new driver:

1. Create a new subpackage directory `driver/<name>/` (the directory name is what users will reference with a blank import, e.g. `_ "github.com/morkid/dbm/driver/<name>"`).
2. Create `driver/<name>/driver.go` with package name `<name>` (typically the directory name in lower-snake form).
3. In `init()`, call `dbm.RegisterDriver("<type>", dbm.ConnectionBuilder{...})`. The `<type>` string is the value of `Config.Type` users will pass when registering a connection and **must be unique across the registry**.
4. Implement `BuildDSN(Config) string` that returns the GORM-compatible DSN string, and `Open(dsn string) gorm.Dialector` that returns a `gorm.Dialector` for the DSN. Provide a `DefaultConfig` with sensible fallback values for host, port, user, timezone, and connection pool sizing.
5. Use the **extras pattern** (DSN `extra=` key with `U+001F`-separated `key:value` pairs) for any driver-specific struct field that does not have a slot on the top-level `Config`. See the `extras` decoder in any built-in driver subpackage (e.g. `driver/mysql/driver.go`) for the canonical implementation.
6. Add a sibling `driver/<name>/driver_test.go` and keep coverage at **100%** for the subpackage.
7. (Optional) If the driver should ship with `driver/all`, register a blank import in `driver/all/driver.go`.

Rules in effect:

- The subpackage directory **must** be named `driver/<name>/` and the file containing `init()` **must** be named `driver.go` (no `_test.go` suffix). Tests live alongside as `driver_test.go`.
- Do **not** modify the top-level `driver.go`, `connection.go`, `seeder.go`, or `hook.go` -- they define the stable interface and stay frozen.
- Avoid CGO dependencies. The whole module must compile with `CGO_ENABLED=0`.
- Keep the API surface minimal: only the `BuildDSN` / `Open` / `DefaultConfig` triple should be exposed.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
