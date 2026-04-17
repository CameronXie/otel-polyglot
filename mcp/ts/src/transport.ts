/**
 * Transport factory — creates the MCP transport layer based on config.
 * Supports stdio (default) and streamable-http via Express.
 */
import { randomUUID } from "node:crypto";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { StreamableHTTPServerTransport } from "@modelcontextprotocol/sdk/server/streamableHttp.js";
import { createMcpExpressApp } from "@modelcontextprotocol/sdk/server/express.js";
import { isInitializeRequest } from "@modelcontextprotocol/sdk/types.js";
import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import type { Request, Response } from "express";
import type { Config } from "./config.js";
import { TransportType } from "./config.js";
import { emitLog } from "./otel.js";
import { SeverityNumber } from "@opentelemetry/api-logs";

/** Factory that creates a fresh McpServer with all tools registered. */
export type McpServerFactory = () => McpServer;

export interface TransportResult {
  connect: () => Promise<void>;
  close: () => Promise<void>;
}

/** Per-session state for the streamable-http transport. */
export interface SessionState {
  transport: StreamableHTTPServerTransport;
  server: McpServer;
}

/** Creates an Express app wired with the /mcp route handler.
 *  Sessions map is provided by the caller so it owns the lifecycle. */
export function createMcpApp(
  host: string,
  serverFactory: McpServerFactory,
  sessions: Map<string, SessionState>,
): ReturnType<typeof createMcpExpressApp> {
  const app = createMcpExpressApp({ host });

  app.all("/mcp", async (req: Request, res: Response) => {
    try {
      const sessionId = req.headers["mcp-session-id"] as string | undefined;

      // Reuse existing session
      if (sessionId) {
        const session = sessions.get(sessionId);
        if (!session) {
          res.status(404).json({ error: "Session not found" });
          return;
        }
        await session.transport.handleRequest(req, res, req.body);
        return;
      }

      // New session — must be an initialize request
      if (req.body && isInitializeRequest(req.body)) {
        const server = serverFactory();
        const transport = new StreamableHTTPServerTransport({
          sessionIdGenerator: () => randomUUID(),
          onsessioninitialized: (sid) => {
            sessions.set(sid, { transport, server });
            transport.onclose = () => {
              sessions.delete(sid);
            };
          },
        });
        await server.connect(transport);
        await transport.handleRequest(req, res, req.body);
        return;
      }

      res.status(400).json({
        jsonrpc: "2.0",
        error: { code: -32000, message: "Bad request: not initialized" },
        id: null,
      });
    } catch (err) {
      emitLog(SeverityNumber.ERROR, "Error handling MCP request", {
        error: err instanceof Error ? err.message : String(err),
      });
      if (!res.headersSent) {
        res.status(500).json({ error: "Internal server error" });
      }
    }
  });

  return app;
}

function createStdioTransport(
  serverFactory: McpServerFactory,
): TransportResult {
  let server: McpServer | undefined;
  const transport = new StdioServerTransport();
  return {
    connect: async () => {
      server = serverFactory();
      await server.connect(transport);
    },
    close: async () => {
      await transport.close();
      await server?.close();
    },
  };
}

function createHttpTransport(
  config: Config,
  serverFactory: McpServerFactory,
): TransportResult {
  const sessions = new Map<string, SessionState>();
  const app = createMcpApp(config.host, serverFactory, sessions);
  let httpServer: ReturnType<typeof app.listen> | undefined;

  return {
    connect: async () => {
      await new Promise<void>((resolve, reject) => {
        httpServer = app.listen(config.port, config.host, () => {
          emitLog(
            SeverityNumber.INFO,
            `MCP server listening on http://${config.host}:${config.port}/mcp`,
          );
          resolve();
        });
        httpServer?.on("error", reject);
      });
    },
    close: async () => {
      for (const session of sessions.values()) {
        await session.transport.close();
        await session.server.close();
      }
      sessions.clear();
      await new Promise<void>((resolve) => {
        if (httpServer) {
          httpServer.close(() => resolve());
        } else {
          resolve();
        }
      });
    },
  };
}

/** Creates the appropriate transport based on config. */
export function createTransport(
  config: Config,
  serverFactory: McpServerFactory,
): TransportResult {
  if (config.transport === TransportType.StreamableHttp) {
    return createHttpTransport(config, serverFactory);
  }
  return createStdioTransport(serverFactory);
}
