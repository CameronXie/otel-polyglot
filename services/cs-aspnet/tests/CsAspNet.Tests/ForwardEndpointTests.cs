using System.Net;
using System.Net.Http.Json;
using CsAspNet.Models;
using CsAspNet.Services;
using Microsoft.Extensions.DependencyInjection;

namespace CsAspNet.Tests;

public class ForwardEndpointTests : IClassFixture<CustomWebApplicationFactory>
{
    private readonly HttpClient _client;

    public ForwardEndpointTests(CustomWebApplicationFactory factory)
    {
        _client = factory.WithWebHostBuilder(builder =>
        {
            builder.UseSetting("CS_ASPNET:ForwardUrls", "https://test.local/get");
            builder.ConfigureServices(services =>
            {
                services.AddHttpClient<IForwardService, ForwardService>(c =>
                    c.Timeout = TimeSpan.FromSeconds(2)
                ).ConfigurePrimaryHttpMessageHandler(() =>
                    new StubHandler(HttpStatusCode.OK, """{"stub":true}""")
                );
            });
        }).CreateClient();
    }

    [Fact]
    public async Task GetForward_Returns200WithResults()
    {
        var response = await _client.GetAsync("/forward");
        var body = await response.Content.ReadFromJsonAsync<ForwardResponse>();

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.NotNull(body);
        Assert.Single(body.Results);
        Assert.Equal("https://test.local/get", body.Results[0].Url);
        Assert.Equal(200, body.Results[0].StatusCode);
        Assert.NotNull(body.Results[0].Body);
    }

    [Fact]
    public async Task GetForward_NoURLs_ReturnsEmptyResults()
    {
        var client = new CustomWebApplicationFactory().WithWebHostBuilder(builder =>
        {
            builder.UseSetting("CS_ASPNET:ForwardUrls", "");
        }).CreateClient();

        var response = await client.GetAsync("/forward");
        var body = await response.Content.ReadFromJsonAsync<ForwardResponse>();

        Assert.NotNull(body);
        Assert.Empty(body.Results);
    }

    public static TheoryData<string, string?, bool> BadUrlCases => new()
    {
        { "http://localhost:1/nonexistent", null, false },
        { "not-a-url", "invalid url", false },
    };

    [Theory]
    [MemberData(nameof(BadUrlCases))]
    public async Task GetForward_BadUrl_ReturnsErrorResult(
        string urls, string? expectedError, bool expectStatusCode)
    {
        var client = new CustomWebApplicationFactory().WithWebHostBuilder(builder =>
        {
            builder.UseSetting("CS_ASPNET:ForwardUrls", urls);
        }).CreateClient();

        var response = await client.GetAsync("/forward");
        var body = await response.Content.ReadFromJsonAsync<ForwardResponse>();

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.NotNull(body);
        Assert.Single(body.Results);
        Assert.NotNull(body.Results[0].Error);
        if (expectedError is not null)
            Assert.Equal(expectedError, body.Results[0].Error);
        Assert.Equal(expectStatusCode, body.Results[0].StatusCode is not null);
    }

    [Fact]
    public async Task GetForward_Upstream4xx_ReturnsStatusCode()
    {
        var client = new CustomWebApplicationFactory().WithWebHostBuilder(builder =>
        {
            builder.UseSetting("CS_ASPNET:ForwardUrls", "https://test.local/missing");
            builder.ConfigureServices(services =>
            {
                services.AddHttpClient<IForwardService, ForwardService>(c =>
                        c.Timeout = TimeSpan.FromSeconds(2))
                    .ConfigurePrimaryHttpMessageHandler(() =>
                        new StubHandler(HttpStatusCode.NotFound, "not found"));
            });
        }).CreateClient();

        var response = await client.GetAsync("/forward");
        var body = await response.Content.ReadFromJsonAsync<ForwardResponse>();

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.NotNull(body);
        Assert.Single(body.Results);
        Assert.Equal(404, body.Results[0].StatusCode);
        Assert.Null(body.Results[0].Error);
    }

    [Fact]
    public async Task GetForward_ServiceThrows_Returns500()
    {
        var client = new CustomWebApplicationFactory().WithWebHostBuilder(builder =>
        {
            builder.UseSetting("CS_ASPNET:ForwardUrls", "https://test.local/get");
            builder.ConfigureServices(services =>
            {
                services.AddSingleton<IForwardService>(new ThrowingForwardService());
            });
        }).CreateClient();

        var response = await client.GetAsync("/forward");

        Assert.Equal(HttpStatusCode.InternalServerError, response.StatusCode);
        var body = await response.Content.ReadFromJsonAsync<ErrorResponse>();
        Assert.NotNull(body);
        Assert.Equal("batch processing failed", body.Error);
    }

