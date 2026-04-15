import { z } from "zod";
import { ToolBuilder, findService, jsonResult } from "./tools.js";
import { PROBE_TIMEOUT_MS } from "../config.js";
import type { ServiceEntry } from "../config.js";
import { emitLog } from "../otel.js";
import { SeverityNumber } from "@opentelemetry/api-logs";

export const healthCheckTool = (services: ServiceEntry[]) =>
  new ToolBuilder(
    "health_check",
    "Check the health of a configured service by calling its /health endpoint",
  )
    .input({
      service: z.string().describe("Name of the service to health check"),
    })
    .handler(async ({ service }) => {
      const found = findService(services, service);
      if (!found.ok) return found.result;
      const entry = found.entry;

      try {
        const res = await fetch(`${entry.url}/health`, {
          signal: AbortSignal.timeout(PROBE_TIMEOUT_MS),
        });
        const body = await res.text();
        emitLog(SeverityNumber.INFO, "health_check completed", {
          service: entry.name,
          healthy: res.ok,
          status: res.status,
        });
        return jsonResult({
          service: entry.name,
          url: entry.url,
          status: res.status,
          healthy: res.ok,
          body,
        });
      } catch (err) {
        emitLog(SeverityNumber.ERROR, "health_check failed", {
          service: entry.name,
          error: err instanceof Error ? err.message : String(err),
        });
        return jsonResult(
          {
            service: entry.name,
            url: entry.url,
            healthy: false,
            error: err instanceof Error ? err.message : String(err),
          },
          true,
        );
      }
    });
