"""Thin versioned FastAPI handlers for real Data ingestion."""

# pyright: reportUnusedFunction=false

from __future__ import annotations

import asyncio

from fastapi import (
    FastAPI,
    Header,
    HTTPException,
    Query,
    Request,
    WebSocket,
    WebSocketDisconnect,
    status,
)
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse

from corthena.contracts.data import (
    CommandDTO,
    CredentialSecretDTO,
    CredentialTestDTO,
    DeleteCommandDTO,
    ErrorDTO,
    ImportPlanDTO,
    PreviewRequestDTO,
    PullRequestDTO,
    ScheduleCommandDTO,
    SchedulePatchDTO,
    SourceKind,
)
from corthena.coordinator.data_service import CancellationSignal, DataCoordinatorService
from corthena.data.errors import DataError


def create_data_app(service: DataCoordinatorService) -> FastAPI:
    """Build the loopback Data API without exposing framework values inward."""
    app = FastAPI(title="Corthena Coordinator", version="1")

    @app.exception_handler(RequestValidationError)
    async def validation_error(request: Request, error: RequestValidationError) -> JSONResponse:
        correlation = request.headers.get("X-Correlation-ID", "request-validation")
        errors = error.errors()
        location = errors[0].get("loc", ()) if errors else ()
        field = ".".join(str(item) for item in location) or None
        body = ErrorDTO(
            code="invalid_request",
            message="Request validation failed",
            correlation_id=correlation,
            retryable=False,
            field=field,
        )
        return JSONResponse(status_code=422, content=body.model_dump(mode="json"))

    @app.exception_handler(DataError)
    async def data_error(request: Request, error: DataError) -> JSONResponse:
        correlation = request.headers.get("X-Correlation-ID", "data-operation")
        code_to_status = {
            "authentication_failed": 401,
            "entitlement_failed": 403,
            "rate_limited": 429,
            "stale_revision": 409,
            "cancelled": 409,
            "capacity_exhausted": 503,
            "validation_failed": 422,
        }
        body = ErrorDTO(
            code=error.code,
            message=str(error),
            field=error.field,
            correlation_id=correlation,
            retryable=error.retryable,
            retry_after_seconds=error.retry_after_seconds,
            provider_request_id=error.provider_request_id,
        )
        return JSONResponse(
            status_code=code_to_status.get(error.code, 500), content=body.model_dump(mode="json")
        )

    @app.get("/api/v1/data/catalog")
    def get_catalog():
        return service.catalog()

    @app.get("/api/v1/data/reconciliation")
    def reconcile_data():
        return service.reconcile()

    @app.post("/api/v1/data/files/preview")
    def preview_file(request: PreviewRequestDTO):
        return service.preview(request)

    @app.post("/api/v1/data/imports", status_code=status.HTTP_202_ACCEPTED)
    def submit_import(request: ImportPlanDTO):
        if request.source_kind is SourceKind.MASSIVE:
            raise HTTPException(status_code=422, detail="use the Massive pulls endpoint")
        return service.submit(request)

    @app.get("/api/v1/data/imports/{operation_id}")
    def get_import(operation_id: str):
        result = service.progress(operation_id)
        if result is None:
            raise HTTPException(status_code=404, detail="operation not found")
        return result

    @app.post("/api/v1/data/imports/{operation_id}/cancel")
    def cancel_import(operation_id: str, request: CommandDTO):
        del request
        try:
            return service.cancel(operation_id)
        except KeyError as error:
            raise HTTPException(status_code=404, detail="operation not found") from error

    @app.get("/api/v1/data/providers/massive/symbols")
    def discover_symbols(
        query: str = Query(default="", max_length=256),
        limit: int = Query(default=20, ge=1, le=1000),
        cursor: str | None = Query(default=None, max_length=4096),
    ):
        return service.discover_symbols(query, limit, cursor, CancellationSignal())

    @app.post("/api/v1/data/providers/massive/pulls", status_code=status.HTTP_202_ACCEPTED)
    def submit_pull(request: PullRequestDTO):
        return service.submit(request)

    @app.get("/api/v1/data/schedules")
    def get_schedules():
        return service.schedules()

    @app.post("/api/v1/data/schedules", status_code=status.HTTP_201_CREATED)
    def create_schedule(request: ScheduleCommandDTO):
        if request.expected_revision != 0:
            raise HTTPException(status_code=409, detail="new schedule expects revision zero")
        return service.create_schedule(request.schedule, request.command_id)

    @app.get("/api/v1/data/schedules/{schedule_id}")
    def get_schedule(schedule_id: str):
        result = next(
            (item for item in service.schedules() if item.schedule_id == schedule_id), None
        )
        if result is None:
            raise HTTPException(status_code=404, detail="schedule not found")
        return result

    @app.patch("/api/v1/data/schedules/{schedule_id}")
    def patch_schedule(schedule_id: str, request: SchedulePatchDTO):
        if schedule_id != request.schedule.schedule_id:
            raise HTTPException(status_code=422, detail="schedule identity mismatch")
        return service.update_schedule(
            request.schedule, request.expected_revision, request.command_id
        )

    @app.delete("/api/v1/data/schedules/{schedule_id}", status_code=status.HTTP_204_NO_CONTENT)
    def delete_schedule(schedule_id: str, request: DeleteCommandDTO):
        service.delete_schedule(schedule_id, request.expected_revision, request.command_id)

    @app.post("/api/v1/data/schedules/{schedule_id}/run", status_code=status.HTTP_202_ACCEPTED)
    def run_schedule(schedule_id: str, request: CommandDTO):
        try:
            return service.run_schedule(schedule_id, request.command_id, request.correlation_id)
        except KeyError as error:
            raise HTTPException(status_code=404, detail="schedule not found") from error

    @app.get("/api/v1/settings/api-tokens/massive")
    def credential_status():
        return service.credential_status()

    @app.put("/api/v1/settings/api-tokens/massive")
    def save_credential(request: CredentialSecretDTO):
        return service.save_credential(request.token.get_secret_value())

    @app.post("/api/v1/settings/api-tokens/massive/test")
    def test_credential(request: CredentialTestDTO):
        token = None if request.token is None else request.token.get_secret_value()
        return service.test_credential(token, CancellationSignal())

    @app.delete("/api/v1/settings/api-tokens/massive")
    def delete_credential(request: CommandDTO):
        del request
        return service.delete_credential()

    @app.get("/api/v1/health")
    def health(x_correlation_id: str | None = Header(default=None)):
        return {
            "api_version": 1,
            "status": "healthy",
            "correlation_id": x_correlation_id,
            "capabilities": ("data", "credentials", "schedules"),
        }

    @app.websocket("/api/v1/events")
    async def events(websocket: WebSocket):
        await websocket.accept()
        sequence = 0
        try:
            while True:
                ready = await asyncio.to_thread(service.events.after, sequence, 1.0)
                for event in ready:
                    await websocket.send_text(event.model_dump_json())
                    sequence = event.sequence
        except WebSocketDisconnect:
            return

    return app
