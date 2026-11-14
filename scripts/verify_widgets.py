#!/usr/bin/env python3
"""Characterize real UTF-8 ZLE/Readline offsets using only scratch shells/files."""
import json
import os
from pathlib import Path
import pty
import select
import shutil
import signal
import subprocess
import sys
import tempfile
import time

shell, script = sys.argv[1:]
if not shutil.which(shell):
    print(f"SKIP: {shell} unavailable")
    sys.exit(77)
# Require a multibyte locale rather than calling a byte-locale result Unicode.
env = dict(os.environ, LC_ALL="C.UTF-8", LANG="C.UTF-8")
probe = subprocess.run([shell, "-c", "s='é界'; test ${#s} -eq 2"], env=env)
if probe.returncode:
    print(f"SKIP: {shell} lacks C.UTF-8 character counting")
    sys.exit(77)

with tempfile.TemporaryDirectory(prefix="cs-widget-") as tmp:
    receipt = Path(tmp) / "receipt"
    env.update(HOME=tmp, XDG_CONFIG_HOME=tmp, SCRIPT=str(Path(script).resolve()), RECEIPT=str(receipt), TERM="xterm")
    master, slave = pty.openpty()
    def terminal():
        os.setsid()
        import fcntl
        import termios
        fcntl.ioctl(slave, termios.TIOCSCTTY, 0)
    args = [shell, "-f", "-i"] if shell == "zsh" else [shell, "--noprofile", "--norc", "-i"]
    process = subprocess.Popen(args, stdin=slave, stdout=slave, stderr=slave, env=env, preexec_fn=terminal)
    os.close(slave)
    transcript = bytearray()
    def drain(seconds):
        deadline = time.monotonic() + seconds
        while time.monotonic() < deadline:
            if select.select([master], [], [], .02)[0]:
                try:
                    transcript.extend(os.read(master, 65536))
                except OSError:
                    break
    try:
        drain(.15)
        setup = 'cs(){ printf %s "$FAKE_COMMAND"; return "$FAKE_STATUS"; }; source "$SCRIPT"; '
        if shell == "zsh":
            setup += 'probe(){ local before=$CURSOR; cs-insert-widget; printf "%s\\n" "$before" "$BUFFER" "$CURSOR" "$RBUFFER" > "$RECEIPT"; BUFFER=""; }; zle -N probe; bindkey "^G" probe; '
        else:
            setup += 'probe(){ local before=$READLINE_POINT; cs-insert-widget; printf "%s\\n" "$before" "$READLINE_LINE" "$READLINE_POINT" > "$RECEIPT"; READLINE_LINE=""; READLINE_POINT=0; }; bind -x \'"\\C-g":probe\'; '
        setup += "printf 'CS_WIDGET_READY\\n'\n"
        os.write(master, setup.encode())
        deadline = time.monotonic() + 5
        while b"CS_WIDGET_READY\r\n" not in transcript:
            drain(.05)
            if time.monotonic() > deadline:
                raise AssertionError(f"shell not ready: {transcript!r}")
        results = []
        for status, command in [("0", "ø雪"), ("0", ""), ("1", "ø雪")]:
            # Set variables with shell commands, then edit a never-executed buffer.
            os.write(master, f"FAKE_STATUS={status}; FAKE_COMMAND='{command}'\n".encode())
            drain(.12)
            if receipt.exists():
                receipt.unlink()
            os.write(master, "é界 右".encode() + b"\x1b[D\x07")
            deadline = time.monotonic() + 5
            while not receipt.exists():
                drain(.05)
                if time.monotonic() > deadline:
                    raise AssertionError(f"widget not called: {transcript!r}")
            drain(.05)
            fields = receipt.read_text().splitlines()
            before, buffer, cursor = int(fields[0]), fields[1], int(fields[2])
            if shell == "zsh":
                want = "é界 ø雪右" if status == "0" and command else "é界 右"
                assert before == 3 and buffer == want and cursor == (5 if want != "é界 右" else 3) and fields[3] == "右", fields
                capability = "UTF-8 ZLE character cursor and unchanged right buffer"
            else:
                # Characterize the actual binding without claiming that every
                # Bash/Readline version uses the same offset convention.
                assert before in (3, 6), fields
                inserted = status == "0" and bool(command)
                want = ("é界 ø雪右" if before == 3 else "é界 右ø雪") if inserted else "é界 右"
                assert buffer == want and cursor == before + (2 if inserted else 0), fields
                capability = ("This Bash uses character offsets; no wider Unicode compatibility claim" if before == 3 else "Readline byte offsets: documented Unicode incompatibility, not a supported insertion")
            results.append(dict(status=int(status), command=command, before=before, buffer=buffer, cursor=cursor, characterization=capability))
        print(json.dumps(dict(shell=shell, locale="C.UTF-8", results=results), ensure_ascii=False, indent=2))
    finally:
        os.killpg(process.pid, signal.SIGKILL)
        process.wait(timeout=5)
        os.close(master)
