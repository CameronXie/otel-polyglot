using CsAspNet.Configuration;
using CsAspNet.Models;
using CsAspNet.Services;
using CsAspNet.Telemetry;

var builder = WebApplication.CreateBuilder(args);

builder.Configuration.AddPrefixedEnvVars();

builder.Services.AddControllers();
builder.Services.AddOpenApi();

builder.Services.Configure<ServiceOptions>(
    builder.Configuration.GetSection(ServiceOptions.SectionName));

var serviceOptions = new ServiceOptions();
builder.Configuration.GetSection(ServiceOptions.SectionName).Bind(serviceOptions);
builder.WebHost.UseUrls($"http://0.0.0.0:{serviceOptions.Port}");

builder.Logging.SetMinimumLevel(ParseLogLevel(serviceOptions.LogLevel));
builder.Logging.AddOpenTelemetry(logging => logging.IncludeFormattedMessage = true);

builder.Services.AddOtel(builder.Configuration, serviceOptions);

builder.Services.AddSingleton<ForwardMetrics>();
builder.Services.AddHttpClient<IForwardService, ForwardService>(client =>
    client.Timeout = TimeSpan.FromSeconds(2));

var app = builder.Build();

if (app.Environment.IsDevelopment())
{
    app.MapOpenApi();
}

app.MapGet("/health", () => Results.Ok(new HealthResponse()));
app.MapControllers();

app.Run();

static LogLevel ParseLogLevel(string value) => value.Trim().ToLowerInvariant() switch
{
    "debug" => LogLevel.Debug,
    "info" or "information" => LogLevel.Information,
    "warning" or "warn" => LogLevel.Warning,
    "error" => LogLevel.Error,
    _ => LogLevel.Information,
};

public partial class Program;
