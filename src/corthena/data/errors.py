"""Typed failures shared by Data application services and adapters."""

from __future__ import annotations


class DataError(RuntimeError):
    code = "data_error"
    retryable = False

    def __init__(
        self,
        message: str,
        *,
        field: str | None = None,
        retry_after_seconds: float | None = None,
        provider_request_id: str | None = None,
    ) -> None:
        super().__init__(message)
        self.field = field
        self.retry_after_seconds = retry_after_seconds
        self.provider_request_id = provider_request_id


class ValidationDataError(DataError):
    code = "validation_failed"


class StaleRevisionError(DataError):
    code = "stale_revision"


class CancelledDataError(DataError):
    code = "cancelled"


class AuthenticationDataError(DataError):
    code = "authentication_failed"


class EntitlementDataError(DataError):
    code = "entitlement_failed"


class RateLimitDataError(DataError):
    code = "rate_limited"
    retryable = True


class ProviderDataError(DataError):
    code = "provider_failed"
    retryable = True


class CapacityDataError(DataError):
    code = "capacity_exhausted"
    retryable = True


class CredentialDataError(DataError):
    code = "credential_failed"


class PublicationDataError(DataError):
    code = "publication_failed"
