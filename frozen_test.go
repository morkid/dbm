// Regression test that contractually freezes the byte content of four
// core database files (per CONTRIBUTING.md):
//
//   - seeder.go
//   - hook.go
//   - connection.go
//   - driver.go
//
// Each entry in `frozenDigests` below is the 64-character hex form of
// the SHA-256 digest of the corresponding file's bytes (with CRLF
// normalized to LF for cross-platform stability).
//
// During `go test`, the current SHA-256 of every protected file is
// computed and compared against `frozenDigests`; any drift fails the
// test.
//
// To rotate the baselines after an intentional change to any frozen
// file, run:
//
//	UPDATE_FROZEN=1 go test -v ./...
//
// and paste the printed map literal into the `frozenDigests` variable
// below. Per CONTRIBUTING.md, casual modification of seeder.go, hook.go,
// connection.go, and driver.go is forbidden; this test enforces that
// rule mechanically.

package dbm

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"testing"
)

// protectedFiles lists every source file whose byte content is
// contractually frozen. Order is irrelevant; verifyLockedFiles iterates
// each name independently.
var protectedFiles = []string{
	"seeder.go",
	"hook.go",
	"connection.go",
	"driver.go",
}

// frozenDigests holds the locked SHA-256 hex digest for each entry in
// protectedFiles. To rotate after an intentional change to a frozen
// file, see the update workflow in the header doc comment above.
var frozenDigests = map[string]string{
	"seeder.go":     "8b25d807e161af168667562e5a57fe3bdd14dd8bc28b3d5aaec2621a311d22db",
	"hook.go":       "9d74bd002aaec886f53dac22b824cf46dc989910fcdc3a34d005ff9e4c00f845",
	"connection.go": "f6c656b191b8314be294294ea83b1c0127afe5092a05685d1b16d5b28adceec9",
	"driver.go":     "fe03bb2934f0d41eac688793f925ebf12520a1f6173109b34bbfa14d9eb666b4",
}

// digestsOf returns SHA-256 hex digests of the byte content of each
// file in `files`, with CRLF normalized to LF so the hashes stay stable
// across Windows and git autocrlf variations. Other line-ending forms
// (UTF-8 BOM, lone \r) are intentionally NOT normalized -- Go source
// files in this project only ever use LF in practice.
func digestsOf(files []string) (map[string]string, error) {
	out := map[string]string{}
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f, err)
		}
		normalized := bytes.ReplaceAll(body, []byte("\r\n"), []byte("\n"))
		out[f] = fmt.Sprintf("%x", sha256.Sum256(normalized))
	}
	return out, nil
}

// maybePrintUpdateHint prints a ready-to-paste map literal for the
// `frozenDigests` variable when UPDATE_FROZEN=1 is set in the
// environment; otherwise it is a no-op. Both branches are exercised by
// TestMaybePrintUpdateHintCover.
func maybePrintUpdateHint(digests map[string]string) {
	if os.Getenv("UPDATE_FROZEN") != "1" {
		return
	}
	var sb strings.Builder
	sb.WriteString("--- UPDATE_FROZEN REQUESTED ---\n")
	sb.WriteString("Replace the frozenDigests map literal in frozen_test.go with:\n")
	sb.WriteString("var frozenDigests = map[string]string{\n")
	for _, f := range protectedFiles {
		fmt.Fprintf(&sb, "\t%q: %q,\n", f, digests[f])
	}
	sb.WriteString("}\n")
	sb.WriteString("-------------------------------\n")
	fmt.Print(sb.String())
}

// verifyLockedFiles checks every entry in `files` against the locked
// digests and returns human-readable diagnostics for any drift. It has
// exactly two branches: (1) entry absent from the locked map, and
// (2) entry present but current digest has drifted from the recorded
// value. Pulled out as a pure helper so both branches can be
// unit-tested directly without monkey-patching on-disk state.
func verifyLockedFiles(files []string, locked, current map[string]string) []string {
	var errs []string
	for _, f := range files {
		want, present := locked[f]
		if !present {
			errs = append(errs, fmt.Sprintf("%s missing from frozenDigests (run UPDATE_FROZEN=1 go test -v to add)", f))
			continue
		}
		if want != current[f] {
			errs = append(errs, fmt.Sprintf(
				"%s has been modified (digest mismatch).\n  expected: %s\n  actual:   %s\n  (if intentional, run: UPDATE_FROZEN=1 go test -v ./... then paste the printed map literal into frozenDigests)",
				f, want, current[f],
			))
		}
	}
	return errs
}

