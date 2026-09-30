#!/usr/bin/env python3
"""Scratch-only terminal receipts. Usage: python3 scripts/verify_pty.py /tmp/cs."""
import errno
import fcntl
import json
import os
from pathlib import Path
import pty
import select
import signal
import statistics
import struct
import subprocess
import sys
import tempfile
import termios
import time

BINARY = str(Path(sys.argv[1]).resolve())

class Session:
    def __init__(self, root, args=(), size=(120, 30)):
        self.master, slave = pty.openpty()
        self.started = time.monotonic()
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", size[1], size[0], 0, 0))
        env = dict(os.environ, HOME=str(root), XDG_CONFIG_HOME=str(root / "xdg"), TERM="xterm-256color", NO_COLOR="1", LANG="C.UTF-8")
        def terminal():
            os.setsid()
            fcntl.ioctl(slave, termios.TIOCSCTTY, 0)
        self.process = subprocess.Popen([BINARY, "--config", str(root / "config.yaml"), "--no-color", *args], cwd=root, env=env, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=slave, preexec_fn=terminal, pass_fds=(slave,))
        os.close(slave)
        self.screen = b""
    def read(self, timeout=.1):
        if select.select([self.master], [], [], timeout)[0]:
            try:
                chunk = os.read(self.master, 65536)
            except OSError as error:
                if error.errno == errno.EIO:
                    return
                raise
            self.screen += chunk
            if b"\x1b[6n" in chunk:
                os.write(self.master, b"\x1b[1;1R")
            if b"\x1b]11;?" in chunk:
                os.write(self.master, b"\x1b]11;rgb:0000/0000/0000\x1b\\")
    def expect(self, text, timeout=5):
        deadline = time.monotonic() + timeout
        while text.encode() not in self.screen:
            self.read()
            if time.monotonic() > deadline:
                raise AssertionError(f"missing {text!r}; exit={self.process.poll()} screen={self.screen[-3000:]!r}")
        return (time.monotonic() - self.started) * 1000
    def send(self, data):
        os.write(self.master, data)
        time.sleep(.08)
        self.read(.02)
    def paste(self, text):
        self.send(b"\x1b[200~" + text.encode() + b"\x1b[201~")
    def finish(self):
        try:
            self.process.wait(timeout=5)
            self.read(.01)
            assert self.process.stdout is not None
            output = self.process.stdout.read()
            assert self.process.returncode == 0, self.screen[-2000:]
            return output
        finally:
            if self.process.poll() is None:
                self.process.kill()
                self.process.wait()
            if self.process.stdout is not None:
                self.process.stdout.close()
            os.close(self.master)


def workload(root, count):
    (root / "config.yaml").write_text("settings:\n  sources: ['source-*.yaml']\n  project_source: false\nsnippets: []\n")
    sources = ["snippets:\n" for _ in range(20)]
    for i in range(count):
        title = f"Inspect resource in workspace {i % 9000:05d}, current status"[:48]
        if i % 97 == 0:
            title = f"日本語 café resource {i % 9000:05d}"
        description = ("Inspect current resource status in a service namespace; narrow by category and discover commands with shared prefixes. " * 2)[:120]
        command = "echo '$0 ${HOME}'; "
        command += "x" * (512 - len(command))
        if i % 100 == 0:
            command += "x" * (4096 - len(command))
        tags = [] if i % 10 == 0 else [f"Tag {((i + j) % 100):02d}" for j in range(1 + i % 4)]
        sources[i % 20] += f"  - name: {json.dumps(title, ensure_ascii=False)}\n    description: {json.dumps(description)}\n    tags: {json.dumps(tags)}\n    command: {json.dumps(command)}\n"
    for i, data in enumerate(sources):
        (root / f"source-{i:02d}.yaml").write_text(data)


