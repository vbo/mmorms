#!/usr/bin/env python3
"""Start overlord and mmorms servers. Cross-platform. Ctrl+C stops both."""
import os
import signal
import subprocess
import sys
from pathlib import Path


def main():
    script_dir = Path(__file__).resolve().parent
    suffix = ".exe" if sys.platform == "win32" else ""
    overlord_exe = script_dir / f"overlord{suffix}"
    mmorms_exe = script_dir / f"mmorms{suffix}"

    for name, path in [("overlord", overlord_exe), ("mmorms", mmorms_exe)]:
        if not path.exists():
            print(f"{name} not found at {path}. Run build_run.py first.", file=sys.stderr)
            sys.exit(1)

    overlord_proc = subprocess.Popen(
        [str(overlord_exe), "--addr=localhost:7070"],
        cwd=script_dir,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    mmorms_proc = subprocess.Popen(
        [str(mmorms_exe), "--addr=localhost:8080", "--overlord=localhost:7070"],
        cwd=script_dir,
    )

    def shutdown(*_):
        overlord_proc.terminate()
        mmorms_proc.terminate()
        overlord_proc.wait()
        mmorms_proc.wait()
        sys.exit(0)

    signal.signal(signal.SIGINT, shutdown)
    signal.signal(signal.SIGTERM, shutdown)
    if hasattr(signal, "SIGBREAK"):
        signal.signal(signal.SIGBREAK, shutdown)

    mmorms_proc.wait()
    overlord_proc.terminate()
    overlord_proc.wait()
    sys.exit(mmorms_proc.returncode or 0)


if __name__ == "__main__":
    main()