// TestProtectedFiles is the regression test. It computes the current
// SHA-256 of every entry in protectedFiles, optionally prints an
// update hint (when UPDATE_FROZEN=1 is set), then asserts that each
// current digest matches the recorded one in frozenDigests.
func TestProtectedFiles(t *testing.T) {
	current, err := digestsOf(protectedFiles)
	if err != nil {
		t.Fatalf("digestsOf: %v", err)
	}
	maybePrintUpdateHint(current)
	for _, e := range verifyLockedFiles(protectedFiles, frozenDigests, current) {
		t.Error(e)
	}
	// Note: the for-loop body that calls t.Error() above is exercised
	// when verifyLockedFiles returns non-empty (digest drift). Branch
	// coverage of verifyLockedFiles itself is achieved directly by
	// TestVerifyLockedFilesCover.
}

// TestMaybePrintUpdateHintCover covers both branches of
// maybePrintUpdateHint: no-op (UPDATE_FROZEN unset or set to anything
// other than "1") and print-on-update (UPDATE_FROZEN=1).
func TestMaybePrintUpdateHintCover(t *testing.T) {
	digests := map[string]string{
		"seeder.go": "abc123",
		"hook.go":   "def456",
	}

	t.Run("default_no_env", func(t *testing.T) {
		t.Setenv("UPDATE_FROZEN", "")
		maybePrintUpdateHint(digests)
	})

	t.Run("env_set_prints", func(t *testing.T) {
		t.Setenv("UPDATE_FROZEN", "1")
		maybePrintUpdateHint(digests)
	})
}

// TestDigestsOfCover covers the happy path (CRLF normalization +
// idempotency) plus the read-error path.
func TestDigestsOfCover(t *testing.T) {
	dir := t.TempDir()

	// CRLF normalization: identical content modulo CRLF yields the
	// same digest.
	lf := dir + "/lf.txt"
	if err := os.WriteFile(lf, []byte("a\nb"), 0644); err != nil {
		t.Fatal(err)
	}
	crlf := dir + "/crlf.txt"
	if err := os.WriteFile(crlf, []byte("a\r\nb"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := digestsOf([]string{lf, crlf})
	if err != nil {
		t.Fatal(err)
	}
	if got[lf] != got[crlf] {
		t.Errorf("CRLF normalization: %s != %s", got[lf], got[crlf])
	}

	// Idempotency: re-reading the same file yields the same digest.
	got2, err := digestsOf([]string{lf})
	if err != nil {
		t.Fatal(err)
	}
	if got[lf] != got2[lf] {
		t.Errorf("idempotency: %s != %s", got[lf], got2[lf])
	}

	// Read-error path (file does not exist).
	if _, err := digestsOf([]string{"/nonexistent/path/dbm_test_missing_xyz"}); err == nil {
		t.Error("expected error for missing file")
	}
}

// TestVerifyLockedFilesCover exercises the three branches of
// verifyLockedFiles: happy (present + match), mismatch (present but
// content drift), and missing (entry absent from locked map).
func TestVerifyLockedFilesCover(t *testing.T) {
	files := []string{"present.go", "missing.go"}

	t.Run("happy", func(t *testing.T) {
		locked := map[string]string{"present.go": "abc", "missing.go": "xyz"}
		current := map[string]string{"present.go": "abc", "missing.go": "xyz"}
		if errs := verifyLockedFiles(files, locked, current); len(errs) != 0 {
			t.Errorf("happy: expected no errors, got %v", errs)
		}
	})

	t.Run("mismatch", func(t *testing.T) {
		locked := map[string]string{"present.go": "abc", "missing.go": "xyz"}
		current := map[string]string{"present.go": "abc", "missing.go": "DIFFERENT"}
		errs := verifyLockedFiles(files, locked, current)
		if len(errs) != 1 {
			t.Fatalf("mismatch: expected 1 error, got %d: %v", len(errs), errs)
		}
		if !strings.Contains(errs[0], "missing.go") ||
			!strings.Contains(errs[0], "digest mismatch") ||
			!strings.Contains(errs[0], "xyz") ||
			!strings.Contains(errs[0], "DIFFERENT") {
			t.Errorf("mismatch error should reference missing.go, digest mismatch, expected (xyz), and actual (DIFFERENT); got %q", errs[0])
		}
	})

	t.Run("missing_entry", func(t *testing.T) {
		// "missing.go" not in locked.
		locked := map[string]string{"present.go": "abc"}
		current := map[string]string{"present.go": "abc", "missing.go": "anything"}
		errs := verifyLockedFiles(files, locked, current)
		if len(errs) != 1 {
			t.Fatalf("missing: expected 1 error, got %d: %v", len(errs), errs)
		}
		if !strings.Contains(errs[0], "missing.go") || !strings.Contains(errs[0], "missing from frozenDigests") {
			t.Errorf("missing-entry error should reference missing.go and 'missing from frozenDigests', got %q", errs[0])
		}
	})
}
