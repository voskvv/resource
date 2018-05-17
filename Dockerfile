FROM golang:1.9-alpine as builder
WORKDIR /go/src/git.containerum.net/ch/resource-service
COPY . .
RUN go build -v -ldflags="-w -s" -tags "jsoniter" -o /bin/resource-service ./cmd/resource-service

FROM alpine:3.7
RUN mkdir -p /app
COPY --from=builder /bin/resource-service /app
ENV CH_RESOURCE_DEBUG="true" \
    CH_RESOURCE_TEXTLOG="true" \
    CH_RESOURCE_MONGO_ADDR="http://mongo:27017" \
    CH_RESOURCE_MONGO_LOGIN="archive" \
    CH_RESOURCE_MONGO_PASSWORD="archive_password" \
    CH_RESOURCE_KUBE_API_ADDR="http://kube-api:1214"
EXPOSE 1213
CMD "/app/resource-service"
