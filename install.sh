#!/bin/bash
set -e

INSTALL_DIR="${1:-/usr/local/bin}"

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

echo "${bold}${cyan}==> Building CTF-dlers...${reset}"
go build -o CTF-dlers ./cmd

if [[ -w "$INSTALL_DIR" ]]; then
    cp -v CTF-dlers "$INSTALL_DIR/"
else
    echo "${yellow}Installing to $INSTALL_DIR requires sudo password:${reset}"
    sudo cp -v CTF-dlers "$INSTALL_DIR/"
fi

echo "${bold}${green}==> Installed to $INSTALL_DIR/CTF-dlers${reset}"
