import logging
import asyncpg

logger = logging.getLogger(__name__)


async def create_pool(database_url: str) -> asyncpg.Pool:
    """Create asyncpg connection pool."""
    return await asyncpg.create_pool(database_url)


async def run_migrations(pool: asyncpg.Pool) -> None:
    """Run database migrations (stub for now)."""
    # TODO: Implement actual migration logic
    pass

_SWEEP_INDEXING_SQL = """
UPDATE ark_files
   SET status    = 'stored',
       error_msg = 'Reset by startup sweep: process restarted while indexing',
       updated_at = now()
 WHERE status = 'indexing'
   AND updated_at < now() - ($1 || ' minutes')::interval
"""

_SWEEP_DEINDEXING_SQL = """
UPDATE ark_files
   SET status    = 'indexed',
       error_msg = 'Reset by startup sweep: process restarted while deindexing',
       updated_at = now()
 WHERE status = 'deindexing'
   AND updated_at < now() - ($1 || ' minutes')::interval
"""


async def sweep_stale_indexing(pool, *, stale_minutes: int = 15) -> int:
    try:
        async with pool.acquire() as conn:
            status = await conn.execute(_SWEEP_INDEXING_SQL, stale_minutes)
        try:
            count = int(status.split()[-1])
        except (ValueError, IndexError) as e:
            logger.error("Failed to parse sweep result '%s': %s", status, e)
            return 0
        logger.info("Startup sweep: reset %d stale 'indexing' rows to 'stored'", count)
        return count
    except Exception as e:
        logger.error("Startup sweep failed for indexing: %s", e)
        return 0


async def sweep_stale_deindexing(pool, *, stale_minutes: int = 15) -> int:
    try:
        async with pool.acquire() as conn:
            status = await conn.execute(_SWEEP_DEINDEXING_SQL, stale_minutes)
        try:
            count = int(status.split()[-1])
        except (ValueError, IndexError) as e:
            logger.error("Failed to parse sweep result '%s': %s", status, e)
            return 0
        logger.info("Startup sweep: reset %d stale 'deindexing' rows to 'indexed'", count)
        return count
    except Exception as e:
        logger.error("Startup sweep failed for deindexing: %s", e)
        return 0
