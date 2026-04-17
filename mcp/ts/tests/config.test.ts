import { describe, it, expect } from "vitest";
import { parseServiceUrls, loadConfig } from "../src/config.js";

describe("parseServiceUrls", () => {
  it.each([
    {
      input: undefined,
      expected: [],
      description: "returns empty array for undefined",
    },
    {
      input: "",
      expected: [],
      description: "returns empty array for empty string",
    },
    {
      input: "go-gin=http://localhost:8080",
      expected: [{ name: "go-gin", url: "http://localhost:8080" }],
      description: "parses a single service",
    },
    {
      input: "a=http://x,b=http://y",
      expected: [
        { name: "a", url: "http://x" },
        { name: "b", url: "http://y" },
      ],
      description: "parses multiple comma-separated services",
    },
    {
      input: "  name = http://x  ",
      expected: [{ name: "name", url: "http://x" }],
      description: "trims whitespace around name and url",
    },
    {
      input: "svc=http://host?a=1",
      expected: [{ name: "svc", url: "http://host?a=1" }],
      description: "handles URLs containing = in query params",
    },
  ] satisfies Array<{
    input: string | undefined;
    expected: Array<{ name: string; url: string }>;
    description: string;
  }>)("$description", ({ input, expected }) => {
    expect(parseServiceUrls(input)).toEqual(expected);
  });

  it.each([
    {
      input: "no-equals-sign",
      expectedMessage: "Invalid service URL format",
      description: "throws when entry is missing =",
    },
    {
      input: "=http://no-name",
      description: "throws when name is empty",
    },
    {
      input: "svc=not-a-url",
      description: "throws for non-url values",
    },
  ] satisfies Array<{
    input: string;
    expectedMessage?: string;
    description: string;
  }>)("$description", ({ input, expectedMessage }) => {
    if (expectedMessage) {
      expect(() => parseServiceUrls(input)).toThrow(expectedMessage);
    } else {
      expect(() => parseServiceUrls(input)).toThrow();
    }
  });
});

describe("loadConfig", () => {
  it("returns defaults when no env vars set", () => {
    const config = loadConfig({});
    expect(config.serviceName).toBe("ts-mcp");
    expect(config.services).toEqual([]);
    expect(config.logLevel).toBe("INFO");
    expect(config.deploymentEnv).toBe("development");
    expect(config.transport).toBe("stdio");
    expect(config.port).toBe(8080);
    expect(config.host).toBe("127.0.0.1");
  });

  it("reads all values from env", () => {
    const config = loadConfig({
      TS_MCP_SERVICE_NAME: "custom",
      TS_MCP_SERVICE_URLS: "svc=http://localhost:3000",
      TS_MCP_LOG_LEVEL: "DEBUG",
      TS_MCP_DEPLOYMENT_ENV: "staging",
      TS_MCP_TRANSPORT: "streamable-http",
      TS_MCP_PORT: "8080",
      TS_MCP_HOST: "0.0.0.0",
    });
    expect(config.serviceName).toBe("custom");
    expect(config.services).toEqual([
      { name: "svc", url: "http://localhost:3000" },
    ]);
    expect(config.logLevel).toBe("DEBUG");
    expect(config.deploymentEnv).toBe("staging");
    expect(config.transport).toBe("streamable-http");
    expect(config.port).toBe(8080);
    expect(config.host).toBe("0.0.0.0");
  });

  it("throws for invalid log level", () => {
    expect(() => loadConfig({ TS_MCP_LOG_LEVEL: "INVALID" })).toThrow();
  });

  it("throws for invalid transport", () => {
    expect(() => loadConfig({ TS_MCP_TRANSPORT: "websocket" })).toThrow();
  });

  it("throws for out-of-range port", () => {
    expect(() => loadConfig({ TS_MCP_PORT: "0" })).toThrow();
    expect(() => loadConfig({ TS_MCP_PORT: "99999" })).toThrow();
  });

  it("falls back to NODE_ENV for deploymentEnv", () => {
    const config = loadConfig({ NODE_ENV: "production" });
    expect(config.deploymentEnv).toBe("production");
  });

  it("prefers TS_MCP_DEPLOYMENT_ENV over NODE_ENV", () => {
    const config = loadConfig({
      TS_MCP_DEPLOYMENT_ENV: "staging",
      NODE_ENV: "production",
    });
    expect(config.deploymentEnv).toBe("staging");
  });
});
