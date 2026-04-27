using CsAspNet.Models;

namespace CsAspNet.Services;

public interface IForwardService
{
    Task<ForwardResponse> ForwardAsync(CancellationToken cancellationToken);
}
