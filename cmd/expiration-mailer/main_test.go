// Copyright 2015 ISRG.  All rights reserved
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"testing"
	"text/template"
	"time"

	"github.com/letsencrypt/boulder/Godeps/_workspace/src/github.com/cactus/go-statsd-client/statsd"
	"github.com/letsencrypt/boulder/Godeps/_workspace/src/github.com/letsencrypt/go-jose"

	"github.com/letsencrypt/boulder/core"
	blog "github.com/letsencrypt/boulder/log"
	"github.com/letsencrypt/boulder/mocks"
	"github.com/letsencrypt/boulder/sa"
	"github.com/letsencrypt/boulder/test"
)

func bigIntFromB64(b64 string) *big.Int {
	bytes, _ := base64.URLEncoding.DecodeString(b64)
	x := big.NewInt(0)
	x.SetBytes(bytes)
	return x
}

func intFromB64(b64 string) int {
	return int(bigIntFromB64(b64).Int64())
}

type mockMail struct {
	Messages []string
}

func (m *mockMail) Clear() {
	m.Messages = []string{}
}

func (m *mockMail) SendMail(to []string, msg string) (err error) {
	for _ = range to {
		m.Messages = append(m.Messages, msg)
	}
	return
}

const testTmpl = `hi, cert for DNS names {{.DNSNames}} is going to expire in {{.DaysToExpiration}} days ({{.ExpirationDate}})`

var jsonKeyA = []byte(`{
  "kty":"RSA",
  "n":"0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
  "e":"AQAB"
}`)
var jsonKeyB = []byte(`{
  "kty":"RSA",
  "n":"z8bp-jPtHt4lKBqepeKF28g_QAEOuEsCIou6sZ9ndsQsEjxEOQxQ0xNOQezsKa63eogw8YS3vzjUcPP5BJuVzfPfGd5NVUdT-vSSwxk3wvk_jtNqhrpcoG0elRPQfMVsQWmxCAXCVRz3xbcFI8GTe-syynG3l-g1IzYIIZVNI6jdljCZML1HOMTTW4f7uJJ8mM-08oQCeHbr5ejK7O2yMSSYxW03zY-Tj1iVEebROeMv6IEEJNFSS4yM-hLpNAqVuQxFGetwtwjDMC1Drs1dTWrPuUAAjKGrP151z1_dE74M5evpAhZUmpKv1hY-x85DC6N0hFPgowsanmTNNiV75w",
  "e":"AAEAAQ"
}`)

var log = mocks.UseMockLog()

func TestSendNags(t *testing.T) {
	tmpl, err := template.New("expiry-email").Parse(testTmpl)
	test.AssertNotError(t, err, "Couldn't parse test email template")
	stats, _ := statsd.NewNoopClient(nil)
	mc := mockMail{}
	m := mailer{
		stats:         stats,
		mailer:        &mc,
		emailTemplate: tmpl,
	}

	cert := &x509.Certificate{
		Subject: pkix.Name{
			CommonName: "happy",
		},
		NotAfter: time.Now().AddDate(0, 0, 2),
		DNSNames: []string{"example.com"},
	}

	email, _ := url.Parse("mailto:rolandshoemaker@gmail.com")
	emailB, _ := url.Parse("mailto:test@gmail.com")

	err = m.sendNags(cert, []core.AcmeURL{core.AcmeURL(*email)})
	test.AssertNotError(t, err, "Failed to send warning messages")
	test.AssertEquals(t, len(mc.Messages), 1)
	test.AssertEquals(t, fmt.Sprintf(`hi, cert for DNS names example.com is going to expire in 2 days (%s)`, cert.NotAfter), mc.Messages[0])

	mc.Clear()
	err = m.sendNags(cert, []core.AcmeURL{core.AcmeURL(*email), core.AcmeURL(*emailB)})
	test.AssertNotError(t, err, "Failed to send warning messages")
	test.AssertEquals(t, len(mc.Messages), 2)
	test.AssertEquals(t, fmt.Sprintf(`hi, cert for DNS names example.com is going to expire in 2 days (%s)`, cert.NotAfter), mc.Messages[0])
	test.AssertEquals(t, fmt.Sprintf(`hi, cert for DNS names example.com is going to expire in 2 days (%s)`, cert.NotAfter), mc.Messages[1])

	mc.Clear()
	err = m.sendNags(cert, []core.AcmeURL{})
	test.AssertNotError(t, err, "Not an error to pass no email contacts")
	test.AssertEquals(t, len(mc.Messages), 0)
}

