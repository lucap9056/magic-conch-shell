# Generate self-signed TLS certificates for the integration test stack.
# The leaf certificate's SAN covers both the Docker service name (DNS:gateway)
# and the host-accessible address (DNS:localhost, IP:127.0.0.1) so the same
# cert works for both in-Docker services and external test runs.
#
# Usage (from the examples/certs/ directory):
#   .\gen-certs.ps1
#
# Output files in ./certs/:
#   gatewayCA.key   CA private key
#   gatewayCA.cert  CA certificate (trusted by auth-service and integration tests)
#   gateway.key     Leaf private key
#   gateway.crt     Leaf certificate (used by Caddy)

$ErrorActionPreference = "Stop"
$certsDir = $PSScriptRoot

Push-Location $certsDir
try {
    # CA
    openssl req -x509 -newkey rsa:2048 -nodes `
        -keyout gatewayCA.key -out gatewayCA.cert -days 3650 `
        -subj "/CN=gatewayCA"

    # Leaf key + CSR
    openssl req -newkey rsa:2048 -nodes `
        -keyout gateway.key -out gateway.csr `
        -subj "/CN=gateway"

    # SAN extension (written to a temp file — avoids bash process substitution)
    Set-Content -Path san.ext -Value "subjectAltName=DNS:gateway,DNS:localhost,IP:127.0.0.1" -Encoding ascii

    # Sign leaf cert
    openssl x509 -req -in gateway.csr `
        -CA gatewayCA.cert -CAkey gatewayCA.key -CAcreateserial `
        -out gateway.crt -days 3650 `
        -extfile san.ext

    Write-Host "`nDone. Generated: gateway.crt, gateway.key, gatewayCA.cert, gatewayCA.key"
} finally {
    Remove-Item gateway.csr, san.ext, gatewayCA.srl -ErrorAction SilentlyContinue
    Pop-Location
}
