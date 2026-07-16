from corthena.compatibility.storage.protocol import StorageProbeProtocol
from corthena.compatibility.storage.windows import WindowsStorageProbe


def test_storage_and_published_matrix() -> None:
    probe: StorageProbeProtocol = WindowsStorageProbe()
    evidence = probe.run()
    assert evidence.rows == 1
    assert evidence.arrow_bytes > 0
    assert evidence.matrix_sum == 10.0
