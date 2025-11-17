FROM python:3.11-slim

# Install any required packages
RUN apt-get update && apt-get install -y \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Copy workload file
ARG WORKLOAD_FILE
COPY ${WORKLOAD_FILE} /app/workload.py

# Create output directory
RUN mkdir -p /output

# Make the workload executable
RUN chmod +x /app/workload.py

# Run the workload
CMD ["python3", "/app/workload.py"]
