package middleware.authz

# Default deny
default allow = false

# Allow health check and metrics without auth
allow {
    input.path == "/health"
}
allow {
    input.path == "/metrics"
}
allow {
    input.path == "/auth/login"
}
allow {
    input.path == "/auth/callback"
}

# Verify JWT token and check permissions
allow {
    # Verify token is valid
    token := decode_verified_token(input.token)
    
    # Extract claims
    claims := token[1]
    
    # Check if token is still valid
    token_is_valid(claims)
    
    # Check if token has required scope
    has_required_scope(claims, input.method, input.path)
}

# Define required scopes for API endpoints
required_scope(method, path) = scope {
    method == "POST"
    path == "/api/v1/users"
    scope := "create:users"
}

required_scope(method, path) = scope {
    method == "GET"
    path == "/api/v1/users"
    scope := "read:users"
}

required_scope(method, path) = scope {
    method == "PUT"
    path == "/api/v1/users"
    scope := "update:users"
}

required_scope(method, path) = scope {
    method == "DELETE"
    path == "/api/v1/users"
    scope := "delete:users"
}

required_scope(method, path) = scope {
    method == "GET"
    path == "/api/v1/commands"
    scope := "read:commands"
}

required_scope(method, path) = scope {
    method == "POST"
    path == "/api/v1/commands"
    scope := "create:commands"
}

required_scope(method, path) = scope {
    method == "GET"
    path == "/api/v1/events"
    scope := "read:events"
}

required_scope(method, path) = scope {
    method == "POST"
    path == "/api/v1/events"
    scope := "create:events"
}

# Cache endpoints
required_scope(method, path) = scope {
    method == "GET"
    path == "/api/v1/cache"
    scope := "read:cache"
}

required_scope(method, path) = scope {
    method == "PUT"
    path == "/api/v1/cache"
    scope := "update:cache"
}

required_scope(method, path) = scope {
    method == "DELETE"
    path == "/api/v1/cache"
    scope := "delete:cache"
}

# Admin endpoints
required_scope(method, path) = scope {
    method == "GET"
    path == "/api/v1/admin"
    scope := "admin"
}

required_scope(method, path) = scope {
    method == "POST"
    path == "/api/v1/admin"
    scope := "admin"
}

# Helper to check if claims have required scope
has_required_scope(claims, method, path) {
    scope := required_scope(method, path)
    claims.scope[_] == scope
}

# Validate token expiry
token_is_valid(claims) {
    current_time := time.now_ns() / 1000000000  # Convert to seconds
    claims.exp > current_time
}