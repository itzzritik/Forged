# Forged — Development Guidelines

## Code Standards

- Enterprise-grade, minimal, clean code. No bloat.
- Modular architecture — each package has a single responsibility.
- No redundant code. If something exists once, don't duplicate it.
- No unnecessary comments. Code should be self-documenting. Only comment *why*, never *what*.
- No placeholder or stub comments (`// TODO`, `// FIXME`) unless tracking a specific, planned follow-up.
- Use the latest stable versions of all dependencies. Check release pages and documentation before adding any dependency.

## Before Implementing

- Always look up current documentation for any library, framework, or protocol before writing code. Training data may be outdated — verify API signatures, function names, and best practices against the latest docs.
- Read the `ARCHITECTURE.md` for system design decisions, component contracts, and phase/batch breakdown.

## Go Conventions

- Follow standard Go project layout (`cmd/`, `internal/`).
- Use `golangci-lint` for linting.
- Prefer stdlib over third-party when the stdlib solution is adequate.
- Error handling: return errors, don't panic. Wrap errors with context using `fmt.Errorf("doing x: %w", err)`.
- No global mutable state. Pass dependencies explicitly.

## Git

- Atomic, well-scoped commits. One logical change per commit.
- Commit messages: imperative mood, concise subject line.
