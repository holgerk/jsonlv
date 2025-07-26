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
  - Very long values are ommited (longer than 50 chars)
    - can be configured with --maxIndexValueLength option
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

- **`set_logs`:** Sends last 1000 logs (filtered if applicable) and the current (flattened) index with counts.

### Message Types

#### Standard Message Format

- Every websocket message is an json object with the properties:
  - `type` containing the message type
  - `payload` containing the require data
- Example:
  ```json
  {
    "type": "<message-type>",
    "payload": "<data>"
  }
  ```

#### Client to server

- **`set_search`:**
  - Payload: `{ type: "set_search", payload: { filters: { [property]: [values...] }, searchTerm?: string } }`
  - Result:
    - Server responds with `set_logs` (last 1000 logs matching filters and search term).
    - Server stores the actual filter in `actualFilter` property
  - The `filters` property contains the property-based filters
  - The `searchTerm` property is optional and contains a string to search for in all log properties

#### Server to client

- **`set_logs`:**
  - Example:
    ```json
    {
      "type": "set_logs",
      "payload": {
        "indexCounts": {
          "context.bindings.userId": {
            "1020-500555": 1
          },
          "level_name": {
            "INFO": 1,
            "ERROR": 2
          }
        },
        "logs": [
          {
            "context": {
              "userId": "1020-500555"
            },
            "level": 200,
            "level_name": "INFO"
          },
          {
            "context": {
              "userId": "1020-500555"
            },
            "level": 200,
            "level_name": "ERROR"
          }
        ]
      }
    }
    ```
  - Result: Clients removes all log entries from display und displays the given log records
  - Implementation Implications:
      - When sending set_logs or add_logs, the server should send only the original log data
        (the Raw field of LogEntry), not the UUID or Flat fields.
      - The payload should be an array of these raw log objects.

- **`add_logs`:**
  - Payload: is identical to `set_logs`
  - Result: Clients adds the given log records to display

- **Live Streaming:**
  - **`add_logs`:**
    - Each new log line broadcasted
    - If `actualFilter` is set then the log line is only broadcasted if the filter matches the log line
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
  - **`set_status`:** Status information sent every 10 seconds.
    - Payload:
      ```json
      {
        "type": "set_status",
        "payload": {
          "allocatedMemory": 1048576,
          "logsStored": 1250
        }
      }
      ```
    - Result: Client updates status display with current memory usage and log count

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

### Status View

The status view is part of the top panel in the turbo-tail frontend UI. It provides users with real-time information about the current state of the log streaming and filtering system.

- **Location:**
  The status view is located in the top panel of the UI, above the filter and log panels.

- **Purpose:**
  The status view provides users with real-time information about the current state of the log streaming and filtering system.

- **Features:**
  - **Connection Status:**
    Indicates whether the frontend is connected to the backend WebSocket (e.g., “Connected”, “Reconnecting”, “Disconnected”).
  - **Log Stream Status:**
    Shows if logs are actively streaming in, paused, or if there is a backlog.
  - **Log Count:**
    Displays the total number of logs currently loaded or visible (e.g., “Showing 1,000 of 10,000 logs”).
  - **Active Filters:**
    Summarizes which filters are currently applied, possibly as a list or a compact summary (e.g., “Filters: level_name=ERROR, channel=testing”).
  - **Search Bar:**
    A text input field that allows users to search for specific terms across all log properties. As the user types, it automatically sends search requests to filter logs in real-time.
  - **Search/Filter Reset:**
    May include a button to clear all filters or reset the view to show all logs.
  - **Fullscreen Toggle:**
    A button to toggle fullscreen mode for the entire application, maximizing the log viewing experience.
  - **Other Status Indicators:**
    Could include information such as the time of the last received log, backend version, or error/warning messages if something goes wrong.

- **User Experience:**
  The status view helps users quickly understand what they are looking at, whether the system is up-to-date, and what filters or search terms are currently affecting the log display.

**Summary:**
The status view is a compact, always-visible area at the top of the UI that keeps users informed about connection health, log counts, active filters, and overall system state, ensuring transparency and confidence while inspecting logs in real time.

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

### Log View

The log view is the right panel of the turbo-tail frontend UI. It displays the log entries in real time and provides a user-friendly interface for inspecting log data.

- **Location:**
  The log view occupies the right panel of the UI, next to the filter view.

- **Display:**
  - Shows up to the last 1,000 log entries, each formatted as JSON with syntax highlighting for properties, strings, and numbers.
  - New logs are appended at the bottom, creating an infinite scroll-like experience similar to a terminal.
  - If the user scrolls up, new logs do not auto-scroll the view, preserving the user's position.
  - When filters are applied, only logs matching the selected filters are shown.

- **Features:**
  - **Real-time streaming:** Logs appear in the view as soon as they are received.
  - **Syntax highlighting:** JSON properties, strings, and numbers are visually distinguished for easier reading.
  - **Infinite scroll:** The view supports efficient navigation through large numbers of logs, with smooth scrolling behavior.
  - **Filter integration:** The log view updates instantly to reflect the current filter selection from the filter view.
  - **Log replacement:** When filters change, the log view is cleared and repopulated with only the matching logs.
  - **Highlighting:** log levels are visually emphasized (rendered bold) for quick identification:
    - info
    - warn
    - warning
    - error
    - critical
    (uppercase variants of this log level are treated equally)
    The following colors are applied:
    - error and critical -> red
    - warn and warning -> orange



- **User Experience:**
  The log view is designed for clarity and speed, allowing users to monitor, search, and inspect logs as they stream in. The combination of real-time updates, syntax highlighting, and filter integration makes it easy to focus on relevant log entries and quickly spot issues or patterns.

**Summary:**
The log view is a dynamic, real-time display of JSON logs, optimized for readability and efficient inspection, tightly integrated with the filter and status views for a seamless log analysis experience.

### Features

- **On Page Load:**
  - Connects to WebSocket.
  - Receives initial index and logs. (`set_logs`)
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

---

# Todos

- limit number of messages in view
- highlight matches in json
- wildcard search
- reduce memory consumption
- persist ui settings (filter, search, resizer positon) in url
