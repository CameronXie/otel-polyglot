import { NodeSDK } from "@opentelemetry/sdk-node";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-grpc";
import { OTLPMetricExporter } from "@opentelemetry/exporter-metrics-otlp-grpc";
import { OTLPLogExporter } from "@opentelemetry/exporter-logs-otlp-grpc";
import { BatchSpanProcessor } from "@opentelemetry/sdk-trace-node";
import { PeriodicExportingMetricReader } from "@opentelemetry/sdk-metrics";
import { BatchLogRecordProcessor } from "@opentelemetry/sdk-logs";
import {
  resourceFromAttributes,
  defaultResource,
} from "@opentelemetry/resources";
import { UndiciInstrumentation } from "@opentelemetry/instrumentation-undici";
import {
  ATTR_SERVICE_NAME,
  ATTR_SERVICE_VERSION,
} from "@opentelemetry/semantic-conventions";
import { logs, SeverityNumber } from "@opentelemetry/api-logs";
import type { AnyValueMap } from "@opentelemetry/api-logs";
import { trace, metrics } from "@opentelemetry/api";
import { VERSION } from "./config.js";
import type { Config } from "./config.js";

const LOG_LEVEL_SEVERITY: Record<string, SeverityNumber> = {
  DEBUG: SeverityNumber.DEBUG,
  INFO: SeverityNumber.INFO,
  WARN: SeverityNumber.WARN,
  ERROR: SeverityNumber.ERROR,
};

const SEVERITY_TEXT: Record<number, string> = {
  [SeverityNumber.DEBUG]: "DEBUG",
  [SeverityNumber.INFO]: "INFO",
  [SeverityNumber.WARN]: "WARN",
  [SeverityNumber.ERROR]: "ERROR",
};

let minSeverity = SeverityNumber.INFO;
let loggerName = "ts-mcp";

/** Set the minimum severity for log emission. */
export function setMinSeverity(level: string): void {
  minSeverity = LOG_LEVEL_SEVERITY[level] ?? SeverityNumber.INFO;
}

/** Emit a structured OTel log record, filtered by the current minimum severity. */
export function emitLog(
  severity: SeverityNumber,
  body: string,
  attributes?: AnyValueMap,
) {
  if (severity < minSeverity) return;
  logs.getLogger(loggerName).emit({
    severityNumber: severity,
    severityText: SEVERITY_TEXT[severity] ?? "UNSPECIFIED",
    body,
    attributes,
  });
}

type Tracer = ReturnType<typeof trace.getTracer>;
type Counter = ReturnType<ReturnType<typeof metrics.getMeter>["createCounter"]>;
type Histogram = ReturnType<
  ReturnType<typeof metrics.getMeter>["createHistogram"]
>;
type UpDownCounter = ReturnType<
  ReturnType<typeof metrics.getMeter>["createUpDownCounter"]
>;

let _tracer: Tracer | undefined;
let _toolCallCounter: Counter | undefined;
let _toolCallDuration: Histogram | undefined;
let _activeToolCounter: UpDownCounter | undefined;

/** Returns the MCP tool tracer. Before initOTel, returns the OTel API no-op. */
export function getTracer(): Tracer {
  if (!_tracer) {
    _tracer = trace.getTracer("ts-mcp", VERSION);
  }
  return _tracer;
}

/** Returns the tool call counter. Attributes: mcp.tool.name, mcp.tool.status */
export function getToolCallCounter(): Counter {
  if (!_toolCallCounter) {
    _toolCallCounter = metrics
      .getMeter("ts-mcp", VERSION)
      .createCounter("mcp.tool.calls", {
        description: "Total MCP tool invocations",
      });
  }
  return _toolCallCounter;
}

/** Returns the tool duration histogram. Attributes: mcp.tool.name, mcp.tool.status */
export function getToolCallDuration(): Histogram {
  if (!_toolCallDuration) {
    _toolCallDuration = metrics
      .getMeter("ts-mcp", VERSION)
      .createHistogram("mcp.tool.duration", {
        description: "MCP tool execution duration",
        unit: "ms",
      });
  }
  return _toolCallDuration;
}

/** Returns the active tool call up-down counter. Attribute: mcp.tool.name */
export function getActiveToolCounter(): UpDownCounter {
  if (!_activeToolCounter) {
    _activeToolCounter = metrics
      .getMeter("ts-mcp", VERSION)
      .createUpDownCounter("mcp.tool.active", {
        description: "Currently in-flight MCP tool calls",
      });
  }
  return _activeToolCounter;
}

/** Initialize the OTel NodeSDK. Must be called before any tool handlers run. */
export function initOTel(config: Config): NodeSDK {
  setMinSeverity(config.logLevel);
  loggerName = config.serviceName;

  const resource = defaultResource().merge(
    resourceFromAttributes({
      [ATTR_SERVICE_NAME]: config.serviceName,
      [ATTR_SERVICE_VERSION]: VERSION,
      "deployment.environment": config.deploymentEnv,
    }),
  );

  const sdk = new NodeSDK({
    resource,
    instrumentations: [new UndiciInstrumentation()],
    spanProcessors: [new BatchSpanProcessor(new OTLPTraceExporter())],
    metricReaders: [
      new PeriodicExportingMetricReader({
        exporter: new OTLPMetricExporter(),
        exportIntervalMillis: 10_000,
        exportTimeoutMillis: 5_000,
      }),
    ],
    logRecordProcessors: [new BatchLogRecordProcessor(new OTLPLogExporter())],
  });

  try {
    sdk.start();
  } catch (err) {
    // eslint-disable-next-line no-console -- OTel SDK failed to start; console is the only output
    console.error(
      "Failed to start OTel SDK, continuing without telemetry:",
      err instanceof Error ? err.message : err,
    );
  }

  emitLog(
    SeverityNumber.INFO,
    `OTel SDK initialized for service: ${config.serviceName}`,
  );
  return sdk;
}

/** Gracefully shut down the OTel SDK, flushing pending telemetry. */
export async function shutdownOTel(sdk: NodeSDK): Promise<void> {
  emitLog(SeverityNumber.INFO, "Shutting down OTel SDK...");
  try {
    await Promise.race([
      sdk.shutdown(),
      new Promise<never>((_, reject) =>
        setTimeout(() => reject(new Error("OTel shutdown timed out")), 4_000),
      ),
    ]);
    // eslint-disable-next-line no-console -- OTel log processor is shut down; console is the only output
    console.error("OTel SDK shut down successfully");
  } catch (err) {
    // eslint-disable-next-line no-console -- OTel log processor is shut down; console is the only output
    console.error(
      "Error shutting down OTel SDK:",
      err instanceof Error ? err.message : String(err),
    );
  }
}
