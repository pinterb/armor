FROM alpine:3.4

MAINTAINER Brad Pinter <brad.pinter@gmail.com>

# Metadata params
ARG VCS_URL
ARG VCS_REF
ARG BUILD_DATE
ARG VERSION

# Metadata
LABEL org.label-schema.name="Armor" \
      org.label-schema.description="Armor is a proxy to HashiCorps Vault server. It possesses super-ninja capabilities." \
      org.label-schema.vendor="CDW" \
      org.label-schema.url="https://cdw.com" \
      org.label-schema.vcs-url=$VCS_URL \
      org.label-schema.vcs-ref=$VCS_REF \
      org.label-schema.version=$VERSION \
      org.label-schema.build-date=$BUILD_DATE \
      org.label-schema.schema-version="1.0.0-rc.1"

RUN apk add --no-cache ca-certificates apache2-utils

COPY bin/amd64/armor /armor

ENTRYPOINT ["/armor"]
