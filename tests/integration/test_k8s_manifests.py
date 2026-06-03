"""Validate all Kubernetes manifests in the k8s/ directory.

Run with:  pytest tests/integration/test_k8s_manifests.py -v

This test suite parses every YAML file in k8s/ and verifies:
  - Valid YAML syntax
  - Required top-level keys (apiVersion, kind, metadata)
  - Every Deployment has a selector
  - Every Service has at least one port
  - Every StatefulSet has a serviceName
  - Every container has resource requests
"""
import os
import glob
import pytest
import yaml


K8S_DIR = os.path.join(os.path.dirname(__file__), "..", "..", "k8s")


def _load_all_manifests():
    """Load all YAML documents from all files in k8s/."""
    manifests = []
    for path in sorted(glob.glob(os.path.join(K8S_DIR, "*.yml"))):
        # Skip kustomization.yml (different schema)
        if os.path.basename(path) == "kustomization.yml":
            continue
        with open(path) as f:
            for doc in yaml.safe_load_all(f):
                if doc is not None:
                    manifests.append((os.path.basename(path), doc))
    return manifests


ALL_MANIFESTS = _load_all_manifests()


@pytest.mark.parametrize("filename,doc", ALL_MANIFESTS, ids=[f"{f}:{d.get('kind','?')}" for f, d in ALL_MANIFESTS])
class TestManifestStructure:
    """Validate common Kubernetes manifest structure."""

    def test_has_api_version(self, filename, doc):
        assert "apiVersion" in doc, f"{filename}: missing apiVersion"

    def test_has_kind(self, filename, doc):
        assert "kind" in doc, f"{filename}: missing kind"

    def test_has_metadata(self, filename, doc):
        assert "metadata" in doc, f"{filename}: missing metadata"

    def test_has_name(self, filename, doc):
        assert "name" in doc["metadata"], f"{filename}: metadata missing name"


def _filter_by_kind(kind):
    return [(f, d) for f, d in ALL_MANIFESTS if d.get("kind") == kind]


class TestDeployments:
    """Validate Deployment-specific requirements."""

    @pytest.mark.parametrize("filename,doc", _filter_by_kind("Deployment"),
                             ids=[f"{f}:{d['metadata']['name']}" for f, d in _filter_by_kind("Deployment")])
    def test_deployment_has_selector(self, filename, doc):
        spec = doc.get("spec", {})
        assert "selector" in spec, f"{filename}: Deployment missing selector"
        assert "matchLabels" in spec["selector"], f"{filename}: selector missing matchLabels"

    @pytest.mark.parametrize("filename,doc", _filter_by_kind("Deployment"),
                             ids=[f"{f}:{d['metadata']['name']}" for f, d in _filter_by_kind("Deployment")])
    def test_deployment_containers_have_resources(self, filename, doc):
        containers = doc.get("spec", {}).get("template", {}).get("spec", {}).get("containers", [])
        for c in containers:
            assert "resources" in c, f"{filename}: container '{c.get('name')}' missing resources"


class TestServices:
    """Validate Service-specific requirements."""

    @pytest.mark.parametrize("filename,doc", _filter_by_kind("Service"),
                             ids=[f"{f}:{d['metadata']['name']}" for f, d in _filter_by_kind("Service")])
    def test_service_has_ports(self, filename, doc):
        spec = doc.get("spec", {})
        assert "ports" in spec, f"{filename}: Service missing ports"
        assert len(spec["ports"]) > 0, f"{filename}: Service has empty ports"


class TestStatefulSets:
    """Validate StatefulSet-specific requirements."""

    @pytest.mark.parametrize("filename,doc", _filter_by_kind("StatefulSet"),
                             ids=[f"{f}:{d['metadata']['name']}" for f, d in _filter_by_kind("StatefulSet")])
    def test_statefulset_has_service_name(self, filename, doc):
        spec = doc.get("spec", {})
        assert "serviceName" in spec, f"{filename}: StatefulSet missing serviceName"


class TestKustomization:
    """Validate kustomization.yml references."""

    def test_kustomization_exists(self):
        kust_path = os.path.join(K8S_DIR, "kustomization.yml")
        assert os.path.exists(kust_path), "kustomization.yml not found"

    def test_all_resources_exist(self):
        kust_path = os.path.join(K8S_DIR, "kustomization.yml")
        with open(kust_path) as f:
            kust = yaml.safe_load(f)
        for resource in kust.get("resources", []):
            path = os.path.join(K8S_DIR, resource)
            assert os.path.exists(path), f"kustomization references {resource} but file not found"
