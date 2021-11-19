package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"os"
	"time"

	"github.com/compose-spec/compose-go/loader"
	"github.com/compose-spec/compose-go/types"
	"gopkg.in/yaml.v2"
)

/*
 root     bogus.mocra.com, ca
 tenant   ns-1234.bogus.mocra.com, ca
 mpauth   mpauth.bogus.mocra.com
 []publishers <cn>.bogus.mocra.com
 []stitchers
 []gateways
 []clients

                                       R
                                      / \
                                     T  MP
                                    / \
                                   CN.....

*/

type Service struct {
}

type Network struct {
	Name       string
	Driver     string
	DriverOpts map[string]string
	Ipam       struct {
		Config map[string]string
	}
}

type Testbed struct {
	Service []Service
	Network []Network
}

func pem_write(fn, head string, b []byte) {

	f, _ := os.Create(fn)
	defer f.Close()

	pem.Encode(f, &pem.Block{
		Type:  head,
		Bytes: b,
	})
}

type KeyPair struct {
	Cert *x509.Certificate
	Key  *rsa.PrivateKey
}

type MorCA struct {
	Root      *KeyPair
	Tenant    *KeyPair
	MpAuth    *KeyPair
	Client    *KeyPair
	Publisher *KeyPair
	Gateway   *KeyPair
	Stitcher  *KeyPair
}

func rootca() error {

	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			Organization:       []string{"Morca, Inc."},
			Country:            []string{"US"},
			Locality:           []string{"Dummyville"},
			StreetAddress:      []string{"One Bogus Way"},
			PostalCode:         []string{"98765"},
			OrganizationalUnit: []string{"bogus"},
			CommonName:         "bogus.morca.com",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return err
	}

	tenantca := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			Organization:  []string{"Morca, Inc.", "Dummy Tenant 12345"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{"1234 Bogus Dr."},
			PostalCode:    []string{"98765"},
			CommonName:    "ns-12345.bogus.morca.com",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	tenantcaPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}

	tenantcaBytes, err := x509.CreateCertificate(rand.Reader, tenantca, ca, &tenantcaPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return err
	}

	pem_write("rootca.pem", "CERTIFICATE", caBytes)
	pem_write("rootca.key", "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(caPrivKey))

	pem_write("tenantca.pem", "CERTIFICATE", tenantcaBytes)
	pem_write("tenantca.key", "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(tenantcaPrivKey))

	return nil
}

func create_cert(cn string, ca *KeyPair, isca bool) (cert *KeyPair) {
	cert = &KeyPair{}
	cert.Cert = &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			Organization:       []string{"Morca, Inc."},
			Country:            []string{"US"},
			Locality:           []string{"Dummyville"},
			StreetAddress:      []string{"One Bogus Way"},
			PostalCode:         []string{"98765"},
			OrganizationalUnit: []string{"bogus"},
			CommonName:         cn,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	if isca {
		cert.Cert.KeyUsage |= x509.KeyUsageCertSign
	}

	var err error
	cert.Key, err = rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		log.Fatalf("%s", err)
	}

	if ca == nil {
		ca = cert
	}
	caBytes, err := x509.CreateCertificate(rand.Reader, cert.Cert, ca.Cert, &cert.Key.PublicKey, ca.Key)
	if err != nil {
		log.Fatalf("%s", err)
	}

	pem_write(cn+".crt", "CERTIFICATE", caBytes)
	pem_write(cn+".key", "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(cert.Key))

	return
}

func (m *MorCA) root() {
	m.Root = create_cert("bogus.morca.com", nil, true)
	m.MpAuth = create_cert("mpauth.bogus.morca.com", m.Root, false)
	m.Tenant = create_cert("tenant.bogus.morca.com", m.Root, false)
	m.Client = create_cert("client", m.Tenant, false)
	m.Publisher = create_cert("publisher", m.Tenant, false)
	m.Gateway = create_cert("gateway", m.Tenant, false)
	m.Stitcher = create_cert("stitcher", m.Tenant, false)

}

func foo() {
	b, _ := ioutil.ReadFile("docker-compose.yml")
	//y, _ := loader.ParseYAML(b)
	//util.Dump("yaml", y)

	config := types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Filename: "docker-compose.yml",
				Content:  b,
			},
		},
		Environment: map[string]string{
			"WORKSPACE": "/npa",
			"TESTBED":   "/root",
		},
	}
	config.WorkingDir, _ = os.Getwd()
	p, err := loader.Load(config)
	if err != nil {
		log.Fatalf("%s", err)
	}

	//x := types.Project{}
	//x.Services = append(x.Services, p.Services[0])

	c, _ := yaml.Marshal(p)
	//util.Dump("content", c)
	fmt.Printf("%s", c)

}

func (m *MorCA) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("This is an example server.\n"))
}

func serve() {
	m := MorCA{}
	err := http.ListenAndServeTLS(":443", "mpauth.bogus.morca.com.crt", "mpauth.bogus.morca.com.key", &m)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func rand_hex(n int) string {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		log.Fatalf("%s", err)
	}
	return hex.EncodeToString(bytes)
}

type Args struct {
	Clients    int `json:"clients"`
	Gateways   int `json:"gateways"`
	Stitchers  int `json:"stitchers"`
	Publishers int `json:"publishers"`
	Servers    int `json:"servers"`
}

func main() {
	//m := MorCA{}
	//m.root()
	foo()
}
