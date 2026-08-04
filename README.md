# 🧠 AI Control Plane — Hybrid Architecture   [![License](https://img.shields.io/badge/License-All%20Rights%20Reserved-red?style=for-the-badge)](#license)

A fully containerized, polyglot distributed system with **Kong Gateway** as the API entry point, backed by **PostgreSQL + pgvector**, **Redis**, and multiple microservices.

## Architecture

```
                       ┌──────────────────┐
                       │   Kong Gateway   │  :8000 (proxy)  :8001 (admin)
                       │   (DB-less)      │
                       └──────┬───────┬───┘
                              │       │
                    /v1/ai    │       │  /v1/admin
                              │       │
              ┌───────────────▼─┐   ┌─▼───────────────────┐
              │   ai-proxy      │   │   monolith-core      │
              │   (Go :8080)    │   │   (Java/Spring :8081)│
              │   Middleware    │   │   Users, Payments    │
              └───────┬─────────┘   └──────────┬───────────┘
                      │                        │
          ┌───────────▼─────────┐              │
          │   agent-brain       │              │
          │   (Python :8082)    │◄─────────────┘
          │   Orchestration     │
          └───────────┬─────────┘
                      │ http://data-bridge:8083
          ┌───────────▼─────────┐
          │   data-bridge       │
          │   (Java :8083)      │
          │   Legacy Connectors │
          └─────────────────────┘

      ┌────────────┐       ┌────────────┐
      │ PostgreSQL │       │   Redis    │
      │ + pgvector │       │   :6379    │
      │   :5432    │       │   State    │
      └────────────┘       └────────────┘
```

## Quick Start

```bash
# 1. Copy env file and fill in secrets
cp .env.example .env

# 2. Build and start everything
docker compose up --build

# 3. Test the routes
curl http://localhost:8000/v1/ai/health     # → Go ai-proxy
curl http://localhost:8000/v1/admin/health   # → Java monolith-core
```

## Services

| Service | Language | Port | Purpose |
|---------|----------|------|---------|
| **kong** | — | 8000 | API Gateway (DB-less) |
| **ai-proxy** | Go | 8080 | Middleware / Security layer |
| **monolith-core** | Java | 8081 | Users, Payments, UI API |
| **agent-brain** | Python | 8082 | AI Orchestration State Graph |
| **data-bridge** | Java | 8083 | Legacy SQL / Oracle Connectors |
| **postgres** | — | 5432 | Database (pgvector enabled) |
| **redis** | — | 6379 | Session / Agent State Cache |

## Inter-Service Communication

Services communicate over the Docker bridge network using their **service names** as DNS hostnames:

```
agent-brain  →  http://data-bridge:8083/query?source=oracle
ai-proxy     →  http://agent-brain:8082/orchestrate
```

## Environment Variables

All secrets live in `.env` (never committed). See `.env.example` for the template.

## Current Project Status

*(For a detailed breakdown, see [PROJECT_STATUS.md](./PROJECT_STATUS.md))*

**✅ Completed / Implemented:**
- **Infrastructure:** Docker compose ecosystem fully running with Kong API Gateway (DB-less routing, rate limiting).
- **ai-proxy (Go):** Core middleware features completed (Prompt injection detection, PII scrubbing, audit logging, guardrails, rate limiting).
- **data-bridge (Java):** Text-to-SQL logic and RAG engine connected to read-only database are implemented and functional via LangChain4j.

**🚧 Pending / To-Do:**
- **monolith-core (Java):** Currently just a stub. Needs User Auth (JWT), Payments integration, and actual UI API endpoints.
- **agent-brain (Python):** Currently a stub. Needs actual AI orchestration state graph (e.g. LangGraph) and Redis state persistence implementation.
- **Frontend / UI:** Not started. Dashboard for administrators/users needs to be built.

