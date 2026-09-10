"""Small PTY driver for real CLI integration checks (requires pyte)."""
import codecs
import fcntl
import os
import pty
import select
import struct
import subprocess
import termios
import time

import pyte


class Terminal:
    def __init__(self, args, output, name, cols=132, rows=42):
        self.output, self.name = output, name
        self.master, slave = pty.openpty()
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", rows, cols, 0, 0))
        self.process = subprocess.Popen([str(x) for x in args], stdin=slave, stdout=slave, stderr=slave, env=dict(os.environ, TERM="xterm-256color", COLORTERM="truecolor"), close_fds=True)
        os.close(slave)
        self.screen = pyte.Screen(cols, rows)
        self.stream = pyte.Stream(self.screen)
        self.decoder = codecs.getincrementaldecoder("utf-8")("replace")
        self.log = ""

    def read(self, timeout=.1):
        if select.select([self.master], [], [], timeout)[0]:
            try:
                value = self.decoder.decode(os.read(self.master, 262144))
            except OSError:
                return
            self.log += value
            self.stream.feed(value)

    def text(self):
        return "\n".join(self.screen.display)

    def wait(self, text, timeout=30):
        until = time.monotonic() + timeout
        while text not in self.text():
            self.read()
            if self.process.poll() is not None or time.monotonic() > until:
                self.save("failed")
                raise RuntimeError("missing screen text " + text + "\n" + self.text())

    def send(self, keys):
        os.write(self.master, keys)
        self.read(.2)

    def settle(self):
        until = time.monotonic() + .3
        while time.monotonic() < until:
            self.read(.05)

    def save(self, suffix):
        self.settle()
        self.output.mkdir(parents=True, exist_ok=True)
        (self.output / (self.name + "-" + suffix + ".txt")).write_text(self.text())
        (self.output / (self.name + ".ansi")).write_text(self.log)

    def resize(self, cols, rows):
        self.screen.resize(rows, cols)
        fcntl.ioctl(self.master, termios.TIOCSWINSZ, struct.pack("HHHH", rows, cols, 0, 0))
        os.kill(self.process.pid, 28)
        self.settle()

    def __enter__(self):
        return self

    def __exit__(self, kind, value, traceback):
        if self.process.poll() is None:
            os.write(self.master, b"\x03")
            until = time.monotonic() + 8
            while self.process.poll() is None and time.monotonic() < until:
                self.read()
            if self.process.poll() is None:
                self.process.kill()
            self.process.wait(timeout=5)
        self.save("closed")
        os.close(self.master)
