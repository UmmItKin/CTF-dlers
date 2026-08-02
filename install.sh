#!/bin/bash
set -e

INSTALL_DIR="${1:-/usr/local/bin}"
BIN="CTF-dlers"

# Colors, but only when writing to a terminal.
if [[ -t 1 ]] && command -v tput >/dev/null 2>&1; then
    bold=$(tput bold 2>/dev/null || true)
    green=$(tput setaf 2 2>/dev/null || true)
    yellow=$(tput setaf 3 2>/dev/null || true)
    cyan=$(tput setaf 6 2>/dev/null || true)
    reset=$(tput sgr0 2>/dev/null || true)
else
    bold= green= yellow= cyan= reset=
fi

echo "${bold}${cyan}==> Building ${BIN}...${reset}"
go build -o "$BIN" ./cmd

if [[ -w "$INSTALL_DIR" ]]; then
    install -m 0755 "$BIN" "$INSTALL_DIR/"
else
    echo "${yellow}Installing to $INSTALL_DIR requires sudo password:${reset}"
    sudo install -m 0755 "$BIN" "$INSTALL_DIR/"
fi

version=$("$INSTALL_DIR/$BIN" -version 2>/dev/null || echo "unknown")
size=$(du -h "$INSTALL_DIR/$BIN" | cut -f1)

echo
echo "${bold}${green}==> Installed${reset}"
printf "    %-8s %s\n" "path"    "$INSTALL_DIR/$BIN"
printf "    %-8s %s\n" "version" "$version"
printf "    %-8s %s\n" "size"    "$size"

case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *) echo "    ${yellow}note${reset}     $INSTALL_DIR is not on your PATH" ;;
esac

echo
echo "    Run ${bold}${BIN} -help${reset} to get started."