def guided_pod(root):
    session = Session(root, ["add"])
    session.expect("Edit command")
    session.paste("Pod sorted, guided")
    session.send(b"\t\t\t")
    literal = "kubectl top pods namespace --no-headers | sort -k3,3 -h -r"
    session.paste(literal)
    def mark(command, start, end):
        session.send(b"\x01" + b"\x1b[C" * start + b"\x10" + b"\x1b[C" * (end - start) + b"\x10")
        session.expect("Mark input")
    start = literal.index("namespace")
    mark(literal, start, start + len("namespace"))
    session.send(b"\x18")
    session.paste("namespace")
    session.send(b"\t\x1b[C\t")
    session.paste("-n")
    session.send(b"\t\r")
    session.send(b"\t" * 6 + b" " + b"\t\r")
    session.paste("all")
    session.send(b"\t")
    session.paste("-A")
    session.send(b"\x1b[1;5D" + b"\t" * 3)
    command = literal.replace("namespace", "{{namespace}}", 1)
    start = command.index("-k3") + 2
    mark(command, start, start + 1)
    session.send(b"\x18")
    session.paste("sort_by")
    session.send(b"\t" + b"\x1b[C" * 2 + b"\t\r")
    session.send(b"\t" * 23 + b"\x18")
    session.paste("CPU")
    session.send(b"\t\x18")
    session.paste("CPU")
    session.send(b"\t\t\t\r")
    session.paste("Memory")
    session.send(b"\t")
    session.paste("4")
    session.send(b"\x1b[1;5D" + b"\t" * 3)
    command = command.replace("-k3", "-k{{sort_by}}", 1)
    start = command.index(",3") + 1
    mark(command, start, start + 1)
    session.send(b"\t\r")
    session.send(b"\x13")
    session.expect("Saved to")
    session.send(b"\x1b")
    assert session.finish() == b""
    command = subprocess.check_output([BINARY, "--config", str(root / "config.yaml"), "render", "Pod sorted, guided", "--set", "namespace=all", "--set", "sort_by=Memory"], cwd=root, env=dict(os.environ, HOME=str(root), XDG_CONFIG_HOME=str(root / "xdg")))
    assert b"-A" in command and b"-k4,4" in command, command
    assert "expressions:" not in (root / "created.snippets.yaml").read_text()


