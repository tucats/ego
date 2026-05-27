#!/bin/zsh
#
# gotests.sh — Run the full Go test suite and report a concise summary.
#
# Differences from plain 'go test ./...':
#   - Counts individual passing tests (not just passing packages).
#   - On success: prints only the total test count and elapsed time.
#   - On failure: prints failing test output in the same style as plain
#     'go test ./...' and appends a failure summary line.
#
# Options:
#   -a, --all    Disable the test result cache; force every test to run
#                even if its package has not changed since the last run.
#                (Equivalent to passing -count=1 to go test.)
#   -h, --help   Print this help message and exit.

# --- argument parsing --------------------------------------------------------

no_cache=0

for arg in "$@"; do
    case "$arg" in
        -a|--all)
            no_cache=1
            ;;
        -h|--help)
            cat <<'EOF'
Usage: tools/gotests.sh [-a] [-h]

Run the full Go test suite (go test ./...) and print a concise summary.

Options:
  -a, --all    Force every test to run regardless of cached results.
               By default, Go skips packages whose source has not changed
               since the last successful run (the normal 'go test' behavior).
               Use this flag when you want a guaranteed fresh run — for
               example, before committing or in CI.
  -h, --help   Print this help message and exit.

Output:
  On success:  "<N> tests passed in <T>s"
  On failure:  Test output in the same format as plain 'go test ./...',
               followed by a summary line showing counts and elapsed time.
               The script exits with a non-zero status code on failure.
EOF
            exit 0
            ;;
        *)
            printf "gotests.sh: unknown option: %s\n" "$arg" >&2
            printf "Run 'tools/gotests.sh --help' for usage.\n" >&2
            exit 1
            ;;
    esac
done

# --- test run ----------------------------------------------------------------

# Load the zsh datetime module so $EPOCHREALTIME is available for
# sub-second elapsed time reporting.
zmodload zsh/datetime 2>/dev/null
start=$EPOCHREALTIME

# Build the go test flags as a zsh array.  -v is always included so individual
# test names are visible in the output and can be counted.  -count=1 is added
# only when the caller requests a cache-bypassing run with -a / --all.
# An array is used because zsh does not word-split unquoted scalar variables
# the way bash does, so "-count=1 -v" in a plain string would be passed as one
# argument rather than two.
if [[ $no_cache -eq 1 ]]; then
    go_flags=(-count=1 -v)
else
    go_flags=(-v)
fi

# 2>&1 merges stderr into stdout so build errors are also captured.
output=$(go test "${go_flags[@]}" ./... 2>&1)
exit_code=$?

# --- reporting ---------------------------------------------------------------

# Compute elapsed time in seconds with one decimal place.
# awk is used for floating-point arithmetic because zsh $(( )) is integer-only.
elapsed=$(awk "BEGIN { printf \"%.1fs\", $EPOCHREALTIME - $start }")

# Count individual test outcomes from the verbose output.
# Each passing test emits exactly one "--- PASS: TestName (Xs)" line.
# Each failing test emits exactly one "--- FAIL: TestName (Xs)" line.
pass_count=$(printf '%s\n' "$output" | grep -c "^--- PASS:")
fail_count=$(printf '%s\n' "$output" | grep -c "^--- FAIL:")
total=$((pass_count + fail_count))

if [[ $exit_code -eq 0 ]]; then
    printf "%d tests passed in %s\n" "$total" "$elapsed"
else
    # Filter the verbose output so it resembles plain 'go test ./...' output.
    # Remove lines that only appear in verbose mode and are not useful for
    # diagnosing failures:
    #   "=== RUN   TestName"   — test start marker
    #   "=== PAUSE TestName"   — parallel test paused
    #   "=== CONT  TestName"   — parallel test resumed
    #   "--- PASS: TestName"   — passing tests (not relevant when diagnosing failures)
    #   "--- SKIP: TestName"   — skipped tests (not shown by plain go test)
    printf '%s\n' "$output" \
        | grep -Ev "^=== (RUN|PAUSE|CONT)[[:space:]]|^--- (PASS|SKIP):"
    printf "\n%d FAILED, %d passed, %s elapsed\n" \
        "$fail_count" "$pass_count" "$elapsed"
    exit 1
fi
