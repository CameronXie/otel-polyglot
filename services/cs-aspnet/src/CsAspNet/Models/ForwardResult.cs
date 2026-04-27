using System.Text.Json.Serialization;

namespace CsAspNet.Models;

public record ForwardResult(
    string Url,
    [property: JsonPropertyName("status_code")] int? StatusCode = null,
    string? Body = null,
    string? Error = null,
    [property: JsonPropertyName("duration_seconds")] double? DurationSeconds = null
);
