"""Consumer-owned client boundary used by the UI effects runtime."""

from corthena.ui.client.errors import RequestCancelledError
from corthena.ui.client.protocol import CancellationSignalProtocol, UIClientProtocol

# Compatibility aliases for the pre-contract package surface.
CancellationSignal = CancellationSignalProtocol
UIClient = UIClientProtocol

__all__ = [
    "CancellationSignal",
    "CancellationSignalProtocol",
    "RequestCancelledError",
    "UIClient",
    "UIClientProtocol",
]
