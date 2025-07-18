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

### Filter View

The filter view is the left panel of the turbo-tail frontend UI. Here’s how it works and what it looks like:

- **One filter box per JSON property:**  
  For every property found in the incoming JSON logs (including nested properties, which are shown in dot notation like `context.bindings.userId`), a filter box is displayed.

- **Values displayed with document count:**  
  Each filter box lists all the unique values seen for that property, along with a count of how many log entries have that value. For example, under `level_name` you might see:
  - INFO (12)
  - ERROR (3)

- **User can select multiple filters:**  
  - You can select multiple values within a single property (e.g., both INFO and ERROR under `level_name`). This acts as an OR filter for that property.
  - You can also select filters across different properties (e.g., channel: "testing" and level_name: "ERROR"). This acts as an AND filter across properties.

- **Selected filters are highlighted:**  
  When you select a filter value, it is visually highlighted to indicate it is active.

- **Filter logic:**  
  - Multiple values for one property: OR logic (matches any selected value for that property)
  - Filters across different properties: AND logic (log must match all selected properties)

- **Dynamic updates:**  
  - As new logs arrive, the filter boxes and their counts update in real time.
  - If a property is removed from the index (e.g., because it has too many unique values), its filter box disappears.

- **Interaction:**  
  - When you select or deselect filters, the displayed logs update to show only those matching the current filter selection.
  - The filter view is always in sync with the current state of the log index.

**Summary:**  
The filter view is an interactive, real-time panel that lets you quickly narrow down logs by property and value, with intuitive multi-select and live-updating counts, making it easy to focus on relevant log entries.

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
