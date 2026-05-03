#!/usr/bin/env bash
set -euo pipefail

applications_dir="${HOME}/.local/share/applications"
icons_root="${HOME}/.local/share/icons/hicolor"

systemctl --user disable --now hpad.service || true

rm -f "${HOME}/.local/bin/hpad"
rm -f "${HOME}/.config/systemd/user/hpad.service"
rm -f "${applications_dir}/hpad.desktop"
rm -f "${icons_root}/1024x1024/apps/hpad.png"

systemctl --user daemon-reload

if command -v update-desktop-database >/dev/null 2>&1; then
	update-desktop-database "${applications_dir}" || true
fi
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
	gtk-update-icon-cache -f -t "${icons_root}" || true
fi

echo "HPAD removed from user application paths."
echo "Config and state were left intact: ${HOME}/.config/hpad ${HOME}/.local/state/hpad"