#!/bin/bash

# Check if OAuth2 provider was specified
if [ -z "$1" ]; then
    echo "Please specify an OAuth2 provider (google, github, etc.)"
    exit 1
fi

# Set provider specific variables
provider=$1
case $provider in
    "google")
        instructions="
1. Go to https://console.cloud.google.com
2. Create a new project or select an existing one
3. Enable the Google+ API
4. Go to Credentials
5. Create OAuth 2.0 Client ID credentials
6. Set the authorized redirect URI to: http://localhost:8080/auth/callback
7. Copy the Client ID and Client Secret"
        provider_url="https://accounts.google.com"
        ;;
    "github")
        instructions="
1. Go to https://github.com/settings/developers
2. Click 'New OAuth App'
3. Fill in the application details
4. Set the authorization callback URL to: http://localhost:8080/auth/callback
5. Copy the Client ID and Client Secret"
        provider_url="https://github.com"
        ;;
    *)
        echo "Unsupported provider: $provider"
        exit 1
        ;;
esac

# Print instructions
echo "=== Setting up OAuth2 for $provider ==="
echo "$instructions"
echo
echo "Please enter the client credentials:"

# Get client credentials
read -p "Client ID: " client_id
read -p "Client Secret: " client_secret
echo

# Update .env file
if [ ! -f .env ]; then
    touch .env
fi

# Update OAuth2 environment variables
if grep -q "^OAUTH2_PROVIDER_URL=" .env; then
    sed -i "s|^OAUTH2_PROVIDER_URL=.*|OAUTH2_PROVIDER_URL=$provider_url|" .env
else
    echo "OAUTH2_PROVIDER_URL=$provider_url" >> .env
fi

if grep -q "^OAUTH2_CLIENT_ID=" .env; then
    sed -i "s/^OAUTH2_CLIENT_ID=.*/OAUTH2_CLIENT_ID=$client_id/" .env
else
    echo "OAUTH2_CLIENT_ID=$client_id" >> .env
fi

if grep -q "^OAUTH2_CLIENT_SECRET=" .env; then
    sed -i "s/^OAUTH2_CLIENT_SECRET=.*/OAUTH2_CLIENT_SECRET=$client_secret/" .env
else
    echo "OAUTH2_CLIENT_SECRET=$client_secret" >> .env
fi

if grep -q "^OAUTH2_REDIRECT_URL=" .env; then
    sed -i "s|^OAUTH2_REDIRECT_URL=.*|OAUTH2_REDIRECT_URL=http://localhost:8080/auth/callback|" .env
else
    echo "OAUTH2_REDIRECT_URL=http://localhost:8080/auth/callback" >> .env
fi

echo "OAuth2 configuration has been saved to .env"
echo "Make sure to keep your client secret secure and never commit it to version control"