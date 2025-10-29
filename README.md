# Accountant bot

A Telegram bot for expense tracking that accepts free-form text or voice messages. Entries are parsed by an LLM and automatically categorized — with categories created on the fly.

## Features

- Parse expenses from text or voice messages
- Automatically create categories and assign expenses to them
- Display spending statistics by category or individual expense for any time period
- Support for multiple currencies

## Deployment via docker

Initialize config and modify cfg/config.toml: add [telegram](https://telegram.me/BotFather) bot token and [groq](https://groq.com/) token.
```sh
make init
```

Deploy containers
```
make deploy
```

And initialize database (need only after first deployment)
```
make docker-set-db
```

## LLM Parsing and Speech Recognition

For speech-to-text and expense parsing, the bot uses the llama-3.1-8b-instant and whisper-large-v3-turbo models via the Groq API.


Support for using a locally deployed Whisper model is fully implemented, but not yet configurable through the config.toml file.
