import os
from gensim.utils import simple_preprocess
from gensim.models import FastText
from gensim.parsing.preprocessing import STOPWORDS

MODEL_PATH = "ml_fasttext.model"
TEXT_PATH = "../not-smart/dump/wikipedia/Machine learning/compress.txt"

# -------------------------------------------------
# Load and preprocess text
# -------------------------------------------------
sentences = []

with open(TEXT_PATH, "r", encoding="utf-8") as f:
    for line in f:
        tokens = [t for t in simple_preprocess(line) if t not in STOPWORDS]
        if tokens:
            sentences.append(tokens)


print(f"Loaded {len(sentences)} sentences")

# -------------------------------------------------
# Train or load FastText model
# -------------------------------------------------
if os.path.exists(MODEL_PATH):
    print("Loading existing FastText model...")
    model = FastText.load(MODEL_PATH)
else:
    print("Training FastText model...")

    model = FastText(
        sentences=sentences,
        vector_size=100,
        window=10,
        min_count=5,
        workers=10,
        sg=1,  # skip-gram
        epochs=15,
        min_n=3,  # character n-grams
        max_n=6,
        sample=1e-4,
    )

    model.save(MODEL_PATH)
    print("Model saved.")

# -------------------------------------------------
# Interactive similarity loop
# -------------------------------------------------
print("\nFastText ready. Type a word (Ctrl+C to exit)\n")

try:
    while True:
        word = input("> ").strip().lower()

        if not word:
            continue

        if word not in model.wv:
            print("⚠️  Word not in vocab (FastText will still approximate)")

        for w, s in model.wv.most_similar(word, topn=10):
            print(f"{w:20s} {s:.4f}")

        for w, s in model.wv.most_similar(positive=["machine", "learning"], topn=10):
            print(f"{w:20s} {s:.4f}")

        print()

except KeyboardInterrupt:
    print("\nBye 👋")
