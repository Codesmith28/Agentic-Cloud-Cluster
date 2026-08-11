FROM python:3.11-slim

WORKDIR /app

# Copy task script
COPY task.py .

# Create output directory
RUN mkdir -p /output

# Run the task
CMD ["python", "-u", "task.py"]
