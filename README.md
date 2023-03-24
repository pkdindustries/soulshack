# Chatbot IRC

This is a simple chatbot IRC client that uses OpenAI's GPT model to generate responses to user messages. The chatbot connects to an IRC server and specified channels, listens for messages, and responds using the OpenAI API.

## Usage

./chatbot -irchost <IRC_SERVER_ADDRESS> -ircport <IRC_SERVER_PORT> -ircnick <BOT_NICKNAME> -ircchannels '#CHANNEL #CHANNEL2'

## Requirements

- Go (tested with version 1.21)
- [girc](https://github.com/lrstanley/girc) (IRC library)
- [go-openai](https://github.com/sashabaranov/go-openai) (OpenAI client library)

## Installation

1. Clone the repository.
2. Run `go build` in the repository folder.
3. Run the generated executable with the appropriate command line options.

## Command Line Options

- `-irchost`: IRC server address (default: "localhost")
- `-ircport`: IRC server port (default: 6667)
- `-ircnick`: Bot's nickname on the IRC server (default: "chatbot")
- `-ircchannels`: Space-separated list of channels to join (e.g., "#channel1 #channel2")
- `-ssl`: Enable SSL for the IRC connection (default: false)
- `-preamble`: Text prepended to prompt (default: "provide a short reply of no more than 3 lines:")
- `-model`: Model to be used for responses (e.g., "gpt-4") (default: openai.GPT4)

## Environment Variables

- `OPENAI_API_KEY`: API key for OpenAI (required)

## Commands

Users can send the following commands to the bot:

- `/set preamble <value>`: Set the preamble used before each prompt.
- `/set model <value>`: Set the OpenAI model used for generating responses.

## Contributing

If you would like to contribute, please open an issue or submit a pull request on the project's GitHub repository.
