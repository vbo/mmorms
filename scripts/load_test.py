#!/usr/bin/env python3
"""Load test: Docker 256MB + 32 netbot clients. Reports CPU/MEM."""
import subprocess
import sys
import time
from pathlib import Path

try:
    from urllib.request import urlopen
except ImportError:
    from urllib2 import urlopen

def main():
    script_dir = Path(__file__).resolve().parent
    root = script_dir.parent

    print("Building Docker image...")
    r = subprocess.run(["docker", "build", "-t", "mmorms-test", "."], cwd=root)
    if r.returncode != 0:
        sys.exit(r.returncode)

    print("Starting container (256MB limit)...")
    proc = subprocess.Popen(
        ["docker", "run", "-d", "-m", "256m", "-p", "8080:8080", "mmorms-test"],
        cwd=root,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    out, err = proc.communicate()
    if proc.returncode != 0:
        print(err, file=sys.stderr)
        sys.exit(proc.returncode)
    cid = out.strip()[:12]

    try:
        print("Waiting for server...")
        for _ in range(30):
            try:
                urlopen("http://localhost:8080/", timeout=2)
                break
            except Exception:
                time.sleep(1)
        else:
            print("Server did not become ready")
            sys.exit(1)

        print("Starting 32 netbot clients...")
        netbot = subprocess.Popen(
            ["go", "run", "./netbot", "--addr=ws://localhost:8080/ws", "--cnt=32"],
            cwd=root,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.PIPE,
        )

        print("Monitoring for 60s...")
        samples = []
        for _ in range(6):
            time.sleep(10)
            r = subprocess.run(
                ["docker", "stats", "--no-stream", "--format",
                 "{{.MemUsage}}\t{{.CPUPerc}}", cid],
                capture_output=True,
                text=True,
            )
            if r.returncode == 0 and r.stdout.strip():
                line = r.stdout.strip().split("\n")[0]
                parts = line.split("\t")
                mem = parts[0].strip() if len(parts) > 0 else "?"
                cpu = parts[1].strip() if len(parts) > 1 else "?"
                samples.append((mem, cpu))
                print(f"  {mem}  {cpu}")

        netbot.terminate()
        netbot.wait(timeout=5)

        if samples:
            print("\nLoad test complete. Peak sample:", samples[-1])
        else:
            print("\nNo stats collected - check if container OOMed")
    finally:
        subprocess.run(["docker", "stop", cid], capture_output=True)
        subprocess.run(["docker", "rm", "-f", cid], capture_output=True)

if __name__ == "__main__":
    main()
