FROM python:3.12-slim

WORKDIR /opt/cloudai

COPY cloudai-benchmark /usr/local/bin/cloudai-benchmark

RUN chmod +x /usr/local/bin/cloudai-benchmark && \
    mkdir -p /output

CMD ["cloudai-benchmark", "cpu-light"]
