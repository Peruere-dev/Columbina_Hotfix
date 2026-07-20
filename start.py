#!/data/data/com.termux/files/usr/bin/python3
"""Serveo tunnel, update config, exit."""

import json
import os
import re
import signal
import subprocess
import sys
import time

ROOT = os.path.dirname(os.path.abspath(__file__))
CONFIG_JSON = os.path.join(ROOT, "config.json")
LOG = "/data/data/com.termux/files/usr/tmp/serveo.log"
URL_FILE = os.path.join(ROOT, ".tunnel_url")


def read_config():
    with open(CONFIG_JSON) as f:
        return json.load(f)


def write_config(cfg):
    with open(CONFIG_JSON, "w") as f:
        json.dump(cfg, f, indent=4, ensure_ascii=False)
        f.write("\n")


def start_tunnel():
    logf = open(LOG, "w")
    proc = subprocess.Popen(
        [
            "ssh",
            "-o", "StrictHostKeyChecking=no",
            "-o", "ServerAliveInterval=30",
            "-o", "ExitOnForwardFailure=yes",
            "-R", "80:localhost:5200",
            "serveo.net",
        ],
        stdout=logf,
        stderr=subprocess.STDOUT,
        start_new_session=True,
    )
    logf.close()
    return proc


def wait_for_url(timeout=30):
    pat = re.compile(rb"https://[\w.-]+\.serveousercontent\.com")
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            with open(LOG, "rb") as f:
                m = pat.search(f.read())
                if m:
                    return m.group().decode()
        except FileNotFoundError:
            pass
        if os.path.exists(LOG) and os.path.getsize(LOG) > 0:
            pass
        time.sleep(0.5)
    return None


def find_pids(name):
    try:
        out = subprocess.check_output(
            ["pgrep", "-f", name], timeout=5
        ).decode().strip()
        return [int(p) for p in out.splitlines()]
    except (subprocess.CalledProcessError, FileNotFoundError):
        return []


def main():
    for pid in find_pids("^./Columbina$"):
        os.kill(pid, signal.SIGTERM)
    for pid in find_pids("serveo.net"):
        os.kill(pid, signal.SIGTERM)

    for attempt in range(3):
        proc = start_tunnel()
        url = wait_for_url()
        if url:
            break
        proc.kill()
        time.sleep(3)
    else:
        print("serveo failed", file=sys.stderr)
        sys.exit(1)

    with open(URL_FILE, "w") as f:
        f.write(url + "\n")

    host = url.removeprefix("https://").removesuffix(":443")
    cfg = read_config()
    cfg.setdefault("server", {})["accessAddress"] = host
    cfg["server"]["accessPort"] = 443
    cfg["server"]["forcePublicHttps"] = True
    write_config(cfg)

    print(url)


if __name__ == "__main__":
    main()
