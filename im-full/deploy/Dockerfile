# 多阶段构建：编译 → 极小运行镜像（纯 Go、无 CGO）
FROM golang:1.22-alpine AS build
ARG VERSION=docker
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/imserver ./cmd/imserver \
 && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/imcli ./cmd/imcli

FROM alpine:3.20
RUN adduser -D -u 10001 im && mkdir -p /data && chown im /data
COPY --from=build /out/imserver /out/imcli /usr/local/bin/
USER im
ENV IM_DATA_DIR=/data IM_HTTP_ADDR=:8080 IM_TCP_ADDR=:9000
VOLUME /data
EXPOSE 8080 9000
HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["imserver"]
