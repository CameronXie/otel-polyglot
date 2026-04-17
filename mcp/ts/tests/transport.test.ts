import { describe, it, expect, vi, afterEach } from "vitest";
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { createTransport, createMcpApp } from "../src/transport.js";
import type { McpServerFactory, SessionState } from "../src/transport.js";
import type { Config } from "../src/config.js";
import { TransportType } from "../src/config.js";
import { emitLog } from "../src/otel.js";

vi.mock("@modelcontextprotocol/sdk/server/stdio.js", () => ({
  StdioServerTransport: vi.fn(function (this: object) {
    return Object.assign(this, {
      close: vi.fn().mockResolvedValue(undefined),
    });
  }),
}));

vi.mock("@opentelemetry/api-logs", () => ({
  SeverityNumber: { DEBUG: 5, INFO: 9, WARN: 13, ERROR: 17, FATAL: 21 },
}));

vi.mock("../src/otel.js", () => ({
  emitLog: vi.fn(),
  initOTel: vi.fn(),
  shutdownOTel: vi.fn(),
}));

const mockMcpServer = {
  connect: vi.fn().mockResolvedValue(undefined),
  close: vi.fn().mockResolvedValue(undefined),
};

const serverFactory: McpServerFactory = () =>
  mockMcpServer as unknown as ReturnType<McpServerFactory>;

const httpConfig: Config = {
  transport: TransportType.StreamableHttp,
  port: 0,
  host: "127.0.0.1",
} as Config;

const toolsListBody = {
  jsonrpc: "2.0",
  method: "tools/list",
  id: 1,
  params: {},
};

const initializeBody = {
  jsonrpc: "2.0",
  method: "initialize",
  id: 1,
  params: {
    protocolVersion: "2025-03-26",
    capabilities: {},
    clientInfo: { name: "test", version: "1.0" },
  },
};

async function startApp(factory: McpServerFactory = serverFactory) {
  const sessions = new Map<string, SessionState>();
  const app = createMcpApp("127.0.0.1", factory, sessions);
  const server = createServer(app);
  await new Promise<void>((resolve) => server.listen(0, resolve));
  const port = (server.address() as AddressInfo).port;
  const base = `http://127.0.0.1:${port}`;
  const close = async () => {
    for (const session of sessions.values()) {
      await session.transport.close();
      await session.server.close();
    }
    sessions.clear();
    await new Promise<void>((resolve) => server.close(() => resolve()));
  };
  return { base, close, sessions };
}

describe("createMcpApp /mcp handler", () => {
  afterEach(() => {
    mockMcpServer.connect.mockClear();
  });

  it("responds 400 for non-initialize request without session", async () => {
    const { base, close } = await startApp();

    const r = await fetch(`${base}/mcp`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(toolsListBody),
    });

    expect(r.status).toBe(400);
    const body = (await r.json()) as { error: { message: string } };
    expect(body.error.message).toContain("not initialized");

    await close();
  });

  it("responds 404 for unknown session id", async () => {
    const { base, close } = await startApp();

    const r = await fetch(`${base}/mcp`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "mcp-session-id": "nonexistent-session",
      },
      body: JSON.stringify(toolsListBody),
    });

    expect(r.status).toBe(404);
    const body = (await r.json()) as { error: string };
    expect(body.error).toContain("Session not found");

    await close();
  });

  it("accepts a valid initialize request and creates a session", async () => {
    const { base, close, sessions } = await startApp();

    const r = await fetch(`${base}/mcp`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json, text/event-stream",
      },
      body: JSON.stringify(initializeBody),
    });

    expect(r.status).toBe(200);
    expect(sessions.size).toBe(1);

    await close();
  });

  it("removes session from map when transport closes", async () => {
    const { base, close, sessions } = await startApp();

    await fetch(`${base}/mcp`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json, text/event-stream",
      },
      body: JSON.stringify(initializeBody),
    });
    expect(sessions.size).toBe(1);

    const session = sessions.values().next().value!;
    await session.transport.close();
    expect(sessions.size).toBe(0);

    await close();
  });

  it("responds 500 when handler throws", async () => {
    const throwingFactory: McpServerFactory = () => {
      throw new Error("factory exploded");
    };
    const { base, close } = await startApp(throwingFactory);

    const r = await fetch(`${base}/mcp`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json, text/event-stream",
      },
      body: JSON.stringify(initializeBody),
    });

    expect(r.status).toBe(500);
    const body = (await r.json()) as { error: string };
    expect(body.error).toBe("Internal server error");
    expect(emitLog).toHaveBeenCalledWith(
      expect.anything(),
      "Error handling MCP request",
      expect.objectContaining({ error: "factory exploded" }),
    );

    await close();
  });

  it("reuses existing session for subsequent request", async () => {
    const { base, close, sessions } = await startApp();

    const initResp = await fetch(`${base}/mcp`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json, text/event-stream",
      },
      body: JSON.stringify(initializeBody),
    });
    expect(initResp.status).toBe(200);

    const sessionId = initResp.headers.get("mcp-session-id");
    expect(sessionId).toBeTruthy();
    expect(sessions.size).toBe(1);

    const followUp = await fetch(`${base}/mcp`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json, text/event-stream",
        "mcp-session-id": sessionId!,
      },
      body: JSON.stringify(toolsListBody),
    });
    expect(followUp.status).toBe(200);
    expect(sessions.size).toBe(1);

    await close();
  });
});

describe("createTransport", () => {
  afterEach(() => {
    mockMcpServer.connect.mockClear();
  });

  it("selects stdio transport when configured", async () => {
    const result = createTransport(
      { transport: TransportType.Stdio } as Config,
      serverFactory,
    );
    await result.connect();
    expect(mockMcpServer.connect).toHaveBeenCalledOnce();
    await expect(result.close()).resolves.toBeUndefined();
  });
});

describe("createTransport streamable-http", () => {
  afterEach(() => {
    mockMcpServer.connect.mockClear();
    mockMcpServer.close.mockClear();
  });

  it("starts and stops an HTTP server", async () => {
    const result = createTransport(httpConfig, serverFactory);
    await result.connect();
    await expect(result.close()).resolves.toBeUndefined();
  });

  it("close without prior connect resolves", async () => {
    const result = createTransport(httpConfig, serverFactory);
    await expect(result.close()).resolves.toBeUndefined();
  });
});
