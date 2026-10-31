# Context is ONLY the verified release/build artifact, never the working tree.
FROM gcr.io/distroless/static-debian12:nonroot
ARG SERVICE
COPY --chown=65532:65532 --chmod=0555 ${SERVICE} /app/server
USER 65532:65532
ENV HTTP_ADDR=0.0.0.0:10000
EXPOSE 10000
ENTRYPOINT ["/app/server"]
