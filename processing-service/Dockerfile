# syntax=docker/dockerfile:1
FROM python:3.13-slim

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY processor.py .

# The IsolationForest model is persisted here at runtime (optional).
VOLUME /app/data

EXPOSE 8001
CMD ["python", "processor.py"]
