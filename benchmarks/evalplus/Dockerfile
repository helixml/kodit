# syntax=docker/dockerfile:1.9
FROM python:3.12.10-slim-bookworm

SHELL ["sh", "-exc"]

ENV PATH=/app/bin:$PATH

# Don't run your app as root.
RUN <<EOT
groupadd -r app
useradd -r -d /app -g app -N app
EOT

STOPSIGNAL SIGINT

# Ensure apt-get doesn't open a menu on you.
ENV DEBIAN_FRONTEND=noninteractive

# install git and c++ (required by cirronlib.cpp)
RUN <<EOT
apt-get update -qy
apt-get install -qyy \
    -o APT::Install-Recommends=false \
    -o APT::Install-Suggests=false \
    git g++
EOT

# upgrade to latest pip
RUN pip install --upgrade pip

RUN pip install "evalplus[vllm]==0.3.1"

RUN mkdir /app

WORKDIR /app

RUN chown -R app:app /app

USER app

# Pre-install the dataset
RUN python3 -c "from evalplus.data import *; get_human_eval_plus(); get_mbpp_plus(); get_evalperf_data()"

CMD ["bash"]