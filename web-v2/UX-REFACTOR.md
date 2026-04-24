# web-v2 UX refactor notes

## Current direction

The user-facing console should move away from a dense operator cockpit and toward a simpler product console.

Reference direction:

- simpler navigation
- fewer cards and fewer visual layers
- fewer table columns on first view
- stronger default focus on the single task of the page
- easier scanning for non-technical customers

## What changed in this pass

### App shell

- removed the noisy top search bar
- removed the extra status and profile cards from the sidebar
- reduced the sidebar to a lean navigation rail
- simplified header actions to a language switch only

### Usage page

- reduced the summary area to 3 core numbers
- reduced filters to token selector and time range
- removed low-signal columns from the default table
- kept model plus endpoint together in one compact column
- kept pagination simple and obvious

## Next simplification passes

1. Apply the same simplification rules to `Dashboard`, `Tokens`, and `Channels`
2. Split user console and admin console visual density
3. Hide advanced operational fields behind secondary drill-down panels
4. Replace decorative hero blocks with quieter summary sections
5. Add a compact mobile-first top navigation for customer pages

## Rule of thumb

If a first-time customer cannot understand the page in 3 seconds, the page still has too much on it.
