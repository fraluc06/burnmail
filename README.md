# Burnmail

🔥 A simple tool to manage temporary email addresses with a TUI

Inspired by [Mailsy](https://github.com/BalliAsghar/Mailsy), using the [mail.tm](https://mail.tm) API.

## Features

- 📧 Generate random disposable emails
- 📬 Read messages interactively
- 📋 Auto-copy to clipboard
- 🔒 Single binary, no runtime deps

## Installation

### macOS/Linux (Homebrew)

```bash
brew install fraluc06/burnmail/burnmail
```

### Go Install

```bash
go install github.com/fraluc06/burnmail@latest
```

### Build from source
```bash
git clone https://github.com/fraluc06/burnmail.git
cd burnmail
go build -o burnmail
sudo mv burnmail /usr/local/bin/
```

## Usage

```bash
# Generate email (auto-copied to clipboard)
burnmail g

# Check inbox (interactive TUI)
burnmail m

# List messages (classic view)
burnmail m list
# or
burnmail m ls

# Show account
burnmail me

# Show version
burnmail v
# or
burnmail version

# Delete account
burnmail d
```

## Example

```bash
$ burnmail g
✓ Email created and copied to clipboard!
x9k2m5p7@mail.tm

$ burnmail m
# Opens interactive TUI with message list
# Use arrow keys to navigate, Enter to read messages

$ burnmail m list
# Shows classic list view of messages
# Use arrow keys to select, Enter to read

$ burnmail m ls
# Same as list, shorter alias
```

## Troubleshooting

**Rate limit exceeded** - Wait a few minutes

**Token expired** - Regenerate: `burnmail d && burnmail g`

**Clipboard not working (Linux)** - Install xclip: `sudo apt install xclip`

## Development

```bash
# Build
make build

# Test
make test

# Cross-compile for all platforms
make build-all
```

## License

This project is open source and available under the [MIT License](LICENSE).

## Credits

Inspired by [Mailsy](https://github.com/BalliAsghar/Mailsy) • Powered by [mail.tm](https://mail.tm)
