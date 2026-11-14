# planillas-ccdr: imagen de producción multi-etapa.
#
#   docker build -t planillas-ccdr:<versión> .
#
# Etapa final: distroless "static" (sin shell ni gestor de paquetes), usuario
# no root (65532). Trae tres binarios estáticos:
#   /app/api      (por defecto)  servidor HTTP; probes: GET /healthz (vivo) y GET /readyz (listo)
#   /app/jobs     verificar-parametros, acumular-vacaciones, cerrar-salidas, cifrar-existentes
#   /app/migrate  up | down N | version   (MIGRATIONS_PATH=/app/db/migrations)
# Ejemplo:  docker run --rm --env-file prod.env planillas-ccdr:<v> /app/migrate up
# Sin HEALTHCHECK: la imagen no tiene shell; los probes los hace el orquestador.

FROM golang:1.26-bookworm AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOOS=linux GOFLAGS=-trimpath
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags="-s -w" -o /out/api ./cmd/api \
 && go build -ldflags="-s -w" -o /out/jobs ./cmd/jobs \
 && go build -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/api /out/jobs /out/migrate /app/
COPY --from=build /src/db/migrations /app/db/migrations
ENV MIGRATIONS_PATH=/app/db/migrations APP_ENV=production
USER 65532:65532
EXPOSE 8080
ENTRYPOINT []
CMD ["/app/api"]