var n = bigIntFromB64("n4EPtAOCc9AlkeQHPzHStgAbgs7bTZLwUBZdR8_KuKPEHLd4rHVTeT-O-XV2jRojdNhxJWTDvNd7nqQ0VEiZQHz_AJmSCpMaJMRBSFKrKb2wqVwGU_NsYOYL-QtiWN2lbzcEe6XC0dApr5ydQLrHqkHHig3RBordaZ6Aj-oBHqFEHYpPe7Tpe-OfVfHd1E6cS6M1FZcD1NNLYD5lFHpPI9bTwJlsde3uhGqC0ZCuEHg8lhzwOHrtIQbS0FVbb9k3-tVTU4fg_3L_vniUFAKwuCLqKnS2BYwdq_mzSnbLY7h_qixoR7jig3__kRhuaxwUkRz5iaiQkqgc5gHdrNP5zw==")
var e = intFromB64("AQAB")
var d = bigIntFromB64("bWUC9B-EFRIo8kpGfh0ZuyGPvMNKvYWNtB_ikiH9k20eT-O1q_I78eiZkpXxXQ0UTEs2LsNRS-8uJbvQ-A1irkwMSMkK1J3XTGgdrhCku9gRldY7sNA_AKZGh-Q661_42rINLRCe8W-nZ34ui_qOfkLnK9QWDDqpaIsA-bMwWWSDFu2MUBYwkHTMEzLYGqOe04noqeq1hExBTHBOBdkMXiuFhUq1BU6l-DqEiWxqg82sXt2h-LMnT3046AOYJoRioz75tSUQfGCshWTBnP5uDjd18kKhyv07lhfSJdrPdM5Plyl21hsFf4L_mHCuoFau7gdsPfHPxxjVOcOpBrQzwQ==")
var p = bigIntFromB64("uKE2dh-cTf6ERF4k4e_jy78GfPYUIaUyoSSJuBzp3Cubk3OCqs6grT8bR_cu0Dm1MZwWmtdqDyI95HrUeq3MP15vMMON8lHTeZu2lmKvwqW7anV5UzhM1iZ7z4yMkuUwFWoBvyY898EXvRD-hdqRxHlSqAZ192zB3pVFJ0s7pFc=")
var q = bigIntFromB64("uKE2dh-cTf6ERF4k4e_jy78GfPYUIaUyoSSJuBzp3Cubk3OCqs6grT8bR_cu0Dm1MZwWmtdqDyI95HrUeq3MP15vMMON8lHTeZu2lmKvwqW7anV5UzhM1iZ7z4yMkuUwFWoBvyY898EXvRD-hdqRxHlSqAZ192zB3pVFJ0s7pFc=")

var testKey = rsa.PrivateKey{
	PublicKey: rsa.PublicKey{N: n, E: e},
	D:         d,
	Primes:    []*big.Int{p, q},
}

