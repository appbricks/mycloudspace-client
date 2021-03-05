#
# Terraform template to generate self-signed 
# CA root with server certificate/key pair.
#

resource "tls_private_key" "root-ca-key" {
  algorithm = "RSA"
  rsa_bits  = "4096"
}

resource "tls_self_signed_cert" "root-ca" {
  key_algorithm   = "RSA"
  private_key_pem = tls_private_key.root-ca-key.private_key_pem

  subject {
    common_name         = "Root CA for MyCS Client Test"
    organization        = "AppBricks, Inc."
    organizational_unit = "Engineering"
    locality            = "Boston"
    province            = "MA"
    country             = "US"
  }

  allowed_uses = [
    "cert_signing",
  ]

  validity_period_hours = 87600
  is_ca_certificate     = true
}

resource "local_file" "mycs-test-root-ca" {
  content  = tls_self_signed_cert.root-ca.cert_pem
  filename = "${path.module}/testTLSRootCA.pem"
}

#
# Self-signed certificate for mycs-test server
#
resource "tls_private_key" "mycs-test" {
  algorithm = "RSA"
  rsa_bits  = "4096"
}

resource "local_file" "mycs-test-server-key" {
  content  = tls_private_key.mycs-test.private_key_pem
  filename = "${path.module}/testTLSServerKey.pem"
}

resource "tls_cert_request" "mycs-test" {
  key_algorithm   = "RSA"
  private_key_pem = tls_private_key.mycs-test.private_key_pem

  dns_names = [
    "mycs-test.local"
  ]

  ip_addresses = [
    "127.0.0.1",
  ]

  subject {
    common_name         = "mycs-test.local"
    organization        = "AppBricks, Inc."
    organizational_unit = "Engineering"
    locality            = "Boston"
    province            = "MA"
    country             = "US"
  }
}

resource "tls_locally_signed_cert" "mycs-test" {
  cert_request_pem = tls_cert_request.mycs-test.cert_request_pem

  ca_key_algorithm = "RSA"

  ca_private_key_pem = tls_private_key.root-ca-key.private_key_pem
  ca_cert_pem        = tls_self_signed_cert.root-ca.cert_pem

  validity_period_hours = 87600

  allowed_uses = [
    "key_encipherment",
    "digital_signature",
    "data_encipherment",
    "server_auth",
  ]
}

resource "local_file" "mycs-test-server-cert" {
  content  = tls_locally_signed_cert.mycs-test.cert_pem
  filename = "${path.module}/testTLSServerCert.pem"
}