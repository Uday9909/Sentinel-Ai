# ADR 0002: Use Isolation Forest for Anomaly Detection

**Status**: Accepted

**Date**: 2026-08-09

## Context

Sentinel needs to identify anomalous log activity without requiring a labelled dataset of known incidents. The current anomaly detection pipeline focuses on detecting unusual changes in log rates over time.

The detector needs to:

- Work without labelled training data
- Operate on numerical log-rate data
- Detect unusual observations rather than classify predefined incident types
- Be lightweight enough for local development and deployment
- Retrain periodically as new log-rate data becomes available
- Avoid flagging low-traffic periods as anomalous purely on relative statistics

## Decision

We will use scikit-learn's Isolation Forest for rate-based anomaly detection.

The model is trained on recent log-rate observations and periodically retrained as new observations become available. An observation is considered anomalous only when both conditions hold: Isolation Forest identifies it as an outlier, and the configured minimum log-rate threshold is exceeded.

The additional rate threshold acts as an operational guard alongside the statistical model. Isolation Forest identifies observations that are unusual relative to the training data, while the threshold requires the observed rate to also exceed a minimum absolute level.

The current implementation uses a minimum rate of 20 logs per second.

## Consequences

### Positive

- **Unsupervised detection**: Does not require labelled examples of anomalous logs
- **Suitable for outliers**: Isolation Forest is designed to identify observations that differ from the majority of the training data
- **Lightweight**: Can run locally without requiring a separate ML service
- **Simple integration**: scikit-learn provides a mature implementation that integrates directly with the Python processing service
- **Periodic retraining**: The model can adapt to changing log-rate patterns as new observations are collected
- **Operational filtering**: The minimum rate threshold prevents model predictions from being treated as anomalies when the observed rate is below the configured operational threshold

### Negative

- **Limited features**: The current implementation uses log rate as the model input, so it cannot detect every type of log anomaly
- **Model sensitivity**: Detection quality depends on the amount and distribution of historical log-rate data
- **False positives/negatives**: Unsupervised detection cannot guarantee that every detected outlier represents a real incident
- **Additional processing**: Model training and prediction add computational overhead to the processing service
- **Threshold tuning**: The model's behaviour is combined with a fixed rate threshold, so changing traffic patterns may require adjustment of the threshold

### Neutral

- The minimum log-rate threshold is currently fixed at 20 logs per second and is not externally configurable
- The model is retrained at most every 60 seconds once sufficient rate history is available
- Training requires at least 10 rate observations
- The model uses a contamination value of 0.05 and a fixed random seed of 42
- The current rate history is limited to 50 observations
- These parameters are implementation defaults and are not documented as having been tuned against a labelled historical incident dataset

## Alternatives Considered

- **Statistical thresholding**: Simpler to implement, but less adaptable to changing log-rate patterns
- **One-Class SVM**: Supports unsupervised outlier detection, but introduces additional model complexity and tuning requirements
- **Local Outlier Factor**: Effective for local density-based anomalies, but less suitable for the current lightweight rate-based detection pipeline
- **Supervised classification**: Could provide more targeted detection, but requires a labelled dataset of known normal and anomalous events
