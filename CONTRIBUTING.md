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
7. Commit with a clear message (`git commit -m "add: ..."`)
8. Push and open a PR

## Code Style

- Follow standard Go conventions (`gofmt`)
- Keep the API surface minimal and clean
- Add comments only for exported symbols (Go doc style)
- Avoid introducing new dependencies unnecessarily
- **Do not add dependencies that require CGO** -- all dependencies must compile with `CGO_ENABLED=0`
- Avoid modify `seeder.go` , `hook.go` , `connection.go` and `driver.go`

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
