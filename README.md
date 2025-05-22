## AI-HR

The main purpose of this repository is to provide a complete AI interview agent,
which will be able to do following:

- Helping to prepare for job interviews
- Conducting interview with applicants

## The final solution will consist out of 4 neural network models and additional services

Models:

1 - Speech to text model - for user interaction
2 - LLM - to analyze user responses, and other cognitive tasks
3 - Text to speech model - to provide feedback to the user
4 - Speech to video model - to provide a visual feedback for the user

Services:

1 - CV parser - to extract relevant information from the CV and ask some questions about it

## Setup

1. Prepare speech to text model

1.1. Download a model suitable for your needs from: https://alphacephei.com/vosk/models
Medium russian model:

```sh
wget https://alphacephei.com/vosk/models/vosk-model-ru-0.42.zip
unzip vosk-model-ru-0.42.zip
```

1.2 Extract the archive and save model folder somewhere
1.3 Run docker container with vosk API

```sh
docker run -d -p 5001:5001 -v ./vosk-model-ru-0.42:/opt/vosk-model-en/model alphacep/kaldi-grpc-en:latest
```

1.4 