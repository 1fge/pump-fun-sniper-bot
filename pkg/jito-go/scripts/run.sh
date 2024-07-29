#!/bin/bash

# Define the path of the .env file
ENV_FILE=".env"

# Check if the .env file already exists
if [ -f "$ENV_FILE" ]; then
    echo "$ENV_FILE exists, appending to it."
else
    echo "$ENV_FILE does not exist, creating it."
    touch "$ENV_FILE"
fi

echo "PRIVATE_KEY=" >> "$ENV_FILE"
echo "JITO_RPC=" >> "$ENV_FILE"
echo "GEYSER_RPC=" >> "$ENV_FILE"
echo "PRIVATE_KEY_WITH_FUNDS=" >> "$ENV_FILE"