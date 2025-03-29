package socketremotesigner

import (
	"crypto/tls"
	"crypto/x509"
	"log"
	"os"

	"google.golang.org/grpc/credentials"
)

func GetTlsSecurityCredential() (credentials.TransportCredentials, error) {
	// Load server certificate and key
	serverCert, err := tls.LoadX509KeyPair("tls/client-cert.pem", "tls/client-key.pem")
	if err != nil {
		log.Fatalf("Failed to load server cert: %v", err)
	}

	// Load CA certificate
	caCert, err := os.ReadFile("tls/ca-cert.pem")
	if err != nil {
		log.Fatalf("Failed to load CA cert: %v", err)
	}

	// Create a certificate pool and add the CA certificate
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		log.Fatal("Failed to add CA certificate to pool")
	}

	// Create TLS configuration
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert, // Require client certificate
		ClientCAs:    certPool,                       // Verify client certificate against this CA
	}

	// Create the TLS credentials
	creds := credentials.NewTLS(tlsConfig)
	return creds, nil

}
