# Configuration Examples

This directory contains example configuration files for the Wheel project. Here's how to use them:

## Global Configuration
Files in `global/` should be copied to `~/.config/wheel/` (or your `$XDG_CONFIG_HOME/wheel/` directory):

```bash
mkdir -p ~/.config/wheel
cp global/* ~/.config/wheel/
```

## Local Project Configuration
Files in `local/` should be copied to your project's `.wheel/` directory:

```bash
mkdir -p .wheel
cp local/* .wheel/
```

## Environment Variables
The following environment variables are supported:

```bash
# Required for using OpenAI models
export OPENAI_API_KEY='your-key-here'

# Required for using Anthropic models
export ANTHROPIC_API_KEY='your-key-here'
```

You can also create a `.env` file in your project directory with these variables.

## Configuration Precedence
1. Environment variables (highest priority)
2. Local project config (.wheel/*)
3. Global user config (~/.config/wheel/*)
4. Default values (lowest priority)

## Adding New Values
To add new configuration values:
1. Add them to the defaults.wheel.yaml file
2. Add any validation rules to the ConfigSchema struct if needed
3. Add any new environment variable mappings to the envVars slice

## Running with Verbose Logging
To see where configuration values are coming from:

```bash
wheel -v
# or
wheel --verbose
```
