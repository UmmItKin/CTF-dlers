#!/bin/bash
set -e

INSTALL_DIR="${1:-/usr/local/bin}"

go build -o CTF-dlers ./cmd

if [[ -w "$INSTALL_DIR" ]]; then
    cp -v CTF-dlers "$INSTALL_DIR/"
else
    echo "Installing CTF-dlers to $INSTALL_DIR requires sudo password:"
    sudo cp -v CTF-dlers "$INSTALL_DIR/"
fi

echo "Installed to $INSTALL_DIR/CTF-dlers"
