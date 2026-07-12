"""Phase 0 compatibility-gate entry point."""

from __future__ import annotations

import json
import os
import platform
import sys
import tempfile
from dataclasses import asdict
from pathlib import Path

from corthena.compatibility.runtime import (
    audit_native_imports,
    probe_runtime,
)


def main() -> None:
    """Run the evidence-producing Phase 0 compatibility gate."""
    cpu_lease = min(max(os.cpu_count() or 1, 1), 4)
    runtime = probe_runtime("phase0-gate", cpu_lease=cpu_lease, library_pool_limit=cpu_lease)
    imports = audit_native_imports()

    # Delay native imports until after the runtime and incremental GIL audit.
    from corthena.compatibility.loopback import run_loopback_probe
    from corthena.compatibility.storage import run_storage_probe
    from corthena.compatibility.ui import capture_hidden_frame
    from corthena.cython_ext import add_checked, native_available

    if not native_available() or add_checked(2, 3) != 5:
        raise RuntimeError("the Cython extension did not build and import")
    with tempfile.TemporaryDirectory(prefix="corthena-phase0-ui-") as directory:
        ui = capture_hidden_frame(Path(directory) / "hidden-frame.png")
        evidence: dict[str, object] = {
            "runtime": runtime.health_payload(),
            "python": platform.python_version(),
            "implementation": platform.python_implementation(),
            "windows": platform.platform(),
            "architecture": platform.machine(),
            "native_imports": [asdict(item) for item in imports],
            "cython_extension": True,
            "ui": {
                "owner_thread": ui.owner_thread,
                "asset_sha256": ui.asset_sha256,
                "capture": str(ui.capture),
            },
        }
        loopback = run_loopback_probe(runtime)
        evidence["loopback"] = {
            "correlation_id": loopback.correlation_id,
            "event_type": loopback.event_type,
        }
        storage = run_storage_probe()
        evidence["storage"] = {
            "rows": storage.rows,
            "arrow_bytes": storage.arrow_bytes,
            "matrix_sum": storage.matrix_sum,
        }
        json.dump(evidence, sys.stdout, indent=2)
        sys.stdout.write("\n")


if __name__ == "__main__":
    main()
