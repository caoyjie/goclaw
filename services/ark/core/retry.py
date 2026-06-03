import asyncio
import logging

logger = logging.getLogger(__name__)


async def retry_async(
    fn,
    *,
    attempts: int = 3,
    base_delay: float = 1.0,
    exceptions: tuple[type[Exception], ...] = (Exception,),
    operation: str = "call",
):
    """
    Retry an async callable with exponential backoff.

    Args:
        fn: Async callable to retry
        attempts: Maximum number of attempts (default 3)
        base_delay: Base delay in seconds (default 1.0)
        exceptions: Tuple of exception types to retry (default (Exception,))
        operation: Operation name for logging (default "call")

    Returns:
        Result from fn()

    Raises:
        Last exception if all attempts fail, or immediately if exception is not retryable
    """
    last_exc = None
    for attempt in range(attempts):
        try:
            return await fn()
        except Exception as exc:
            if not isinstance(exc, exceptions):
                raise
            last_exc = exc
            if attempt < attempts - 1:
                delay = base_delay * (2**attempt)
                logger.warning(
                    "%s failed (attempt %d/%d), retrying in %.1fs: %s",
                    operation,
                    attempt + 1,
                    attempts,
                    delay,
                    exc,
                )
                await asyncio.sleep(delay)
    raise last_exc
