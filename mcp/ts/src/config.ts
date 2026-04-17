import { z } from "zod";
import pkg from "../package.json" with { type: "json" };

export const VERSION = pkg.version;

/** Timeout for lightweight probes (health checks, connectivity). */
export const PROBE_TIMEOUT_MS = 5_000;

/** Timeout for full HTTP requests (forward_request). */
export const REQUEST_TIMEOUT_MS = 30_000;

/** Maximum response body size in bytes for forwarded requests. */
export const MAX_RESPONSE_BYTES = 1_000_000;

/** Supported MCP transport modes. */
export const TransportType = {
  Stdio: "stdio",
  StreamableHttp: "streamable-http",
} as const;

export type TransportType = (typeof TransportType)[keyof typeof TransportType];

/** A backend service entry, parsed from TS_MCP_SERVICE_URLS (name=url,...). */
export interface ServiceEntry {
  name: string;
  url: string;
}

const serviceEntrySchema = z.object({
  name: z.string().min(1, "Service name must not be empty"),
  url: z.url("Invalid service URL"),
});

const configSchema = z.object({
  serviceName: z.string().default("ts-mcp"),
  services: z.string().optional().transform(parseServiceUrls),
  logLevel: z.enum(["DEBUG", "INFO", "WARN", "ERROR"]).default("INFO"),
  deploymentEnv: z.string().default("development"),
  transport: z
    .enum([TransportType.Stdio, TransportType.StreamableHttp])
    .default(TransportType.Stdio),
  port: z.coerce.number().int().min(1).max(65535).default(8080),
  host: z.string().default("127.0.0.1"),
});

export type Config = z.infer<typeof configSchema>;

export function loadConfig(env = process.env): Config {
  return configSchema.parse({
    serviceName: env.TS_MCP_SERVICE_NAME,
    services: env.TS_MCP_SERVICE_URLS,
    logLevel: env.TS_MCP_LOG_LEVEL,
    deploymentEnv: env.TS_MCP_DEPLOYMENT_ENV ?? env.NODE_ENV,
    transport: env.TS_MCP_TRANSPORT,
    port: env.TS_MCP_PORT,
    host: env.TS_MCP_HOST,
  });
}

/** Parses a comma-separated "name=url" string into typed service entries.
 *  Format: `"name=url,name2=url2"` (e.g. `"go-gin=http://localhost:8080"`)
 */
export function parseServiceUrls(raw: string | undefined): ServiceEntry[] {
  if (!raw) return [];
  return raw.split(",").map((pair: string) => {
    const eqIndex = pair.indexOf("=");
    if (eqIndex === -1) {
      throw new Error(`Invalid service URL format: ${pair}. Expected name=url`);
    }
    return serviceEntrySchema.parse({
      name: pair.slice(0, eqIndex).trim(),
      url: pair.slice(eqIndex + 1).trim(),
    });
  });
}
