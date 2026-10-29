"""Fetch a verified Fiach revision and apply the explicit CI overlay. No secrets."""

import json
from pathlib import Path
import shutil
import subprocess
import sys


def fetch(destination):
    root = Path(__file__).resolve().parent
    source = json.loads((root / "upstream.json").read_text())
    destination = Path(destination)
    if destination.exists():
        raise SystemExit("Source destination already exists; refusing to overwrite it")
    subprocess.run(["git", "clone", "--no-checkout", "--filter=blob:none",
                    source["repository"], str(destination)], check=True)
    subprocess.run(["git", "checkout", "--detach", source["revision"]], cwd=destination, check=True)
    actual = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=destination, text=True).strip()
    if actual != source["revision"]:
        raise SystemExit("Upstream revision verification failed")
    for patch in sorted((root / "patches").glob("*.patch")):
        subprocess.run(["git", "apply", "--check", str(patch)], cwd=destination, check=True)
        subprocess.run(["git", "apply", str(patch)], cwd=destination, check=True)
    binary = destination / "src/bin"
    binary.mkdir(exist_ok=True)
    shutil.copyfile(root / "export.rs", binary / "nutmix-export.rs")
    shutil.copyfile(root / "boundary_tests.rs", binary / "nutmix-boundary-tests.rs")


if __name__ == "__main__":
    fetch(sys.argv[1])
