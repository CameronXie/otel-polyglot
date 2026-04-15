import { describe, it, expect, vi, beforeEach } from "vitest";
import { healthCheckTool } from "../../src/tools/health-check.js";
import { TEST_SERVICES, getText } from "../fixtures.js";

describe("health_check tool", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("returns health status for a healthy service", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        text: () => Promise.resolve('{"status":"ok"}'),
      }),
    );

    const result = await healthCheckTool(TEST_SERVICES).callback({
      service: "go-gin",
    });

    expect(result.isError).toBeUndefined();
    const body = JSON.parse(getText(result));
    expect(body.healthy).toBe(true);
    expect(body.status).toBe(200);
    expect(body.body).toBe('{"status":"ok"}');
    expect(fetch).toHaveBeenCalledWith(
      "http://localhost:8080/health",
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
  });

  it("returns error for unhealthy service", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: false,
        status: 503,
        text: () => Promise.resolve("unavailable"),
      }),
    );

    const result = await healthCheckTool(TEST_SERVICES).callback({
      service: "go-gin",
    });

    expect(result.isError).toBeUndefined(); // HTTP error isn't a tool error
    const body = JSON.parse(getText(result));
    expect(body.healthy).toBe(false);
    expect(body.status).toBe(503);
  });

  it("returns isError for unknown service", async () => {
    const result = await healthCheckTool(TEST_SERVICES).callback({
      service: "unknown",
    });

    expect(result.isError).toBe(true);
    expect(getText(result)).toContain("Unknown service: unknown");
  });

  it("returns isError when fetch throws", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockRejectedValue(new Error("ECONNREFUSED")),
    );

    const result = await healthCheckTool(TEST_SERVICES).callback({
      service: "python",
    });

    expect(result.isError).toBe(true);
    const body = JSON.parse(getText(result));
    expect(body.healthy).toBe(false);
    expect(body.error).toBe("ECONNREFUSED");
  });
});
