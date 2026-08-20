FROM node:24-alpine AS frontend
WORKDIR /app/web/frontend
COPY web/frontend/package*.json ./
RUN npm ci
COPY web/frontend/ ./
RUN npm run build

FROM golang:1.26-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/web/frontend/dist ./web/frontend/dist
RUN CGO_ENABLED=0 go build -o /voiceline-diary .

FROM gcr.io/distroless/static-debian12
COPY --from=backend /voiceline-diary /voiceline-diary
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/voiceline-diary"]
