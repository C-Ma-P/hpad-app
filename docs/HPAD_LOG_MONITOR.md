# HPAD Log Monitor

## What It Does

`hpad-log` is a small developer CLI that watches `/dev/serial/by-id`, discovers the HPAD macropad and HPAD dongle CDC ACM serial ports, opens both when available, prefixes each log line by device, and reconnects automatically after unplug, reboot, or reflashing.

Example output:

```text
[pad]    booting...
[dongle] USB initialized
[pad]    key 3 pressed
[dongle] rx key=3
```

## Why It Uses /dev/serial/by-id

`/dev/ttyACM0`, `/dev/ttyACM1`, and similar ACM node numbers are not stable. They can change when the pad or dongle reboots, when either device is reflashed, or when the USB attach order changes.

`/dev/serial/by-id` is the stable discovery path because it is based on the USB identity that Linux sees for each serial device. The monitor resolves those symlinks so connect messages show both the stable path and the current underlying ACM node.

Do not rely on `/dev/ttyACM0` ordering.

## How To Run

```bash
task logs:devices
task logs:devices:timestamps
task logs:devices:list
```

You can also run the CLI directly:

The monitor tasks build `bin/hpad-log` as your user and then run only the monitor binary under `sudo` when serial access is needed. This is the clean standard path on Linux because it:

- uses the normal `sudo` password prompt in your terminal
- avoids creating root-owned Go build cache entries
- avoids relying on `sudo` to find `task` or `go` in its secure `PATH`

Run `task logs:devices`, not `sudo task logs:devices`.

```bash
go run ./cmd/hpad-log
go run ./cmd/hpad-log --timestamps
go run ./cmd/hpad-log --list
```

For the standard privileged path, build the binary as your user and run the monitor binary itself under `sudo`:

```bash
task build:hpad-log
sudo ./bin/hpad-log
sudo ./bin/hpad-log --timestamps
./bin/hpad-log --list
```

If your local serial permissions are already configured, `go run ./cmd/hpad-log` and `go run ./cmd/hpad-log --timestamps` still work without the task wrapper.

Useful flags:

```text
--list
--timestamps
--pad-only
--dongle-only
--debug
```

## Expected Workflow

1. Start `task logs:devices`.
2. Flash or reboot the macropad or dongle as usual.
3. Leave the log monitor running.
4. Glance at the prefixed logs when needed.

The monitor starts cleanly if neither device is present, waits for each selected device, and reconnects automatically after UF2 flashing, reboot, or unplug.

## Troubleshooting

- Run `ls -l /dev/serial/by-id`.
- Run `task logs:devices:list`.
- If `task logs:devices` prompts for your password, that is expected. The task is asking `sudo` to run only the serial monitor binary.
- Do not use `sudo task logs:devices`; many systems do not include `task` in `sudo`'s secure `PATH`.
- If the pad currently appears as `usb-Zephyr_Project_CDC_ACM_serial_backend_*`, the monitor will use that as a pad fallback when there is exactly one such generic Zephyr CDC ACM device and no explicit HPAD pad identifier is present.
- If devices are not clearly identified, the firmware USB manufacturer, product, or serial descriptors may need to be more explicit.
- Do not rely on `/dev/ttyACM0` ordering.

Current repo inspection notes:

- `hpadv2-dongle` already defines explicit USB string descriptors in `src/usb_device.c` with manufacturer `HPad` and product `Dongle`.
- No equally explicit pad-side USB manufacturer or product string was obvious in the current `hpadv2` repo scan. In the current environment the pad shows up as `usb-Zephyr_Project_CDC_ACM_serial_backend_*`, so the host tool now treats a single such generic Zephyr CDC ACM device as the pad fallback until the firmware advertises a clearer HPAD-specific name in `/dev/serial/by-id`.

Suggested descriptor naming if the firmware needs to be tightened later:

- Manufacturer: `HPAD`
- Product: `HPAD Macropad`
- Product: `HPAD Dongle`
- Serial examples: `HPAD-PAD-001`, `HPAD-DONGLE-001`