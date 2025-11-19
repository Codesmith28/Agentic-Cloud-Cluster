FROM python:3.11-slim

# Build argument for process number
ARG PROCESS_NUM=1

LABEL maintainer="cloudai"
LABEL description="Mixed-Intensive Process Container"

# Install system dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Install Python packages
RUN pip install --no-cache-dir \
    psutil==5.9.6 \
    numpy==1.26.2 \
    matplotlib

# Create results directory
RUN mkdir -p /results

# Copy all mixed-intensive process scripts
COPY mixed_process_*.py /app/

# Set working directory
WORKDIR /app

# Make scripts executable
RUN chmod +x /app/*.py

# Create a build-time entrypoint script (PROCESS_NUM substituted at build time)
RUN printf '%s\n' '#!/bin/sh' "exec python3 mixed_process_${PROCESS_NUM}.py \"$@\"" > /app/entrypoint.sh && \
    chmod +x /app/entrypoint.sh

# Use the baked entrypoint
ENTRYPOINT ["/app/entrypoint.sh"]
