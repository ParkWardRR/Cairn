#!/usr/bin/env bash
# Installs one self-hosted CI runner for one repository, as a rootless Podman container
# under the shared cairn-ci.slice. Run it ON the CI host, as the runner's user.
#
#   scp cairn-ci.slice install-runner.sh <runner-user>@<host>:
#   gh api -X POST repos/<owner>/<repo>/actions/runners/registration-token --jq .token \
#     | ssh <runner-user>@<host> 'bash install-runner.sh <repo>'
#
# The registration token (valid for an hour) is read from the FIRST LINE OF STDIN, never
# from an argument, so it is not in a process list or a shell history, and it is written
# only to a 0600 file. The runner's registration is persisted (CONFIGURED_ACTIONS_RUNNER_
# FILES_DIR), so a restart or a reboot does not need a fresh token.
#
# Why one runner per repository: a runner is registered to a repository (this is a
# personal account, not an organisation), and every runner here is held to the trusted-
# code policy in tests/check-runners.sh.
set -euo pipefail
umask 077

repo="${1:?usage: install-runner.sh <repo-name> [owner]}"
owner="${2:-ParkWardRR}"
image="docker.io/myoung34/github-runner:ubuntu-jammy"
unit="cairn-runner-$repo"
base="$HOME/cairn-runners/$repo"
slice_src="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" 2>/dev/null && pwd)/cairn-ci.slice"

IFS= read -r token
[ -n "$token" ] || { echo "no registration token on stdin" >&2; exit 1; }

mkdir -p "$base/work" "$base/config" "$HOME/.config/cairn-runner" "$HOME/.config/systemd/user"
printf 'RUNNER_TOKEN=%s\n' "$token" > "$HOME/.config/cairn-runner/$repo.env"
unset token

if [ ! -f "$HOME/.config/systemd/user/cairn-ci.slice" ]; then
  if [ -f "$slice_src" ]; then
    install -m 0644 "$slice_src" "$HOME/.config/systemd/user/cairn-ci.slice"
  else
    echo "cairn-ci.slice is not installed and not next to this script; refusing to run a runner outside the memory cap" >&2
    exit 1
  fi
fi

cat > "$HOME/.config/systemd/user/$unit.service" <<UNIT
[Unit]
Description=Cairn CI runner for $repo
Wants=network-online.target
After=network-online.target
RequiresMountsFor=%t/containers

[Service]
Slice=cairn-ci.slice
Environment=PODMAN_SYSTEMD_UNIT=%n
Restart=always
RestartSec=10
TimeoutStopSec=70
ExecStart=/usr/bin/podman run \\
	--cidfile=%t/%n.ctr-id \\
	--cgroups=no-conmon \\
	--rm \\
	--sdnotify=conmon \\
	--replace \\
	-d \\
	--name github-runner-$repo \\
	-e REPO_URL=https://github.com/$owner/$repo \\
	--env-file=%h/.config/cairn-runner/$repo.env \\
	-e RUNNER_NAME=cairn-vm-$repo \\
	-e RUNNER_LABELS=cairn,self-hosted,linux,x64 \\
	-e RUNNER_WORKDIR=/runner/_work \\
	-e CONFIGURED_ACTIONS_RUNNER_FILES_DIR=/runner/config \
	-e DISABLE_AUTOMATIC_DEREGISTRATION=true \
	-v $base/work:/runner/_work:Z \\
	-v $base/config:/runner/config:Z \\
	$image
ExecStop=/usr/bin/podman stop --ignore -t 10 --cidfile=%t/%n.ctr-id
ExecStopPost=/usr/bin/podman rm -f --ignore -t 10 --cidfile=%t/%n.ctr-id
Type=notify
NotifyAccess=all

[Install]
WantedBy=default.target
UNIT

systemctl --user daemon-reload
systemctl --user enable --now "$unit.service"
echo "installed $unit"
