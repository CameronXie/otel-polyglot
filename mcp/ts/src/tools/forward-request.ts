import { z } from "zod";
import { ToolBuilder, findService, validatePath, jsonResult } from "./tools.js";
import { REQUEST_TIMEOUT_MS, MAX_RESPONSE_BYTES } from "../config.js";
import type { ServiceEntry } from "../config.js";
import { emitLog } from "../otel.js";
import { SeverityNumber } from "@opentelemetry/api-logs";

export const forwardRequestTool = (services: ServiceEntry[]) =>
  new ToolBuilder(
    "forward_request",
    "Forward an HTTP request to a configured service and return the response",
  )
    .input({
      service: z.string().describe("Name of the target service"),
      method: z
        .enum(["GET", "POST", "PUT", "PATCH", "DELETE"])
        .default("GET")
        .describe("HTTP method (defaults to GET)"),
      path: z
        .string()
        .default("/forward")
        .describe("Request path (defaults to /forward)"),
      headers: z
        .record(z.string(), z.string())
        .optional()
        .describe("Optional request headers"),
      body: z.string().optional().describe("Optional request body as string"),
    })
    .handler(async ({ service, method, path, headers, body }) => {
      // Check service first for better error messaging
      const found = findService(services, service);
      if (!found.ok) return found.result;
      const entry = found.entry;

      const pathError = validatePath(path);
      if (pathError) return pathError;

      const url = `${entry.url}${path}`;
      try {
        const res = await fetch(url, {
          method,
          headers,
          body,
          signal: AbortSignal.timeout(REQUEST_TIMEOUT_MS),
        });

        const contentLength = parseInt(
          res.headers.get("content-length") ?? "",
          10,
        );
        if (!isNaN(contentLength) && contentLength > MAX_RESPONSE_BYTES) {
          emitLog(SeverityNumber.WARN, "forward_request response too large", {
            service: entry.name,
            url,
            bytes: contentLength,
          });
          return jsonResult(
            {
              service: entry.name,
              url,
              error: `Response too large: ${contentLength} bytes exceeds ${MAX_RESPONSE_BYTES} limit`,
            },
            true,
          );
        }

        const responseBody = res.body
          ? await readBodyWithLimit(res.body, MAX_RESPONSE_BYTES)
          : await res.text();

        emitLog(SeverityNumber.INFO, "forward_request completed", {
          service: entry.name,
          url,
          method,
          status: res.status,
        });

        return jsonResult({
          service: entry.name,
          url,
          status: res.status,
          headers: Object.fromEntries(res.headers.entries()),
          body: responseBody,
        });
      } catch (err) {
        emitLog(SeverityNumber.ERROR, "forward_request failed", {
          service: entry.name,
          url,
          error: err instanceof Error ? err.message : String(err),
        });
        return jsonResult(
          {
            service: entry.name,
            url,
            error: err instanceof Error ? err.message : String(err),
          },
          true,
        );
      }
    });

/** Read a stream into a string, stopping at maxBytes. */
async function readBodyWithLimit(
  body: ReadableStream<Uint8Array>,
  maxBytes: number,
): Promise<string> {
  const chunks: Uint8Array[] = [];
  let totalBytes = 0;
  for await (const chunk of body as AsyncIterable<Uint8Array>) {
    totalBytes += chunk.byteLength;
    if (totalBytes > maxBytes) {
      break;
    }
    chunks.push(chunk);
  }
  return await new Blob(chunks as BlobPart[]).text();
}
