package remotesigner

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"log"
	"os"

	"github.com/lescuer97/nutmix/internal/signer"
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
func OrderKeysetByUnit(keysets []MintPublicKeyset) signer.GetKeysResponse {
	var typesOfUnits = make(map[string][]MintPublicKeyset)
	for _, keyset := range keysets {
		if len(typesOfUnits[keyset.Unit]) == 0 {
			typesOfUnits[keyset.Unit] = append(typesOfUnits[keyset.Unit], keyset)
			continue
		} else {
			typesOfUnits[keyset.Unit] = append(typesOfUnits[keyset.Unit], keyset)
		}
	}
	res := signer.GetKeysResponse{}
	res.Keysets = []signer.KeysetResponse{}
	for _, unitKeysets := range typesOfUnits {
		for _, mintKey := range unitKeysets {
			keyset := signer.KeysetResponse{}
			keyset.Id = hex.EncodeToString(mintKey.Id)
			keyset.Unit = mintKey.Unit
			keyset.Keys = mintKey.Keys
			res.Keysets = append(res.Keysets, keyset)
		}
	}
	return res
}
