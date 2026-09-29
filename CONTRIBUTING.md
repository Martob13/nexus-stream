# Contributing to nexus-stream

We welcome contributions to optimize algorithms, improve test coverage, and harden concurrency pipelines.

## Development Workflow
1. Fork and clone the repository.
2. Ensure you have Go 1.22+, Rust 1.77+, and Python 3.11+ installed.
3. Run `make test` locally to verify that all suites pass.
4. Run `make bench` to ensure no throughput regressions occur in the FFI kernel.
5. Create a feature branch and open a Pull Request following the PR template.
