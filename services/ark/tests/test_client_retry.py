import pytest
from unittest.mock import AsyncMock, MagicMock
from core.client import VikingDBClient


@pytest.mark.asyncio
async def test_add_doc_retries_on_transient_failure():
    mock_collection = MagicMock()
    mock_collection.add_doc = MagicMock(
        side_effect=[Exception("transient"), Exception("transient"), {"doc_id": "doc123"}]
    )

    client = VikingDBClient(collection=mock_collection, base_delay=0.01)
    result = await client.add_doc("products/manual.pdf")

    assert result == {"doc_id": "doc123"}
    assert mock_collection.add_doc.call_count == 3


@pytest.mark.asyncio
async def test_delete_doc_retries_on_transient_failure():
    mock_collection = MagicMock()
    mock_collection.delete_doc = MagicMock(
        side_effect=[Exception("transient"), None]
    )

    client = VikingDBClient(collection=mock_collection, base_delay=0.01)
    await client.delete_doc("doc123")

    assert mock_collection.delete_doc.call_count == 2


@pytest.mark.asyncio
async def test_add_doc_raises_after_all_retries_exhausted():
    mock_collection = MagicMock()
    mock_collection.add_doc = MagicMock(side_effect=Exception("permanent failure"))

    client = VikingDBClient(collection=mock_collection, base_delay=0.01)
    with pytest.raises(Exception, match="permanent failure"):
        await client.add_doc("products/manual.pdf")

    assert mock_collection.add_doc.call_count == 3


@pytest.mark.asyncio
async def test_delete_doc_raises_after_all_retries_exhausted():
    mock_collection = MagicMock()
    mock_collection.delete_doc = MagicMock(side_effect=Exception("permanent failure"))

    client = VikingDBClient(collection=mock_collection, base_delay=0.01)
    with pytest.raises(Exception, match="permanent failure"):
        await client.delete_doc("doc123")

    assert mock_collection.delete_doc.call_count == 3
