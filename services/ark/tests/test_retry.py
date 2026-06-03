import pytest
from unittest.mock import AsyncMock
from core.retry import retry_async


@pytest.mark.asyncio
async def test_retry_async_success_first_attempt():
    fn = AsyncMock(return_value="success")
    result = await retry_async(fn, attempts=3, base_delay=1.0)
    assert result == "success"
    assert fn.call_count == 1


@pytest.mark.asyncio
async def test_retry_async_success_second_attempt():
    fn = AsyncMock(side_effect=[Exception("transient"), "success"])
    result = await retry_async(fn, attempts=3, base_delay=1.0)
    assert result == "success"
    assert fn.call_count == 2


@pytest.mark.asyncio
async def test_retry_async_raises_after_all_attempts():
    fn = AsyncMock(side_effect=Exception("permanent"))
    with pytest.raises(Exception, match="permanent"):
        await retry_async(fn, attempts=3, base_delay=1.0)
    assert fn.call_count == 3


@pytest.mark.asyncio
async def test_retry_async_exponential_backoff():
    import asyncio
    from unittest.mock import patch

    fn = AsyncMock(side_effect=[Exception("1"), Exception("2"), "success"])
    delays = []

    original_sleep = asyncio.sleep

    async def capture_sleep(delay):
        delays.append(delay)
        await original_sleep(0)  # Don't actually wait

    with patch("asyncio.sleep", side_effect=capture_sleep):
        result = await retry_async(fn, attempts=3, base_delay=1.0)

    assert result == "success"
    assert fn.call_count == 3
    assert delays == [1.0, 2.0]  # Exponential: 1s, 2s (no delay after final success)


@pytest.mark.asyncio
async def test_retry_async_non_retryable_exception_raises_immediately():
    fn = AsyncMock(side_effect=ValueError("not transient"))
    with pytest.raises(ValueError, match="not transient"):
        await retry_async(fn, attempts=3, base_delay=1.0, exceptions=(RuntimeError,))
    assert fn.call_count == 1  # Must not retry
