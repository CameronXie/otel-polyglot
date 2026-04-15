import { z } from "zod";
import { ToolBuilder, findService, jsonResult } from "./tools.js";
import { PROBE_TIMEOUT_MS } from "../config.js";
import type { ServiceEntry } from "../config.js";
import { emitLog } from "../otel.js";
import { SeverityNumber } from "@opentelemetry/api-logs";

export const checkConnectivityTool = (services: ServiceEntry[]) =>
  new ToolBuilder(
    "check_connectivity",
    "Check network connectivity to a configured service by making a lightweight HEAD request",
  )
    .input({
      service: z
        .string()
        .describe("Name of the service to check connectivity to"),
    })
    .handler(async ({ service }) => {
      const found = findService(services, service);
      if (!found.ok) return found.result;
      const entry = found.entry;

      const start = performance.now();
      try {
        const res = await fetch(entry.url, {
          method: "HEAD",
          signal: AbortSignal.timeout(PROBE_TIMEOUT_MS),
        });
        const latencyMs = Math.round(performance.now() - start);
        emitLog(SeverityNumber.INFO, "check_connectivity completed", {
          service: entry.name,
          reachable: true,
          status: res.status,
          latencyMs,
        });
        return jsonResult({
          service: entry.name,
          url: entry.url,
          reachable: true,
          status: res.status,
          latencyMs,
        });
      } catch (err) {
        const latencyMs = Math.round(performance.now() - start);
        emitLog(SeverityNumber.WARN, "check_connectivity unreachable", {
          service: entry.name,
          error: err instanceof Error ? err.message : String(err),
        });
        return jsonResult(
          {
            service: entry.name,
            url: entry.url,
            reachable: false,
            latencyMs,
            error: err instanceof Error ? err.message : String(err),
          },
          true,
        );
      }
    });
