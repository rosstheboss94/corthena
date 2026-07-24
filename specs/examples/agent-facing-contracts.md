# Agent-Facing Contract Examples

**Status:** Non-authoritative examples
**Owner:** Architecture
**Last updated:** 2026-07-16
**Related:** [Agent-facing contract specification](../general/contract.md)

This document illustrates the rules in `contract.md`. It is not required
context for ordinary tasks and does not add normative requirements.

The examples keep contracts, implementations, and usage together for
demonstration only. Production code separates them as required by
`contract.md`.

## Stub layout

```text
text_analyzer/
|-- protocol.py
|-- text_length.py
`-- text_length.pyi
```

Example `text_length.pyi`:

```python
from typing import ClassVar


class TextLengthAnalyzer:
    analyzer_type: ClassVar[str]

    def __init__(self, name: str, language: str = "en") -> None: ...

    @property
    def name(self) -> str: ...

    @name.setter
    def name(self, value: str) -> None: ...

    @property
    def language(self) -> str: ...

    @language.setter
    def language(self, value: str) -> None: ...

    @property
    def is_ready(self) -> bool: ...

    @is_ready.setter
    def is_ready(self, value: bool) -> None: ...

    def analyze(self, text: str) -> int: ...
```

## Package organization

```text
src/
`-- anime_recommender/
    |-- __init__.py
    |-- text_analyzer/
    |   |-- __init__.py
    |   |-- protocol.py       # TextAnalyzerProtocol
    |   |-- text_length.py    # TextLengthAnalyzer
    |   |-- text_length.pyi   # Optional concrete public interface
    |   |-- _tokenizer.py     # Internal; hidden from agent context
    |   `-- _normalizer.py    # Internal; hidden from agent context
    `-- vector_store/
        |-- __init__.py
        |-- protocol.py       # VectorStoreProtocol
        |-- chroma.py         # ChromaVectorStore
        |-- chroma.pyi        # Optional concrete public interface
        |-- pinecone.py       # PineconeVectorStore
        |-- pinecone.pyi      # Optional concrete public interface
        |-- in_memory.py      # InMemoryVectorStore
        `-- in_memory.pyi     # Optional concrete public interface
```

Example imports:

```python
from anime_recommender.text_analyzer.protocol import TextAnalyzerProtocol
from anime_recommender.text_analyzer.text_length import TextLengthAnalyzer
```

## Generic contract

Use a generic protocol when implementations share behavior but operate on
different request and result types. Request types are contravariant and result
types are covariant so substitution follows the direction values flow.

```python
from collections.abc import Callable
from typing import ClassVar, Generic, Protocol, TypeVar


RequestT_contra = TypeVar("RequestT_contra", contravariant=True)
ResultT_co = TypeVar("ResultT_co", covariant=True)
RequestT = TypeVar("RequestT")
ResultT = TypeVar("ResultT")


class ComponentProtocol(Protocol[RequestT_contra, ResultT_co]):
    """Accept a request and produce a result."""

    component_type: ClassVar[str]

    @property
    def name(self) -> str: ...

    @name.setter
    def name(self, value: str) -> None: ...

    @property
    def is_ready(self) -> bool: ...

    @is_ready.setter
    def is_ready(self, value: bool) -> None: ...

    def execute(self, request: RequestT_contra) -> ResultT_co: ...


class FunctionComponent(Generic[RequestT, ResultT]):
    """Adapt a callable to the example contract."""

    component_type: ClassVar[str] = "function"

    def __init__(self, name: str, handler: Callable[[RequestT], ResultT]) -> None:
        self._name = name
        self._is_ready = True
        self._handler = handler

    @property
    def name(self) -> str:
        return self._name

    @name.setter
    def name(self, value: str) -> None:
        self._name = value

    @property
    def is_ready(self) -> bool:
        return self._is_ready

    @is_ready.setter
    def is_ready(self, value: bool) -> None:
        self._is_ready = value

    def execute(self, request: RequestT) -> ResultT:
        return self._handler(request)


component: ComponentProtocol[str, int] = FunctionComponent(
    name="text length",
    handler=len,
)
result = component.execute("anime")
```

## Non-generic contract

Use fixed types when a contract has one clear input and output shape.

```python
from typing import ClassVar, Protocol


class TextAnalyzerProtocol(Protocol):
    """Accept text and return an integer."""

    analyzer_type: ClassVar[str]

    @property
    def name(self) -> str: ...

    @name.setter
    def name(self, value: str) -> None: ...

    @property
    def language(self) -> str: ...

    @language.setter
    def language(self, value: str) -> None: ...

    @property
    def is_ready(self) -> bool: ...

    @is_ready.setter
    def is_ready(self, value: bool) -> None: ...

    def analyze(self, text: str) -> int: ...


class TextLengthAnalyzer:
    analyzer_type: ClassVar[str] = "text-length"

    def __init__(self, name: str, language: str = "en") -> None:
        self._name = name
        self._language = language
        self._is_ready = True

    @property
    def name(self) -> str:
        return self._name

    @name.setter
    def name(self, value: str) -> None:
        self._name = value

    @property
    def language(self) -> str:
        return self._language

    @language.setter
    def language(self, value: str) -> None:
        self._language = value

    @property
    def is_ready(self) -> bool:
        return self._is_ready

    @is_ready.setter
    def is_ready(self, value: bool) -> None:
        self._is_ready = value

    def analyze(self, text: str) -> int:
        return len(text)


text_analyzer: TextAnalyzerProtocol = TextLengthAnalyzer(name="anime title length")
text_result = text_analyzer.analyze("Naruto")
```
