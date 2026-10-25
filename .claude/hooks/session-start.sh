#!/bin/bash
set -euo pipefail

if [ "${CLAUDE_CODE_REMOTE:-}" != "true" ] || [ -z "${CLAUDE_ENV_FILE:-}" ]; then
    exit 0
fi

if [ -x /opt/pw-browsers/chromium ]; then
    {
        echo 'export PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH=/opt/pw-browsers/chromium'
        echo 'export PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1'
    } >> "$CLAUDE_ENV_FILE"
fi
