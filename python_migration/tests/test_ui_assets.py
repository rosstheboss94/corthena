from typing import Never

import pytest

from corthena.ui import assets


def test_packaged_assets_and_notices_validate() -> None:
    with assets.AssetLease() as bundle:
        assert bundle.inter_font.is_file()
        assert bundle.mono_font.is_file()
        assert bundle.icon_atlas.is_file()
        assert all(len(digest) == 64 for digest in bundle.sha256)


def test_invalid_asset_fails_before_path_lease(monkeypatch: pytest.MonkeyPatch) -> None:
    def invalid_content(resource: assets.ReadableResource) -> bytes:
        del resource
        return b"invalid"

    monkeypatch.setattr(assets, "_read_resource_bytes", invalid_content)
    with (
        pytest.raises(assets.AssetValidationError, match=r"InterVariable\.ttf"),
        assets.AssetLease(),
    ):
        pytest.fail("invalid assets must not yield")


def test_invalid_asset_fails_before_native_factory(monkeypatch: pytest.MonkeyPatch) -> None:
    from corthena.ui.lifecycle import LaunchConfig, launch

    def invalid_content(resource: assets.ReadableResource) -> bytes:
        del resource
        return b"invalid"

    native_factory_called = False

    def forbidden_factory() -> Never:
        nonlocal native_factory_called
        native_factory_called = True
        raise AssertionError("native factory must not be called")

    monkeypatch.setattr(assets, "_read_resource_bytes", invalid_content)
    with pytest.raises(assets.AssetValidationError):
        launch(LaunchConfig(hidden=True, max_frames=1), adapter_factory=forbidden_factory)
    assert not native_factory_called
