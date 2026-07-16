"""Typed failures raised by UI client implementations."""


class RequestCancelledError(Exception):
    """Raised when cooperative cancellation wins a client request."""


__all__ = ["RequestCancelledError"]
