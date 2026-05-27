# Archie TG

Open-source Telegram anti-spam userbot for personal accounts with a TUI interface.

Bot strategy:
- first message from unknown user: delete chat
- next messages: delete chat + block user

## Download (Recommended)

You do **not** need Go or source code to use Archie TG.

1. Open [Releases](https://github.com/Archik008/Archie-tg/releases)
2. Download the binary for your OS
3. Run it directly

## First Start

At first launch in TUI:
1. Choose `Авторизоваться`
2. Enter `app_id` and `app_hash` (from Telegram API)
3. Enter your phone number
4. Enter confirmation code
5. Enter 2FA password (if enabled)

Then choose `Старт приложения`.

## Main Features

- TUI interface with runtime logs
- Telegram user auth (session-based)
- Whitelist management from TUI
- SQLite storage for chat/whitelist state
- Handles incoming private messages with anti-spam flow

## Data Storage

Local files are stored in `.archie/` (near executable by default):
- `tg.session`
- `session.meta.json`
- `archie.db`

## Environment Variable (Optional)

Override runtime data directory:

```powershell
$env:ARCHIE_SESSION_DIR="C:\path\to\data"
.\archie.exe
```

## For Contributors

Source code and architecture are fully open in this repository.
