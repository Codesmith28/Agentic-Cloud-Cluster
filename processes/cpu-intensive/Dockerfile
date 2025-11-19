FROM python:3.11-slim

# Build argument for process number
ARG PROCESS_NUM=1

LABEL maintainer="cloudai"
LABEL description="CPU-Intensive Process Container"

# Install required packages
RUN pip install --no-cache-dir \
    psutil==5.9.6 \
    numpy==1.26.2 \
    matplotlib

# Create results directory
RUN mkdir -p /results

# Copy all CPU processes
COPY cpu_process_*.py /app/

WORKDIR /app

# Make scripts executable
RUN chmod +x /app/*.py

# Create a build-time entrypoint script (PROCESS_NUM is substituted at build time)
# This bakes the target script name into the image so runtime doesn't need env expansion.
RUN printf '%s\n' '#!/bin/sh' "exec python3 cpu_process_${PROCESS_NUM}.py \"$@\"" > /app/entrypoint.sh && \
    chmod +x /app/entrypoint.sh

# Use the baked entrypoint
ENTRYPOINT ["/app/entrypoint.sh"]
