import pandas as pd
from sklearn.feature_extraction.text import TfidfVectorizer
from sklearn.ensemble import GradientBoostingClassifier
import joblib
from sklearn.model_selection import train_test_split
from datasets import load_dataset
from sklearn.decomposition import TruncatedSVD


# Load your datasets
quora_dataset = load_dataset("quora", split='train')
df_questions = pd.DataFrame(quora_dataset)

cnn_dataset = load_dataset("cnn_dailymail", "3.0.0", split='train')
df_non_questions = pd.DataFrame(cnn_dataset)

# Prepare the data
questions = [question for sublist in df_questions['questions'].tolist() for question in sublist]
non_questions = df_non_questions['article'].tolist()

# Filter out articles with question marks
non_questions = [article for article in non_questions if '?' not in article]

# Balance the dataset
if len(non_questions) < len(questions):
    questions = questions[:len(non_questions)]
else:
    non_questions = non_questions[:len(questions)]

texts = questions + non_questions
labels = [1]*len(questions) + [0]*len(non_questions)

# Split the data
X_train, X_test, y_train, y_test = train_test_split(texts, labels, test_size=0.2, random_state=42)

# Create a vectorizer
vectorizer = TfidfVectorizer()

# Transform the text data to vectors
X_train = vectorizer.fit_transform(X_train)
X_test = vectorizer.transform(X_test)

svd = TruncatedSVD(n_components=100)
X_train = svd.fit_transform(X_train)
X_test = svd.transform(X_test)

# Create and train the classifier
classifier = GradientBoostingClassifier(n_estimators=100, learning_rate=1.0, max_depth=1, random_state=0)
classifier.fit(X_train, y_train)

# Save the classifier and the vectorizer
joblib.dump(classifier, 'QuestionHandler/Data/classifier.joblib')
joblib.dump(vectorizer, 'QuestionHandler/Data/vectorizer.joblib')
joblib.dump(svd, 'QuestionHandler/Data/svd.joblib')
