# Agent-Facing Contract Specification

**Status:** Authoritative  
**Owner:** Architecture  
**Last updated:** 2026-07-13  
**Related:** [Repository guidance](../AGENTS.md), [Specification index](README.md), [Design pattern](design-pattern.md), [API](api.md), [Quality](quality.md)

This document is the mandatory baseline for every task in the Python/Cython
migration. Agents must read it before planning, editing, reviewing, testing, or
selecting additional task context. It defines how agents write and organize
Python contracts and how they choose the minimum implementation context needed
for a task.

The examples keep contracts, implementations, and usage together for
demonstration only.

In production code, split these concerns as follows:

- Protocol classes go in `<capability>/protocol.py`.
- Concrete classes go in implementation-specific modules.
- Static conformance checks and usage examples go in tests or examples.

Production `protocol.py` and `.pyi` files contain only the imports, types,
properties, and method signatures required by their consumers. Do not include
tutorial comments, implementation details, or usage examples. Add a concise
behavioral docstring only when the signature cannot express an important
guarantee, constraint, side effect, or error.

This specification contains generic and non-generic templates for small,
agent-facing contracts.

## Contract recommendations

- Name contracts by capability with a `Protocol` suffix, for example
  `VectorStoreProtocol` or `EmbeddingProviderProtocol`.
- Name implementations by their concrete behavior or technology, for example
  `ChromaVectorStore` or `TextLengthAnalyzer`.
- Organize packages by capability, not by individual class.
- Name each contract module `protocol.py` and each implementation module after
  its concrete implementation.
- Add `__init__.py` to every importable package directory.
- Keep implementation imports and implementation details out of contracts.
- Keep only the properties and methods that a consumer actually needs.
- Expose public instance state through properties. Shared class metadata
  remains a `ClassVar`.

## Agent context policy

- Every behavior-based class or object that an agent uses directly, or that is
  passed across a package boundary, must have a compact agent-facing contract.
- Give an agent only that contract and the request/result models it references;
  do not load concrete implementations into the agent's context.
- Internal helper classes stay hidden behind the public contract. They need
  their own contracts only when an agent or another package uses them directly.
- Use `Protocol` when one behavioral contract may have interchangeable
  implementations.
- Use a `.pyi` stub when an agent needs a signature-only view of one specific
  concrete class rather than an interchangeable behavioral interface.
- Use dataclasses, `TypedDict`, or Pydantic models for data-only contracts
  instead of creating behavioral protocols for them.

## Agent task-to-context rules

- To call an injected capability, give the agent `protocol.py` and any data
  models referenced by that protocol.
- To implement a new adapter, give the agent `protocol.py` plus only the
  implementation-specific requirements it needs.
- To construct or directly use a specific implementation, give the agent that
  implementation's `.pyi` stub and its referenced data models.
- To modify implementation internals, give the agent the relevant `.py` file
  and only the internal dependencies needed for the change.
- Default to `protocol.py`. Add a `.pyi` file only when concrete public API
  details, such as a constructor, are required.
- Pyright uses protocols to check structural compatibility and `.pyi` files to
  type-check callers against a concrete module's public interface.

## Stub rules

- A stub has the same base name and package location as its implementation:
  `text_length.py` is described by `text_length.pyi`.
- A stub contains public imports, types, properties, constructors, and method
  signatures with `...` in place of implementation bodies.
- A stub is used by type checkers and agent context; Python does not execute it
  as the runtime implementation.
- When both files exist, Pyright uses the `.pyi` public interface when
  type-checking callers of that module.
- Update the stub whenever the implementation's public API changes. A stale
  stub can make callers appear correct even when the runtime API differs.
- Do not add a stub by default. Add one when an agent or external consumer needs
  the public API of a specific concrete implementation.

### Example stub layout

```text
text_analyzer/
|-- protocol.py
|-- text_length.py
`-- text_length.pyi
```

### Example `text_length.pyi`

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

## Package organization example

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

## Generic contract example

Use a generic protocol when implementations share behavior but operate on
different request and result types. Declare request types contravariant and
result types covariant so structural substitution follows the direction in
which values flow through the contract.

```python
from collections.abc import Callable
from typing import ClassVar, Generic, Protocol, TypeVar


RequestT_contra = TypeVar("RequestT_contra", contravariant=True)
ResultT_co = TypeVar("ResultT_co", covariant=True)
RequestT = TypeVar("RequestT")
ResultT = TypeVar("ResultT")


class ComponentProtocol(Protocol[RequestT_contra, ResultT_co]):
    """Generic contract: accept a request and produce a result."""

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
    """Example implementation that adapts any callable to the contract."""

    component_type: ClassVar[str] = "function"

    def __init__(
        self,
        name: str,
        handler: Callable[[RequestT], ResultT],
    ) -> None:
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


# Static contract check and example usage.
component: ComponentProtocol[str, int] = FunctionComponent(
    name="text length",
    handler=len,
)

result = component.execute("anime")
print("Generic result:", result)
```

## Non-generic contract example

Use fixed types when a contract has one clear input and output shape.

```python
from typing import ClassVar, Protocol


class TextAnalyzerProtocol(Protocol):
    """Contract that always accepts text and returns an integer."""

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
    """Concrete implementation of the non-generic contract."""

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


# Static contract check and example usage.
text_analyzer: TextAnalyzerProtocol = TextLengthAnalyzer(
    name="anime title length"
)
text_analyzer.is_ready = True
text_result = text_analyzer.analyze("Naruto")
print("Non-generic result:", text_result)
```
