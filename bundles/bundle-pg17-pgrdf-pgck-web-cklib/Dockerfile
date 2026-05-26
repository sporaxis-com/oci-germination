# syntax=docker/dockerfile:1.7
FROM ghcr.io/sporaxis-com/ociger-pg17-pgrdf-pgck:v0.1.1 AS base

FROM ghcr.io/conceptkernel/ck-lib-js:1.2.0 AS cklib_source

FROM python:3.11-slim AS python_venv
RUN python -m venv /opt/venv && \
    . /opt/venv/bin/activate && \
    pip install --no-cache-dir fastapi uvicorn

FROM base
WORKDIR /opt/pgck-web

COPY --from=python_venv /opt/venv /opt/venv
RUN mkdir -p ./app/static ./app/cklib

# Copy cklib files from ck-lib-js image
COPY --from=cklib_source / ./app/cklib/ 2>/dev/null || echo "cklib placeholder"

RUN printf 'from fastapi import FastAPI\nfrom fastapi.staticfiles import StaticFiles\nfrom pathlib import Path\n\napp = FastAPI(title="pgck-web", version="0.1.0")\n\nstatic_dir = Path(__file__).parent / "static"\nif static_dir.exists():\n    app.mount("/static", StaticFiles(directory=str(static_dir)), name="static")\n\ncklib_dir = Path(__file__).parent / "cklib"\nif cklib_dir.exists():\n    app.mount("/cklib", StaticFiles(directory=str(cklib_dir)), name="cklib")\n\n@app.get("/")\nasync def root():\n    return {"message": "pgck-web + cklib", "version": "0.1.0"}\n' > ./app/main.py

RUN touch ./app/__init__.py
RUN mkdir -p /usr/local/bin && printf '#!/bin/sh\nset -e\ncd /opt/pgck-web\nPATH=/opt/venv/bin:$PATH\nexec python -m uvicorn app.main:app --host 0.0.0.0 --port 8000\n' > /usr/local/bin/pgck-web-launcher && chmod +x /usr/local/bin/pgck-web-launcher

EXPOSE 5432 8000
ENTRYPOINT ["/usr/local/bin/pgck-web-launcher"]
