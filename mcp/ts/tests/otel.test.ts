import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  setMinSeverity,
  emitLog,
  initOTel,
  shutdownOTel,
  getTracer,
  getToolCallCounter,
  getToolCallDuration,
  getActiveToolCounter,
} from "../src/otel.js";
import { SeverityNumber, logs } from "@opentelemetry/api-logs";
import type { Config } from "../src/config.js";
import type { NodeSDK } from "@opentelemetry/sdk-node";

vi.mock("@opentelemetry/sdk-node", () => {
  const mockStart = vi.fn();
  const mockShutdown = vi.fn().mockResolvedValue(undefined);
  return {
    NodeSDK: vi.fn(function (this: any) {
      this.start = mockStart;
      this.shutdown = mockShutdown;
    }),
  };
});

vi.mock("@opentelemetry/sdk-trace-node", () => ({
  BatchSpanProcessor: vi.fn(),
}));

vi.mock("@opentelemetry/sdk-metrics", () => ({
  PeriodicExportingMetricReader: vi.fn(),
}));

vi.mock("@opentelemetry/exporter-trace-otlp-grpc", () => ({
  OTLPTraceExporter: vi.fn(),
}));

vi.mock("@opentelemetry/exporter-metrics-otlp-grpc", () => ({
  OTLPMetricExporter: vi.fn(),
}));

vi.mock("@opentelemetry/exporter-logs-otlp-grpc", () => ({
  OTLPLogExporter: vi.fn(),
}));

vi.mock("@opentelemetry/sdk-logs", () => ({
  BatchLogRecordProcessor: vi.fn(),
}));

vi.mock("@opentelemetry/instrumentation-undici", () => ({
  UndiciInstrumentation: vi.fn(),
}));

describe("emitLog", () => {
  const mockEmit = vi.fn();

  beforeEach(() => {
    vi.restoreAllMocks();
    mockEmit.mockReset();
    vi.spyOn(logs, "getLogger").mockReturnValue({ emit: mockEmit } as any);
    setMinSeverity("INFO");
  });

  it("emits logs at or above the minimum severity", () => {
    emitLog(SeverityNumber.INFO, "test message", { key: "value" });
    expect(mockEmit).toHaveBeenCalledOnce();
    expect(mockEmit.mock.calls[0][0].body).toBe("test message");
    expect(mockEmit.mock.calls[0][0].attributes).toEqual({ key: "value" });
  });

  it("suppresses logs below the minimum severity", () => {
    setMinSeverity("ERROR");
    emitLog(SeverityNumber.INFO, "should be suppressed");
    expect(mockEmit).not.toHaveBeenCalled();
  });

  it("derives severityText from SeverityNumber", () => {
    emitLog(SeverityNumber.WARN, "warning message");
    expect(mockEmit.mock.calls[0][0].severityText).toBe("WARN");
    expect(mockEmit.mock.calls[0][0].severityNumber).toBe(SeverityNumber.WARN);
  });

  it("falls back to INFO severity for unknown level string", () => {
    setMinSeverity("UNKNOWN_LEVEL");
    emitLog(SeverityNumber.INFO, "should pass at default INFO");
    expect(mockEmit).toHaveBeenCalledOnce();
  });
});

describe("shutdownOTel", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.spyOn(logs, "getLogger").mockReturnValue({ emit: vi.fn() } as any);
    setMinSeverity("INFO");
  });

  it("calls sdk.shutdown and logs success via console", async () => {
    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    const mockShutdown = vi.fn().mockResolvedValue(undefined);
    const sdk = { shutdown: mockShutdown } as unknown as NodeSDK;
    await shutdownOTel(sdk);
    expect(mockShutdown).toHaveBeenCalledOnce();
    expect(consoleSpy).toHaveBeenCalledWith("OTel SDK shut down successfully");
  });

  it("logs error via console when sdk.shutdown rejects", async () => {
    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    const mockShutdown = vi.fn().mockRejectedValue(new Error("flush failed"));
    const sdk = { shutdown: mockShutdown } as unknown as NodeSDK;
    await shutdownOTel(sdk);
    expect(consoleSpy).toHaveBeenCalledWith(
      "Error shutting down OTel SDK:",
      "flush failed",
    );
  });

  it("logs error via console when sdk.shutdown times out", async () => {
    vi.useFakeTimers();
    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    const mockShutdown = vi.fn().mockReturnValue(new Promise(() => {}));
    const sdk = { shutdown: mockShutdown } as unknown as NodeSDK;
    const promise = shutdownOTel(sdk);
    vi.advanceTimersByTime(5_000);
    await promise;
    expect(consoleSpy).toHaveBeenCalledWith(
      "Error shutting down OTel SDK:",
      "OTel shutdown timed out",
    );
    vi.useRealTimers();
  });

  it("handles non-Error rejection from sdk.shutdown", async () => {
    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    const mockShutdown = vi.fn().mockRejectedValue("raw string error");
    const sdk = { shutdown: mockShutdown } as unknown as NodeSDK;
    await shutdownOTel(sdk);
    expect(consoleSpy).toHaveBeenCalledWith(
      "Error shutting down OTel SDK:",
      "raw string error",
    );
  });
});

describe("initOTel", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.spyOn(logs, "getLogger").mockReturnValue({ emit: vi.fn() } as any);
    setMinSeverity("INFO");
  });

  it("returns a NodeSDK instance", () => {
    const config = {
      serviceName: "test-service",
      logLevel: "INFO",
      deploymentEnv: "test",
    } as Config;
    const sdk = initOTel(config);
    expect(sdk).toBeDefined();
    expect(sdk.start).toHaveBeenCalled();
  });

  it("sets the minimum severity from config", () => {
    const mockEmit = vi.fn();
    vi.spyOn(logs, "getLogger").mockReturnValue({ emit: mockEmit } as any);
    initOTel({ logLevel: "DEBUG" } as Config);
    expect(mockEmit).toHaveBeenCalled();
    emitLog(SeverityNumber.DEBUG, "post-init debug");
    expect(mockEmit).toHaveBeenCalledTimes(2);
  });

  it("emits initialization log with service name", () => {
    const mockEmit = vi.fn();
    vi.spyOn(logs, "getLogger").mockReturnValue({ emit: mockEmit } as any);
    initOTel({ serviceName: "my-service", logLevel: "INFO" } as Config);
    expect(mockEmit).toHaveBeenCalledOnce();
    expect(mockEmit.mock.calls[0][0].body).toContain("OTel SDK initialized");
    expect(mockEmit.mock.calls[0][0].body).toContain("my-service");
  });
});

describe("metric and tracer helpers", () => {
  it("returns consistent singletons for tracer and metric instruments", () => {
    expect(getTracer()).toBe(getTracer());
    expect(getToolCallCounter()).toBe(getToolCallCounter());
    expect(getToolCallDuration()).toBe(getToolCallDuration());
    expect(getActiveToolCounter()).toBe(getActiveToolCounter());
  });
});
