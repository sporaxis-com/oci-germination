# syntax=docker/dockerfile:1.7
FROM ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck-nats-micro:v0.3 AS base

FROM ghcr.io/conceptkernel/ck-lib-js:1.2.0 AS cklib_source

FROM python:3.11-slim AS builder
RUN python -m venv /opt/venv && \
    . /opt/venv/bin/activate && \
    pip install --no-cache-dir fastapi uvicorn

COPY --from=cklib_source / /app/cklib/
RUN mkdir -p /app/static /app/cklib && \
    printf 'from fastapi import FastAPI\nfrom fastapi.staticfiles import StaticFiles\nfrom pathlib import Path\n\napp = FastAPI(title="pgck-web", version="0.1.0")\n\nstatic_dir = Path(__file__).parent / "static"\nif static_dir.exists():\n    app.mount("/static", StaticFiles(directory=str(static_dir)), name="static")\n\ncklib_dir = Path(__file__).parent / "cklib"\nif cklib_dir.exists():\n    app.mount("/cklib", StaticFiles(directory=str(cklib_dir)), name="cklib")\n\n@app.get("/")\nasync def root():\n    return {"message": "pgck-web + cklib + NATS WSS", "version": "0.1.0"}\n' > /app/main.py && \
    touch /app/__init__.py

RUN mkdir -p /launcher && \
    printf '#!/bin/sh\nset -e\ncd /opt/pgck-web\nPATH=/opt/venv/bin:$PATH\nexec python -m uvicorn app.main:app --host 0.0.0.0 --port 8000\n' > /launcher/pgck-web-launcher && \
    chmod +x /launcher/pgck-web-launcher

FROM base
WORKDIR /opt/pgck-web

COPY --from=builder /opt/venv /opt/venv
COPY --from=builder /app ./app/
COPY --from=builder /launcher/pgck-web-launcher /usr/local/bin/pgck-web-launcher

EXPOSE 5432 8000 4222 9222
ENTRYPOINT ["/usr/local/bin/ociger-supervisor"]
