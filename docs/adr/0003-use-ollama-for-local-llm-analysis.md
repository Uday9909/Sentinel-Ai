# ADR 0003: Use Ollama for Local LLM Analysis

**Status**: Accepted

**Date**: 2026-08-09

## Context

Sentinel uses an LLM to provide root cause analysis when the anomaly detection pipeline identifies a potentially anomalous log.

Log data can contain sensitive operational information. The LLM integration therefore needs to avoid making the processing pipeline dependent on a remote cloud API.

The integration should also:

- Keep log processing local during development
- Avoid requiring cloud API credentials
- Avoid per-request cloud inference costs
- Provide a simple interface for running a local model
- Allow the LLM host to be configured for different deployment environments
- Keep inference lightweight enough for a real-time log processing pipeline

## Decision

We will use Ollama as the local LLM runtime for AI-powered root cause analysis.

The processing service communicates with Ollama through its Python client, with the Ollama host configurable through `OLLAMA_HOST`.

The configured model is `llama3.2:1b`. This is a deliberate trade-off toward a smaller local model with lower resource requirements and faster inference rather than using a larger model with potentially greater analysis capability and higher resource requirements.

LLM analysis is only performed after the existing anomaly detection checks identify a potential anomaly. This prevents every processed log from requiring an LLM inference request.

If an Ollama request fails, the processing service records the failure and applies a 30-second cooldown before attempting another LLM request. This prevents repeated failures from causing continuous inference attempts.

## Consequences

### Positive

- **Local processing**: Log data can be analyzed without sending it to an external LLM provider
- **No cloud API dependency**: Local development does not require an external API key or provider account
- **Cost**: Local inference avoids per-request cloud API charges
- **Simple integration**: Ollama provides a straightforward interface for running and querying local models
- **Configurable deployment**: `OLLAMA_HOST` allows the Ollama service to run separately from the processing service when required
- **Reduced resource requirements**: The 1B-parameter model requires fewer resources than larger local models
- **Selective inference**: LLM analysis only runs after an anomaly has already been detected

### Negative

- **Hardware requirements**: Local inference still requires sufficient CPU and memory resources
- **Inference performance**: Response time depends on the hardware running the model
- **Model capability**: The 1B model may provide less capable analysis than larger models
- **Model management**: Users must install Ollama and obtain the configured model before AI analysis can run
- **Local availability**: If Ollama is unavailable, AI analysis cannot be performed

### Neutral

- The Ollama host is configurable through `OLLAMA_HOST`, but the model name `llama3.2:1b` is currently hardcoded in the processing service
- Ollama failures trigger a 30-second cooldown before another LLM request is attempted
- LLM analysis is only invoked for logs already identified as anomalous by the keyword or rate-based detectors
- The current implementation does not expose a configuration setting for selecting a different model

## Alternatives Considered

- **Cloud LLM APIs**: Easier to provision and potentially more capable, but require external services, credentials, network access, and may send log data outside the deployment
- **Larger local model through Ollama**: Could provide more capable root cause analysis, but would require more memory and computational resources and could increase inference latency
- **Self-hosted model server without Ollama**: Provides local inference, but requires more infrastructure and model-serving configuration
- **No LLM analysis**: Eliminates inference requirements, but would remove the AI-powered root cause analysis capability
