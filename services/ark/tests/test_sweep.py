import pytest
from unittest.mock import AsyncMock, MagicMock


def _make_pool(execute_return: str):
    mock_conn = AsyncMock()
    mock_conn.execute = AsyncMock(return_value=execute_return)

    # Create a context manager mock
    mock_cm = MagicMock()
    mock_cm.__aenter__ = AsyncMock(return_value=mock_conn)
    mock_cm.__aexit__ = AsyncMock(return_value=False)

    mock_pool = MagicMock()
    mock_pool.acquire = MagicMock(return_value=mock_cm)
    return mock_pool, mock_conn


@pytest.mark.asyncio
async def test_sweep_indexing_returns_count():
    mock_pool, mock_conn = _make_pool("UPDATE 3")
    from core.db import sweep_stale_indexing
    count = await sweep_stale_indexing(mock_pool, stale_minutes=10)
    assert count == 3
    sql, minutes = mock_conn.execute.call_args.args
    assert "indexing" in sql
    assert "'stored'" in sql
    assert minutes == 10


@pytest.mark.asyncio
async def test_sweep_deindexing_returns_count():
    mock_pool, mock_conn = _make_pool("UPDATE 1")
    from core.db import sweep_stale_deindexing
    count = await sweep_stale_deindexing(mock_pool, stale_minutes=10)
    assert count == 1
    sql, minutes = mock_conn.execute.call_args.args
    assert "deindexing" in sql
    assert "'indexed'" in sql
    assert minutes == 10


@pytest.mark.asyncio
async def test_sweep_returns_zero_when_no_stale_rows():
    mock_pool, mock_conn = _make_pool("UPDATE 0")
    from core.db import sweep_stale_indexing
    count = await sweep_stale_indexing(mock_pool, stale_minutes=10)
    assert count == 0


@pytest.mark.asyncio
async def test_sweep_deindexing_returns_zero_when_no_stale_rows():
    mock_pool, mock_conn = _make_pool("UPDATE 0")
    from core.db import sweep_stale_deindexing
    count = await sweep_stale_deindexing(mock_pool, stale_minutes=10)
    assert count == 0


@pytest.mark.asyncio
async def test_sweep_handles_malformed_result():
    """Sweep should return 0 and log error if result parsing fails."""
    mock_pool, mock_conn = _make_pool("MALFORMED")
    from core.db import sweep_stale_indexing
    count = await sweep_stale_indexing(mock_pool, stale_minutes=10)
    assert count == 0


@pytest.mark.asyncio
async def test_sweep_handles_database_error():
    """Sweep should return 0 and log error if database query fails."""
    mock_conn = AsyncMock()
    mock_conn.execute = AsyncMock(side_effect=Exception("connection lost"))
    mock_pool = MagicMock()
    mock_pool.acquire.return_value.__aenter__ = AsyncMock(return_value=mock_conn)
    mock_pool.acquire.return_value.__aexit__ = AsyncMock(return_value=False)

    from core.db import sweep_stale_indexing
    count = await sweep_stale_indexing(mock_pool, stale_minutes=10)
    assert count == 0
