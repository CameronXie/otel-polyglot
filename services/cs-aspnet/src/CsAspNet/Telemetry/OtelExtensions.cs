using System.Reflection;
using CsAspNet.Configuration;
using OpenTelemetry.Logs;
using OpenTelemetry.Metrics;
using OpenTelemetry.Resources;
using OpenTelemetry.Trace;

namespace CsAspNet.Telemetry;

/// <summary>
/// Extension methods for registering OpenTelemetry SDK with the DI container.
/// </summary>
public static class OtelExtensions
{
    private static readonly string ServiceVersion =
        Assembly.GetExecutingAssembly().GetName().Version?.ToString() ?? "0.0.0";

    /// <summary>
    /// Registers tracing, metrics, and logging providers with the OTLP gRPC exporter.
    /// </summary>
    public static IServiceCollection AddOtel(
        this IServiceCollection services,
        IConfiguration configuration,
        ServiceOptions serviceOptions)
    {
        var serviceName = !string.IsNullOrEmpty(serviceOptions.ServiceName)
            ? serviceOptions.ServiceName
            : configuration["OTEL_SERVICE_NAME"] ?? "unknown_service";

        services.AddOpenTelemetry()
            .ConfigureResource(resource => resource
                .AddService(serviceName: serviceName, serviceVersion: ServiceVersion)
                .AddEnvironmentVariableDetector())
            .WithTracing(tracing => tracing
                .AddSource(TelemetryNames.Source)
                .AddAspNetCoreInstrumentation()
                .AddHttpClientInstrumentation()
                .AddOtlpExporter())
            .WithMetrics(metrics => metrics
                .AddMeter(TelemetryNames.Source)
                .AddAspNetCoreInstrumentation()
                .AddView("forward.duration", new ExplicitBucketHistogramConfiguration
                {
                    Boundaries = [0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10]
                })
                .AddOtlpExporter())
            .WithLogging(logging =>
            {
                logging.AddOtlpExporter();
            });

        return services;
    }
}
