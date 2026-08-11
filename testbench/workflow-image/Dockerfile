FROM python:3.12-alpine

WORKDIR /usr/local/bin
COPY cloudai-benchmark /usr/local/bin/cloudai-benchmark
RUN chmod +x /usr/local/bin/cloudai-benchmark && mkdir -p /output

ENTRYPOINT ["cloudai-benchmark"]
