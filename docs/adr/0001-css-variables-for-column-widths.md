# CSS custom properties for column resize

Column Widths are stored as CSS custom properties on `:root` (e.g. `--col-ts`, `--col-c-service`) and updated with a single `setProperty` call per `requestAnimationFrame` tick during drag. The alternative — setting `width` inline on every cell — would be O(50 000) DOM writes per frame since all Log Entries accumulate in the DOM without virtualisation, making drag unusable at scale.

## Considered options

- **Inline style per cell**: obvious, but O(n) DOM writes per frame. Rejected.
- **CSS custom properties on `:root`**: O(1) write; the browser propagates the variable to all cells. Chosen.
