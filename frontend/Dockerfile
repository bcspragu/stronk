# docker build -t 192.168.5.3:5000/fivethreeone-fe .
FROM gcr.io/distroless/nodejs16-debian11
COPY ./build/ /app
COPY package.json /app
WORKDIR /app
ENV PUBLIC_API_BASE_URL=http://fivethreeone:8080
CMD ["/app"]
