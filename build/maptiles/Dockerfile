FROM node:22-bookworm-slim

ARG TIPPECANOE_VERSION=2.78.0
ARG FONTNIK_VERSION=0.7.7

# The font comes from the distribution rather than a download, so it is pinned by
# this image and its licence text is installed alongside it.
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      build-essential ca-certificates fonts-noto-core git libsqlite3-dev \
      python3 zlib1g-dev \
 && rm -rf /var/lib/apt/lists/*

RUN git clone --depth 1 --branch ${TIPPECANOE_VERSION} \
      https://github.com/felt/tippecanoe.git /src/tippecanoe \
 && make -C /src/tippecanoe -j"$(nproc)" \
 && make -C /src/tippecanoe install \
 && rm -rf /src/tippecanoe

RUN mkdir -p /opt/glyphs \
 && cd /opt/glyphs \
 && npm install --no-audit --no-fund fontnik@${FONTNIK_VERSION}

ENV NODE_PATH=/opt/glyphs/node_modules

WORKDIR /work
ENTRYPOINT ["/bin/bash"]
