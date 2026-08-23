FROM eclipse-temurin:21-jdk-noble

ARG TIPPECANOE_VERSION=2.78.0
ARG PLANETILER_OMT_VERSION=v3.16
ARG PLANETILER_OMT_SHA256=246cd5c9c10102a3bcc58465ae7dde5b97aa4cee6524ea25788e23333ba2579d

RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      build-essential ca-certificates curl git libsqlite3-dev osmium-tool python3 zlib1g-dev \
 && rm -rf /var/lib/apt/lists/*

RUN git clone --depth 1 --branch ${TIPPECANOE_VERSION} \
      https://github.com/felt/tippecanoe.git /src/tippecanoe \
 && make -C /src/tippecanoe -j"$(nproc)" \
 && make -C /src/tippecanoe install \
 && rm -rf /src/tippecanoe

RUN mkdir -p /opt/planetiler \
 && curl -fsSL --retry 3 -o /opt/planetiler/planetiler-openmaptiles.jar \
      "https://github.com/openmaptiles/planetiler-openmaptiles/releases/download/${PLANETILER_OMT_VERSION}/planetiler-openmaptiles.jar" \
 && if ! echo "${PLANETILER_OMT_SHA256}  /opt/planetiler/planetiler-openmaptiles.jar" | sha256sum -c -; then \
      echo "the planetiler-openmaptiles jar for ${PLANETILER_OMT_VERSION} does not match" >&2; \
      echo "PLANETILER_OMT_SHA256. Moving the version means moving the digest with it:" >&2; \
      echo "set both in the Makefile, or pass both to make." >&2; \
      exit 1; \
    fi

ENV PLANETILER_JAR=/opt/planetiler/planetiler-openmaptiles.jar

WORKDIR /work
ENTRYPOINT ["/bin/bash"]
