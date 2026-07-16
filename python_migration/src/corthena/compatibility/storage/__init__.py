"""Windows storage compatibility capability."""

from corthena.compatibility.storage.models import StorageEvidence
from corthena.compatibility.storage.protocol import StorageProbeProtocol
from corthena.compatibility.storage.windows import WindowsStorageProbe


def run_storage_probe() -> StorageEvidence:
    """Run the default Windows storage compatibility probe."""
    return WindowsStorageProbe().run()


__all__ = ("StorageEvidence", "StorageProbeProtocol", "WindowsStorageProbe", "run_storage_probe")
