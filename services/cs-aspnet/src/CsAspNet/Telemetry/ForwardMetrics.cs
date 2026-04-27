using System.Diagnostics;
using System.Diagnostics.Metrics;

namespace CsAspNet.Telemetry;

/// <summary>
/// Custom metrics for forward requests. Uses <see cref="IMeterFactory"/> for DI-aware
/// meter lifecycle.
/// </summary>
public class ForwardMetrics
{
    private readonly Counter<long> _requestCounter;
    private readonly Histogram<double> _durationHistogram;

    public ForwardMetrics(IMeterFactory meterFactory)
    {
        var meter = meterFactory.Create(TelemetryNames.Source);

        _requestCounter = meter.CreateCounter<long>(
            "forward.requests",
            unit: "1",
            description: "Total outbound forward requests"
        );

        _durationHistogram = meter.CreateHistogram<double>(
            "forward.duration",
            unit: "s",
            description: "Outbound forward request duration"
        );
    }

    public void RecordRequest(string serverAddress, string urlScheme, double duration)
    {
        var tags = new TagList
        {
            { "server.address", serverAddress },
            { "url.scheme", urlScheme }
        };

        _requestCounter.Add(1, tags);
        _durationHistogram.Record(duration, tags);
    }
}
