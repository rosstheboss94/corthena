from corthena.compatibility.loopback.protocol import LoopbackProbeProtocol
from corthena.compatibility.loopback.uvicorn_probe import UvicornLoopbackProbe
from corthena.compatibility.runtime.models import RuntimeCapabilities


def test_loopback_http_websocket_lifecycle() -> None:
    runtime = RuntimeCapabilities(
        python_version="3.14.2",
        python_abi="cp314-win_amd64",
        platform_tag="win_amd64",
        process_role="test",
        thread_count=1,
        process_count=1,
        task_count=0,
        cpu_lease=1,
        library_pool_limit=1,
        status="healthy",
    )
    probe: LoopbackProbeProtocol = UvicornLoopbackProbe()
    evidence = probe.run(runtime)
    assert evidence.correlation_id == "phase0-correlation"
    assert evidence.event_type == "compatibility.ready"
