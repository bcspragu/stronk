# npm run build:cloud
# docker build -t 192.168.5.3:5000/stronk-fe .
FROM gcr.io/distroless/nodejs16-debian11
COPY ./build/ /app
COPY package.json /app
WORKDIR /app
CMD ["/app"]
