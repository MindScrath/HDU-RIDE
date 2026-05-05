FROM golang:1.26-alpine AS build
WORKDIR /src
ARG GOPROXY=https://goproxy.cn,direct
ARG GOSUMDB=sum.golang.google.cn
ENV GOPROXY=$GOPROXY
ENV GOSUMDB=$GOSUMDB
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -o /out/hdu-ride-backend .

FROM alpine:3.22
RUN adduser -D -u 10001 app
USER app
COPY --from=build /out/hdu-ride-backend /hdu-ride-backend
EXPOSE 8080
ENTRYPOINT ["/hdu-ride-backend"]
