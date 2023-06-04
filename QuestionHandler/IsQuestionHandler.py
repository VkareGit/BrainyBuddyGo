from flask import Flask, request, jsonify
import re
import joblib
from langdetect import detect

app = Flask(__name__)

classifier = joblib.load('QuestionHandler/Data/classifier.joblib')
vectorizer = joblib.load('QuestionHandler/Data/vectorizer.joblib')
svd = joblib.load('QuestionHandler/Data/svd.joblib')

q_elements = ["who", "what", "when", "where", "why", "how", "?"]
q_starters = ["which", "won't", "can't", "isn't",
              "aren't", "is", "do", "does", "will", "can", "is"]
pad_chars = ["?", "-", "/"]

whitelist = [
    # League of Legends terms
    'kill', 'death', 'attack', 'fight', 'assassin', 'bomb', 'hit', 'strike', 'warrior',
    'tower', 'destroy', 'blade', 'magic', 'damage', 'heal', 'shield', 'battle', 'gun',

    # Programming and technical terms
    'execute', 'deadlock', 'daemon', 'kill', 'terminate', 'abort', 'dump', 'fatal', 'flag',
    'hack', 'illegal', 'violation', 'warning', 'bug', 'error', 'exception', 'fault', 'invalid',
    'crash', 'attack', 'break', 'exploit', 'threat', 'injection', 'overflow', 'virus', 'malware',
    'spyware', 'adware', 'phishing', 'firewall', 'encryption', 'decryption', 'protocol', 'binary', 'code',

    # General words that might be considered offensive in some contexts
    'race', 'religion', 'color', 'nationality', 'sex', 'orientation', 'identity', 'politics', 'ideology'
]

profane_words = set(
    [
        'suck',
        'stupid',
        'pimp',
        'dumb',
        'homo',
        'slut',
        'damn',
        'ass',
        'rape',
        'poop',
        'cock',
        'lol',
        'crap',
        'sex',
        'nazi',
        'neo-nazi',
        'fuck',
        'bitch',
        'pussy',
        'penis',
        'vagina',
        'whore',
        'shit',
        'nigger',
        'nigga',
        'cocksucker',
        'assrape',
        'motherfucker',
        'wanker',
        'cunt',
        'faggot',
        'fags',
        'asshole',
        'piss',
        'cum']
)


def is_profane(text):
    words = text.split()
    for word in words:
        if word in whitelist:
            continue
        if word in profane_words:
            return True
    return False


def sanitize_sentence(sentence):
    for pad_char in pad_chars:
        sentence = sentence.replace(pad_char, f" {pad_char} ")
    return sentence


def check_question(sentence):
    sentence = sentence.lower()
    features = vectorizer.transform([sentence])
    features = svd.transform(features)
    is_question = classifier.predict(features)[0]

    if is_question == 0:
        words = sentence.split()

        if words[0] in q_starters:
            return True

        if any(element in words for element in q_elements):
            return True

    return bool(is_question)


@app.route('/is_question', methods=['POST'])
def is_question():
    data = request.get_json()
    sentence = data.get('sentence', "")

    if len(sentence) < 5 or re.match(r"^(.)\1*$", sentence):
        return jsonify({'is_question': False})

    if detect(sentence) != 'en' or is_profane(sentence):
        return jsonify({'is_question': False})

    try:
        sentence = sanitize_sentence(sentence)
        is_question = check_question(sentence)
        return jsonify({'is_question': is_question})
    except Exception as e:
        return jsonify({'error': str(e)}), 500


if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8080)
