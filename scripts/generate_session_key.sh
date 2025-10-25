#!/bin/bash

# Generate a 32-byte random key and encode it as base64
key=$(openssl rand -base64 32)

echo "Generated session key: $key"
echo "Add this key to your environment variables or config file as SESSION_SECRET"

# Create or update .env file if it doesn't exist
if [ ! -f .env ]; then
    touch .env
fi

# Check if SESSION_SECRET already exists in .env
if grep -q "^SESSION_SECRET=" .env; then
    # Replace existing SESSION_SECRET
    sed -i "s/^SESSION_SECRET=.*/SESSION_SECRET=$key/" .env
else
    # Add new SESSION_SECRET
    echo "SESSION_SECRET=$key" >> .env
fi

# Print instructions
echo "The session key has been added to .env file"
echo "Make sure to add .env to your .gitignore if you haven't already"