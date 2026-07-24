"""UI-thread-owned transient secret entry with explicit clearing."""

from __future__ import annotations

import threading


class SecretEntryBuffer:
    """Hold transient token characters outside immutable application state."""

    def __init__(self, capacity: int = 512) -> None:
        if capacity < 1:
            raise ValueError("secret buffer capacity must be positive")
        self._owner_thread_id = threading.get_native_id()
        self._capacity = capacity
        self._characters: list[str] = []

    @property
    def length(self) -> int:
        self._assert_owner()
        return len(self._characters)

    def append(self, character: str) -> None:
        self._assert_owner()
        if len(character) != 1 or not character.isprintable():
            raise ValueError("secret buffer accepts one printable character")
        if len(self._characters) >= self._capacity:
            raise ValueError("secret buffer capacity exceeded")
        self._characters.append(character)

    def backspace(self) -> None:
        self._assert_owner()
        if self._characters:
            self._characters.pop()

    def take(self) -> str:
        self._assert_owner()
        value = "".join(self._characters)
        self.clear()
        return value

    def clear(self) -> None:
        self._assert_owner()
        for index in range(len(self._characters)):
            self._characters[index] = "\0"
        self._characters.clear()

    def _assert_owner(self) -> None:
        if threading.get_native_id() != self._owner_thread_id:
            raise RuntimeError("secret entry buffer used from a non-owner OS thread")


__all__ = ["SecretEntryBuffer"]
