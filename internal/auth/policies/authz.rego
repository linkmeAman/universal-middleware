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

# Verify JWT token and check permissions
allow {
    # Verify token is valid
    token := decode_verified_token(input.token)
    
    # Extract claims
    claims := token[1]
    
    # Check if token has required scope
    has_required_scope(claims, input.method, input.path)
}

# Define required scopes for each endpoint
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

# Helper to check if claims have required scope
has_required_scope(claims, method, path) {
    scope := required_scope(method, path)
    claims.scope[_] == scope
}