namespace CsAspNet.Configuration;

/// <summary>
/// Maps flat <c>CS_ASPNET_*</c> environment variables (single underscore) to the
/// <c>CS_ASPNET:*</c> config hierarchy so .NET's options pattern binding works.
/// </summary>
internal sealed class PrefixedEnvVarConfigurationSource : IConfigurationSource
{
    private static readonly Dictionary<string, string> Mapping = new()
    {
        ["CS_ASPNET_PORT"] = "CS_ASPNET:Port",
        ["CS_ASPNET_FORWARD_URLS"] = "CS_ASPNET:ForwardUrls",
        ["CS_ASPNET_SERVICE_NAME"] = "CS_ASPNET:ServiceName",
        ["CS_ASPNET_LOG_LEVEL"] = "CS_ASPNET:LogLevel",
    };

    public IConfigurationProvider Build(IConfigurationBuilder builder) =>
        new Provider(Mapping);

    private sealed class Provider(Dictionary<string, string> mapping) : ConfigurationProvider
    {
        public override void Load()
        {
            foreach (var (envVar, configKey) in mapping)
            {
                var value = Environment.GetEnvironmentVariable(envVar);
                if (value is not null)
                    Set(configKey, value);
            }
        }
    }
}

public static class PrefixedEnvVarConfigurationExtensions
{
    public static IConfigurationBuilder AddPrefixedEnvVars(this IConfigurationBuilder builder)
    {
        builder.Add(new PrefixedEnvVarConfigurationSource());
        return builder;
    }
}
