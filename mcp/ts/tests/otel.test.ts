import { describe, it, expect, vi, beforeEach } from "vitest";
import { setMinSeverity, emitLog, shutdownOTel } from "../src/otel.js";
import { SeverityNumber, logs } from "@opentelemetry/api-logs";
import type { NodeSDK } from "@opentelemetry/sdk-node";

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
});
