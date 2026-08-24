# Workflow Selection

The lightweight tier is selected because the change moves existing configuration controls between two frontends while preserving the established `ImageInputPrice` backend contract. It is local, reversible, and introduces no billing arithmetic, persistence, security, concurrency, deployment, or public API changes.

Requirements use fast mode because the user explicitly narrowed rollback scope to the default frontend's `ImageInputPrice` UI and confirmed classic as the target frontend.
