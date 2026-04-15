/**
 * Entry point — initializes OTel, builds the MCP server, registers tools,
 * and connects over stdio.
 */
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { loadConfig, VERSION } from "./config.js";
import { emitLog, initOTel, shutdownOTel } from "./otel.js";
import { healthCheckTool } from "./tools/health-check.js";
import { forwardRequestTool } from "./tools/forward-request.js";
import { listServicesTool } from "./tools/list-services.js";
import { checkConnectivityTool } from "./tools/check-connectivity.js";
import { SeverityNumber } from "@opentelemetry/api-logs";

const config = loadConfig();
const sdk = initOTel(config);
const services = config.services;

const server = new McpServer({
  name: config.serviceName,
  version: VERSION,
});

// Register tools — each tool self-registers with the server
const tools = [
  healthCheckTool(services),
  forwardRequestTool(services),
  listServicesTool(services),
  checkConnectivityTool(services),
];
for (const tool of tools) {
  tool.register(server);
}

const transport = new StdioServerTransport();

async function gracefulExit() {
  setTimeout(() => process.exit(1), 5_000).unref();
  await shutdown();
  process.exit(0);
}

async function shutdown() {
  await server.close();
  await shutdownOTel(sdk);
}

process.on("SIGTERM", gracefulExit);
process.on("SIGINT", gracefulExit);

try {
  await server.connect(transport);
} catch (err) {
  emitLog(SeverityNumber.ERROR, "Failed to start MCP server", {
    error: err instanceof Error ? err.message : String(err),
  });
  await shutdown();
  process.exit(1);
}
