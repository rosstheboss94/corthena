from typing import Protocol

from corthena.compatibility.storage.models import StorageEvidence


class StorageProbeProtocol(Protocol):
    def run(self) -> StorageEvidence: ...
