#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
repo_root="$(cd -- "${script_dir}/.." && pwd -P)"

binary_source="${repo_root}/bin/hpad"
binary_target="${HOME}/.local/bin/hpad"
applications_dir="${HOME}/.local/share/applications"
desktop_file="${applications_dir}/hpad.desktop"
icons_dir="${HOME}/.local/share/icons/hicolor/1024x1024/apps"
icon_source="${repo_root}/internal/res/hpad-icon.png"
icon_target="${icons_dir}/hpad.png"
service_dir="${HOME}/.config/systemd/user"
service_file="${service_dir}/hpad.service"
config_dir="${HOME}/.config/hpad"
state_dir="${HOME}/.local/state/hpad"

if [[ ! -f "${binary_source}" ]]; then
	echo "missing built binary: ${binary_source}" >&2
	exit 1
fi

install -d -m 0755 "${HOME}/.local/bin"
install -d -m 0755 "${applications_dir}"
install -d -m 0755 "${icons_dir}"
install -d -m 0755 "${service_dir}"
install -d -m 0700 "${config_dir}"
install -d -m 0700 "${state_dir}"

install -m 0755 "${binary_source}" "${binary_target}"
install -m 0644 "${repo_root}/build/linux/hpad.service" "${service_file}"

cat > "${desktop_file}" <<EOF
[Desktop Entry]
Type=Application
Name=HPAD
Comment=HPAD desktop controller
Exec=${binary_target}
Icon=hpad
Terminal=false
Categories=Utility;
StartupNotify=false
EOF

if [[ -f "${icon_source}" ]]; then
	install -m 0644 "${icon_source}" "${icon_target}"
else
	echo "warning: icon not found at ${icon_source}; skipping icon install" >&2
fi

systemctl --user daemon-reload
systemctl --user enable --now hpad.service

if command -v update-desktop-database >/dev/null 2>&1; then
	update-desktop-database "${applications_dir}" || true
fi
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
	gtk-update-icon-cache -f -t "${HOME}/.local/share/icons/hicolor" || true
fi

echo "HPAD installed for user ${USER}."
echo "Check status: systemctl --user status hpad.service"
echo "Follow logs: journalctl --user -u hpad.service -f"