    public static TheoryData<Exception, string> ExceptionCases => new()
    {
        { new HttpRequestException("connection refused"), "connection refused" },
        { new InvalidOperationException("unexpected error"), "unexpected error" },
    };

    [Theory]
    [MemberData(nameof(ExceptionCases))]
    public async Task GetForward_HandlerException_ReturnsErrorResult(
        Exception exception, string expectedError)
    {
        var client = new CustomWebApplicationFactory().WithWebHostBuilder(builder =>
        {
            builder.UseSetting("CS_ASPNET:ForwardUrls", "https://test.local/get");
            builder.ConfigureServices(services =>
            {
                services.AddHttpClient<IForwardService, ForwardService>(c =>
                    c.Timeout = TimeSpan.FromSeconds(2)
                ).ConfigurePrimaryHttpMessageHandler(() =>
                    new ThrowingHandler(exception));
            });
        }).CreateClient();

        var response = await client.GetAsync("/forward");
        var body = await response.Content.ReadFromJsonAsync<ForwardResponse>();

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.NotNull(body);
        Assert.Single(body.Results);
        Assert.Equal(expectedError, body.Results[0].Error);
    }

    [Fact]
    public async Task GetForward_Cancelled_PropagatesCancellation()
    {
        var client = new CustomWebApplicationFactory().WithWebHostBuilder(builder =>
        {
            builder.UseSetting("CS_ASPNET:ForwardUrls", "https://test.local/get");
            builder.ConfigureServices(services =>
            {
                services.AddHttpClient<IForwardService, ForwardService>(c =>
                    c.Timeout = TimeSpan.FromSeconds(2)
                ).ConfigurePrimaryHttpMessageHandler(() =>
                    new ThrowingHandler(new OperationCanceledException("cancelled")));
            });
        }).CreateClient();

        try
        {
            var response = await client.GetAsync("/forward");
            Assert.Equal(HttpStatusCode.InternalServerError, response.StatusCode);
        }
        catch (HttpRequestException)
        {
            // Connection aborted by server — OCE propagated correctly
        }
    }

    [Fact]
    public async Task GetForward_StreamReadFailure_ReturnsResultWithNullBody()
    {
        var client = new CustomWebApplicationFactory().WithWebHostBuilder(builder =>
        {
            builder.UseSetting("CS_ASPNET:ForwardUrls", "https://test.local/get");
            builder.ConfigureServices(services =>
            {
                services.AddHttpClient<IForwardService, ForwardService>(c =>
                    c.Timeout = TimeSpan.FromSeconds(2)
                ).ConfigurePrimaryHttpMessageHandler(() => new FaultingStreamHandler());
            });
        }).CreateClient();

        var response = await client.GetAsync("/forward");
        var body = await response.Content.ReadFromJsonAsync<ForwardResponse>();

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.NotNull(body);
        Assert.Single(body.Results);
        Assert.Equal(200, body.Results[0].StatusCode);
        Assert.Null(body.Results[0].Body);
    }

    [Fact]
    public async Task GetForward_StreamReadCancelled_PropagatesCancellation()
    {
        var client = new CustomWebApplicationFactory().WithWebHostBuilder(builder =>
        {
            builder.UseSetting("CS_ASPNET:ForwardUrls", "https://test.local/get");
            builder.ConfigureServices(services =>
            {
                services.AddHttpClient<IForwardService, ForwardService>(c =>
                    c.Timeout = TimeSpan.FromSeconds(2)
                ).ConfigurePrimaryHttpMessageHandler(() => new CancellingReadStreamHandler());
            });
        }).CreateClient();

        try
        {
            var response = await client.GetAsync("/forward");
            Assert.Equal(HttpStatusCode.InternalServerError, response.StatusCode);
        }
        catch (HttpRequestException)
        {
            // Connection aborted by server — OCE propagated correctly
        }
    }

    private sealed class StubHandler(HttpStatusCode statusCode, string content) : HttpMessageHandler
    {
        protected override Task<HttpResponseMessage> SendAsync(
            HttpRequestMessage request, CancellationToken cancellationToken) =>
            Task.FromResult(new HttpResponseMessage(statusCode)
            {
                Content = new StringContent(content, System.Text.Encoding.UTF8,
                    "application/json")
            });
    }

    private sealed class ThrowingForwardService : IForwardService
    {
        public Task<ForwardResponse> ForwardAsync(CancellationToken cancellationToken) =>
            throw new Exception("test failure");
    }
}
