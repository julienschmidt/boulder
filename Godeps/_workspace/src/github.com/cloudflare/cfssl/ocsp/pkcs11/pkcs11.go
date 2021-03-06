// +build !nopkcs11

// Package pkcs11 in the ocsp directory provides a way to construct a
// PKCS#11-based OCSP signer.
package pkcs11

import (
	"github.com/letsencrypt/boulder/Godeps/_workspace/src/github.com/cloudflare/cfssl/crypto/pkcs11key"
	"github.com/letsencrypt/boulder/Godeps/_workspace/src/github.com/cloudflare/cfssl/errors"
	"github.com/letsencrypt/boulder/Godeps/_workspace/src/github.com/cloudflare/cfssl/helpers"
	"github.com/letsencrypt/boulder/Godeps/_workspace/src/github.com/cloudflare/cfssl/log"
	"github.com/letsencrypt/boulder/Godeps/_workspace/src/github.com/cloudflare/cfssl/ocsp"
	ocspConfig "github.com/letsencrypt/boulder/Godeps/_workspace/src/github.com/cloudflare/cfssl/ocsp/config"
	"io/ioutil"
)

// Enabled is set to true if PKCS #11 support is present.
const Enabled = true

// NewPKCS11Signer returns a new PKCS #11 signer.
func NewPKCS11Signer(cfg ocspConfig.Config) (ocsp.Signer, error) {
	log.Debugf("Loading PKCS #11 module %s", cfg.PKCS11.Module)
	certData, err := ioutil.ReadFile(cfg.CACertFile)
	if err != nil {
		return nil, errors.New(errors.CertificateError, errors.ReadFailed)
	}

	cert, err := helpers.ParseCertificatePEM(certData)
	if err != nil {
		return nil, err
	}

	PKCS11 := cfg.PKCS11
	priv, err := pkcs11key.New(PKCS11.Module, PKCS11.Token, PKCS11.PIN,
		PKCS11.Label)
	if err != nil {
		return nil, errors.New(errors.PrivateKeyError, errors.ReadFailed)
	}

	return ocsp.NewSigner(cert, cert, priv, cfg.Interval)
}
