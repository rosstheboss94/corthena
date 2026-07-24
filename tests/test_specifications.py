"""Structural checks for the living specification taxonomy."""

from __future__ import annotations

import re
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
SPECS = ROOT / "specs"
LINK_RE = re.compile(r"\[[^]]*\]\(([^)]+)\)")
METADATA = ("**Status:**", "**Owner:**", "**Last updated:**")


def _markdown_links(path: Path) -> list[str]:
    links: list[str] = []
    for raw_target in LINK_RE.findall(path.read_text(encoding="utf-8")):
        target = raw_target.strip().split(maxsplit=1)[0].strip("<>")
        if target.startswith(("http://", "https://", "mailto:")):
            continue
        links.append(target)
    return links


def _resolve_link(source: Path, target: str) -> tuple[Path, str | None]:
    path_text, separator, fragment = target.partition("#")
    resolved = (source.parent / path_text).resolve() if path_text else source.resolve()
    return resolved, fragment if separator else None


def _anchor(text: str) -> str:
    return re.sub(r"[^a-z0-9 -]", "", text.lower()).replace(" ", "-")


def test_expected_taxonomy_exists() -> None:
    expected_directories = (
        SPECS / "general",
        SPECS / "general" / "quality",
        SPECS / "general" / "ui",
        SPECS / "pages",
        SPECS / "history",
        SPECS / "history" / "migration",
        SPECS / "history" / "routing",
        *(
            SPECS / "pages" / page
            for page in (
                "data",
                "research",
                "experiments",
                "jobs",
                "results",
                "models",
                "inference",
            )
        ),
    )
    for directory in expected_directories:
        assert directory.is_dir(), directory

    for retired in (SPECS / "ui", SPECS / "routing", SPECS / "frontend"):
        assert not retired.exists() or not any(retired.iterdir()), retired


def test_authoritative_documents_have_metadata() -> None:
    documents = list((SPECS / "general").rglob("*.md")) + list((SPECS / "pages").rglob("*.md"))
    assert documents
    for document in documents:
        text = document.read_text(encoding="utf-8")
        for field in METADATA:
            assert field in text, f"{field} missing from {document}"
        assert "**Status:** Authoritative" in text


@pytest.mark.parametrize(
    ("directory", "index"),
    (
        (SPECS / "general", SPECS / "general" / "README.md"),
        (SPECS / "general" / "quality", SPECS / "general" / "quality" / "README.md"),
        (SPECS / "general" / "ui", SPECS / "general" / "ui" / "README.md"),
        (SPECS / "pages", SPECS / "pages" / "README.md"),
        *(
            (SPECS / "pages" / page, SPECS / "pages" / page / "README.md")
            for page in (
                "data",
                "research",
                "experiments",
                "jobs",
                "results",
                "models",
                "inference",
            )
        ),
    ),
)
def test_indexes_cover_direct_children(directory: Path, index: Path) -> None:
    assert index.is_file()
    links = [_resolve_link(index, target)[0] for target in _markdown_links(index)]
    children = [child for child in directory.glob("*.md") if child.name != "README.md"]
    child_directories = [
        child / "README.md"
        for child in directory.iterdir()
        if child.is_dir() and (child / "README.md").is_file()
    ]
    for child in [*children, *child_directories]:
        assert links.count(child.resolve()) == 1, f"{child} is not indexed exactly once"


def test_local_markdown_links_resolve() -> None:
    documents = [*SPECS.rglob("*.md"), ROOT / "AGENTS.md", ROOT / "README.md"]
    for document in documents:
        for target in _markdown_links(document):
            resolved, fragment = _resolve_link(document, target)
            assert resolved.exists(), f"{document}: {target}"
            if fragment and resolved.suffix.lower() == ".md":
                headings = {
                    _anchor(match.group(1))
                    for match in re.finditer(
                        r"^#{1,6}\s+(.+?)\s*$", resolved.read_text(encoding="utf-8"), re.MULTILINE
                    )
                }
                assert fragment.lower() in headings, f"{document}: {target}"


def test_literal_spec_paths_resolve() -> None:
    files = [
        ROOT / "AGENTS.md",
        ROOT / "README.md",
        *Path(ROOT / ".agents").rglob("*.md"),
        *Path(ROOT / "scripts").rglob("*.go"),
        *SPECS.rglob("*.md"),
    ]
    literal = re.compile(r"(?<![A-Za-z0-9_.-])(specs/[A-Za-z0-9_./*-]+)")
    for document in files:
        for value in literal.findall(document.read_text(encoding="utf-8")):
            if "*" in value:
                continue
            assert (ROOT / value).exists(), f"{document}: {value}"
