/**
 * Entry point — initializes OTel, builds the MCP server, registers tools,
 * and connects over the configured transport (stdio or streamable-http).
 */
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { loadConfig, VERSION } from "./config.js";
import { emitLog, initOTel, shutdownOTel } from "./otel.js";
import { createTransport } from "./transport.js";
import { healthCheckTool } from "./tools/health-check.js";
import { forwardRequestTool } from "./tools/forward-request.js";
import { listServicesTool } from "./tools/list-services.js";
import { checkConnectivityTool } from "./tools/check-connectivity.js";
import { SeverityNumber } from "@opentelemetry/api-logs";

const config = loadConfig();
const sdk = initOTel(config);

/** Creates a fresh McpServer with all tools registered. */
function createMcpServer(): McpServer {
  const server = new McpServer({
    name: config.serviceName,
    version: VERSION,
  });
  const tools = [
    healthCheckTool(config.services),
    forwardRequestTool(config.services),
    listServicesTool(config.services),
    checkConnectivityTool(config.services),
  ];
  for (const tool of tools) {
    tool.register(server);
  }
  return server;
}

const transportResult = createTransport(config, createMcpServer);

async function gracefulExit() {
  setTimeout(() => process.exit(1), 5_000).unref();
  await shutdown();
  process.exit(0);
}

async function shutdown() {
  await transportResult.close();
  await shutdownOTel(sdk);
}

process.on("SIGTERM", gracefulExit);
process.on("SIGINT", gracefulExit);

try {
  await transportResult.connect();
} catch (err) {
  emitLog(SeverityNumber.ERROR, "Failed to start MCP server", {
    error: err instanceof Error ? err.message : String(err),
  });
  await shutdown();
  process.exit(1);
}
