#!/bin/sh
# Starts fail2ban with a polling sshd jail (no iptables needed). With arguments
# they run as the test command once the socket answers; without, the server stays in the foreground.
set -eu
SOCK=/run/fail2ban/fail2ban.sock
touch /tmp/auth.log
mkdir -p /run/fail2ban
if [ "$#" -eq 0 ]; then
	exec fail2ban-server -f -x -s "$SOCK" -p /run/fail2ban/fail2ban.pid
fi
fail2ban-server -b -x -s "$SOCK" -p /run/fail2ban/fail2ban.pid
i=0
until fail2ban-client -s "$SOCK" ping >/dev/null 2>&1; do
	i=$((i + 1))
	[ "$i" -lt 50 ] || { echo "fail2ban did not start" >&2; exit 1; }
	sleep 0.2
done
fail2ban-client -s "$SOCK" version
exec "$@"
