FROM jrottenberg/ffmpeg:alpine

COPY tube2pod /app/tube2pod

ENTRYPOINT ["/app/tube2pod"]

CMD []
