package ports

import (
	"strings"
)

type HashAlgo string

func (a HashAlgo) IsCrypt() bool {
	return strings.HasPrefix(string(a), "crypt-")
}

const (
	AlgoCryptMD5    HashAlgo = "crypt-md5"    // $1$
	AlgoCryptSHA256 HashAlgo = "crypt-sha256" // $5$
	AlgoCryptSHA512 HashAlgo = "crypt-sha512" // $6$
	AlgoRawMD5      HashAlgo = "raw-md5"      // 32 hex
	AlgoRawSHA1     HashAlgo = "raw-sha1"     // 40 hex
	AlgoRawSHA256   HashAlgo = "raw-sha256"   // 64 hex
	AlgoRawSHA512   HashAlgo = "raw-sha512"   // 128 hex
)

type Hasher interface {
	DefaultHash(plain string) (hash string, err error)
	Hash(plain string, alg HashAlgo, rounds *int, saltLen *int) (hash string, err error)
	Verify(hashed, plain string) (verified bool, alg HashAlgo, err error)
	SupportedAlgorithms() []HashAlgo
}

func ParseHashAlgo(s string) (HashAlgo, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "crypt-md5":
		return AlgoCryptMD5, nil
	case "crypt-sha256":
		return AlgoCryptSHA256, nil
	case "crypt-sha512":
		return AlgoCryptSHA512, nil
	case "raw-md5":
		return AlgoRawMD5, nil
	case "raw-sha1":
		return AlgoRawSHA1, nil
	case "raw-sha256":
		return AlgoRawSHA256, nil
	case "raw-sha512":
		return AlgoRawSHA512, nil
	default:
		return "", ErrUnsupportedAlgorithm
	}
}

// DetectHashAlgo inspects the stored hash format and returns its algorithm class.
func DetectHashAlgo(hashed string) (HashAlgo, error) {
	s := strings.TrimSpace(hashed)
	ls := strings.ToLower(s)

	// crypt(3) markers
	switch {
	case strings.HasPrefix(s, "$6$"):
		return AlgoCryptSHA512, nil
	case strings.HasPrefix(s, "$5$"):
		return AlgoCryptSHA256, nil
	case strings.HasPrefix(s, "$1$"):
		return AlgoCryptMD5, nil
	}

	// raw hex digests (lowercase normalize for the check)
	switch {
	case isHexLen(ls, 32):
		return AlgoRawMD5, nil
	case isHexLen(ls, 40):
		return AlgoRawSHA1, nil
	case isHexLen(ls, 64):
		return AlgoRawSHA256, nil
	case isHexLen(ls, 128):
		return AlgoRawSHA512, nil
	default:
		return "", ErrUnsupportedAlgorithm
	}
}

// isHexLen returns true if s is exactly n hex chars (0-9a-f).
func isHexLen(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for i := 0; i < n; i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}
