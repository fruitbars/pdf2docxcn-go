package pdfconv

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"
)

func newNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// sign computes hex(HMAC-SHA256(secret, signString)) for the given fields.
// Extracted for deterministic unit testing.
func (c *Client) sign(method, path, query, ts, nonce, bodyLen string) string {
	signStr := method + "\n" + path + "\n" + query + "\n" + ts + "\n" + nonce + "\n" + bodyLen
	h := hmac.New(sha256.New, []byte(c.apiSecret))
	h.Write([]byte(signStr))
	return hex.EncodeToString(h.Sum(nil))
}

// authHeaders returns the 4 required HMAC-SHA256 auth headers.
// Sign string format: METHOD\nPATH\nQUERY\nTIMESTAMP\nNONCE\nBODY_LENGTH
func (c *Client) authHeaders(method, path, query, bodyLen string) map[string]string {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := newNonce()
	return map[string]string{
		"X-Api-Key":   c.apiKey,
		"X-Timestamp": ts,
		"X-Nonce":     nonce,
		"X-Signature": c.sign(method, path, query, ts, nonce, bodyLen),
	}
}
