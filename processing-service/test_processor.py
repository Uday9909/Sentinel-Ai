"""Tests for processor.py detection helpers.

Run with:  pytest test_processor.py -v
"""
import time
from collections import deque
from threading import Event
from unittest.mock import patch

import numpy as np
import pytest
from sklearn.ensemble import IsolationForest

from processor import (
    KAFKA_RECONNECT_BACKOFF_MAX_MS,
    KAFKA_RECONNECT_BACKOFF_MS,
    KAFKA_RECONNECT_INITIAL_DELAY_SEC,
    KAFKA_RECONNECT_MAX_DELAY_SEC,
    KAFKA_RETRY_BACKOFF_MS,
    OLLAMA_COOLDOWN_SEC,
    TRAIN_INTERVAL,
    build_kafka_consumer,
    check_rate_anomaly,
    get_next_reconnect_delay,
    has_keyword_indicator,
    process_kafka_stream,
    run_ai_analysis,
    train_if_needed,
)

# ---------------------------------------------------------------------------
# has_keyword_indicator
# ---------------------------------------------------------------------------

@pytest.mark.parametrize("text,expected", [
    ("Database connection error", True),
    ("CRITICAL: out of memory", True),
    ("Task failed with code 1", True),
    ("User login successful", False),
    ("Health check passed", False),
    ("Cache refreshed", False),
    ("", False),
    ("Error", True),          # single word
    ("something went wrong with failover", True),   # "fail" substring in "failover"
])
def test_has_keyword_indicator(text, expected):
    assert has_keyword_indicator(text) is expected


# ---------------------------------------------------------------------------
# check_rate_anomaly
# ---------------------------------------------------------------------------

def _trained_model():
    """Return a small IsolationForest fitted on low-rate data."""
    x = np.array([[1.0], [2.0], [1.5], [3.0], [2.5], [1.8], [0.5], [4.0], [2.2], [1.2]])
    model = IsolationForest(contamination=0.05, random_state=42)
    model.fit(x)
    return model


def test_rate_anomaly_detects_high_rate():
    model = _trained_model()
    history = deque([[x] for x in [1.0, 2.0, 1.5, 3.0, 2.5, 1.8, 0.5, 4.0, 2.2, 1.2]], maxlen=50)
    assert check_rate_anomaly(100.0, history, model) is True


def test_rate_anomaly_ignores_low_rate():
    model = _trained_model()
    history = deque([[x] for x in [1.0, 2.0, 1.5, 3.0, 2.5, 1.8, 0.5, 4.0, 2.2, 1.2]], maxlen=50)
    # Rate is high but below the floor
    assert check_rate_anomaly(5.0, history, model) is False


def test_rate_anomaly_returns_false_when_model_none():
    history = deque([[1.0]], maxlen=50)
    assert check_rate_anomaly(100.0, history, None) is False


def test_rate_anomaly_returns_false_when_short_history():
    model = _trained_model()
    history = deque(maxlen=50)  # empty
    assert check_rate_anomaly(100.0, history, model) is False


# ---------------------------------------------------------------------------
# train_if_needed
# ---------------------------------------------------------------------------

def test_train_if_needed_creates_model_from_none():
    # Provide enough history (2D arrays as produced by the main loop)
    history = deque([[1.0], [2.0], [1.5], [3.0], [2.5], [1.8], [0.5], [4.0], [2.2], [1.2]])
    last_time = time.time() - TRAIN_INTERVAL - 10  # stale
    model = None
    new_model, _ = train_if_needed(model, history, last_time)
    assert new_model is not None
    assert hasattr(new_model, "predict")


def test_train_if_needed_skips_when_fresh():
    history = deque([1.0, 2.0])
    last_time = time.time() - 10  # well within interval
    model_old = _trained_model()
    new_model, t = train_if_needed(model_old, history, last_time)
    assert new_model is model_old  # same object, not retrained


def test_train_if_needed_skips_when_too_few_samples():
    history = deque([1.0])  # only 1 sample
    last_time = time.time() - TRAIN_INTERVAL - 10
    model, t = train_if_needed(None, history, last_time)
    assert model is None  # still None, couldn't train
    assert t == last_time  # last_train_time unchanged


# ---------------------------------------------------------------------------
# run_ai_analysis
# ---------------------------------------------------------------------------

def test_run_ai_analysis_cooldown():
    """When within cooldown window, should return cooldown message."""
    now = time.time()
    recent_failure = now - 5  # 5 seconds ago, well within 30s cooldown
    is_anom, summary, new_failure = run_ai_analysis(
        "some error", {"service": "test"}, recent_failure
    )
    assert is_anom is True
    assert "cooldown" in summary.lower()
    assert new_failure == recent_failure  # unchanged


def test_run_ai_analysis_ollama_returns_normal():
    """When Ollama returns 'Normal', anomaly should be cleared."""
    log_data = {"service": "test"}
    past_failure = time.time() - OLLAMA_COOLDOWN_SEC - 10  # well past cooldown

    with patch("processor._ollama_client.chat") as mock_chat:
        mock_chat.return_value = {
            "message": {"content": "Normal"}
        }
        is_anom, summary, new_failure = run_ai_analysis(
            "some info log", log_data, past_failure
        )
    assert is_anom is False
    assert summary == "Normal"
    assert new_failure == past_failure


