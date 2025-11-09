FROM python:3.11-slim

LABEL maintainer="cloudai"
LABEL description="IO-Intensive Process Container"

# Install required packages
RUN pip install --no-cache-dir \
    psutil==5.9.6 \
    numpy==1.26.2

# Create results directory
RUN mkdir -p /results

# Copy all IO processes
COPY io_process_1.py /app/io_process_1.py
COPY io_process_2.py /app/io_process_2.py
COPY io_process_3.py /app/io_process_3.py
COPY io_process_4.py /app/io_process_4.py
COPY io_process_5.py /app/io_process_5.py
COPY io_process_6.py /app/io_process_6.py
COPY io_process_7.py /app/io_process_7.py

WORKDIR /app

# Default command (can be overridden)
CMD ["python", "io_process_1.py"]
