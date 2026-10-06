#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

# All fixture archives, installer state and HTTP traffic stay local. Payloads
# must never execute: these checks exercise download-only installation.
python3 - "${ROOT_DIR}" <<'PY'
import io
import os
import platform
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

repo = Path(sys.argv[1])
goos = {"Linux": "linux", "Darwin": "darwin"}.get(platform.system())
goarch = {"x86_64": "amd64", "amd64": "amd64", "arm64": "arm64", "aarch64": "arm64"}.get(platform.machine())
if not goos or not goarch:
    raise SystemExit("link release compatibility selftest requires a supported Unix host")

version = "v0.0.0-link-compat"
names = {
    name: f"codex-feishu-{name}_{version[1:]}_{goos}_{goarch}.tar.gz"
    for name in ("link", "relay")
}


def package(name, include_payload=True):
    archive = io.BytesIO()
    root = names[name].removesuffix(".tar.gz")
    with tarfile.open(fileobj=archive, mode="w:gz") as bundle:
        directory = tarfile.TarInfo(root + "/")
        directory.type, directory.mode = tarfile.DIRTYPE, 0o755
        bundle.addfile(directory)
        if include_payload:
            payload = f'#!/bin/sh\n# {name} fixture\nprintf executed > "$LINK_INSTALL_TEST_EXEC_MARKER"\nexit 99\n'.encode()
            binary = tarfile.TarInfo(root + "/codex-feishu-" + name)
            binary.mode, binary.size = 0o755, len(payload)
            bundle.addfile(binary, io.BytesIO(payload))
    return archive.getvalue()


assets = {
    "canonical": {names["link"]: (200, package("link")), names["relay"]: (200, package("relay"))},
    "legacy": {names["relay"]: (200, package("relay"))},
    "server_error": {names["link"]: (503, b"unavailable"), names["relay"]: (200, package("relay"))},
    "missing_payload": {names["link"]: (200, package("link", False)), names["relay"]: (200, package("relay"))},
}
requests = []


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        parts = self.path.strip("/").split("/", 1)
        scenario, asset = parts if len(parts) == 2 else ("", "")
        requests.append((scenario, asset))
        status, body = assets.get(scenario, {}).get(asset, (404, b"not found"))
        self.send_response(status)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *_):
        pass


server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
worker = threading.Thread(target=server.serve_forever, daemon=True)
worker.start()
try:
    with tempfile.TemporaryDirectory(prefix="link-release-compat-") as directory:
        work = Path(directory)
        for scenario, expected_names, succeeds in (
            ("canonical", [names["link"]], True),
            ("legacy", [names["link"], names["relay"]], True),
            ("server_error", [names["link"]], False),
            ("missing_payload", [names["link"]], False),
        ):
            case = work / scenario
            case.mkdir()
            install_root = case / "releases"
            target = install_root / version
            target.mkdir(parents=True)
            sentinel = target / "preserve-existing-installation"
            sentinel.write_text("existing payload")
            current = install_root / "current"
            current.symlink_to(target)
            previous_link = os.readlink(current)
            marker = case / "payload-was-executed"
            env = dict(os.environ)
            for key in tuple(env):
                if key.lower().endswith("_proxy"):
                    del env[key]
            for key in ("HOME", "USERPROFILE", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "TMPDIR"):
                isolated = case / key.lower()
                isolated.mkdir()
                env[key] = str(isolated)
            env.update({
                "CODEX_FEISHU_RELAY_BASE_URL": f"http://127.0.0.1:{server.server_port}/{scenario}",
                "CODEX_FEISHU_RELAY_SKIP_SETUP": "0",
                "LINK_INSTALL_TEST_EXEC_MARKER": str(marker),
            })
            result = subprocess.run(
                ["bash", str(repo / "install-release.sh"), "--version", version,
                 "--install-root", str(install_root), "--download-only"],
                env=env, capture_output=True, text=True, timeout=30,
            )
            observed = [asset for case_name, asset in requests if case_name == scenario]
            assert observed == expected_names, (scenario, observed)
            assert (result.returncode == 0) == succeeds, (scenario, result.returncode, result.stderr[-1500:])
            assert not marker.exists(), (scenario, "download-only ran the payload")
            if succeeds:
                binary_name = "codex-feishu-" + ("link" if scenario == "canonical" else "relay")
                assert sorted(p.name for p in target.iterdir()) == [binary_name], scenario
                assert os.access(target / binary_name, os.X_OK), scenario
                assert current.resolve() == target.resolve(), scenario
            else:
                assert sentinel.read_text() == "existing payload", scenario
                assert sorted(p.name for p in target.iterdir()) == [sentinel.name], scenario
                assert current.is_symlink() and os.readlink(current) == previous_link, scenario
                if scenario == "missing_payload":
                    assert "expected executable" in result.stderr, result.stderr[-1500:]
            print(f"PASS {scenario}")
finally:
    server.shutdown()
    server.server_close()
    worker.join(timeout=5)

print("link release compatibility selftest passed (4 cases; no payload executed)")
PY