def flows(root):
    main = "settings:\n  sources: ['*.snippets.yaml']\n  default_source: created.snippets.yaml\nsnippets: []\n"
    (root / "config.yaml").write_text(main)
    marker = root / "must-not-execute"
    literal = f"printf bad > {marker}"
    owner = root / "owned.snippets.yaml"
    owner.write_text(f"# owner\nsnippets:\n  - name: Original command\n    tags: [Kubernetes, Monitoring]\n    command: {literal}\n  - name: Valid port\n    tags: [Kubernetes]\n    command: echo {{{{port}}}}\n    inputs:\n      - name: port\n        required: true\n        validate: {{range: [1, 65535]}}\n  - name: Two inputs\n    command: echo {{{{first}}}} {{{{second}}}}\n    inputs:\n      - {{name: first, default: one}}\n      - {{name: second, default: two}}\n  - name: Conditional\n    command: echo {{{{mode}}}} {{{{namespace}}}}\n    inputs:\n      - name: mode\n        kind: choice\n        default: Hide\n        choices:\n          - {{label: Hide, value: \"\"}}\n          - {{label: Show, value: show}}\n      - name: namespace\n        default: team-a\n        required: true\n        visible_when: {{input: mode, equals: Show}}\n")
    session = Session(root)
    session.expect("CS — command library")
    session.paste("Original command")
    session.send(b"\r")
    assert session.finish() == (literal + "\n").encode()
    assert not marker.exists()
    # Explicit execution gets a scratch-only stub shell: record the authored
    # command as argv, never evaluate it. The prompt must honor n and Ctrl-C.
    shell_marker = root / "shell-called"
    shell = root / "sh"
    shell.write_text(f"#!/bin/sh\nprintf '%s\\n' \"$*\" > '{shell_marker}'\nprintf 'stub executed\\n'\n")
    shell.chmod(0o700)
    original_path = os.environ.get("PATH")
    try:
        os.environ["PATH"] = str(root)
        for answer in (b"n", b"\x03", b"y"):
            session = Session(root, ["exec", "Original command", "--prompt"])
            session.expect("Execute this command?")
            session.send(answer)
            expected = b"stub executed\n" if answer == b"y" else b""
            assert session.finish() == expected
            assert shell_marker.exists() == (answer == b"y"), "prompt cancellation executed"
        assert shell_marker.read_text() == f"-c {literal}\n"
        assert not marker.exists(), "stub evaluated command text"
    finally:
        if original_path is None:
            os.environ.pop("PATH", None)
        else:
            os.environ["PATH"] = original_path
    session = Session(root)
    session.expect("CS — command library")
    session.send(b"\x03")
    assert session.finish() == b""
    session = Session(root, ["exec", "Valid port", "--set", "port=0"])
    session.expect("invalid")
    session.send(b"\x13")
    assert session.process.poll() is None, "invalid Ctrl-S submitted"
    session.send(b"\x18")
    session.paste("8080")
    session.send(b"\r")
    assert session.finish() == b"echo 8080\n"
    session = Session(root, ["exec", "Two inputs"])
    session.expect("Fill inputs")
    session.send(b"\x13")
    assert session.finish() == b"echo one two\n", "Ctrl-S did not submit from the first field"
    session = Session(root, ["exec", "Conditional"])
    session.expect("Fill inputs")
    session.send(b"\x1b[C\t\x18")
    session.expect("required")
    session.send(b"\x1b[Z\x1b[D\x13")
    assert session.finish() == b"echo  \n", "hidden required input blocked submission or leaked value"
    session = Session(root, ["add"])
    session.expect("Edit command")
    session.paste("New command")
    session.send(b"\t\t\t")
    session.paste("echo hi")
    session.send(b"\x13")
    session.expect("Saved to")
    session.send(b"\x1b")
    assert session.finish() == b""
    assert "New command" in (root / "created.snippets.yaml").read_text()
    saved_ids = []
    guided_pod(root)
    for old, new in [("Original command", "Renamed command"), ("Renamed command", "Renamed again")]:
        session = Session(root, ["edit", old])
        session.expect("Edit command")
        session.send(b"\x18")
        session.paste(new)
        session.send(b"\x13")
        session.expect("Saved to")
        session.send(b"\x1b")
        assert session.finish() == b""
        ids = [line for line in owner.read_text().splitlines() if "id:" in line]
        if old == "Original command":
            saved_ids = ids
        else:
            assert saved_ids == ids
    assert (root / "config.yaml").read_text() == main
    session = Session(root)
    session.expect("CS — command library")
    session.paste("no-matches-here")
    session.expect("No matches")
    session.screen = b""
    session.send(b"\x0c\t" + b"\x1b[B" * 2 + b"\r")
    session.expect("Commands (2)")
    session.screen = b""
    session.send(b"\x1b[B\r")
    session.expect("Commands (1)")
    session.screen = b""
    session.send(b"\x1b[A" * 3 + b"\r")
    session.expect("Commands (6)")
    session.screen = b""
    session.send(b"\x1b[B\r")
    session.expect("Commands (4)")
    session.send(b"\x03")
    assert session.finish() == b""
    for size in [(140, 40), (90, 24), (60, 18), (40, 10)]:
        session = Session(root, size=size)
        session.expect("CS — command library")
        session.send(b"\x03")
        assert session.finish() == b""
    session = Session(root)
    session.expect("CS — command library")
    session.send(b"\x0f")
    session.expect("Library settings")
    session.send(b"\t" * 7 + b"\x1b[C" * 2 + b"\x13")
    session.expect("Settings saved.")
    session.send(b"\x1b")
    assert session.finish() == b""
    assert "color: never" in (root / "config.yaml").read_text()
    recovery = root / "recovery"
    recovery.mkdir()
    (recovery / "config.yaml").write_text("settings:\n  sources: [missing.yaml]\nsnippets: []\n")
    session = Session(recovery)
    session.expect("Library needs repair")
    session.send(b"\x0f")
    session.expect("Library settings")
    session.send(b"\t" * 3 + b"\r\x13")
    session.expect("Settings saved.")
    session.expect("No commands yet")
    session.send(b"\x1b")
    assert session.finish() == b""
    session = Session(root, size=(20, 5))
    session.expect("Resize terminal")
    fcntl.ioctl(session.master, termios.TIOCSWINSZ, struct.pack("HHHH", 24, 90, 0, 0))
    os.kill(session.process.pid, signal.SIGWINCH)
    session.expect("CS — command library")
    session.send(b"\x03")
    assert session.finish() == b""
    return ["controlling terminal with stdin redirected", "search/use without execution", "explicit execution prompt uses scratch stub and honors n/Ctrl-C/y", "cancel with empty stdout", "invalid all-preset form blocked by Ctrl-S then valid insertion", "Ctrl-S submit from a non-last input field", "conditional required field hides and emits typed zero", "guided add/save without emission", "pod authoring by marking, flag/special, mapped choices and explicit duplicate linking", "owner-local rename with stable ID", "tag intersections, All/Untagged and no-match recovery", "settings controls and invalid-source recovery", "responsive terminal dimensions", "20x5 resize to 90x24"]


with tempfile.TemporaryDirectory(prefix="cs-pty-") as directory:
    root = Path(directory)
    receipts = {"flows": flows(root), "terminal": "140x40, 120x30, 90x24, 60x18, 40x10, 20x5 and resize", "filesystem": "local scratch; warm after first launch", "binary": BINARY}
    perf = root / "performance"
    perf.mkdir()
    workload(perf, 10000)
    timings = []
    for _ in range(20):
        session = Session(perf)
        timings.append(session.expect("CS — command library", timeout=15))
        session.send(b"\x03")
        assert session.finish() == b""
    timings.sort()
    receipts["warm_launch_10000_ms"] = {"samples": timings, "p95": timings[18], "median": statistics.median(timings), "budget": 500, "passed": timings[18] <= 500}
    print(json.dumps(receipts, indent=2))
    if timings[18] > 500:
        raise SystemExit("warm launch budget exceeded")
