from contextlib import asynccontextmanager
from fastapi import FastAPI
from core.config import settings
from core.db import create_pool, run_migrations, sweep_stale_indexing, sweep_stale_deindexing
import logging

logger = logging.getLogger(__name__)


@asynccontextmanager
async def lifespan(app: FastAPI):
    pool = await create_pool(settings.DATABASE_URL)
    try:
        await run_migrations(pool)

        idx_count = await sweep_stale_indexing(pool, stale_minutes=settings.STALE_MINUTES)
        deidx_count = await sweep_stale_deindexing(pool, stale_minutes=settings.STALE_MINUTES)
        if idx_count or deidx_count:
            logger.warning(
                "Startup sweep recovered %d stale indexing + %d stale deindexing rows",
                idx_count, deidx_count,
            )

        app.state.pool = pool
        yield
    finally:
        await pool.close()


app = FastAPI(lifespan=lifespan)
