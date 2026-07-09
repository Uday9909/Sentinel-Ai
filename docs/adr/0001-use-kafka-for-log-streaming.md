# ADR 0001: Use Kafka for Log Streaming

**Status**: Accepted

**Date**: 2026-07-09

## Context

Sentinel needs a durable, scalable message broker between the log ingestion layer and the AI processing layer. The ingestion service receives logs via HTTP and must reliably deliver them to the processor without blocking the producer.

Key requirements:
- At-least-once delivery semantics for critical log streams
- Backpressure handling when the processor falls behind
- Ability to replay historical logs for incident reconstruction
- Partitioning to preserve per-service log ordering
- Decoupling ingestion throughput from processing speed

## Decision

We will use Apache Kafka as the streaming backbone.

## Consequences

### Positive

- **Durability**: Kafka persists messages to disk, so logs survive processor restarts
- **Ordering**: Partitioning by `service` key guarantees ordered delivery per service
- **Replay**: Consumer offset management enables replaying historical data
- **Backpressure**: Kafka acts as a massive buffer — the ingestion service never blocks on slow processing
- **Ecosystem**: Kafka has a mature Go client (`segmentio/kafka-go`) and Python client (`kafka-python`)

### Negative

- **Operational overhead**: Requires Zookeeper (or KRaft in newer versions) — added complexity versus a simpler broker like Redis Streams or RabbitMQ
- **Resource usage**: Kafka is memory and disk-heavy, especially for small deployments
- **Cold start**: First-time setup requires pulling Docker images and waiting for the cluster to initialize

### Neutral

- For single-node development, we use a 1-broker setup with replication factor 1 — this would need to change for production deployments
- The `Hash` balancer ensures consistent partitioning, but adds a dependency on the topic having enough partitions

## Alternatives Considered

- **Redis Streams**: Simpler setup, but lacks Kafka's durability guarantees and replay capabilities
- **RabbitMQ**: Good for routing, but weaker log replay and ordering guarantees
- **Direct gRPC**: No buffering — a slow processor would backpressure the ingestion API
- **NATS**: Fast and simple, but lacks Kafka's partitioning and log compaction features
