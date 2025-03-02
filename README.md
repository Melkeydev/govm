# GoVM - Go Version Manager

GoVM is a command-line tool designed to simplify the management of multiple Go installations on your system.

## Features

- Install and manage multiple Go versions simultaneously
- Switch between different Go versions
- Automatically download and install specific Go releases

## Installation

### Quick Install

```bash
go install github.com/melkeydev/govm@latest
```

Or manually:

```bash
git clone https://github.com/melkeydev/govm.git
go build
go install
```

### Setup

After installation, add GVM to your path:

```bash
echo 'export PATH="$HOME/.govm/shim:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

For ZSH users:

```bash
echo 'export PATH="$HOME/.govm/shim:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

## Usage

```bash
gvm
```

## License

Licensed under [MIT License](./LICENSE)

