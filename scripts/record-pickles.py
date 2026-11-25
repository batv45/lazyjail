#!/usr/bin/env python3
"""Records raw fail2ban socket replies for the proto decoder tests.

Run inside the integration container: record-pickles.py OUTDIR VERSION_TAG
"""
import json, os, pickle, socket, sys

SOCK = "/run/fail2ban/fail2ban.sock"
END = b"<F2B_END_COMMAND>"
CLOSE = b"<F2B_CLOSE_COMMAND>"

def call(cmd):
    s = socket.socket(socket.AF_UNIX)
    s.connect(SOCK)
    s.sendall(pickle.dumps(list(cmd), 0) + END)
    buf = b""
    while not buf.endswith(END):
        d = s.recv(65536)
        if not d:
            break
        buf += d
    s.close()
    return buf

out, tag = sys.argv[1], sys.argv[2]
os.makedirs(out, exist_ok=True)
cmds = {
    "ping": ["ping"],
    "version": ["version"],
    "status": ["status"],
    "status_sshd": ["status", "sshd"],
    "status_sshd_flavor": ["status", "sshd", "short"],
    "banip_withtime": ["get", "sshd", "banip", "--with-time"],
    "banip": ["get", "sshd", "banip"],
    "bantime": ["get", "sshd", "bantime"],
    "findtime": ["get", "sshd", "findtime"],
    "maxretry": ["get", "sshd", "maxretry"],
    "logpath": ["get", "sshd", "logpath"],
    "failregex": ["get", "sshd", "failregex"],
    "ignoreregex": ["get", "sshd", "ignoreregex"],
    "ignoreip": ["get", "sshd", "ignoreip"],
    "actions": ["get", "sshd", "actions"],
    "logtarget": ["get", "logtarget"],
    "err_unknown_jail": ["status", "nosuchjail"],
    "err_unknown_cmd": ["frobnicate"],
}
for name, cmd in cmds.items():
    with open(f"{out}/{tag}_{name}.pickle", "wb") as f:
        f.write(call(cmd))
# mutate: ban + record
for name, cmd in {"set_banip": ["set", "sshd", "banip", "192.0.2.10"],
                  "banip_withtime_after": ["get", "sshd", "banip", "--with-time"],
                  "set_unbanip": ["set", "sshd", "unbanip", "192.0.2.10"],
                  "set_addignoreip": ["set", "sshd", "addignoreip", "192.0.2.0/24"],
                  "ignoreip_after": ["get", "sshd", "ignoreip"]}.items():
    with open(f"{out}/{tag}_{name}.pickle", "wb") as f:
        f.write(call(cmd))
print("ok")
