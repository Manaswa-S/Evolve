import os
from gensim.utils import simple_preprocess
from gensim.models import Word2Vec
from gensim.models.phrases import Phrases, Phraser

MODEL_PATH = "ml.model"
TEXT_PATH = "../not-smart/dump/wikipedia/Machine learning/compress.txt"

# -----------------------------
# Step 1: Load or train model
# -----------------------------

if os.path.exists(MODEL_PATH):
    print("[+] Loading existing model...")
    model = Word2Vec.load(MODEL_PATH)

else:
    print("[+] Training new Word2Vec model...")

    sentences = []
    with open(TEXT_PATH, "r", encoding="utf-8") as f:
        for line in f:
            tokens = simple_preprocess(line)
            if tokens:
                sentences.append(tokens)

    # phrases = Phrases(sentences, min_count=5, threshold=13)
    # bigram = Phraser(phrases)

    # sentences = [bigram[s] for s in sentences]

    model = Word2Vec(
        sentences=sentences,
        vector_size=150,
        window=15,
        min_count=5,
        workers=12,
        sg=1,
        epochs=25,
    )

    model.save(MODEL_PATH)
    print("[+] Model trained and saved as ml.model")

# -----------------------------
# Step 2: Interactive query loop
# -----------------------------

print("\nType a word to get similar words (Ctrl+C to exit)\n")

try:
    while True:
        word = input("> ").strip().lower()

        if not word:
            continue

        if word not in model.wv:
            print("[-] Word not in vocabulary\n")
            continue

        similar = model.wv.most_similar(word)

        for w, score in similar:
            print(f"{w:15s} {score:.4f}")
        print()

except KeyboardInterrupt:
    print("\n[+] Exiting. Bye 👋")
