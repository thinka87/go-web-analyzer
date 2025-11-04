FROM golang:1.22 as build
WORKDIR /app
COPY go.mod .
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o web-analyzer

FROM gcr.io/distroless/base-debian12
WORKDIR /
COPY --from=build /app/web-analyzer /web-analyzer
COPY --from=build /app/templates /templates
ENV ADDR=:8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/web-analyzer"]
