#!/usr/bin/env bash
set -euo pipefail

BINARY_NAME="cc-journal"
INSTALL_DIR="${HOME}/.local/bin"

VERSION="dev"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"
if [ -n "$(git status --porcelain 2>/dev/null)" ]; then
    COMMIT="${COMMIT}-dirty"
fi
BUILD_TIME="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"

echo "Building ${BINARY_NAME} (${COMMIT} @ ${BUILD_TIME})..."
go build -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildTime=${BUILD_TIME}" -o "${BINARY_NAME}" .

mkdir -p "${INSTALL_DIR}"
cp "${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

echo "Installed to ${INSTALL_DIR}/${BINARY_NAME}"

# Create config directory if needed
CONFIG_DIR="${XDG_CONFIG_HOME:-${HOME}/.config}/cc-journal"
if [ ! -f "${CONFIG_DIR}/config.yaml" ]; then
    mkdir -p "${CONFIG_DIR}"
    cat > "${CONFIG_DIR}/config.yaml" << 'YAML'
# cc-journal configuration
# See: https://github.com/natefaerber/cc-journal

# journal_dir: ~/claude-journal
# model: claude-sonnet-4-20250514

exclude: []
  # - ~/private-project

slack:
  # command: slack-send
  # channel: "#standup"

links:
  issues: {}
    # LPE: https://linear.app/your-org/issue
  # confluence: https://your-org.atlassian.net/wiki
  github_repos: []
    # - https://github.com/your-org/your-repo
YAML
    echo "Created default config at ${CONFIG_DIR}/config.yaml"
fi

# Create journal directory if needed
JOURNAL_DIR="${HOME}/claude-journal"
mkdir -p "${JOURNAL_DIR}"

echo ""
echo "Next steps:"
echo "  1. Edit ${CONFIG_DIR}/config.yaml with your settings"
echo "  2. Add the SessionEnd hook to ~/.claude/settings.json:"
echo ""
echo '     "SessionEnd": [{'
echo '       "hooks": [{'
echo '         "type": "command",'
echo '         "command": "TMP=$(mktemp /tmp/cc-journal-in.XXXXXX); cat > \"$TMP\"; ( cc-journal hook < \"$TMP\" >/tmp/cc-journal.log 2>&1; rm -f \"$TMP\" ) &",'
echo '         "timeout": 1'
echo '       }]'
echo '     }]'
echo ""
echo "  3. Run: cc-journal serve"
