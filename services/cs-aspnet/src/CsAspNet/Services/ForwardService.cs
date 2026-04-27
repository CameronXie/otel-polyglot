using System.Diagnostics;
using System.Diagnostics.CodeAnalysis;
using CsAspNet.Configuration;
using CsAspNet.Models;
using CsAspNet.Telemetry;
using Microsoft.Extensions.Options;
using OpenTelemetry;

namespace CsAspNet.Services;

/// <summary>
/// Handles concurrent fan-out of GET requests to configured URLs with
/// OpenTelemetry spans, metrics, and structured logging.
/// </summary>
public class ForwardService(
    IOptions<ServiceOptions> options,
    HttpClient httpClient,
    ForwardMetrics metrics,
    ILogger<ForwardService> logger) : IForwardService
{
    private static readonly ActivitySource ActivitySource = new(TelemetryNames.Source);

    private const int MaxResponseBodySize = 1 << 20; // 1 MB

    private readonly ServiceOptions _options = options.Value;

    [SuppressMessage("Design", "CA1031:Do not catch general exception types",
        Justification = "Fan-out must isolate per-URL failures")]
    public async Task<ForwardResponse> ForwardAsync(CancellationToken cancellationToken)
    {
        var urls = _options.GetForwardUrls();

        using var batchSpan = ActivitySource.StartActivity("forward.batch", ActivityKind.Internal);
        if (batchSpan is not null)
        {
            batchSpan.SetTag("forward.urls", string.Join(",", urls));
            batchSpan.SetTag("forward.batch_size", urls.Count);
        }

        var baggage = Baggage.GetBaggage();
        logger.LogInformation("Starting forward batch (url.count={UrlCount}, baggage={Baggage})",
            urls.Count, baggage);

        try
        {
            var tasks = urls.Select(url => ForwardSingleAsync(url, cancellationToken));
            var results = await Task.WhenAll(tasks);

            logger.LogInformation("Forward batch completed (results.count={ResultsCount})",
                results.Length);

            batchSpan?.SetStatus(ActivityStatusCode.Ok);

            return new ForwardResponse([.. results]);
        }
        catch (OperationCanceledException)
        {
            logger.LogInformation("Forward batch cancelled");
            throw;
        }
        catch (Exception ex)
        {
            logger.LogError(ex, "Batch processing failed");
            batchSpan?.SetStatus(ActivityStatusCode.Error, ex.Message);
            batchSpan?.AddException(ex);
            throw;
        }
    }

    private async Task<ForwardResult> ForwardSingleAsync(string rawUrl, CancellationToken cancellationToken)
    {
        var startTimestamp = Stopwatch.GetTimestamp();

        if (!Uri.TryCreate(rawUrl, UriKind.Absolute, out var uri))
        {
            logger.LogWarning("Invalid URL in forward list: {Url}", rawUrl);
            return new ForwardResult(Url: rawUrl, Error: "invalid url",
                DurationSeconds: GetElapsedSeconds(startTimestamp));
        }

        using var span = ActivitySource.StartActivity("forward.request", ActivityKind.Client);
        span?.SetTag("url.full", rawUrl);
        span?.SetTag("http.request.method", "GET");
        span?.SetTag("server.address", uri.Host);
        span?.SetTag("url.scheme", uri.Scheme);

        try
        {
            using var response = await httpClient.GetAsync(uri, HttpCompletionOption.ResponseHeadersRead, cancellationToken);
            var duration = GetElapsedSeconds(startTimestamp);

            metrics.RecordRequest(uri.Host, uri.Scheme, duration);

            span?.SetTag("http.response.status_code", (int)response.StatusCode);

            if (!response.IsSuccessStatusCode)
            {
                if ((int)response.StatusCode >= 400)
                    span?.SetStatus(ActivityStatusCode.Error, $"Upstream returned {(int)response.StatusCode}");

                logger.LogWarning(
                    "Upstream returned error status (http.response.status_code={StatusCode})",
                    (int)response.StatusCode);
                return new ForwardResult(uri.ToString(), (int)response.StatusCode,
                    Body: null, Error: null, DurationSeconds: duration);
            }

            logger.LogInformation("Request completed successfully");

            var body = await ReadResponseBodyAsync(response, cancellationToken);

            span?.SetStatus(ActivityStatusCode.Ok);

            return new ForwardResult(uri.ToString(), (int)response.StatusCode,
                body, DurationSeconds: duration);
        }
        catch (OperationCanceledException)
        {
            throw;
        }
        catch (Exception ex)
        {
            var duration = GetElapsedSeconds(startTimestamp);

            metrics.RecordRequest(uri.Host, uri.Scheme, duration);

            if (ex is HttpRequestException)
            {
                logger.LogError(ex, "Failed to execute request");
            }
            else
            {
                logger.LogError(ex, "Unexpected error in forward task");
            }

            span?.SetStatus(ActivityStatusCode.Error, ex.Message);

            return new ForwardResult(Url: rawUrl, Error: ex.Message,
                DurationSeconds: duration);
        }
    }

    private static async Task<string?> ReadResponseBodyAsync(HttpResponseMessage response, CancellationToken ct)
    {
        try
        {
            await using var stream = await response.Content.ReadAsStreamAsync(ct);
            using var reader = new StreamReader(stream, leaveOpen: true);
            var buffer = new char[MaxResponseBodySize];
            var bytesRead = await reader.ReadAsync(buffer.AsMemory());
            return new string(buffer, 0, bytesRead);
        }
        catch (OperationCanceledException)
        {
            throw;
        }
        catch
        {
            return null;
        }
    }

    private static double GetElapsedSeconds(long startTimestamp) =>
        Stopwatch.GetElapsedTime(startTimestamp).TotalSeconds;
}
