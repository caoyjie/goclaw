import asyncio
from core.retry import retry_async


class VikingDBClient:
    def __init__(self, collection, *, base_delay: float = 1.0):
        self._collection = collection
        self._base_delay = base_delay

    async def add_doc(self, tos_key: str) -> dict:
        """
        Add a document from TOS to the VikingDB collection with retry.

        Args:
            tos_key: TOS object key (e.g. "products/manual.pdf")

        Returns:
            Dict with doc_id and other metadata from VikingDB
        """
        def sync_add():
            return self._collection.add_doc(add_type="tos", tos_path=tos_key)

        return await retry_async(
            lambda: asyncio.to_thread(sync_add),
            attempts=3,
            base_delay=self._base_delay,
            # TODO: Replace with SDK-specific transient error classes when available
            # (e.g., volcengine.VikingDBTransientError)
            exceptions=(Exception,),
            operation=f"VikingDB add_doc({tos_key})",
        )

    async def delete_doc(self, doc_id: str) -> None:
        """
        Delete a document from the VikingDB collection with retry.

        Args:
            doc_id: VikingDB document ID
        """
        def sync_delete():
            return self._collection.delete_doc(doc_id)

        await retry_async(
            lambda: asyncio.to_thread(sync_delete),
            attempts=3,
            base_delay=self._base_delay,
            # TODO: Replace with SDK-specific transient error classes when available
            # (e.g., volcengine.VikingDBTransientError)
            exceptions=(Exception,),
            operation=f"VikingDB delete_doc({doc_id})",
        )
