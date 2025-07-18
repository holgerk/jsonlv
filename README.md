# 🚀 `turbo-tail` Specification

A command-line and web-based real-time log inspection tool written in Go.

---

## 📦 General Overview

- **Name:** `turbo-tail`
- **Language:** Go
- **Purpose:** Realtime JSON log streaming, indexing, filtering, and display via a browser-based interface.
- **One complete binary**
  - html, js and css is embedded into the binary

---

## 🖥️ Command-Line Behavior

- **Input:** Standard input (`stdin`)
- **Output:** Standard output (`stdout`)
- **Behavior:**
  - Each line from `stdin` is echoed to `stdout`.
  - If the line is a valid JSON object:
    - A UUID is generated.
    - The object is stored in an internal map keyed by UUID.
    - The object is flattened (dot notation for nested properties).
    - The index is updated with property keys and values.

---

## 🧠 JSON Processing

- **UUID Generation:** Each JSON log entry gets a unique UUID.
- **Storage:** Entries are stored as:
  ```go
  map[uuid]LogEntry
  ```
- **Flattening:**
  - Nested structures are flattened using dot notation.
  - Example:
    ```json
    {
      "context": {
        "empty": "",
        "bindings": {
          "userId": "1020"
        }
      }
    }
    ```
    → `context.bindings.userId = "1020"`

- **Index Structure:**
  ```go
  map[propertyKey][propertyValue] -> []uuid
  ```
  
- **Indexing:**
  - Empty values are ommited
  - Example:
    ```json
    {
      "context": {
        "empty": "",
      }
    }
    ```
    → `context.empty` is not the index because it empty
  - If one property contains more then 10 different values:
    - the property is removed from index
    - and the property is blacklisted, so that it will skipped in the future
    - the `index_drop` message to the connected client
  

---

## 🕓 History Management

- **Max Logs Stored:** 10,000 recent JSON documents.
- **Old Logs:** Dropped when limit is exceeded.
  - dropped logs are removed from index
  - index update is sent via websocket

---

## 🌐 Web Server

- **Port:** `8080`
- **Routes:**
  - `/`: Serves static HTML/JS/CSS frontend.
  - `/ws`: WebSocket endpoint for live updates.

---

## 🔌 WebSocket Protocol

### On Client Connect

- `index`: Sends the current (flattened) index with counts.
- `log_bulk`: Sends last 1000 logs (filtered if applicable).

### Message Types

- **`set_filter`:**
  - Payload: `{ filters: { [property]: [values...] } }`
  - Result: Server responds with `log_bulk` (last 1000 logs matching filters).

- **Live Streaming:**
  - **`log`:** Each new log line broadcast as:
    ```json
    {
      "type": "log",
      "payload": {
        "uuid": "…",
        "document": { … }
      }
    }
    ```
  - **`index`:** Index updates sent with only changed properties and counts.
  - **`index_drop`:** Index properties that should be removed.

---

## 💻 Frontend UI

### Layout

| Left Panel (Filters)                 | Right Panel (Log View)                          |
|-------------------------------------|-------------------------------------------------|
| One filter box per JSON property    | Displays last 1000 logs (JSON syntax highlighting) |
| Values displayed with document count | Infinite scroll-like terminal behavior         |
| User can select multiple filters     | New logs auto-scroll unless user scrolls up     |

### Features

- **On Page Load:**
  - Connects to WebSocket.
  - Receives initial index and logs.
- **Filtering:**
  - Selecting filters updates the displayed logs.
  - Only logs matching selected filters are shown.
- **Log Streaming:**
  - New logs are streamed in real time.
  - Scroll position preserved if user scrolls up.
- **Syntax Highlighting:**
  - via vanilla js for properties, strings and numbers
- **Filters**
  - selected filters are highlighted
  - multiple filters for one property have or-semantic
  - filters for different properties have an and-semantic


### Technologies

- **HTML / CSS / JS** (vanilla)
- **WebSocket**

---

## 📌 Example Log and Filters

Given input:

```json
{
  "channel": "testing",
  "context": {
    "bindings": {
      "templateId": "0815",
      "userId": "1020-500555"
    }
  },
  "level": 200,
  "level_name": "INFO"
}
```

The following filter boxes appear:

- `channel`
- `context.bindings.templateId`
- `context.bindings.userId`
- `level`
- `level_name`
