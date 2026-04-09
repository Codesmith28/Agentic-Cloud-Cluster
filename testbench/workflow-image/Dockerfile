FROM python:3.12-alpine3.20

WORKDIR /opt/cloudai

COPY workflow.py /usr/local/bin/workflow
RUN chmod +x /usr/local/bin/workflow

CMD ["/usr/local/bin/workflow", "cpu-light"]
