import { describe, it, expect, vi, beforeEach } from "vitest";
import { forwardRequestTool } from "../../src/tools/forward-request.js";
import { TEST_SERVICES, getText } from "../fixtures.js";

describe("forward_request tool", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it.each([
    {
      method: "GET",
      path: "/api/data",
      status: 200,
      response: '{"data":"hello"}',
      description: "GET with custom path",
    },
    {
      method: "GET",
      path: "/forward",
      status: 200,
      response: "ok",
      description: "GET with default path",
    },
  ])(
    "forwards $description and returns response",
    async ({ method, path, status, response }) => {
      vi.stubGlobal(
        "fetch",
        vi.fn().mockResolvedValue({
          status,
          headers: new Headers(),
          text: () => Promise.resolve(response),
        }),
      );

      const result = await forwardRequestTool(TEST_SERVICES).callback({
        service: "go-gin",
        method,
        path,
      });

      expect(result.isError).toBeUndefined();
      const body = JSON.parse(getText(result));
      expect(body.status).toBe(status);
      expect(body.body).toBe(response);
      expect(fetch).toHaveBeenCalledWith(
        `http://localhost:8080${path}`,
        expect.objectContaining({ method }),
      );
    },
  );

  it("forwards a POST request with body and headers", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        status: 201,
        headers: new Headers(),
        text: () => Promise.resolve("created"),
      }),
    );

    const result = await forwardRequestTool(TEST_SERVICES).callback({
      service: "go-gin",
      method: "POST",
      path: "/api/items",
      headers: { "content-type": "application/json" },
      body: '{"name":"test"}',
    });

    expect(result.isError).toBeUndefined();
    const body = JSON.parse(getText(result));
    expect(body.status).toBe(201);
    expect(fetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/items",
      expect.objectContaining({
        method: "POST",
        body: '{"name":"test"}',
        headers: { "content-type": "application/json" },
      }),
    );
  });

  it("returns isError when fetch throws", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("timeout")));

    const result = await forwardRequestTool(TEST_SERVICES).callback({
      service: "go-gin",
      method: "GET",
      path: "/slow",
    });

    expect(result.isError).toBe(true);
    const body = JSON.parse(getText(result));
    expect(body.error).toBe("timeout");
  });

  it.each([
    { path: "no-slash", expectedError: "Path must start with /" },
    { path: "/../admin", expectedError: "unsafe" },
    { path: "/foo//bar", expectedError: "unsafe" },
  ])(
    "returns isError for invalid path: $path",
    async ({ path, expectedError }) => {
      const result = await forwardRequestTool(TEST_SERVICES).callback({
        service: "go-gin",
        method: "GET",
        path,
      });

      expect(result.isError).toBe(true);
      expect(getText(result)).toContain(expectedError);
    },
  );

  it("returns isError when content-length exceeds limit", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        status: 200,
        headers: new Headers({ "content-length": "2000000" }),
        text: () => Promise.resolve("too large"),
      }),
    );

    const result = await forwardRequestTool(TEST_SERVICES).callback({
      service: "go-gin",
      method: "GET",
      path: "/big",
    });

    expect(result.isError).toBe(true);
    const body = JSON.parse(getText(result));
    expect(body.error).toContain("too large");
  });

  it("truncates streaming body at byte limit", async () => {
    const firstChunk = new Uint8Array(600_000).fill(65); // 'A'
    const secondChunk = new Uint8Array(600_000).fill(66); // 'B'
    async function* mockStream() {
      yield firstChunk;
      yield secondChunk;
    }

    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        status: 200,
        headers: new Headers(),
        body: { [Symbol.asyncIterator]: mockStream },
      }),
    );

    const result = await forwardRequestTool(TEST_SERVICES).callback({
      service: "go-gin",
      method: "GET",
      path: "/stream",
    });

    expect(result.isError).toBeUndefined();
    const body = JSON.parse(getText(result));
    expect(body.body.length).toBe(600_000);
    // First chunk was collected, second was discarded
    expect(body.body).toBe("A".repeat(600_000));
  });
});
