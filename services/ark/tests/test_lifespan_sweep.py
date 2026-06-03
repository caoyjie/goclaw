import pytest
from unittest.mock import AsyncMock, patch
from fastapi import FastAPI


@pytest.mark.asyncio
async def test_lifespan_calls_both_sweeps():
    """FastAPI lifespan startup must call both sweep functions."""
    with patch("main.create_pool", new_callable=AsyncMock) as mock_pool_factory, \
         patch("main.run_migrations", new_callable=AsyncMock), \
         patch("main.sweep_stale_indexing", new_callable=AsyncMock) as mock_idx, \
         patch("main.sweep_stale_deindexing", new_callable=AsyncMock) as mock_deidx:
        mock_pool = AsyncMock()
        mock_pool_factory.return_value = mock_pool
        mock_idx.return_value = 0
        mock_deidx.return_value = 0

        from main import lifespan
        app = FastAPI()
        async with lifespan(app):
            pass

        mock_idx.assert_awaited_once()
        mock_deidx.assert_awaited_once()


@pytest.mark.asyncio
async def test_lifespan_closes_pool_on_startup_error():
    """Pool must be closed even if startup fails."""
    with patch("main.create_pool", new_callable=AsyncMock) as mock_pool_factory, \
         patch("main.run_migrations", new_callable=AsyncMock) as mock_migrations, \
         patch("main.sweep_stale_indexing", new_callable=AsyncMock), \
         patch("main.sweep_stale_deindexing", new_callable=AsyncMock):
        mock_pool = AsyncMock()
        mock_pool_factory.return_value = mock_pool
        mock_migrations.side_effect = Exception("migration failed")

        from main import lifespan
        app = FastAPI()

        try:
            async with lifespan(app):
                pass
        except Exception:
            pass  # Expected

        mock_pool.close.assert_awaited_once()


@pytest.mark.asyncio
async def test_lifespan_closes_pool_on_shutdown():
    """Pool must be closed on normal shutdown."""
    with patch("main.create_pool", new_callable=AsyncMock) as mock_pool_factory, \
         patch("main.run_migrations", new_callable=AsyncMock), \
         patch("main.sweep_stale_indexing", new_callable=AsyncMock) as mock_idx, \
         patch("main.sweep_stale_deindexing", new_callable=AsyncMock) as mock_deidx:
        mock_pool = AsyncMock()
        mock_pool_factory.return_value = mock_pool
        mock_idx.return_value = 0
        mock_deidx.return_value = 0

        from main import lifespan
        app = FastAPI()
        async with lifespan(app):
            pass

        mock_pool.close.assert_awaited_once()


@pytest.mark.asyncio
async def test_lifespan_logs_warning_when_stale_rows_found():
    """Warning must be logged when sweeps recover stale rows."""
    with patch("main.create_pool", new_callable=AsyncMock) as mock_pool_factory, \
         patch("main.run_migrations", new_callable=AsyncMock), \
         patch("main.sweep_stale_indexing", new_callable=AsyncMock) as mock_idx, \
         patch("main.sweep_stale_deindexing", new_callable=AsyncMock) as mock_deidx, \
         patch("main.logger") as mock_logger:
        mock_pool = AsyncMock()
        mock_pool_factory.return_value = mock_pool
        mock_idx.return_value = 2
        mock_deidx.return_value = 1

        from main import lifespan
        app = FastAPI()
        async with lifespan(app):
            pass

        mock_logger.warning.assert_called_once()
        call_args = mock_logger.warning.call_args[0]
        assert "stale indexing" in call_args[0]
        assert "stale deindexing" in call_args[0]
        assert call_args[1] == 2
        assert call_args[2] == 1
