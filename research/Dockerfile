# Bioconductor's release image binds R 4.6.1 to Bioconductor 3.23. The digest
# pins the linux/amd64 manifest, so a mutable release tag cannot change a
# published analysis environment after review.
FROM bioconductor/bioconductor_docker@sha256:81bf4c3e9f1b08b0ada0a45ef16ef51d03dc91b0b06cda4b408d667cb8c2c645

USER root

ARG PIPELINE_COMMIT

# Bind the reviewed source revision to both image metadata and a file that an
# unprivileged runtime can read but cannot rewrite. Runtime environment values
# are operator claims, so the R verifier accepts them only when they agree with
# this baked evidence.
LABEL name="pv-signal-radar-research" \
    description="Reproducible FAERS research batch pipeline" \
    license="MIT source; bundled R packages retain their independent licenses" \
    maintainer="BMaeda84" \
    url="https://github.com/BMaeda84/pv-signal-radar" \
    vendor="PV Signal Radar" \
    version="" \
    org.opencontainers.image.authors="Bruno Maeda" \
    org.opencontainers.image.base.digest="sha256:81bf4c3e9f1b08b0ada0a45ef16ef51d03dc91b0b06cda4b408d667cb8c2c645" \
    org.opencontainers.image.base.name="docker.io/bioconductor/bioconductor_docker" \
    org.opencontainers.image.created="" \
    org.opencontainers.image.description="Reproducible FAERS research batch pipeline" \
    org.opencontainers.image.licenses="MIT" \
    org.opencontainers.image.revision="${PIPELINE_COMMIT}" \
    org.opencontainers.image.source="https://github.com/BMaeda84/pv-signal-radar" \
    org.opencontainers.image.title="PV Signal Radar research pipeline" \
    org.opencontainers.image.url="https://github.com/BMaeda84/pv-signal-radar" \
    org.opencontainers.image.vendor="PV Signal Radar" \
    org.opencontainers.image.version=""
RUN case "${PIPELINE_COMMIT}" in *[!0-9a-f]*|'') exit 1 ;; esac \
    && commit_length="$(printf '%s' "${PIPELINE_COMMIT}" | wc -c)" \
    && { [ "${commit_length}" -eq 40 ] || [ "${commit_length}" -eq 64 ]; } \
    && install -d -o root -g root -m 0755 /usr/local/share/pv-signal-radar \
    && printf '%s\n' "${PIPELINE_COMMIT}" > /tmp/pv-radar-pipeline-commit \
    && install -o root -g root -m 0444 /tmp/pv-radar-pipeline-commit /usr/local/share/pv-signal-radar/pipeline-commit \
    && rm -f /tmp/pv-radar-pipeline-commit

# The pinned upstream image currently carries obsolete Ubuntu kernel headers.
# They are not used as a container kernel, but leaving fixable packages in the
# scientific image makes the supply-chain gate both noisy and misleading. Pin
# the patched repository revision and fail if it is no longer obtainable.
RUN mkdir -p /var/lib/apt/lists/partial \
    && apt-get update \
    && DEBIAN_FRONTEND=noninteractive apt-get install --yes --no-install-recommends linux-libc-dev=6.8.0-138.138 \
    && rm -rf /var/lib/apt/lists/*

ENV LANG=C.UTF-8 \
    LC_ALL=C.UTF-8 \
    TZ=Etc/UTC \
    PV_RADAR_LOCALE=C.UTF-8 \
    RENV_CONFIG_CACHE_ENABLED=FALSE \
    RENV_CONFIG_REPOS_OVERRIDE=https://p3m.dev/cran/__linux__/noble/2026-09-02 \
    R_LIBS_USER=/opt/pv-signal-radar/research/renv/library/linux-ubuntu-noble/R-4.6/x86_64-pc-linux-gnu

WORKDIR /opt/pv-signal-radar/research

# Restore before copying the pipeline so dependency layers change only when
# the reviewed lock or bootstrap contract changes.
COPY renv.lock ./renv.lock
COPY R/bootstrap.R ./R/bootstrap.R
RUN mkdir -p "${R_LIBS_USER}" \
    && Rscript R/bootstrap.R --restore

COPY R ./R
COPY config ./config
COPY sql ./sql
COPY tests ./tests
COPY README.md ./README.md

# Synthetic fixtures are a build-time compatibility gate for the locked R
# runtime. They do not qualify the pipeline against real FAERS quarters.
RUN Rscript tests/run_tests.R \
    && mkdir -p input output \
    && chown -R rstudio:rstudio input output

USER rstudio

ENTRYPOINT ["Rscript", "R/build_snapshot.R"]
