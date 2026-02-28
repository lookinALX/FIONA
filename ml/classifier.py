import fastapi
import torch
import json
import sys
import os

from PIL import Image
from transformers import CLIPProcessor, CLIPModel
from typing import Dict, List
from torch.utils.data import Dataset, DataLoader

CONFIG_FILE_PATH = ""

def validate_config(data: dict) -> str | None:
    if not isinstance(data, dict):
        return "config must be a JSON object"
    
    if len(data) == 0:
        return "config must have at least one category"
    
    for key, value in data.items():
        if not isinstance(key, str) or not key.strip():
            return f"invalid key: {repr(key)}, must be a non-empty string"
        if not isinstance(value, str) or not value.strip():
            return f"invalid prompt for '{key}', must be a non-empty string"
        
    return None

class ImageDataset(Dataset):
    def __init__(self, image_paths):
        super().__init__()
        self.image_paths = image_paths
    
    def __len__(self):
        return len(self.image_paths)
    
    def __getitem__(self, index):
        path = self.image_paths[index]
        image = Image.open(self.image_paths[index]).convert("RGB")
        return image, path

model_name = "openai/clip-vit-base-patch32"
device = "cuda" if torch.cuda.is_available() else "cpu"


def make_collate_fn(prompts):
    def collate_fn(batch):
        images, paths = zip(*batch)

        inputs = processor(
            text=prompts,
            images=list(images),
            return_tensors="pt",
            padding=True
        )
        
        return inputs, paths
    
    return collate_fn

try:
    model = CLIPModel.from_pretrained(model_name).to(device)
    processor = CLIPProcessor.from_pretrained(model_name)
    print("Model is loaded successfully")
except OSError as e:
    print("Model loading error:", e)
except Exception as e:
    print("Unexpected error:", e)

app = fastapi.FastAPI()



CATEGORY_PROMPTS = {
    "people": "a photograph of a person or people, including portraits, selfies, group photos, and candid shots of individuals",
    
    "family": "a family photo with parents, children, relatives gathering together for holidays, celebrations, or everyday moments",
    
    "pets": "a photo of a pet animal such as a dog, cat, or other domestic companion animal at home or outdoors",
    
    "nature": "a beautiful nature landscape photograph with mountains, forests, lakes, rivers, fields, or natural scenery",
    
    "food": "a photo of food, meals, dishes on a plate, cooking, baking, or dining at a restaurant or home",
    
    "travel": "a travel vacation photo from a trip, showing tourist destinations, hotels, transportation, or sightseeing adventures",
    
    "events": "a special event photo such as a birthday party, wedding, celebration, concert, or social gathering with people",
    
    "sports": "a sports or fitness photo showing athletic activities, games, exercise, gym workouts, or outdoor recreation",
    
    "home": "a home interior photo showing rooms, furniture, decorations, living spaces, bedrooms, kitchens, or household items",
    
    "vehicles": "a photo of vehicles including cars, motorcycles, bicycles, trucks, buses, trains, airplanes, or other transportation",
    
    "work": "a work or office environment photo showing desks, computers, meetings, coworkers, or professional business settings",
    
    "documents": "a scanned document, paper form, invoice, receipt, contract, letter, certificate, or printed text page",
    
    "screenshots": "a computer or phone screenshot showing apps, websites, social media, messages, or digital screen content",
    
    "other": "other types of photos including abstract images, art, random objects, or miscellaneous content that doesn't fit other categories"
}

CATEGORIES = list(CATEGORY_PROMPTS.keys())


@app.post("/classify")
async def classify_image(data: Dict[str, List[str]]):
    """
    Classify multiple images by file paths
    
    Request body:
    {
      "paths": ["/home/user/image1.png", "/home/user/image2.png"]
    }
    """
    try:
        go_results = {}
        all_results = {}

        categories = ",".join(CATEGORIES)

        category_list = [cat.strip() for cat in categories.split(",")]

        prompts = [CATEGORY_PROMPTS[cat] for cat in category_list]

        image_paths = []
        not_image_paths = [] 

        for path in data["paths"]:
            try:
                Image.open(path).verify()
                image_paths.append(path)
            except Exception:
                not_image_paths.append(path)
        
        for path in not_image_paths:
            go_results[path] = "__not_image__"

        dataset = ImageDataset(image_paths)

        num_workers = min(4, os.cpu_count())

        loader = DataLoader(
            dataset,
            batch_size=32,
            shuffle=False,
            num_workers=num_workers,
            pin_memory=device == "cuda",
            collate_fn=make_collate_fn(prompts)
        )

        model.eval()

        with torch.no_grad():
            for inputs, paths in loader:
                inputs = {k: v.to(device) for k, v in inputs.items()}
                
                outputs = model(**inputs)
                probs = outputs.logits_per_image.softmax(dim=1) 

                for path, image_probs in zip(paths,probs):
                    results = {
                        category: float(prob)
                        for category, prob in zip(category_list, image_probs)
                    }

                    top_category = max(results, key=results.get)

                    all_results[path] = {
                        "top_category": top_category,
                        "confidence": results[top_category],
                        "all_scores": results
                    }

                    go_results[path] = top_category
                
            
            cwd_path = os.getcwd()
            with open(os.path.join(cwd_path, "ml_log_results.json"), "w", encoding="utf-8") as f:
                json.dump(all_results, f, ensure_ascii=False, indent=4)

            return go_results
    
    except Exception as e:
        return {"error": str(e)}
    

@app.get("/health")
async def health():
    return {"status": "ok", "model": model_name}


if __name__ == "__main__":
    if len(sys.argv) > 2:
        print("ERROR python script: ")
        print(" ------------- more arguments then allowed --> only file path is expected")
        sys.exit(1)

    if len(sys.argv) == 2:
        CONFIG_FILE_PATH = sys.argv[1] 
        with open(CONFIG_FILE_PATH, "r", encoding = "utf-8") as f:
            configuration = json.load(f)
        
        error = validate_config(configuration)
        if error:
            print(f"invalid config: {error}")
            sys.exit(1)
        
        CATEGORY_PROMPTS = configuration
        CATEGORIES = list(CATEGORY_PROMPTS.keys())


    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)