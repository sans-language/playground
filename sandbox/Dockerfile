FROM ubuntu:22.04

RUN apt-get update && apt-get install -y --no-install-recommends \
    wget gnupg ca-certificates \
    libcurl4-openssl-dev libssl-dev zlib1g-dev \
    llvm-17 \
    && rm -rf /var/lib/apt/lists/*

COPY sans /usr/local/bin/sans
RUN chmod +x /usr/local/bin/sans

RUN useradd -m -s /bin/bash runner
USER runner
WORKDIR /home/runner
