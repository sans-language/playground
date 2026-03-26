FROM ubuntu:22.04

RUN apt-get update && apt-get install -y --no-install-recommends \
    wget gnupg ca-certificates software-properties-common \
    libcurl4-openssl-dev libssl-dev zlib1g-dev pkg-config \
    && wget -qO- https://apt.llvm.org/llvm-snapshot.gpg.key | apt-key add - \
    && echo "deb http://apt.llvm.org/jammy/ llvm-toolchain-jammy-17 main" > /etc/apt/sources.list.d/llvm-17.list \
    && apt-get update && apt-get install -y --no-install-recommends llvm-17 gcc \
    && ln -s /usr/lib/llvm-17/bin/llc /usr/local/bin/llc \
    && ln -s /usr/lib/llvm-17/bin/opt /usr/local/bin/opt \
    && ln -s /usr/bin/gcc /usr/local/bin/cc \
    && rm -rf /var/lib/apt/lists/*

COPY sans /usr/local/bin/sans
RUN chmod +x /usr/local/bin/sans

RUN useradd -m -s /bin/bash runner
USER runner
WORKDIR /home/runner
