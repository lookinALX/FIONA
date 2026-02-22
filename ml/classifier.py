import fastapi
import uvicorn
import torch
import torchvision

from PIL import Image
from transformers import CLIPProcessor, CLIPModel

model_name = "openai/clip-vit-base-patch32"

try:
    model = CLIPModel.from_pretrained(model_name)
    processor = CLIPProcessor.from_pretrained(model_name)
    print("Model is loaded successfully")
except OSError as e:
    print("Model loading error:", e)
except Exception as e:
    print("Unexpected error:", e)

app = fastapi.FastAPI()

