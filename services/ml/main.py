from fastapi import FastAPI
from pydantic import BaseModel
from sentence_transformers import SentenceTransformer

app = FastAPI(redirect_slashes=False)
model = SentenceTransformer("all-MiniLM-L6-v2")

class EmbedRequest(BaseModel):
    text: str

@app.post("/embed")
def embed(req: EmbedRequest):
    vector = model.encode(req.text)
    return {"embedding": vector.tolist()}