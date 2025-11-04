FROM golang:1.22 as build
WORKDIR /app

# Copy module files first to leverage layer cache
COPY go.mod ./
# If you already have go.sum locally, copy it too:
# COPY go.mod go.sum ./

# Pre-fetch deps (ok even if go.sum isn't present yet)
RUN go mod download

# Now copy the rest
COPY . .

# Ensure go.sum gets generated if it wasn't present
RUN go mod tidy

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build -o web-analyzer

# ---- Runtime image ----
FROM gcr.io/distroless/base-debian12
WORKDIR /
COPY --from=build /app/web-analyzer /web-analyzer
COPY --from=build /app/templates /templates
ENV ADDR=:8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/web-analyzer"]

