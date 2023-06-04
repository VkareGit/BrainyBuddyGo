# BrainyBuddy Discord Bot

BrainyBuddy is a Discord bot that uses machine learning models to interact with users. It's capable of understanding and answering questions by integrating with the OpenAI GPT-3.5 Turbo model. The bot filters messages, identifying if a message is a question. Only questions are processed and passed on to the OpenAI GPT-3.5 Turbo API for generating an intelligent response.

## Components
The bot primarily consists of two major components:

- **Gradient Boosting Question Classifier**: This is a Gradient Boosting model trained to identify if an input is a question. It's implemented in Python and hosted locally via Flask as an API endpoint. The model is trained on a balanced dataset, ensuring reliable identification of question and non-question input.
+ **Discord Interaction and OpenAI Integration**: This part is implemented in Go and includes the logic for interacting with the Discord API, forwarding user messages to the Question Classifier API, and managing responses from OpenAI's GPT-3.5 Turbo API.s

## Features

- The bot uses gradient-boosting based machine learning model to identify questions in the chat.
* Non-question sentences are ignored, reducing unnecessary API calls.
+ The bot utilizes the OpenAI GPT-3.5 Turbo API to generate meaningful responses to the questions.
- Efficient handling of API calls using worker queues and backoff strategy.

## Setup Instructions

Follow these steps to setup the BrainyBuddy Discord bot:

1. Clone the repository to your local machine or server.

 ```
	git clone https://github.com/user/brainybuddy-discord-bot.git
	cd brainybuddy-discord-bot
 ```
2. If you want to generate ML objects by yourself, run the classifier script in the repository. Alternatively, you can download already generated ML files from the following location:
[ML Files Download Link](https://drive.google.com/drive/folders/1EWQeVj_qaam3_-SXvbC9T4gHqHoafFNY?usp=sharing)

3. Once downloaded, place the 'Data' folder in this location: 'BrainyBuddyGo/QuestionHandler/'.

4. Update the prompt.txt file with your desired content. This will be used as the prompt for the GPT API assistant.

5. Create a .env file in the project folder with the following content:
```
export DISCORD_BOT_TOKEN=your-discord-bot-token
export OPENAI_API_KEY=your-openai-api-key
```
<sub>Replace discord-bot-token and openai-api-key with your actual Discord bot token and OpenAI API key, respectively.<sub>

6. Start the Flask server hosting the Gradient Boosting model locally. Make sure you have the necessary Python libraries installed (Flask, pandas, sklearn, joblib, etc.). You may want to use a virtual environment.
```
python3 BrainyBuddyGo/QuestionHandler/IsQuestionHandler.py
```
  
7. In a new terminal, navigate to the project's root directory and start the Go server.
```
go run cmd/brainybuddy/main.go
```
8. The bot should now be up and running. Invite it to your Discord server and start interacting by asking questions!

## Contributing

Feel free to submit issues or conntact me for any improvements or fixes you'd like to see in this project. I'am always looking for ways to enhance the bot and make it more useful.
