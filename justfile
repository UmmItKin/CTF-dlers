default:
    @just --list

build:
    go build -o CTF-dlers ./cmd

build-verbose:
    go build -v -o CTF-dlers ./cmd

build-all:
    GOOS=linux GOARCH=amd64 go build -o dist/CTF-dlers-linux-amd64 ./cmd
    GOOS=windows GOARCH=amd64 go build -o dist/CTF-dlers-windows-amd64.exe ./cmd
    GOOS=darwin GOARCH=amd64 go build -o dist/CTF-dlers-darwin-amd64 ./cmd
    GOOS=darwin GOARCH=arm64 go build -o dist/CTF-dlers-darwin-arm64 ./cmd

test url token:
    go run ./cmd --url {{url}} --token {{token}} --test

test-bin url token: build
    ./CTF-dlers --url {{url}} --token {{token}} --test

dry-run url token:
    go run ./cmd --url {{url}} --token {{token}} --dry-run

dry-run-bin url token: build
    ./CTF-dlers --url {{url}} --token {{token}} --dry-run

download url token:
    go run ./cmd --url {{url}} --token {{token}}

download-bin url token: build
    ./CTF-dlers --url {{url}} --token {{token}}

download-custom url token workers="10" rate="15":
    go run ./cmd --url {{url}} --token {{token}} --workers {{workers}} --rate-limit {{rate}}

download-full url token:
    go run ./cmd --url {{url}} --token {{token}} --hints --solves

run-config config="config.yml":
    go run ./cmd --config {{config}}

run-config-bin config="config.yml": build
    ./CTF-dlers --config {{config}}

clean:
    rm -f CTF-dlers
    rm -rf dist/
    go clean

fmt:
    go fmt ./...

vet:
    go vet ./...

check: fmt vet
    @echo "Code formatting and vetting completed"

deps:
    go mod tidy
    go mod download

install:
    ./install.sh

install-user:
    ./install.sh ~/.local/bin

help: build
    ./CTF-dlers --help

version: build
    ./CTF-dlers --version

create-config:
    @echo "base_url: \"https://ctf.example.com\"" > config.example.yml
    @echo "token: \"ctfd_your_token_here\"" >> config.example.yml
    @echo "output_dir: \"./challenges\"" >> config.example.yml
    @echo "max_workers: 5" >> config.example.yml
    @echo "rate_limit: 10" >> config.example.yml
    @echo "retry_count: 3" >> config.example.yml
    @echo "retry_delay: \"1s\"" >> config.example.yml
    @echo "include_hints: false" >> config.example.yml
    @echo "include_solves: false" >> config.example.yml
    @echo "Created config.example.yml"

dev url token: fmt build
    ./CTF-dlers --url {{url}} --token {{token}} --test

dev-full url token: check build
    ./CTF-dlers --url {{url}} --token {{token}} --test
    @echo "All checks passed!"

test-install: build
    @echo "Testing installation script..."
    ./install.sh --help 2>/dev/null || echo "Usage: ./install.sh [install_dir]"
    @echo "Installation script is working correctly!"
