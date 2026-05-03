# HPAD on Linux

HPAD runs as a single-process Wails tray app. The HID handling, tray icon, hidden/openable window, and command assignment logic all stay in the same `hpad` process managed by `systemd --user`.

## Install

Build and install the current user service:

```bash
task install-user
```

This installs:

- `~/.local/bin/hpad`
- `~/.config/systemd/user/hpad.service`
- `~/.local/share/applications/hpad.desktop`
- `~/.local/share/icons/hicolor/1024x1024/apps/hpad.png` when the repository icon is present

The install script enables and starts the user service immediately.

## Day-to-day commands

Check service status:

```bash
task status
```

Follow journald logs:

```bash
task logs
```

Restart after rebuilding or changing runtime behavior:

```bash
task restart-user
```

Stop the background app:

```bash
task stop-user
```

Uninstall the binary, service, desktop launcher, and icon:

```bash
task uninstall-user
```

`task uninstall-user` intentionally leaves config and state behind.

## Development mode

Run:

```bash
task dev
```

`task dev` stops the installed `hpad.service` first so the Wails dev process can own the HID device without competing with the background service.

## Filesystem locations

- Config: `~/.config/hpad`
- State: `~/.local/state/hpad`
- Runtime: `$XDG_RUNTIME_DIR/hpad`

If `XDG_RUNTIME_DIR` is unavailable, HPAD falls back to `~/.local/state/hpad/runtime` for its runtime lock file.

## Udev

User installation does not install udev rules. If hidraw access still needs a rule on your machine, install the existing rule separately:

```bash
task linux:install-udev-rule
```

The current repository rule targets VID `0xCAFE` and PID `0xB00B`.