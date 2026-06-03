"""Validate all Dockerfiles exist and have required instructions.

Run with:  pytest tests/integration/test_docker_build.py -v

This test suite verifies each service has a Dockerfile containing
the minimum required instructions (FROM, EXPOSE, CMD/ENTRYPOINT).
"""
import os
import re
import pytest


REPO_ROOT = os.path.join(os.path.dirname(__file__), "..", "..")

SERVICES = [
    ("ingestion-service", 8080),
    ("processing-service", 8001),
    ("dashboard", 3001),
]


@pytest.mark.parametrize("service,expected_port", SERVICES, ids=[s[0] for s in SERVICES])
class TestDockerfile:
    """Validate Dockerfile structure for each service."""

    def _read_dockerfile(self, service):
        path = os.path.join(REPO_ROOT, service, "Dockerfile")
        assert os.path.exists(path), f"Dockerfile not found for {service}"
        with open(path) as f:
            return f.read()

    def test_dockerfile_exists(self, service, expected_port):
        path = os.path.join(REPO_ROOT, service, "Dockerfile")
        assert os.path.exists(path), f"Missing Dockerfile: {service}/Dockerfile"

    def test_has_from_instruction(self, service, expected_port):
        content = self._read_dockerfile(service)
        assert re.search(r"^FROM\s+", content, re.MULTILINE), f"{service}: missing FROM"

    def test_has_expose(self, service, expected_port):
        content = self._read_dockerfile(service)
        # Check that the expected port is exposed
        assert re.search(rf"EXPOSE.*{expected_port}", content), \
            f"{service}: expected EXPOSE {expected_port}"

    def test_has_cmd_or_entrypoint(self, service, expected_port):
        content = self._read_dockerfile(service)
        has_cmd = re.search(r"^(CMD|ENTRYPOINT)\s+", content, re.MULTILINE)
        assert has_cmd, f"{service}: missing CMD or ENTRYPOINT"

    def test_has_workdir(self, service, expected_port):
        content = self._read_dockerfile(service)
        assert re.search(r"^WORKDIR\s+", content, re.MULTILINE), f"{service}: missing WORKDIR"
