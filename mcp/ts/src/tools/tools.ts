import type { ZodRawShapeCompat } from "@modelcontextprotocol/sdk/server/zod-compat.js";
import type { ToolCallback } from "@modelcontextprotocol/sdk/server/mcp.js";
import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import type { CallToolResult } from "@modelcontextprotocol/sdk/types.js";
import type { ServiceEntry } from "../config.js";
import {
  getTracer,
  getToolCallCounter,
  getToolCallDuration,
  getActiveToolCounter,
} from "../otel.js";
import { SpanStatusCode } from "@opentelemetry/api";

const ATTR_MCP_TOOL_NAME = "mcp.tool.name";
const ATTR_MCP_TOOL_STATUS = "mcp.tool.status";

/** A built tool that can register itself with an MCP server. */
export interface BuiltTool {
  readonly name: string;
  readonly description: string;
  readonly callback: (...args: unknown[]) => Promise<CallToolResult>;
  register(server: McpServer): void;
}

/**
 * Type-safe builder for MCP tools.
 * Usage: `new ToolBuilder(name, desc).input(schema).handler(cb)` or
 * `new ToolBuilder(name, desc).handler(cb)` for no-input tools.
 */
export class ToolBuilder<S extends ZodRawShapeCompat | undefined = undefined> {
  private schema: S = undefined as S;

  constructor(
    private name: string,
    private description: string,
  ) {}

  /** Provide an input schema shape and narrow the builder. */
  input<T extends ZodRawShapeCompat>(schema: T): ToolBuilder<T> {
    const builder = new ToolBuilder<T>(this.name, this.description);
    builder.schema = schema;
    return builder;
  }

  /** Finalize the tool definition with a type-safe register method. */
  handler(callback: ToolCallback<S>): BuiltTool {
    const name = this.name;
    const description = this.description;

    const wrappedCallback = async (
      ...args: unknown[]
    ): Promise<CallToolResult> => {
      const start = performance.now();
      const nameAttrs = { [ATTR_MCP_TOOL_NAME]: name };
      getActiveToolCounter().add(1, nameAttrs);

      return getTracer().startActiveSpan(`mcp.tool/${name}`, async (span) => {
        span.setAttribute(ATTR_MCP_TOOL_NAME, name);
        let status: "success" | "error" = "success";
        let errorMessage: string | undefined;

        try {
          const result = await (
            callback as (...a: unknown[]) => Promise<CallToolResult>
          )(...args);
          if (result.isError) status = "error";
          return result;
        } catch (err) {
          status = "error";
          errorMessage = err instanceof Error ? err.message : String(err);
          throw err;
        } finally {
          span.setAttribute(ATTR_MCP_TOOL_STATUS, status);
          span.setStatus({
            code: status === "error" ? SpanStatusCode.ERROR : SpanStatusCode.OK,
            ...(errorMessage && { message: errorMessage }),
          });
          const metricAttrs = { ...nameAttrs, [ATTR_MCP_TOOL_STATUS]: status };
          getToolCallCounter().add(1, metricAttrs);
          getToolCallDuration().record(performance.now() - start, metricAttrs);
          span.end();
          getActiveToolCounter().add(-1, nameAttrs);
        }
      });
    };

    if (this.schema !== undefined) {
      const schema = this.schema;
      return {
        name,
        description,
        callback: wrappedCallback,
        register(server: McpServer) {
          server.registerTool(
            name,
            { description, inputSchema: schema },
            wrappedCallback as ToolCallback<typeof schema>,
          );
        },
      };
    }
    return {
      name,
      description,
      callback: wrappedCallback,
      register(server: McpServer) {
        server.registerTool(
          name,
          { description },
          wrappedCallback as ToolCallback<undefined>,
        );
      },
    };
  }
}

/** Discriminated union: either a found service entry or an error result. */
export type ServiceLookup =
  | { readonly ok: true; readonly entry: ServiceEntry }
  | { readonly ok: false; readonly result: CallToolResult };

/** Look up a service by name, returning the entry or an error result. */
export function findService(
  services: ServiceEntry[],
  name: string,
): ServiceLookup {
  const entry = services.find((s) => s.name === name);
  if (!entry) {
    return {
      ok: false,
      result: jsonResult(
        {
          error: `Unknown service: ${name}`,
          available: services.map((s) => s.name),
        },
        true,
      ),
    };
  }
  return { ok: true, entry };
}

/** Validate an externally-controlled request path. */
export function validatePath(path: string): CallToolResult | undefined {
  if (!path.startsWith("/")) {
    return jsonResult({ error: "Path must start with /" }, true);
  }
  // Decode before checking to catch encoded traversal attempts
  let decoded: string;
  try {
    decoded = decodeURIComponent(path);
  } catch {
    return jsonResult(
      { error: "Path contains invalid percent-encoding" },
      true,
    );
  }
  // Block path traversal (..) and protocol-relative URLs (//)
  if (decoded.includes("..") || /\/\//.test(decoded)) {
    return jsonResult({ error: "Path contains unsafe segments" }, true);
  }
  return undefined;
}

/** Build a JSON text content result for MCP tool responses. */
export function jsonResult(data: unknown, isError = false): CallToolResult {
  return {
    content: [{ type: "text", text: JSON.stringify(data) }],
    ...(isError && { isError: true }),
  };
}
