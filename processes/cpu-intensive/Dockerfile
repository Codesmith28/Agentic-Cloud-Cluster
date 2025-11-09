FROM python:3.11-slim

LABEL maintainer="cloudai"
LABEL description="CPU-Intensive Process Container"

# Install required packages
RUN pip install --no-cache-dir \
    psutil==5.9.6 \
    numpy==1.26.2

# Create results directory
RUN mkdir -p /results

# Copy all CPU processes
COPY cpu_process_1.py /app/cpu_process_1.py
COPY cpu_process_2.py /app/cpu_process_2.py
COPY cpu_process_3.py /app/cpu_process_3.py
COPY cpu_process_4.py /app/cpu_process_4.py
COPY cpu_process_5.py /app/cpu_process_5.py
COPY cpu_process_6.py /app/cpu_process_6.py
COPY cpu_process_7.py /app/cpu_process_7.py

WORKDIR /app

# Default command (can be overridden)
CMD ["python", "cpu_process_1.py"]
