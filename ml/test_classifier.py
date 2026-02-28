"""
Tests for ml/classifier.py

Run with:
    pytest ml/test_classifier.py -v

Requirements:
    pip install pytest pytest-asyncio httpx fastapi pillow
"""

import json
import os
import sys
import pytest
import tempfile
import numpy as np

from pathlib import Path
from PIL import Image
from unittest.mock import patch, MagicMock
from httpx import AsyncClient, ASGITransport


# ── helpers ───────────────────────────────────────────────────────────────────

def create_test_image(path: str, color: tuple = (255, 0, 0), size: tuple = (224, 224)):
    """Create a minimal valid JPEG at the given path."""
    img = Image.fromarray(np.full((*size, 3), color, dtype=np.uint8))
    img.save(path, format="JPEG")
    return path


# ── validate_config ───────────────────────────────────────────────────────────

class TestValidateConfig:

    def setup_method(self):
        # Import here so tests can run without model download
        from classifier import validate_config
        self.validate_config = validate_config

    def test_valid_config(self):
        data = {
            "cats": "a photo of a cat",
            "dogs": "a photo of a dog",
        }
        assert self.validate_config(data) is None

    def test_not_a_dict(self):
        assert self.validate_config(["cats", "dogs"]) is not None
        assert self.validate_config("cats") is not None
        assert self.validate_config(42) is not None

    def test_empty_dict(self):
        assert self.validate_config({}) is not None

    def test_empty_key(self):
        assert self.validate_config({"": "a photo of something"}) is not None

    def test_whitespace_only_key(self):
        assert self.validate_config({"   ": "a photo of something"}) is not None

    def test_empty_prompt(self):
        assert self.validate_config({"cats": ""}) is not None

    def test_whitespace_only_prompt(self):
        assert self.validate_config({"cats": "   "}) is not None

    def test_non_string_value(self):
        assert self.validate_config({"cats": 42}) is not None
        assert self.validate_config({"cats": None}) is not None
        assert self.validate_config({"cats": ["a", "b"]}) is not None

    def test_single_category_valid(self):
        assert self.validate_config({"other": "anything else"}) is None

    def test_many_categories_valid(self):
        data = {f"cat_{i}": f"prompt for category {i}" for i in range(50)}
        assert self.validate_config(data) is None


# ── ImageDataset ──────────────────────────────────────────────────────────────

class TestImageDataset:

    def setup_method(self):
        from classifier import ImageDataset
        self.ImageDataset = ImageDataset

    def test_len(self, tmp_path):
        paths = []
        for i in range(3):
            p = str(tmp_path / f"img{i}.jpg")
            create_test_image(p)
            paths.append(p)

        dataset = self.ImageDataset(paths)
        assert len(dataset) == 3

    def test_getitem_returns_image_and_path(self, tmp_path):
        p = str(tmp_path / "img.jpg")
        create_test_image(p)

        dataset = self.ImageDataset([p])
        image, path = dataset[0]

        assert isinstance(image, Image.Image)
        assert path == p

    def test_image_converted_to_rgb(self, tmp_path):
        # Create RGBA image
        p = str(tmp_path / "rgba.png")
        img = Image.fromarray(np.zeros((10, 10, 4), dtype=np.uint8), mode="RGBA")
        img.save(p)

        dataset = self.ImageDataset([p])
        image, _ = dataset[0]

        assert image.mode == "RGB"

    def test_missing_file_raises(self, tmp_path):
        dataset = self.ImageDataset([str(tmp_path / "nonexistent.jpg")])
        with pytest.raises(Exception):
            _ = dataset[0]


# ── /health endpoint ──────────────────────────────────────────────────────────

@pytest.mark.asyncio
class TestHealthEndpoint:

    async def test_health_returns_ok(self):
        from classifier import app
        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            response = await client.get("/health")

        assert response.status_code == 200
        body = response.json()
        assert body["status"] == "ok"
        assert "model" in body

    async def test_health_model_name(self):
        from classifier import app, model_name
        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            response = await client.get("/health")

        assert response.json()["model"] == model_name


