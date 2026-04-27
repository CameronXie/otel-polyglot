namespace CsAspNet.Configuration;

/// <summary>
/// Application options bound to <c>CS_ASPNET_*</c> environment variables.
/// </summary>
public sealed class ServiceOptions
{
    public const string SectionName = "CS_ASPNET";

    public int Port { get; init; } = 8080;
    public string LogLevel { get; init; } = "Information";
    public string ForwardUrls { get; init; } = "";
    public string ServiceName { get; init; } = "";

    /// <summary>
    /// Parses the comma-separated <see cref="ForwardUrls"/> into a list.
    /// </summary>
    public List<string> GetForwardUrls() =>
        string.IsNullOrWhiteSpace(ForwardUrls)
            ? []
            : ForwardUrls.Split(',', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries)
                         .ToList();
}
