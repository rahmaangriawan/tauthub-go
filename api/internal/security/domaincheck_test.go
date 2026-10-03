package security

import (
	"errors"
	"testing"
)

func TestTXTRecordName(t *testing.T) {
	cases := map[string]string{
		"www.pojoksatu.id":        "_tauthub-verify.www.pojoksatu.id",
		"Pojoksatu.ID":            "_tauthub-verify.pojoksatu.id",
		"  fimela.com  ":          "_tauthub-verify.fimela.com",
		"sub.domain.example.com.": "_tauthub-verify.sub.domain.example.com",
	}
	for host, want := range cases {
		if got := TXTRecordName(host); got != want {
			t.Errorf("TXTRecordName(%q) = %q, mau %q", host, got, want)
		}
	}
}

func TestDomainErrCode(t *testing.T) {
	if got := DomainErrCode(errors.New("dial tcp: lookup ijklmn.id: no such host: ENOTFOUND")); got != "ENOTFOUND" {
		t.Errorf("ENOTFOUND tidak terdeteksi, dapat %q", got)
	}
	if got := DomainErrCode(errors.New("x509: certificate has expired: CERT_HAS_EXPIRED")); got != "CERT_HAS_EXPIRED" {
		t.Errorf("CERT_HAS_EXPIRED tidak terdeteksi, dapat %q", got)
	}
	if got := DomainErrCode(errors.New("connection reset")); got != "" {
		t.Errorf("error tanpa kode harus kosong, dapat %q", got)
	}
}

// Klasifikasi 403 → blocked_by_waf diuji lewat outcome mapper pure: txtVerified
// dengan token kosong harus false tanpa melakukan lookup DNS.
func TestTxtVerifiedEmptyToken(t *testing.T) {
	trace := []string{}
	if txtVerified(nil, "https://www.pojoksatu.id/", "", &trace) {
		t.Error("token kosong tidak boleh memicu TXT lookup")
	}
}
