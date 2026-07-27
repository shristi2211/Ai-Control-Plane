"""
Agent Brain — Orchestration State Graph
FastAPI service that coordinates AI agent workflows.
"""

import os
from datetime import datetime, timezone

import httpx  # type: ignore
from fastapi import FastAPI, Request  # type: ignore
from fastapi.responses import JSONResponse  # type: ignore

app = FastAPI(title="Agent Brain", version="0.1.0")

# ── Inter-service URLs (Docker DNS) ──────────────────
DATA_BRIDGE_URL = os.getenv("DATA_BRIDGE_URL", "http://data-bridge:8083")
INTERNAL_API_KEY = os.getenv("INTERNAL_API_KEY", "")


@app.get("/health")
async def health():
    """Health check — used by Docker & Kong."""
    return {
        "service": "agent-brain",
        "status": "healthy",
        "timestamp": datetime.now(timezone.utc).isoformat(),
    }


@app.api_route("/{path:path}", methods=["GET", "POST", "PUT", "DELETE", "PATCH"])
async def catch_all(request: Request, path: str):
    """Catch-all placeholder for orchestration logic."""
    return JSONResponse(
        content={
            "service": "agent-brain",
            "status": "ok",
            "message": f"Orchestration endpoint. Path: /{path}",
            "data_bridge_url": DATA_BRIDGE_URL,
            "timestamp": datetime.now(timezone.utc).isoformat(),
        }
    )


# ── Example: calling data-bridge via Docker DNS ─────
async def fetch_from_data_bridge(endpoint: str) -> dict:  # type: ignore[type-arg]
    """
    Demonstrates inter-service communication.
    The agent-brain calls data-bridge by its Docker service name.
    """
    async with httpx.AsyncClient() as client:
        headers = {"X-Internal-Key": INTERNAL_API_KEY}
        resp = await client.get(f"{DATA_BRIDGE_URL}/{endpoint}", headers=headers)
        resp.raise_for_status()
        result: dict = resp.json()  # type: ignore[assignment]
    return result


if __name__ == "__main__":
    import uvicorn  # type: ignore

    port = int(os.getenv("PORT", "8082"))
    uvicorn.run("main:app", host="0.0.0.0", port=port, reload=False)
