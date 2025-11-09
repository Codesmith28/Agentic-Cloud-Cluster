FROM nvidia/cuda:12.1.0-base-ubuntu22.04

LABEL maintainer="cloudai"
LABEL description="GPU-Intensive Process Container with PyTorch"

# Install Python and dependencies
RUN apt-get update && apt-get install -y \
    python3.11 \
    python3-pip \
    && rm -rf /var/lib/apt/lists/*

# Install PyTorch with CUDA support
RUN pip3 install --no-cache-dir \
    torch==2.1.2 \
    torchvision==0.16.2 \
    --index-url https://download.pytorch.org/whl/cu121

# Create results directory
RUN mkdir -p /results

# Copy all GPU processes
COPY gpu_process_1.py /app/gpu_process_1.py
COPY gpu_process_2.py /app/gpu_process_2.py
COPY gpu_process_3.py /app/gpu_process_3.py
COPY gpu_process_4.py /app/gpu_process_4.py
COPY gpu_process_5.py /app/gpu_process_5.py
COPY gpu_process_6.py /app/gpu_process_6.py

WORKDIR /app

# Default command (can be overridden)
CMD ["python3", "gpu_process_1.py"]
