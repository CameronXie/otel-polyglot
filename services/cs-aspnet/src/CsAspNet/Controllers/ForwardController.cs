using CsAspNet.Models;
using CsAspNet.Services;
using Microsoft.AspNetCore.Mvc;

namespace CsAspNet.Controllers;

[ApiController]
[Route("forward")]
public class ForwardController(IForwardService forwardService) : ControllerBase
{
    [HttpGet]
    public async Task<ActionResult<ForwardResponse>> Get(CancellationToken cancellationToken)
    {
        try
        {
            var result = await forwardService.ForwardAsync(cancellationToken);
            return Ok(result);
        }
        catch (OperationCanceledException)
        {
            throw;
        }
        catch
        {
            return StatusCode(500, new ErrorResponse("batch processing failed"));
        }
    }
}
