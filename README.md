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
    → `context.empty` is not indexed, because the value is empty
  - If one property contains more then 10 different values:
    - the property is removed from index
    - and the property is blacklisted, so that it will skipped in the future
    - the `drop_index` send message to the connected client


---

## 🕓 History Management

- **Max Logs Stored:** 10,000 recent JSON documents.
- **Old Logs:** Dropped when limit is exceeded.
  - dropped logs are removed from index
  - index update is sent via websocket (`update_index` message)

---

## 🌐 Web Server

- **Port:** `8080`
- **Routes:**
  - `/`: Serves static HTML/JS/CSS frontend.
  - `/ws`: WebSocket endpoint for live updates.

---

## 🔌 WebSocket Protocol

### On Client Connect

- **`set_index`:** Sends the current (flattened) index with counts.
  - Payload:
    ```json
    {
      "type": "set_index",
      "payload": {
        "channel": {
          "testing": 1
        },
        "context.bindings.userId": {
          "1020-500555": 1
        },
        "level": {
          "200": 1
        },
        "level_name": {
          "INFO": 1,
          "ERROR": 2
        }
      }
    }
    ```
- `set_logs`: Sends last 1000 logs (filtered if applicable).

### Message Types

#### Client to server

- **`set_filter`:**
  - Payload: `{ action: "set_filter", data: { [property]: [values...] } }`
  - Result: Server responds with `set_logs` (last 1000 logs matching filters).

#### Server to client

- **`set_logs`:**
  - Payload: `{ action: "set_logs", data: [records...] }`
  - Result: Clients removes all log entries from display und displays the given log records

- **`add_logs`:**
  - Payload: is identical to `set_logs`
  - Result: Clients adds the given log records to display

- **Live Streaming:**
  - **`add_logs`:** Each new log line broadcasted
  - **`update_index`:** Index updates sent with only changed properties and counts.
    - Payload:
      ```json
      {
        "type": "update_index",
        "payload": {
          "level_name": {
            "ERROR": 3
          }
        }
      }
      ```
    - Result: Client updates the value count
  - **`drop_index`:** Index properties that should be removed.
    - Payload:
      ```json
      {
        "type": "drop_index",
        "payload": ["property1", "property2"]
      }
      ```
    - Example:
      ```json
      {
        "type": "drop_index",
        "payload": ["context.bindings.userId", "level_name"]
      }
      ```
    - Result: Client removes the given filter boxes from display

---

## 💻 Frontend UI

### Layout

| Top Panel (Status and Search View)                                                        |
|--------------------------------------|----------------------------------------------------|
| Left Panel (Filter View)             | Right Panel (Log View)                             |
|--------------------------------------|----------------------------------------------------|
| One filter box per JSON property     | Displays last 1000 logs (JSON syntax highlighting) |
| Values displayed with document count | Infinite scroll-like terminal behavior             |
| User can select multiple filters     | New logs auto-scroll unless user scrolls up        |

### Features

- **On Page Load:**
  - Connects to WebSocket.
  - Receives initial index and logs. (`set_index` and `set_logs`)
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
      "userId": "1020-500555"
    }
  },
  "level": 200,
  "level_name": "INFO"
}
```

The following filter boxes appear:

- `channel`
- `context.bindings.userId`
- `level`
- `level_name`