def test_run_ai_analysis_ollama_returns_analysis():
    """When Ollama returns analysis text, anomaly stands and summary is preserved."""
    log_data = {"service": "test"}
    past_failure = time.time() - OLLAMA_COOLDOWN_SEC - 10

    with patch("processor._ollama_client.chat") as mock_chat:
        mock_chat.return_value = {
            "message": {
                "content": (
                    "Database pool exhausted. Fix: restart pool, "
                    "increase max connections."
                )
            }
        }
        is_anom, summary, new_failure = run_ai_analysis(
            "DB connection error", log_data, past_failure
        )
    assert is_anom is True
    assert "Database pool exhausted" in summary
    assert new_failure == past_failure


def test_run_ai_analysis_ollama_exception():
    """When Ollama raises, should update failure time and return fallback."""
    log_data = {"service": "test"}
    past_failure = time.time() - OLLAMA_COOLDOWN_SEC - 10

    with patch("processor._ollama_client.chat", side_effect=Exception("Ollama down")):
        is_anom, summary, new_failure = run_ai_analysis(
            "some error", log_data, past_failure
        )
    assert is_anom is True
    assert "Unavailable" in summary
    # new_failure should be recent (updated to now)
    assert new_failure > past_failure


# ---------------------------------------------------------------------------
# Kafka reconnect/backoff behavior
# ---------------------------------------------------------------------------

@pytest.mark.parametrize(
    "previous,expected",
    [
        (0, KAFKA_RECONNECT_INITIAL_DELAY_SEC),
        (-1, KAFKA_RECONNECT_INITIAL_DELAY_SEC),
        (1, 2),
        (2, 4),
        (30, 60),
        (60, KAFKA_RECONNECT_MAX_DELAY_SEC),
        (120, KAFKA_RECONNECT_MAX_DELAY_SEC),
    ],
)
def test_get_next_reconnect_delay(previous, expected):
    assert get_next_reconnect_delay(previous) == expected


def test_build_kafka_consumer_sets_backoff_config():
    with patch("processor.KafkaConsumer") as mock_consumer:
        build_kafka_consumer("localhost:9092")

    _, kwargs = mock_consumer.call_args
    assert kwargs["retry_backoff_ms"] == KAFKA_RETRY_BACKOFF_MS
    assert kwargs["reconnect_backoff_ms"] == KAFKA_RECONNECT_BACKOFF_MS
    assert kwargs["reconnect_backoff_max_ms"] == KAFKA_RECONNECT_BACKOFF_MAX_MS


def test_process_kafka_stream_stops_during_reconnect_wait():
    stop_event = Event()
    waits = []

    def wait_fn(delay):
        waits.append(delay)
        stop_event.set()
        return True

    with patch("processor.build_kafka_consumer", side_effect=Exception("broker unavailable")):
        model, last_train_time, last_ollama_failure = process_kafka_stream(
            kafka_broker="localhost:9092",
            template_miner=None,
            es=None,
            log_timestamps=deque(maxlen=100),
            rate_history=deque(maxlen=50),
            model=None,
            last_train_time=123.0,
            last_ollama_failure=456.0,
            stop_event=stop_event,
            wait_fn=wait_fn,
        )

    assert waits == [KAFKA_RECONNECT_INITIAL_DELAY_SEC]
    assert model is None
    assert last_train_time == 123.0
    assert last_ollama_failure == 456.0


def test_process_kafka_stream_uses_exponential_backoff_sequence():
    stop_event = Event()
    waits = []

    def wait_fn(delay):
        waits.append(delay)
        if len(waits) >= 3:
            stop_event.set()
            return True
        return False

    with patch("processor.build_kafka_consumer", side_effect=Exception("broker unavailable")):
        process_kafka_stream(
            kafka_broker="localhost:9092",
            template_miner=None,
            es=None,
            log_timestamps=deque(maxlen=100),
            rate_history=deque(maxlen=50),
            model=None,
            last_train_time=0.0,
            last_ollama_failure=0.0,
            stop_event=stop_event,
            wait_fn=wait_fn,
        )

    assert waits == [1, 2, 4]


def test_process_kafka_stream_resets_backoff_after_successful_connect():
    stop_event = Event()
    waits = []

    class FailingConsumer:
        def __iter__(self):
            raise Exception("stream interrupted")

        def close(self):
            return None

    def wait_fn(delay):
        waits.append(delay)
        if len(waits) >= 2:
            stop_event.set()
            return True
        return False

    with patch(
        "processor.build_kafka_consumer",
        side_effect=[Exception("connect fail"), FailingConsumer()],
    ):
        process_kafka_stream(
            kafka_broker="localhost:9092",
            template_miner=None,
            es=None,
            log_timestamps=deque(maxlen=100),
            rate_history=deque(maxlen=50),
            model=None,
            last_train_time=0.0,
            last_ollama_failure=0.0,
            stop_event=stop_event,
            wait_fn=wait_fn,
        )

    # First wait after initial connect failure is 1s; second wait confirms reset
    # back to 1s after a successful reconnect.
    assert waits == [1, 1]
