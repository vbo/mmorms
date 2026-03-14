#!/usr/bin/env python3
"""Kill overlord and mmorms processes. Cross-platform (Windows and Linux)."""
import subprocess
import sys


def main():
    if sys.platform == "win32":
        for name in ("overlord.exe", "mmorms.exe"):
            subprocess.run(
                ["taskkill", "/F", "/IM", name],
                capture_output=True,
            )
    else:
        for name in ("overlord", "mmorms"):
            subprocess.run(
                ["pkill", name],
                capture_output=True,
            )
    print("Cleanup done.")


if __name__ == "__main__":
    main()
