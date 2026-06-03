"""Tests for processor.py detection helpers.

Run with:  pytest test_processor.py -v
"""
import time
from collections import deque
from unittest.mock import patch

import numpy as np
import pytest
from sklearn.ensemble import IsolationForest

from processor import (
    OLLAMA_COOLDOWN_SEC,
    TRAIN_INTERVAL,
    check_rate_anomaly,
    has_keyword_indicator,
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
