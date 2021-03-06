package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base32"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"

	"github.com/direct-connect/go-dcpp/hub"
)

var (
	f_host  = flag.String("host", ":1411", "host to listen on")
	f_sign  = flag.String("sign", "127.0.0.1", "host or IP to sign TLS certs for")
	f_name  = flag.String("name", "GoTestHub", "hub name")
	f_desc  = flag.String("desc", "Hybrid hub", "hub description")
	f_pprof = flag.Bool("pprof", false, "run pprof")
)

func main() {
	if *f_pprof {
		go http.ListenAndServe(":6060", nil)
	}
	flag.Parse()
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cert, kp, err := loadCert()
	if err != nil {
		return err
	}

	conf := &tls.Config{
		Certificates: []tls.Certificate{*cert},
	}
	h := hub.NewHub(hub.Info{
		Name: *f_name,
		Desc: *f_desc,
	}, conf)

	_, port, _ := net.SplitHostPort(*f_host)
	addr := *f_sign + ":" + port
	log.Println("listening on", *f_host)
	log.Printf(`

[ Hub URIs ]
adcs://%s?kp=%s
adcs://%s
adc://%s
dchub://%s

[ IRC chat ]
ircs://%s/hub
irc://%s/hub

[ HTTPS stats ]
https://%s

`,
		addr, kp,
		addr,
		addr,
		addr,

		addr,
		addr,

		addr,
	)
	return h.ListenAndServe(*f_host)
}

func loadCert() (*tls.Certificate, string, error) {
	// generate a new key-pair
	rootKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, "", err
	}

	rootCertTmpl, err := CertTemplate()
	if err != nil {
		return nil, "", err
	}
	// describe what the certificate will be used for
	rootCertTmpl.IsCA = true
	rootCertTmpl.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature
	rootCertTmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
	if ip := net.ParseIP(*f_sign); ip != nil {
		rootCertTmpl.IPAddresses = []net.IP{ip}
	} else {
		rootCertTmpl.DNSNames = []string{*f_sign}
	}

	_, rootCertPEM, err := CreateCert(rootCertTmpl, rootCertTmpl, &rootKey.PublicKey, rootKey)
	if err != nil {
		log.Fatalf("error creating cert: %v", err)
	}

	// PEM encode the private key
	rootKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rootKey),
	})

	h := sha256.Sum256(rootCertPEM)
	kp := "SHA256/" + base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(h[:])

	// Create a TLS cert using the private key and certificate
	rootTLSCert, err := tls.X509KeyPair(rootCertPEM, rootKeyPEM)
	if err != nil {
		return nil, "", err
	}
	log.Println("generated cert for", *f_sign)
	return &rootTLSCert, kp, nil
}

// helper function to create a cert template with a serial number and other required fields
func CertTemplate() (*x509.Certificate, error) {
	// generate a random serial number (a real cert authority would have some logic behind this)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, errors.New("failed to generate serial number: " + err.Error())
	}

	tmpl := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{Organization: []string{"Go Hub"}},
		SignatureAlgorithm:    x509.SHA256WithRSA,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour * 24 * 356),
		BasicConstraintsValid: true,
	}
	return &tmpl, nil
}

func CreateCert(template, parent *x509.Certificate, pub interface{}, parentPriv interface{}) (
	cert *x509.Certificate, certPEM []byte, err error) {

	certDER, err := x509.CreateCertificate(rand.Reader, template, parent, pub, parentPriv)
	if err != nil {
		return
	}
	// parse the resulting certificate so we can use it again
	cert, err = x509.ParseCertificate(certDER)
	if err != nil {
		return
	}
	// PEM encode the certificate (this is a standard TLS encoding)
	b := pem.Block{Type: "CERTIFICATE", Bytes: certDER}
	certPEM = pem.EncodeToMemory(&b)
	return
}
