import fastapi
import torch
import io

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

DEFAULT_CATEGORIES = list(CATEGORY_PROMPTS.keys())


@app.post("/classify")
async def classify_image(
    file: fastapi.UploadFile = fastapi.File(...),
    categories: str = ",".join(DEFAULT_CATEGORIES)
):
    """
    Classify an image using CLIP zero-shot classification.
    
    Args:
        file: Image file to classify
        categories: Comma-separated list of category labels
    
    Returns:
        dict with category scores
    """    
    try:
        image_data = await file.read()
        image = Image.open(io.BytesIO(image_data)).convert("RGB")

        category_list = [cat.strip() for cat in categories.split(",")]

        prompts = [CATEGORY_PROMPTS[cat] for cat in category_list]

        inputs = processor(
            text=prompts,
            images=image,
            return_tensors="pt",
            padding=True
        )

        with torch.no_grad():
            outputs = model(**inputs)
            logits_per_image = outputs.logits_per_image
            probs = logits_per_image.softmax(dim=1)[0]

        results = {
            category: float(prob)
            for category, prob in zip(category_list, probs)
        }

        top_category = max(results, key=results.get)

        return {
            "top_category": top_category,
            "confidence": results[top_category],
            "all_scores": results
        }
    
    except Exception as e:
        return {"error": str(e)}
    

@app.get("/health")
async def health():
    return {"status": "ok", "model": model_name}


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)