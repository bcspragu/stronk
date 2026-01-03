# npm run build:cloud
# docker build -t registryext.bsprague.com/stronk-fe .
FROM gcr.io/distroless/nodejs22-debian12
COPY ./build/ /app
COPY package.json /app
WORKDIR /app
CMD ["/app"]
