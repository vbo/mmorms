#!/usr/bin/env python3
"""Build overlord and mmorms, then start both. Cross-platform."""
import subprocess
import sys
from pathlib import Path


def main():
    script_dir = Path(__file__).resolve().parent

    for target, args in [
        ("mmorms", ["go", "build", "-o", "mmorms", "."]),
        ("overlord", ["go", "build", "-o", "overlord", "./overlord"]),
    ]:
        print(f"Building {target}...")
        r = subprocess.run(args, cwd=script_dir)
        if r.returncode != 0:
            sys.exit(r.returncode)

    print("Starting servers...")
    r = subprocess.run([sys.executable, str(script_dir / "singlebox.py")], cwd=script_dir)
    sys.exit(r.returncode)


if __name__ == "__main__":
    main()
