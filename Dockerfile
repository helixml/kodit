# syntax=docker/dockerfile:1.9
ARG PYTHON_VERSION=3.13.5
FROM python:${PYTHON_VERSION}-slim-bookworm AS build

# The following does not work in Podman unless you build in Docker
# compatibility mode: <https://github.com/containers/podman/issues/8477>
# You can manually prepend every RUN script with `set -ex` too.
SHELL ["sh", "-exc"]

# Ensure apt-get doesn't open a menu on you.
ENV DEBIAN_FRONTEND=noninteractive

RUN <<EOT
apt-get update -qy
apt-get install -qyy \
    -o APT::Install-Recommends=false \
    -o APT::Install-Suggests=false \
    git \
    build-essential
EOT

# Security-conscious organizations should package/review uv themselves.
COPY --from=ghcr.io/astral-sh/uv:0.7.2 /uv /usr/local/bin/uv

# - Silence uv complaining about not being able to use hard links,
# - tell uv to byte-compile packages for faster application startups,
# - prevent uv from accidentally downloading isolated Python builds,
# - pick a Python (use `/usr/bin/python3.13` on uv 0.5.0 and later),
# - and finally declare `/app` as the target for `uv sync`.
ENV UV_LINK_MODE=copy \
    UV_COMPILE_BYTECODE=1 \
    UV_PYTHON_DOWNLOADS=never \
    UV_PROJECT_ENVIRONMENT=/app

# Write the PYTHON_VERSION to a .python-version
RUN echo ${PYTHON_VERSION} > .python-version

# Synchronize DEPENDENCIES without the application itself.
# This layer is cached until uv.lock or pyproject.toml change, which are
# only temporarily mounted into the build container since we don't need
# them in the production one.
# You can create `/app` using `uv venv` in a separate `RUN`
# step to have it cached, but with uv it's so fast, it's not worth
# it, so we let `uv sync` create it for us automagically.
RUN --mount=type=cache,target=/root/.cache/uv \
    --mount=type=bind,source=uv.lock,target=uv.lock \
    --mount=type=bind,source=pyproject.toml,target=pyproject.toml \
    UV_PYTHON="python$(echo ${PYTHON_VERSION} | cut -d. -f1-2)" \
    uv sync \
        --locked \
        --no-dev \
        --no-install-project

# Now install the rest from `/src`: The APPLICATION w/o dependencies.
# `/src` will NOT be copied into the runtime container.
# LEAVE THIS OUT if your application is NOT a proper Python package.
COPY . /src
WORKDIR /src
RUN --mount=type=cache,target=/root/.cache/uv \
    UV_PYTHON="python$(echo ${PYTHON_VERSION} | cut -d. -f1-2)" \
    uv sync \
        --locked \
        --no-dev \
        --no-editable

##########################################################################

FROM python:${PYTHON_VERSION}-slim-bookworm
SHELL ["sh", "-exc"]

RUN <<EOT
apt-get update -qy
apt-get install -qyy \
    -o APT::Install-Recommends=false \
    -o APT::Install-Suggests=false \
    curl \
    git 
EOT

ENV PATH=/app/bin:$PATH

WORKDIR /app
ENTRYPOINT ["/app/bin/kodit"]

# Copy the pre-built `/app` directory to the runtime container
COPY --from=build /app /app
