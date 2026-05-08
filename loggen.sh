#!/usr/bin/env bash

LEVELS=("INFO" "INFO" "INFO" "WARN" "ERROR" "DEBUG" "DEBUG")
SERVICES=("api-gateway" "auth-service" "user-service" "order-service" "payment-service" "notification-service" "cache-service" "db-proxy" "scheduler" "metrics-collector")
HOSTS=("node-01" "node-02" "node-03" "node-04" "node-05")
ENVS=("production" "staging")

INFO_MSGS=(
  "Request processed successfully"
  "User logged in"
  "Session created"
  "Cache hit for key"
  "Database query executed in"
  "Health check passed"
  "Configuration reloaded"
  "Connection pool initialized"
  "Token refreshed"
  "Background job started"
  "Background job completed"
  "Rate limit check passed"
  "File uploaded successfully"
  "Email notification sent"
  "Payment processed"
  "Order created"
  "User profile updated"
  "Password changed"
  "API key validated"
  "Webhook delivered"
  "Metrics flushed to backend"
  "Index rebuilt successfully"
  "Snapshot created"
  "TLS certificate valid"
  "Queue consumer started"
  "Message acknowledged"
  "Retry succeeded"
  "Circuit breaker closed"
  "Feature flag evaluated"
  "Audit log written"
)

WARN_MSGS=(
  "Slow query detected"
  "High memory usage detected"
  "Connection pool nearly exhausted"
  "Retry attempt"
  "Response time above threshold"
  "Disk usage above 80%"
  "Rate limit approaching"
  "Certificate expires in 14 days"
  "Deprecated API endpoint called"
  "Cache miss rate elevated"
  "Queue depth increasing"
  "Unexpected null value in response"
  "Fallback activated for service"
  "Token expiring soon"
)

ERROR_MSGS=(
  "Database connection failed"
  "Timeout while calling upstream service"
  "Authentication failed: invalid credentials"
  "Payment gateway returned error"
  "Unhandled exception in request handler"
  "Failed to write to disk"
  "Message delivery failed after 3 retries"
  "Circuit breaker opened"
  "Out of memory: killing process"
  "TLS handshake failed"
  "Invalid request signature"
  "Service unavailable"
)

DEBUG_MSGS=(
  "Entering handler function"
  "Exiting handler function"
  "SQL query prepared"
  "Cache key computed"
  "Parsed request body"
  "Serialized response"
  "Middleware chain executed"
  "Header injected"
  "Span created"
  "Span closed"
)

rand_int() { echo $(( RANDOM % $1 )); }
rand_ms()  { echo $(( RANDOM % 2000 + 1 )); }
rand_id()  { printf '%08x-%04x-%04x-%04x-%012x' $RANDOM $RANDOM $RANDOM $RANDOM $RANDOM; }
rand_ip()  { echo "$(( RANDOM % 223 + 1 )).$(( RANDOM % 255 )).$(( RANDOM % 255 )).$(( RANDOM % 255 ))"; }

make_msg() {
  local level="${LEVELS[$(rand_int ${#LEVELS[@]})]}"
  local service="${SERVICES[$(rand_int ${#SERVICES[@]})]}"
  local host="${HOSTS[$(rand_int ${#HOSTS[@]})]}"
  local env="${ENVS[$(rand_int ${#ENVS[@]})]}"
  local ts
  local ms
  ms=$(printf '%03d' $(( RANDOM % 1000 )))
  ts="$(date -u +"%Y-%m-%dT%H:%M:%S").${ms}Z"
  local trace_id
  trace_id=$(rand_id)
  local span_id
  span_id=$(printf '%016x' $RANDOM)
  local duration
  duration=$(rand_ms)
  local status_code
  local msg

  case "$level" in
    INFO)
      msg="${INFO_MSGS[$(rand_int ${#INFO_MSGS[@]})]}"
      status_code=200
      ;;
    WARN)
      msg="${WARN_MSGS[$(rand_int ${#WARN_MSGS[@]})]}"
      status_code=429
      ;;
    ERROR)
      msg="${ERROR_MSGS[$(rand_int ${#ERROR_MSGS[@]})]}"
      status_code=500
      ;;
    DEBUG)
      msg="${DEBUG_MSGS[$(rand_int ${#DEBUG_MSGS[@]})]}"
      status_code=200
      ;;
  esac

  printf '{"timestamp":"%s","level":"%s","service":"%s","host":"%s","env":"%s","message":"%s","trace_id":"%s","span_id":"%s","duration_ms":%d,"status_code":%d,"client_ip":"%s"}\n' \
    "$ts" "$level" "$service" "$host" "$env" "$msg" "$trace_id" "$span_id" "$duration" "$status_code" "$(rand_ip)"
}

# Burst: 500 messages immediately
for i in $(seq 1 500); do
  make_msg
done

# Then one per second indefinitely
while true; do
  sleep 1
  make_msg
done
