import { test, expect } from "@playwright/test";
import { spawn } from "child_process";
import path from "path";
import { fileURLToPath } from "url";
import net from "net";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

test.describe("Turbo-tail Live Log Streaming", () => {
  let turboTailProcess;
  let page;
  let startPort = 8181;
  let port;

  test.beforeAll(async ({ browser }) => {
    const context = await browser.newContext();
    page = await context.newPage();
  });

  test.afterAll(async () => {
    if (page) {
      await page.close();
    }
  });

  test.beforeEach(async () => {
    port = startPort;
    startPort++;
    turboTailProcess = spawn("go", ["run", ".", "--port", port], {
      cwd: path.join(__dirname, "../.."),
      stdio: ["pipe", "ignore", "inherit"],
    });
    turboTailProcess.on("close", (code) => {
      if (code) {
        throw new Error(`process exited with code ${code}`);
      }
    });
    await waitForPort("localhost", port);
  });

  test.afterEach(async () => {
    turboTailProcess.kill("SIGKILL");
  });

  test("should filter and search logs", async () => {
    await page.goto("http://localhost:" + port);
    await page.waitForSelector("#log-panel");

    const initialLogs = [
      {
        timestamp: "2024-01-15T10:00:00Z",
        level: "INFO",
        message: "Test info 1",
        service: "web-server",
      },
      {
        timestamp: "2024-01-15T10:00:01Z",
        level: "DEBUG",
        message: "Test debug 1",
        service: "web-server",
      },
      {
        timestamp: "2024-01-15T10:00:02Z",
        level: "WARN",
        message: "Test warn 1",
        service: "database",
      },
    ];

    for (const log of initialLogs) {
      turboTailProcess.stdin.write(JSON.stringify(log) + "\n");
    }

    const logPanel = page.locator("#log-panel");
    const logEntries = page.locator("#log-panel .log-entry");
    await expect(logEntries).toHaveCount(3);

    // Verify initial log content
    await expect(logPanel).toContainText("Test info 1");
    await expect(logPanel).toContainText("Test debug 1");
    await expect(logPanel).toContainText("Test warn 1");

    // Filter logs
    await page.getByTestId("filter-btn:service:web-server").click();
    await expect(logEntries).toHaveCount(2);

    // Reset filter
    await page.getByTestId("reset-filter-btn").click();
    await expect(logEntries).toHaveCount(3);

    // Seach logs
    await page.getByTestId("search-logs-input").fill("info");
    await expect(logEntries).toHaveCount(1);

    // Reset search
    await page.getByTestId("search-logs-input-reset-button").click();
    await expect(logEntries).toHaveCount(3);
  });

  test("should render thousands of logs", async () => {
    await page.goto("http://localhost:" + port);
    await page.waitForSelector("#log-panel");

    const logEntries = page.locator("#log-panel .log-entry");

    for (let i = 0; i < 2000; i++) {
      turboTailProcess.stdin.write(
        JSON.stringify({
          timestamp: new Date().toISOString(),
          level: ["INFO", "WARN", "ERROR"][i % 3],
          message: "Test " + i,
        }) + "\n",
      );
    }

    await expect(logEntries).toHaveCount(1000);
  });
});

function waitForPort(host, port, timeoutMs = 10000) {
  return new Promise((resolve, reject) => {
    const start = Date.now();

    function tryConnect() {
      const socket = new net.Socket();
      socket.setTimeout(1000);

      socket.once("connect", () => {
        socket.destroy();
        resolve();
      });

      socket.once("timeout", () => {
        socket.destroy();
        retry();
      });

      socket.once("error", () => {
        socket.destroy();
        retry();
      });

      socket.connect(port, host);

      function retry() {
        if (Date.now() - start >= timeoutMs) {
          reject(
            new Error(`Port ${port} on ${host} not open after ${timeoutMs}ms`),
          );
        } else {
          setTimeout(tryConnect, 500);
        }
      }
    }

    tryConnect();
  });
}
