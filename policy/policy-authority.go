// Copyright 2014 ISRG.  All rights reserved
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package policy

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/letsencrypt/boulder/core"
	blog "github.com/letsencrypt/boulder/log"
)

// PolicyAuthorityImpl enforces CA policy decisions.
type PolicyAuthorityImpl struct {
	log *blog.AuditLogger

	PublicSuffixList map[string]bool // A copy of the DNS root zone
	Blacklist        map[string]bool // A blacklist of denied names
}

// NewPolicyAuthorityImpl constructs a Policy Authority.
func NewPolicyAuthorityImpl() *PolicyAuthorityImpl {
	logger := blog.GetAuditLogger()
	logger.Notice("Policy Authority Starting")

	pa := PolicyAuthorityImpl{log: logger}

	// TODO: Add configurability
	pa.PublicSuffixList = PublicSuffixList
	pa.Blacklist = blacklist

	return &pa
}

const maxLabels = 10

var dnsLabelRegexp = regexp.MustCompile("^[a-zA-Z0-9][a-zA-Z0-9-]{0,62}$")
var punycodeRegexp = regexp.MustCompile("^xn--")

func isDNSCharacter(ch byte) bool {
	return ('a' <= ch && ch <= 'z') ||
		('A' <= ch && ch <= 'Z') ||
		('0' <= ch && ch <= '9') ||
		ch == '.' || ch == '-'
}

// Test whether the domain name indicated by the label set is a label-wise
// suffix match for the provided suffix set.  If the `properSuffix` flag is
// set, then the name is required to not be in the suffix set (i.e., it must
// have at least one label beyond any suffix in the set).
func suffixMatch(labels []string, suffixSet map[string]bool, properSuffix bool) bool {
	for i := range labels {
		if domain := strings.Join(labels[i:], "."); suffixSet[domain] {
			// If we match on the whole domain, gate on properSuffix
			return !properSuffix || (i > 0)
		}
	}
	return false
}

// Given a domain name, return a truncated name of the form <label>.PSL
// This method does not verify that the putative domain name is actually
// a domain name; that should be done elsewhere.
func (pa PolicyAuthorityImpl) PSLPlusOne(domain string) (string, error) {
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return "", fmt.Errorf("String has only one label")
	} else if !suffixMatch(labels, pa.PublicSuffixList, true) {
		return "", fmt.Errorf("String does not have a public suffix")
	}

	// Return the first set of labels that is a proper suffix match
	for i := len(labels) - 2; i >= 0; i -= 1 {
		fmt.Println("i =", i)
		if suffixMatch(labels[i:], pa.PublicSuffixList, true) {
			return strings.Join(labels[i:], "."), nil
		}
	}

	return "", fmt.Errorf("Reached unreachable point in PSLPlusOne")
}

// InvalidIdentifierError indicates that we didn't understand the IdentifierType
// provided.
type InvalidIdentifierError struct{}

// SyntaxError indicates that the user input was not well formatted.
type SyntaxError struct{}

// NonPublicError indicates that one or more identifiers were not on the public
// Internet.
type NonPublicError struct{}

// BlacklistedError indicates we have blacklisted one or more of these identifiers.
type BlacklistedError struct{}

func (e InvalidIdentifierError) Error() string { return "Invalid identifier type" }
func (e SyntaxError) Error() string            { return "Syntax error" }
func (e NonPublicError) Error() string         { return "Name does not end in a public suffix" }
func (e BlacklistedError) Error() string       { return "Name is blacklisted" }

// WillingToIssue determines whether the CA is willing to issue for the provided
// identifier.
//
// We place several criteria on identifiers we are willing to issue for:
//
//  * MUST self-identify as DNS identifiers
//  * MUST contain only bytes in the DNS hostname character set
//  * MUST NOT have more than maxLabels labels
//  * MUST follow the DNS hostname syntax rules in RFC 1035 and RFC 2181
//    In particular:
//    * MUST NOT contain underscores
//  * MUST NOT contain IDN labels (xn--)
//  * MUST NOT match the syntax of an IP address
//  * MUST end in a public suffix
//  * MUST have at least one label in addition to the public suffix
//  * MUST NOT be a label-wise suffix match for a name on the black list,
//    where comparison is case-independent (normalized to lower case)
//
// XXX: Is there any need for this method to be constant-time?  We're
//      going to refuse to issue anyway, but timing could leak whether
//      names are on the blacklist.
//
// XXX: We should probably fold everything to lower-case somehow.
func (pa PolicyAuthorityImpl) WillingToIssue(id core.AcmeIdentifier) error {
	if id.Type != core.IdentifierDNS {
		return InvalidIdentifierError{}
	}
	domain := id.Value

	for _, ch := range []byte(domain) {
		if !isDNSCharacter(ch) {
			return SyntaxError{}
		}
	}

	domain = strings.ToLower(domain)
	if len(domain) > 255 {
		return SyntaxError{}
	}

	if ip := net.ParseIP(domain); ip != nil {
		return SyntaxError{}
	}

	labels := strings.Split(domain, ".")
	if len(labels) > maxLabels || len(labels) < 2 {
		return SyntaxError{}
	}
	for _, label := range labels {
		// DNS defines max label length as 63 characters. Some implementations allow
		// more, but we will be conservative.
		if len(label) < 1 || len(label) > 63 {
			return SyntaxError{}
		}

		if !dnsLabelRegexp.MatchString(label) {
			return SyntaxError{}
		}

		if punycodeRegexp.MatchString(label) {
			return SyntaxError{}
		}
	}

	// Require match to PSL, plus at least one label
	if !suffixMatch(labels, pa.PublicSuffixList, true) {
		return NonPublicError{}
	}

	// Require no match against blacklist
	if suffixMatch(labels, pa.Blacklist, false) {
		return BlacklistedError{}
	}

	return nil
}

// ChallengesFor makes a decision of what challenges, and combinations, are
// acceptable for the given identifier.
//
// Note: Current implementation is static, but future versions may not be.
func (pa PolicyAuthorityImpl) ChallengesFor(identifier core.AcmeIdentifier) (challenges []core.Challenge, combinations [][]int) {
	challenges = []core.Challenge{
		core.SimpleHTTPChallenge(),
		core.DvsniChallenge(),
		core.DNSChallenge(),
	}
	combinations = [][]int{
		[]int{0},
		[]int{1},
		[]int{2},
	}
	return
}
