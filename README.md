# tgmsgcleaner

Clean up your Telegram presence — delete old messages, reactions, and leave groups you no longer need.

## Features

- Delete your messages from any group, supergroup, or channel
- Delete your reactions
- Export messages to plaintext before deleting
- View messages in a built-in paginated viewer
- Find channels/supergroups you already left and clean those too
- Multi-account support
- FLOOD_WAIT handling — won't get you rate-limited

## Setup

1. Get `api_id` and `api_hash` at [my.telegram.org/apps](https://my.telegram.org/apps)
2. Run the app:
   ```
   go run ./cmd/tgmsgcleaner/
   ```
3. Enter credentials and auth code on first run

Config stored in `~/.config/tgmsgcleaner/`.

Use `--reset` to wipe all settings and accounts.

## Hotkeys

| Key | Action |
|-----|--------|
| `Enter` | Open action menu for selected group |
| `Tab` / `Shift+Tab` | Switch filter |
| `/` | Search |
| `l` | Find left channels |
| `a` | Add account |
| `s` | Switch account |
| `?` | Help |
| `q` | Quit |
