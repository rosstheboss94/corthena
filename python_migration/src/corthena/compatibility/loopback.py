"""Owned loopback HTTP and WebSocket compatibility probe."""

# pyright: reportUnusedFunction=false

from __future__ import annotations

import asyncio
import socket
import threading
from dataclasses import dataclass

import httpx
import uvicorn
import websockets
from fastapi import FastAPI, WebSocket

from corthena.compatibility.runtime import RuntimeCapabilities


@dataclass(frozen=True, slots=True)
class LoopbackEvidence:
    """Observed typed handshake outcome."""

    correlation_id: str
    event_type: str


def create_app(runtime: RuntimeCapabilities) -> FastAPI:
    """Create the isolated Phase 0 loopback probe application."""
    app = FastAPI()

    @app.get("/api/v1/health")
    async def _health() -> dict[str, object]:
        return {"api_version": "v1", **runtime.health_payload()}

    @app.websocket("/api/v1/events")
    async def _events(websocket: WebSocket) -> None:
        await websocket.accept()
        correlation_id = await websocket.receive_text()
        await websocket.send_json(
            {
                "event_id": "phase0-1",
                "event_type": "compatibility.ready",
                "schema_version": "1",
                "correlation_id": correlation_id,
            }
        )
        await websocket.close()

    return app


async def _handshake(port: int, runtime: RuntimeCapabilities) -> LoopbackEvidence:
    correlation_id = "phase0-correlation"
    async with httpx.AsyncClient(timeout=httpx.Timeout(3.0)) as client:
        response = await client.get(
            f"http://127.0.0.1:{port}/api/v1/health",
            headers={"X-Correlation-ID": correlation_id},
        )
        response.raise_for_status()
        if response.json() != {"api_version": "v1", **runtime.health_payload()}:
            raise RuntimeError("unexpected health response")
    async with websockets.connect(
        f"ws://127.0.0.1:{port}/api/v1/events",
        open_timeout=3.0,
        close_timeout=3.0,
    ) as websocket:
        await websocket.send(correlation_id)
        event = await asyncio.wait_for(websocket.recv(decode=True), timeout=3.0)
        if correlation_id not in event or "compatibility.ready" not in event:
            raise RuntimeError("unexpected event envelope")
    return LoopbackEvidence(correlation_id, "compatibility.ready")


def run_loopback_probe(runtime: RuntimeCapabilities) -> LoopbackEvidence:
    """Start, probe, and cleanly stop an owned loopback server thread."""
    listener = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    listener.bind(("127.0.0.1", 0))
    listener.listen(8)
    port = listener.getsockname()[1]
    config = uvicorn.Config(create_app(runtime), log_level="warning", lifespan="off")
    server = uvicorn.Server(config)
    thread = threading.Thread(
        target=server.run,
        kwargs={"sockets": [listener]},
        name="phase0-loopback",
        daemon=False,
    )
    thread.start()
    try:
        for _ in range(100):
            if server.started:
                break
            threading.Event().wait(0.01)
        if not server.started:
            raise TimeoutError("loopback server did not start")
        return asyncio.run(asyncio.wait_for(_handshake(port, runtime), timeout=10.0))
    finally:
        server.should_exit = True
        thread.join(timeout=5.0)
        listener.close()
        if thread.is_alive():
            raise RuntimeError("loopback server leaked its owner thread")
