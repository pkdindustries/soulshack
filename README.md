    //  ____                    _   ____    _                      _
    // / ___|    ___    _   _  | | / ___|  | |__     __ _    ___  | | __
    // \___ \   / _ \  | | | | | | \___ \  | '_ \   / _` |  / __| | |/ /
    //  ___) | | (_) | | |_| | | |  ___) | | | | | | (_| | | (__  |   <
    // |____/   \___/   \__,_| |_| |____/  |_| |_|  \__,_|  \___| |_|\_\
    //  .  .  .  because  real  people  are  overrated

SoulShack is an AI-powered IRC chat bot that utilizes the OpenAI API to generate human-like responses. 


## Dependencies

- Go (tested with version 1.21)
- Cobra: https://github.com/spf13/cobra
- GIRC: https://github.com/lrstanley/girc
- Go Figure: https://github.com/common-nighthawk/go-figure
- OpenAI Go Client: https://github.com/sashabaranov/go-openai
- Viper: https://github.com/spf13/viper

## Features

- Connects to an IRC server and joins a specified channel
- Utilizes the OpenAI GPT-4 model to generate realistic and human-like responses
- Allows dynamic configuration of bot settings through commands
- Supports SSL connections for secure communication
- Can adopt various personalities by changing configuration files


## Building

```bash
go build .
```

```
docker build . -t soulshack:dev
```

## Usage

```bash
soulshack --server server --port port --channel '#channelname' --become personality --ssl 
```

```bash
> echo "echo"|soulshack -f --prompt 'you are the unix program /bin/echo'
echo
>
```

## Configuration

SoulShack can be configured using command line flags, environment variables, or personality configuration files. It uses Viper to manage configuration settings.

### Flags

- `--server`: The IRC server address (default: "localhost")
- `--port`: The IRC server port (default: 6667)
- `--ssl`: Enable SSL for the IRC connection (default: false)
- `--channel`: The IRC channel to join
- `--openaikey`: Your OpenAI API key
- `--become`: The named personality to adopt (default: "chatbot")
- `--nick`: The bot's nickname on the IRC server
- `--model`: The AI model to use for generating responses (default: GPT-4)
- `--maxtokens`: The maximum number of tokens to generate with the OpenAI model (default: 512)
- `--greeting`: response prompt to the channel on join (default: "hello.")
- `--goodbye`:  response prompt to the channel on part (default: "goodbye.")
- `--prompt`: The initial character prompt for the AI, initilizes the personality
- `--answer`: appended to prompt for conditioning the answer to a question

### Environment Variables

All flags can also be set via environment variables with the prefix `SOULSHACK_`. For example, `SOULSHACK_SERVER` for the `--server` flag.

### Personality Configuration Files

Personality configuration files are stored in the `personalities` directory and use the YAML format. They can be loaded using the `--become` flag. A personality file can contain any of the settings that can be set via flags.

# Adding a Personality

To add a new personality to SoulShack, follow these steps:

1. Create a new YML file in the `personalities` directory with the desired name (e.g., `marvin.yml`).
2. Add your desired settings to the YML file. For example:

```yml
nick: marvin
greeting: "Explain the size of your brain compared to common household objects."
goodbye: "goodbye."
prompt: "respond with a text message from marvin the paranoid android:"
answer: "catastrophically highlight all the things that can go wrong with scenerios associated with the text: "
```

```bash
soulshack --server localhost --channel '#marvinshouse' --become marvin 
```

## Commands

- `/set`: Set a configuration parameter (e.g., `/set nick NewNick`)
- `/get`: Get the current value of a configuration parameter (e.g., `/get nick`)
- `/save`: Save the current configuration as a personality (e.g., `/save mypersonality`)
- `/become`: Adopt a new personality (e.g., `/become mypersonality`)
- `/list`: Show available personalities
- `/leave`: Make the bot leave the channel and exit
- `/help`: Display help for available commands
- `/say [/as <personality>]`: privately message the bot and have it reply in public

[jacob, high five me](https://i.redd.it/8y2blwiyvira1.png)

## named as tribute to my old friend dayv, sp0t, who i think of often
