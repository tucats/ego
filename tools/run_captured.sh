# run_captured.sh
#
# Defines run_captured, a shell function shared by tools/test.sh and
# tools/test_container_entrypoint.sh. Meant to be sourced, not run directly.
#
# run_captured runs a command with its output fully captured (stdout and
# stderr merged) instead of streamed live. On success it prints only that
# command's own "TEST: Completed ..." summary line; on failure it dumps the
# complete captured output so the failure can actually be diagnosed. Both
# "ego test" and the apitest tool already end a clean run with a line
# matching "^TEST: Completed", so this one function covers every caller.
#
# This is the same cache-and-reveal-on-failure strategy tools/gotests.sh uses
# for the Go suite (see its "output=$(go test ...)" and the branch on
# $exit_code). Streaming is deliberately given up in exchange for a quiet
# successful run -- there's no partial output while the command is in
# flight, only the final verdict.
#
# Written for POSIX-ish function syntax (no zsh-only or bash-only features)
# since it is sourced by both a zsh script (test.sh) and a bash script
# (test_container_entrypoint.sh).
run_captured() {
    # Not named "status": zsh reserves that as a read-only alias for $?.
    local output exit_code summary

    output=$("$@" 2>&1)
    exit_code=$?

    if [ "$exit_code" -eq 0 ]; then
        summary=$(printf '%s\n' "$output" | grep "^TEST: Completed" | tail -1)
        printf '%s\n' "${summary:-$output}"
    else
        printf '%s\n' "$output"
    fi

    return $exit_code
}
