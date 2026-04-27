using Microsoft.AspNetCore.Mvc.Testing;

namespace CsAspNet.Tests;

/// <summary>
/// Test factory that suppresses OTel exporters to avoid cert/connection errors in CI.
/// Clears OTel environment variables before the application builds.
/// </summary>
public class CustomWebApplicationFactory : WebApplicationFactory<Program>
{
    public CustomWebApplicationFactory()
    {
        // Clear before the app builds in the base constructor
        Environment.SetEnvironmentVariable("OTEL_EXPORTER_OTLP_ENDPOINT", null);
        Environment.SetEnvironmentVariable("OTEL_EXPORTER_OTLP_CERTIFICATE", null);
        Environment.SetEnvironmentVariable("OTEL_EXPORTER_OTLP_INSECURE", null);
        Environment.SetEnvironmentVariable("OTEL_LOGS_EXPORTER", null);
        Environment.SetEnvironmentVariable("CS_ASPNET_FORWARD_URLS", null);
    }
}
