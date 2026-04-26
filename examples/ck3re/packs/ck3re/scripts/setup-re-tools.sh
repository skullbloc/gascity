#!/usr/bin/env bash
# setup-re-tools.sh — Verify RE tools are available for ck3re prospector agents.
#
# Run this before starting prospector agents to ensure all required tools
# are installed and accessible. Exits non-zero if critical tools are missing.

set -euo pipefail

RED='\033[0;31m'
YELLOW='\033[0;33m'
GREEN='\033[0;32m'
NC='\033[0m'

errors=0
warnings=0

check_required() {
    local name="$1"
    local cmd="$2"
    if command -v "$cmd" &>/dev/null; then
        printf "${GREEN}[OK]${NC}   %s (%s)\n" "$name" "$(command -v "$cmd")"
    else
        printf "${RED}[FAIL]${NC} %s — '%s' not found in PATH\n" "$name" "$cmd"
        errors=$((errors + 1))
    fi
}

check_optional() {
    local name="$1"
    local cmd="$2"
    if command -v "$cmd" &>/dev/null; then
        printf "${GREEN}[OK]${NC}   %s (%s)\n" "$name" "$(command -v "$cmd")"
    else
        printf "${YELLOW}[WARN]${NC} %s — '%s' not found (optional)\n" "$name" "$cmd"
        warnings=$((warnings + 1))
    fi
}

check_dir() {
    local name="$1"
    local dir="$2"
    if [ -d "$dir" ]; then
        printf "${GREEN}[OK]${NC}   %s (%s)\n" "$name" "$dir"
    else
        printf "${YELLOW}[WARN]${NC} %s — directory '%s' does not exist (will be created on first use)\n" "$name" "$dir"
        warnings=$((warnings + 1))
    fi
}

echo "=== CK3 RE Tools Check ==="
echo ""

echo "--- Required Tools ---"
check_required "Ghidra headless analyzer" "analyzeHeadless"
check_required "jq (JSON processing)" "jq"
check_required "Git" "git"

echo ""
echo "--- Optional Tools ---"
check_optional "GDB (runtime probing)" "gdb"
check_optional "lldb (runtime probing)" "lldb"
check_optional "objdump (binary inspection)" "objdump"
check_optional "readelf (ELF inspection)" "readelf"
check_optional "strings (string extraction)" "strings"
check_optional "xxd (hex dump)" "xxd"
check_optional "radare2 (alternative disassembler)" "r2"

echo ""
echo "--- RE Scripts Directory ---"
RE_TOOLS_DIR="${RE_TOOLS_DIR:-scripts/re}"
check_dir "RE scripts directory" "$RE_TOOLS_DIR"

if [ -d "$RE_TOOLS_DIR" ]; then
    for script in "$RE_TOOLS_DIR"/*.sh; do
        [ -f "$script" ] || continue
        if [ -x "$script" ]; then
            printf "${GREEN}[OK]${NC}   Script executable: %s\n" "$script"
        else
            printf "${YELLOW}[WARN]${NC} Script not executable: %s\n" "$script"
            warnings=$((warnings + 1))
        fi
    done
fi

echo ""
echo "--- Ghidra Project ---"
GHIDRA_PROJECT="${GHIDRA_PROJECT:-.ghidra/ck3}"
check_dir "Ghidra project directory" "$GHIDRA_PROJECT"

echo ""
echo "--- Wiki Directory ---"
WIKI_DIR="${WIKI_DIR:-wiki}"
check_dir "Wiki directory" "$WIKI_DIR"

echo ""
echo "=== Summary ==="
if [ "$errors" -gt 0 ]; then
    printf "${RED}%d critical tool(s) missing.${NC} Install them before running prospector agents.\n" "$errors"
    exit 1
elif [ "$warnings" -gt 0 ]; then
    printf "${YELLOW}%d warning(s).${NC} Some optional tools or directories are missing.\n" "$warnings"
    printf "Prospector agents can start but may have reduced capabilities.\n"
    exit 0
else
    printf "${GREEN}All tools available.${NC} Ready for RE investigation.\n"
    exit 0
fi
