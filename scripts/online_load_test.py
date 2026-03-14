#!/usr/bin/env python3
"""Online load test: 32 netbot clients to Fly URL. Requires FLY_API_TOKEN for metrics."""
import argparse
import subprocess
import sys
import time
from pathlib import Path

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--app", default="mmorms", help="Fly app name")
    args = ap.parse_args()

    root = Path(__file__).resolve().parent.parent
    url = f"wss://{args.app}.fly.dev/ws"

    print(f"Connecting 32 clients to {url}...")
    netbot = subprocess.Popen(
        ["go", "run", "./netbot", f"--addr={url}", "--cnt=32"],
        cwd=root,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.PIPE,
    )

    print("Running 60s...")
    for i in range(6):
        time.sleep(10)
        r = subprocess.run(
            ["fly", "alloc", "status", "-a", args.app],
            capture_output=True,
            text=True,
        )
        if r.returncode == 0:
            print(r.stdout)
        else:
            print("fly alloc status failed (need FLY_API_TOKEN?):", r.stderr)

    netbot.terminate()
    netbot.wait(timeout=5)
    print("Done.")

if __name__ == "__main__":
    main()
