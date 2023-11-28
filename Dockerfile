ARG DOCKER_USERNAME
FROM $DOCKER_USERNAME:alpine
RUN addgroup -S nonroot && adduser -u 65530 -S nonroot -G nonroot
USER 65530
WORKDIR /app
COPY tls.crt tls.crt
COPY tls.key tls.key