func TestFindExpiringCertificates(t *testing.T) {
	dbMap, err := sa.NewDbMap("sqlite3", ":memory:")
	test.AssertNotError(t, err, "Couldn't connect to SQLite")
	err = dbMap.CreateTablesIfNotExists()
	test.AssertNotError(t, err, "Couldn't create tables")
	tmpl, err := template.New("expiry-email").Parse(testTmpl)
	test.AssertNotError(t, err, "Couldn't parse test email template")
	stats, _ := statsd.NewNoopClient(nil)
	mc := mockMail{}
	m := mailer{
		log:           blog.GetAuditLogger(),
		stats:         stats,
		mailer:        &mc,
		emailTemplate: tmpl,
		dbMap:         dbMap,
		nagTimes:      []time.Duration{time.Hour * 24, time.Hour * 24 * 4, time.Hour * 24 * 7},
		limit:         100,
	}

	log.Clear()
	err = m.findExpiringCertificates()
	test.AssertNotError(t, err, "Failed on no certificates")
	test.AssertEquals(t, len(log.GetAllMatching("Searching for certificates that expire between.*")), 3)

	// Add some expiring certificates and registrations
	emailA, _ := url.Parse("mailto:one@mail.com")
	emailB, _ := url.Parse("mailto:twp@mail.com")
	var keyA jose.JsonWebKey
	var keyB jose.JsonWebKey
	err = json.Unmarshal(jsonKeyA, &keyA)
	test.AssertNotError(t, err, "Failed to unmarshal public JWK")
	err = json.Unmarshal(jsonKeyB, &keyB)
	test.AssertNotError(t, err, "Failed to unmarshal public JWK")
	regA := &core.Registration{
		ID: 1,
		Contact: []core.AcmeURL{
			core.AcmeURL(*emailA),
		},
		Key: keyA,
	}
	regB := &core.Registration{
		ID: 2,
		Contact: []core.AcmeURL{
			core.AcmeURL(*emailB),
		},
		Key: keyB,
	}
	rawCertA := x509.Certificate{
		Subject: pkix.Name{
			CommonName: "happy A",
		},
		NotAfter:     time.Now().AddDate(0, 0, 1),
		DNSNames:     []string{"example-a.com"},
		SerialNumber: big.NewInt(1337),
	}
	certDerA, _ := x509.CreateCertificate(rand.Reader, &rawCertA, &rawCertA, &testKey.PublicKey, &testKey)
	certA := &core.Certificate{
		RegistrationID: 1,
		Status:         core.StatusValid,
		Serial:         "001",
		Expires:        time.Now().AddDate(0, 0, 1),
		DER:            certDerA,
	}
	// Already sent a nag but too long ago
	certStatusA := &core.CertificateStatus{Serial: "001", LastExpirationNagSent: time.Now().Add(-time.Hour * 24 * 3)}
	rawCertB := x509.Certificate{
		Subject: pkix.Name{
			CommonName: "happy B",
		},
		NotAfter:     time.Now().AddDate(0, 0, 3),
		DNSNames:     []string{"example-b.com"},
		SerialNumber: big.NewInt(1337),
	}
	certDerB, _ := x509.CreateCertificate(rand.Reader, &rawCertB, &rawCertB, &testKey.PublicKey, &testKey)
	certB := &core.Certificate{
		RegistrationID: 1,
		Status:         core.StatusValid,
		Serial:         "002",
		Expires:        time.Now().AddDate(0, 0, 3),
		DER:            certDerB,
	}
	// Already sent a nag for this period
	certStatusB := &core.CertificateStatus{Serial: "002", LastExpirationNagSent: time.Now().Add(-time.Hour * 24 * 3)}
	rawCertC := x509.Certificate{
		Subject: pkix.Name{
			CommonName: "happy C",
		},
		NotAfter:     time.Now().AddDate(0, 0, 7),
		DNSNames:     []string{"example-c.com"},
		SerialNumber: big.NewInt(1337),
	}
	certDerC, _ := x509.CreateCertificate(rand.Reader, &rawCertC, &rawCertC, &testKey.PublicKey, &testKey)
	certC := &core.Certificate{
		RegistrationID: 2,
		Status:         core.StatusValid,
		Serial:         "003",
		Expires:        time.Now().AddDate(0, 0, 7),
		DER:            certDerC,
	}
	certStatusC := &core.CertificateStatus{Serial: "003"}
	err = dbMap.Insert(regA)
	test.AssertNotError(t, err, "Couldn't add regA")
	err = dbMap.Insert(regB)
	test.AssertNotError(t, err, "Couldn't add regB")
	err = dbMap.Insert(certA)
	test.AssertNotError(t, err, "Couldn't add certA")
	err = dbMap.Insert(certB)
	test.AssertNotError(t, err, "Couldn't add certB")
	err = dbMap.Insert(certC)
	test.AssertNotError(t, err, "Couldn't add certC")
	err = dbMap.Insert(certStatusA)
	test.AssertNotError(t, err, "Couldn't add certStatusA")
	err = dbMap.Insert(certStatusB)
	test.AssertNotError(t, err, "Couldn't add certStatusB")
	err = dbMap.Insert(certStatusC)
	test.AssertNotError(t, err, "Couldn't add certStatusC")

	log.Clear()
	err = m.findExpiringCertificates()
	test.AssertNotError(t, err, "Failed to find expiring certs")
	// Should get 001 and 003
	test.AssertEquals(t, len(mc.Messages), 2)

	test.AssertEquals(t, fmt.Sprintf(`hi, cert for DNS names example-a.com is going to expire in 1 days (%s)`, rawCertA.NotAfter.UTC().Format("2006-01-02 15:04:05 -0700 MST")), mc.Messages[0])
	test.AssertEquals(t, fmt.Sprintf(`hi, cert for DNS names example-c.com is going to expire in 7 days (%s)`, rawCertC.NotAfter.UTC().Format("2006-01-02 15:04:05 -0700 MST")), mc.Messages[1])

	// A consecutive run shouldn't find anything
	mc.Clear()
	log.Clear()
	err = m.findExpiringCertificates()
	test.AssertNotError(t, err, "Failed to find expiring certs")
	test.AssertEquals(t, len(mc.Messages), 0)
}
