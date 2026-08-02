#!/usr/bin/env bash
#
# dashboard_check.sh
#
# Loads the Ego admin dashboard in a headless DOM and verifies that it starts
# up: every script evaluates, a tab is highlighted with its content displayed,
# and typing into the Code editor reaches the syntax-highlight layer. See
# tools/dashboard/check.js for the full rationale and for what this does not
# cover.
#
# The dashboard's JavaScript is split across several files that share one
# global scope. A file whose top-level code references something declared in a
# later file throws during page load, silently killing the rest of that file --
# the dashboard renders its shell and does nothing, with no clue in the UI as
# to why. Every file is valid JavaScript on its own, so neither a syntax check
# nor the Go test suite can see it. Loading the page is the only way.
#
# This needs Node and its one dependency, jsdom. jsdom is roughly 16MB of
# third-party JavaScript, so it is deliberately NOT committed: the repository
# holds only package.json and package-lock.json, and node_modules/ is in
# .gitignore. The first run here installs it, and every run afterwards finds it
# already present and goes straight to the check.
#
# Both Node and a working install are optional. If Node is absent, or the
# install cannot be done (no network, for instance), this reports that the
# check was skipped and why, then exits 0 -- a contributor without a JavaScript
# toolchain is never blocked from running tools/test.sh. Skipped is reported as
# skipped, never as passed.
#
# Usage:
#   tools/dashboard_check.sh            # check lib/assets/dashboard
#   tools/dashboard_check.sh <dir>      # check another copy of the assets
#
# Exit codes:
#   0  the dashboard starts up correctly, or the check was skipped
#   1  the dashboard does not start up correctly

set -o pipefail

# Resolve the repository root from this script's own location so the check can
# be run from any working directory.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHECK_DIR="${SCRIPT_DIR}/dashboard"

if ! command -v node >/dev/null 2>&1; then
    echo "DASHBOARD: skipped -- node is not installed."
    echo "DASHBOARD: install Node.js to run the dashboard startup check."
    exit 0
fi

# Install jsdom on first use. tools/test_container.sh's image already ran this
# during the build, so the container takes the fast path here.
if [ ! -d "${CHECK_DIR}/node_modules/jsdom" ]; then
    if ! command -v npm >/dev/null 2>&1; then
        echo "DASHBOARD: skipped -- npm is not installed, so jsdom cannot be fetched."
        exit 0
    fi

    echo "DASHBOARD: installing jsdom (first run only; not stored in the repository)"

    # "npm ci" installs exactly the versions package-lock.json pins, so every
    # machine checks against the same jsdom. It needs network access the first
    # time; failing that, say so and skip rather than failing the test run.
    if ! (cd "${CHECK_DIR}" && npm ci --no-audit --no-fund >/dev/null 2>&1); then
        echo "DASHBOARD: skipped -- could not install jsdom (offline?)."
        echo "DASHBOARD: run 'cd tools/dashboard && npm ci' when a network is available."
        exit 0
    fi
fi

echo "DASHBOARD: checking that the dashboard starts up"

node "${CHECK_DIR}/check.js" "$@"
STATUS=$?

if [ $STATUS != 0 ]; then
    echo "DASHBOARD: the dashboard does not start up correctly"
    exit 1
fi

exit 0
