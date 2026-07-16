"""Phase 0 compatibility-gate entry point."""

from __future__ import annotations

import json
import os
import platform
import sys
import tempfile
from dataclasses import asdict
from pathlib import Path

from corthena.compatibility.runtime.protocol import (
    NativeImportProbeProtocol,
    RuntimeProbeProtocol,
)
from corthena.compatibility.runtime.regular_cpython import RegularCpythonRuntimeProbe


def main() -> None:
    """Run the evidence-producing Phase 0 compatibility gate."""
    cpu_lease = min(max(os.cpu_count() or 1, 1), 4)
    concrete_runtime_probe = RegularCpythonRuntimeProbe()
    runtime_probe: RuntimeProbeProtocol = concrete_runtime_probe
    native_import_probe: NativeImportProbeProtocol = concrete_runtime_probe
    runtime = runtime_probe.probe(
        "phase0-gate",
        cpu_lease=cpu_lease,
        library_pool_limit=cpu_lease,
    )
    imports = native_import_probe.audit_native_imports()

    # Delay native imports until after the runtime and incremental GIL audit.
    from corthena.compatibility.assets.legacy import LegacyAssetStager
    from corthena.compatibility.loopback.protocol import LoopbackProbeProtocol
    from corthena.compatibility.loopback.uvicorn_probe import UvicornLoopbackProbe
    from corthena.compatibility.storage.protocol import StorageProbeProtocol
    from corthena.compatibility.storage.windows import WindowsStorageProbe
    from corthena.compatibility.ui.protocol import UiProbeProtocol
    from corthena.compatibility.ui.raylib_probe import RaylibUiProbe
    from corthena.cython_ext import add_checked, native_available

    loopback_probe: LoopbackProbeProtocol = UvicornLoopbackProbe()
    storage_probe: StorageProbeProtocol = WindowsStorageProbe()
    ui_probe: UiProbeProtocol = RaylibUiProbe(LegacyAssetStager())

    if not native_available() or add_checked(2, 3) != 5:
        raise RuntimeError("the Cython extension did not build and import")
    with tempfile.TemporaryDirectory(prefix="corthena-phase0-ui-") as directory:
        ui = ui_probe.capture(Path(directory) / "hidden-frame.png")
        evidence: dict[str, object] = {
            "runtime": asdict(runtime),
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
        loopback = loopback_probe.run(runtime)
        evidence["loopback"] = {
            "correlation_id": loopback.correlation_id,
            "event_type": loopback.event_type,
        }
        storage = storage_probe.run()
        evidence["storage"] = {
            "rows": storage.rows,
            "arrow_bytes": storage.arrow_bytes,
            "matrix_sum": storage.matrix_sum,
        }
        json.dump(evidence, sys.stdout, indent=2)
        sys.stdout.write("\n")


if __name__ == "__main__":
    main()
