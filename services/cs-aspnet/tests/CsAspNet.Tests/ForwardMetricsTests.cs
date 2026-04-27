using System.Diagnostics.Metrics;
using CsAspNet.Telemetry;

namespace CsAspNet.Tests;

public class ForwardMetricsTests
{
    [Fact]
    public void RecordRequest_IncrementsCounterAndRecordsDuration()
    {
        using var meter = new Meter(TelemetryNames.Source);
        var factory = new TestMeterFactory(meter);
        var metrics = new ForwardMetrics(factory);

        long totalRequests = 0;
        var durations = new List<double>();

        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, listener) =>
        {
            if (instrument.Name is "forward.requests" or "forward.duration")
                listener.EnableMeasurementEvents(instrument);
        };
        listener.SetMeasurementEventCallback<long>((_, value, _, _) => totalRequests += value);
        listener.SetMeasurementEventCallback<double>((_, value, _, _) => durations.Add(value));
        listener.Start();

        metrics.RecordRequest("example.com", "https", 0.5);
        metrics.RecordRequest("api.example.com", "http", 1.2);

        listener.RecordObservableInstruments();

        Assert.Equal(2, totalRequests);
        Assert.Equal([0.5, 1.2], durations);
    }

    private sealed class TestMeterFactory(Meter meter) : IMeterFactory
    {
        public Meter Create(MeterOptions options) => meter;
        public void Dispose() { }
    }
}
