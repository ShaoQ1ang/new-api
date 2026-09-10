# Chat Model Display Price Design

## Goal

Make the chat-model catalog `price` a weighted per-million-token estimate only. Relay billing remains unchanged.

## Rule

For ratio-priced chat models, calculate:

`round6(group_ratio * (input * 0.01 + output * 0.05 + cache_read * 0.94) / 2.98)`

`2.98` is the display baseline price, so a calculated price of `2.98` returns `1x`. `input` is `ModelRatio`, `output` is `ModelRatio * CompletionRatio`, and `cache_read` is `ModelRatio * CacheRatio`. A missing cache ratio uses the input price.

Fixed-price models retain their existing display calculation. For `tiered_expr` models, read `tier("base", ...)` and evaluate its input, output, and cache-read unit prices. A model without a base tier is unavailable in the chat catalog rather than receiving a guessed price.

## Scope

Only `controller/chat_model.go` catalog responses change. No relay, quota, settlement, or stored pricing configuration changes.
