import { describe, it, expect, vi } from "vitest";
import { z } from "zod";
import {
  ToolBuilder,
  findService,
  validatePath,
  jsonResult,
} from "../../src/tools/tools.js";
import { TEST_SERVICES, getText } from "../fixtures.js";

describe("findService", () => {
  it("returns matching service entry", () => {
    const result = findService(TEST_SERVICES, "go-gin");
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.entry).toEqual({
        name: "go-gin",
        url: "http://localhost:8080",
      });
    }
  });

  it("returns error result with available names for unknown service", () => {
    const result = findService(TEST_SERVICES, "missing");
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.result.isError).toBe(true);
      const body = JSON.parse(getText(result.result));
      expect(body.error).toContain("Unknown service: missing");
      expect(body.available).toEqual(["go-gin", "python"]);
    }
  });

  it("returns error for unknown service with empty services array", () => {
    const result = findService([], "anything");
    expect(result.ok).toBe(false);
    if (!result.ok) {
      const body = JSON.parse(getText(result.result));
      expect(body.available).toEqual([]);
    }
  });
});

describe("validatePath", () => {
  it.each([
    { input: "/api/data", description: "valid root-relative path" },
    { input: "/", description: "valid bare slash" },
    { input: "/users/123", description: "valid path with segments" },
    {
      input: "no-slash",
      expectedError: "Path must start with /",
      description: "missing leading slash",
    },
    {
      input: "/../admin",
      expectedError: "unsafe",
      description: "path traversal with ..",
    },
    {
      input: "/foo//bar",
      expectedError: "unsafe",
      description: "double slashes",
    },
    {
      input: "/api/../secret",
      expectedError: "unsafe",
      description: "path traversal in middle",
    },
    {
      input: "/%2e%2e/admin",
      expectedError: "unsafe",
      description: "percent-encoded traversal",
    },
    {
      input: "/%ZZ",
      expectedError: "invalid percent-encoding",
      description: "invalid percent-encoding",
    },
  ] satisfies Array<{
    input: string;
    expectedError?: string;
    description: string;
  }>)("$description", ({ input, expectedError }) => {
    const result = validatePath(input);
    if (expectedError) {
      expect(result?.isError).toBe(true);
      expect(getText(result!)).toContain(expectedError);
    } else {
      expect(result).toBeUndefined();
    }
  });
});

describe("jsonResult", () => {
  it("returns a success result without isError", () => {
    const result = jsonResult({ key: "value" });
    expect(result.isError).toBeUndefined();
    expect(result.content[0].type).toBe("text");
    expect(JSON.parse(getText(result))).toEqual({ key: "value" });
  });

  it("returns an error result when isError is true", () => {
    const result = jsonResult({ error: "bad" }, true);
    expect(result.isError).toBe(true);
    expect(JSON.parse(getText(result))).toEqual({ error: "bad" });
  });
});

describe("ToolBuilder", () => {
  it("registers a tool with schema via server.registerTool", () => {
    const mockRegister = vi.fn();
    const mockServer = { registerTool: mockRegister } as any;
    const schema = { query: z.string().describe("test input") };

    const tool = new ToolBuilder("test_tool", "A test")
      .input(schema)
      .handler(async () => jsonResult({ ok: true }));

    tool.register(mockServer);

    expect(mockRegister).toHaveBeenCalledWith(
      "test_tool",
      expect.objectContaining({
        description: "A test",
        inputSchema: schema,
      }),
      expect.any(Function),
    );
  });

  it("registers a tool without schema via server.registerTool", () => {
    const mockRegister = vi.fn();
    const mockServer = { registerTool: mockRegister } as any;

    const tool = new ToolBuilder("no_input_tool", "No input").handler(
      async () => jsonResult({ ok: true }),
    );

    tool.register(mockServer);

    expect(mockRegister).toHaveBeenCalledWith(
      "no_input_tool",
      expect.objectContaining({ description: "No input" }),
      expect.any(Function),
    );
  });

  it("callback wraps handler in span and records metrics", async () => {
    const tool = new ToolBuilder("metric_test", "Test metrics").handler(
      async () => jsonResult({ result: "ok" }),
    );

    const result = await tool.callback();
    expect(result.isError).toBeUndefined();
    const body = JSON.parse(getText(result));
    expect(body.result).toBe("ok");
  });

  it("callback records error status when handler returns isError", async () => {
    const tool = new ToolBuilder("error_test", "Test error").handler(async () =>
      jsonResult({ error: "fail" }, true),
    );

    const result = await tool.callback();
    expect(result.isError).toBe(true);
  });

  it("callback propagates exceptions from handler", async () => {
    const tool = new ToolBuilder("throw_test", "Test throw").handler(
      async () => {
        throw new Error("boom");
      },
    );

    await expect(tool.callback()).rejects.toThrow("boom");
  });
});
