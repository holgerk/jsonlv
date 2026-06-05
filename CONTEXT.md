# jsonlv

A macOS desktop app for viewing and filtering structured JSON log streams in real time.

## Language

**Log Entry**:
A single line from a log stream, rendered as one row in the viewer.
_Avoid_: record, event, line

**Source**:
The basename of the file a Log Entry came from. Empty when the entry arrives via stdin.
_Avoid_: filename, origin

**Column**:
A fixed-width display field in a Log Entry row. Built-in columns are timestamp, level badge, and source; Custom Columns are user-defined.
_Avoid_: field, cell

**Custom Column**:
A Column added by the user, bound to a specific JSON key from the Log Entry.
_Avoid_: user column, extra column

**Column Width**:
The explicit pixel width a user has assigned to a Column by dragging. Persisted across sessions. When absent, the Column falls back to its default em-based width.
_Avoid_: size, preferred width

**Drag Handle**:
The narrow resize control at the right edge of a Column header cell. Dragging it sets the Column Width; double-clicking resets to the default.
_Avoid_: resize grip, splitter

**Broker**:
The in-memory hub that buffers Log Entries and fans them out to all active subscribers.
_Avoid_: dispatcher, bus

## Relationships

- A **Log Entry** belongs to at most one **Source**
- A **Custom Column** is a subtype of **Column**
- A **Column** may have a **Column Width** (explicit) or use its default em width (implicit)
- A **Drag Handle** belongs to exactly one **Column**

## Example dialogue

> **Dev:** "The user dragged the timestamp Column narrower — do we save that immediately?"
> **Domain expert:** "Yes, save the Column Width on drag end via POST /set-col-width. On next launch, the prefs load restores it before the first Log Entry arrives."