# ── /classify endpoint ────────────────────────────────────────────────────────

@pytest.mark.asyncio
class TestClassifyEndpoint:

    async def test_classify_returns_dict_of_path_to_category(self, tmp_path):
        """Response must be {path: category} flat dict."""
        p1 = str(tmp_path / "a.jpg")
        p2 = str(tmp_path / "b.jpg")
        create_test_image(p1, color=(255, 0, 0))
        create_test_image(p2, color=(0, 255, 0))

        from classifier import app
        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            response = await client.post("/classify", json={"paths": [p1, p2]})

        assert response.status_code == 200
        body = response.json()

        assert isinstance(body, dict)
        assert p1 in body
        assert p2 in body
        assert isinstance(body[p1], str)
        assert isinstance(body[p2], str)

    async def test_classify_category_is_known(self, tmp_path):
        """Every returned category must be in CATEGORIES."""
        p = str(tmp_path / "img.jpg")
        create_test_image(p)

        from classifier import app, CATEGORIES
        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            response = await client.post("/classify", json={"paths": [p]})

        body = response.json()
        assert body[p] in CATEGORIES

    async def test_classify_empty_paths(self):
        """Empty paths list should return empty dict."""
        from classifier import app
        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            response = await client.post("/classify", json={"paths": []})

        assert response.status_code == 200
        assert response.json() == {}

    async def test_classify_nonexistent_path_returns_error(self):
        """Nonexistent file should not crash the server."""
        from classifier import app
        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            response = await client.post("/classify", json={"paths": ["/nonexistent/ghost.jpg"]})

        assert response.status_code == 200
        body = response.json()
        # Either an error key or the path mapped to "other"
        assert "error" in body or body.get("/nonexistent/ghost.jpg") == "other"

    async def test_classify_single_image(self, tmp_path):
        """Single image should return single entry."""
        p = str(tmp_path / "single.jpg")
        create_test_image(p)

        from classifier import app
        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            response = await client.post("/classify", json={"paths": [p]})

        body = response.json()
        assert len(body) == 1
        assert p in body

    async def test_classify_many_images(self, tmp_path):
        """All paths in request must appear in response."""
        paths = []
        for i in range(10):
            p = str(tmp_path / f"img{i}.jpg")
            create_test_image(p, color=(i * 25, 0, 0))
            paths.append(p)

        from classifier import app
        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            response = await client.post("/classify", json={"paths": paths})

        body = response.json()
        assert len(body) == len(paths)
        for p in paths:
            assert p in body


# ── config loading ────────────────────────────────────────────────────────────

class TestConfigLoading:

    def test_valid_config_file_is_loaded(self, tmp_path):
        config = {
            "cats": "a photo of a cat",
            "dogs": "a photo of a dog",
        }
        config_path = str(tmp_path / "config.json")
        with open(config_path, "w", encoding="utf-8") as f:
            json.dump(config, f)

        # Simulate what __main__ does
        with open(config_path, "r", encoding="utf-8") as f:
            loaded = json.load(f)

        from classifier import validate_config
        assert validate_config(loaded) is None
        assert loaded == config

    def test_invalid_config_file_caught(self, tmp_path):
        config_path = str(tmp_path / "bad.json")
        with open(config_path, "w", encoding="utf-8") as f:
            json.dump({}, f)  # empty dict — invalid

        with open(config_path, "r", encoding="utf-8") as f:
            loaded = json.load(f)

        from classifier import validate_config
        assert validate_config(loaded) is not None

    def test_malformed_json_raises(self, tmp_path):
        config_path = str(tmp_path / "broken.json")
        with open(config_path, "w", encoding="utf-8") as f:
            f.write("this is not json {{{")

        with pytest.raises(json.JSONDecodeError):
            with open(config_path, "r", encoding="utf-8") as f:
                json.load(f)