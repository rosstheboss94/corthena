# Visualization and Golden Verification

Verify docking, input replay, layout migration, chart transforms, large-table
virtualization, and linked views. Capture named scenario golden UI images at
1280x720 and 1920x1080 with 100%, 150%, and 200% scaling where feasible.
Store metadata beside each image with phase, scenario, viewport, scale factor,
seed, scenario clock, dataset fixture, layout name, serialized app state,
asset versions, rendering backend, dependency versions, and build revision.

Maintain smooth 60 FPS during normal chart interaction; chart work is
proportional to viewport width after LOD and large tables are virtualized. Keep
all filesystem, decoding, database, and network work off the render thread.
Run an out-of-CI benchmark near ten million rows recording throughput,
allocations, Python/native memory, and peak memory without hardware-specific
pass times; benchmark hot numerical kernels with representative missingness and
row counts.


**Status:** Authoritative
**Owner:** Engineering
**Last updated:** 2026-07-23
**Related:** [Quality index](README.md), [General index](../README.md)
