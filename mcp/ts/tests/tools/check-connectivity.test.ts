import { describe, it, expect, vi, beforeEach } from "vitest";
import { checkConnectivityTool } from "../../src/tools/check-connectivity.js";
import { TEST_SERVICES, getText } from "../fixtures.js";

describe("check_connectivity tool", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("reports reachable service with latency", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        status: 200,
      }),
    );

    const result = await checkConnectivityTool(TEST_SERVICES).callback({
      service: "go-gin",
    });

    expect(result.isError).toBeUndefined();
    const body = JSON.parse(getText(result));
    expect(body.reachable).toBe(true);
    expect(body.latencyMs).toBeGreaterThanOrEqual(0);
    expect(fetch).toHaveBeenCalledWith(
      "http://localhost:8080",
      expect.objectContaining({ method: "HEAD" }),
    );
  });

  it("returns isError for unknown service", async () => {
    const result = await checkConnectivityTool(TEST_SERVICES).callback({
      service: "missing",
    });

    expect(result.isError).toBe(true);
    expect(getText(result)).toContain("Unknown service: missing");
  });

  it("returns isError when service is unreachable", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockRejectedValue(new Error("ECONNREFUSED")),
    );

    const result = await checkConnectivityTool(TEST_SERVICES).callback({
      service: "go-gin",
    });

    expect(result.isError).toBe(true);
    const body = JSON.parse(getText(result));
    expect(body.reachable).toBe(false);
    expect(body.error).toBe("ECONNREFUSED");
  });
});
