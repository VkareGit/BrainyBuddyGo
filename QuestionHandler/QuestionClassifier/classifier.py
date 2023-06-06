import pandas as pd
from sklearn.feature_extraction.text import TfidfVectorizer
from sklearn.ensemble import GradientBoostingClassifier
from sklearn.model_selection import GridSearchCV
from sklearn.metrics import classification_report
from imblearn.over_sampling import SMOTE
import joblib
from sklearn.model_selection import train_test_split
from sklearn.decomposition import TruncatedSVD
from datasets import load_dataset
from nltk.tokenize import sent_tokenize
import nltk
import logging
import numpy as np
import gc

nltk.download('punkt')
RANDOM_STATE = 42
TEST_SIZE = 0.2
SVD_COMPONENTS = 100
LOG_FORMAT = "%(levelname)s %(asctime)s - %(message)s"

# Setup logging
logging.basicConfig(filename="question_classifier.log",
                    level=logging.INFO, format=LOG_FORMAT)
logger = logging.getLogger()

def is_question(sentence):
    question_words = ['who', 'what', 'where', 'when', 'why', 'how', 'is', 'are', 'do', 'does', 'did', 'was', 'were', 'have', 'has', 'had']
    if sentence[-1] == "?" or sentence.split()[0].lower() in question_words:
        return True
    return False

def load_and_prepare_data(sample_size=10000):
    quora_dataset = load_dataset("quora", split=f'train[:{sample_size}]')
    df_questions = pd.DataFrame(quora_dataset)

    cnn_dataset = load_dataset("cnn_dailymail", "3.0.0", split=f'train[:{sample_size}]')
    df_non_questions = pd.DataFrame(cnn_dataset)

    questions = [question for sublist in df_questions['questions'].tolist() for question in sublist]

    non_questions = []
    for article in df_non_questions['article'].tolist():
        sentences = sent_tokenize(article)
        non_questions.extend([sentence for sentence in sentences if not is_question(sentence)])
    
    texts = questions + non_questions
    labels = [1]*len(questions) + [0]*len(non_questions)

    del df_questions, df_non_questions, quora_dataset, cnn_dataset, non_questions, questions
    gc.collect()

    return texts, labels


def vectorize_data(texts, labels):
    X_train, X_test, y_train, y_test = train_test_split(texts, labels, test_size=TEST_SIZE, random_state=RANDOM_STATE)

    vectorizer = TfidfVectorizer()
    X_train = vectorizer.fit_transform(X_train)
    X_test = vectorizer.transform(X_test)

    svd = TruncatedSVD(n_components=SVD_COMPONENTS)
    X_train = svd.fit_transform(X_train)
    X_test = svd.transform(X_test)

    joblib.dump(vectorizer, './vectorizer.joblib')
    joblib.dump(svd, './svd.joblib')

    return X_train, X_test, y_train, y_test

def balance_data(X_train, y_train):
    smote = SMOTE(random_state=42)
    X_train_res, y_train_res = smote.fit_resample(X_train, y_train)
    return X_train_res, y_train_res

def train_and_evaluate(X_train, X_test, y_train, y_test):
    param_grid = {
        'n_estimators': [50, 100, 200],
        'learning_rate': [0.01, 0.1, 1.0],
        'max_depth': [1, 3, 5]
    }

    classifier = GradientBoostingClassifier(random_state=0)
    
    grid_search = GridSearchCV(estimator=classifier, param_grid=param_grid, cv=3)
    grid_search.fit(X_train, y_train)

    best_classifier = grid_search.best_estimator_
    
    print(f"Validation score: {best_classifier.score(X_test, y_test)}")

    y_pred = best_classifier.predict(X_test)
    print(classification_report(y_test, y_pred))

    joblib.dump(best_classifier, './classifier.joblib')

if __name__ == "__main__":
    # Ensure reproducibility
    np.random.seed(RANDOM_STATE)
  
    # Main pipeline
    try:
        logger.info("Starting data loading and preparation.")
        texts, labels = load_and_prepare_data()

        logger.info("Vectorizing data.")
        X_train, X_test, y_train, y_test = vectorize_data(texts, labels)

        logger.info("Balancing data.")
        X_train_res, y_train_res = balance_data(X_train, y_train)

        logger.info("Training and evaluating the model.")
        train_and_evaluate(X_train_res, X_test, y_train_res, y_test)

        logger.info("Pipeline completed successfully.")
    except Exception as e:
        logger.error(f"An error occurred: {e}")