package security

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"fs-access-api/internal/app/config"
	"fs-access-api/internal/app/ports"
	"hash"
	"io"
	"strings"

	"github.com/GehirnInc/crypt"
	"github.com/GehirnInc/crypt/md5_crypt"
	"github.com/GehirnInc/crypt/sha256_crypt"
	"github.com/GehirnInc/crypt/sha512_crypt"
)

// DefaultHasher produces hashes compatible with `ftpasswd --hash --sha256 --sha512`
// (i.e. glibc crypt(3) SHA-356|SHA-512, format `$5|6$rounds=<N>$<salt>$<digest>`).
type DefaultHasher struct {
	rr             io.Reader
	defaultAlg     ports.HashAlgo
	defaultAlgId   int
	defaultCrypter crypt.Crypter
	defaultRounds  int
	defaultSaltLen int
}

// Enforce compile-time conformance to the interface
var _ ports.Hasher = (*DefaultHasher)(nil)

// NewDefaultHasher returns encoder with sane defaults (rounds=5000, salt=16).
func NewDefaultHasher() (*DefaultHasher, error) {
	return NewDefaultHasherFromConfig(config.HasherConfig{
		DefaultAlgorithm: "crypt-sha256",
		DefaultRounds:    5000,
		DefaultSaltLen:   16,
	})
}

func NewDefaultHasherFromConfig(cfg config.HasherConfig) (*DefaultHasher, error) {
	alg, err := ports.ParseHashAlgo(cfg.DefaultAlgorithm)
	if err != nil {
		return nil, err
	}
	rr := rand.Reader

	err = validateParams(cfg.DefaultRounds, cfg.DefaultSaltLen)
	if err != nil {
		return nil, err
	}

	algId, crypter, err := resolveCrypter(alg)
	if err != nil {
		return nil, err
	}

	return &DefaultHasher{
		rr:             rr,
		defaultAlg:     alg,
		defaultAlgId:   algId,
		defaultCrypter: crypter,
		defaultRounds:  cfg.DefaultRounds,
		defaultSaltLen: cfg.DefaultSaltLen,
	}, nil
}

func validateParams(rounds int, saltLen int) error {
	if rounds < 1000 || rounds > 1000000 { //999999999 {
		return fmt.Errorf("rounds must be positive between 1000 and 999999999")
	}
	if saltLen <= 0 || saltLen > 16 {
		return fmt.Errorf("salt length must be positive and <= 16")
	}
	return nil
}

func prepareSaltSpec(rr io.Reader, algId int, rounds int, saltLen int) (saltSpec string, err error) {
	err = validateParams(rounds, saltLen)
	if err != nil {
		return "", err
	}
	salt, err := randomSalt(saltLen, rr)
	if err != nil {
		return "", err
	}
	// Build salt spec per crypt(3): $algId$[rounds=N$]<salt>
	return fmt.Sprintf("$%d$rounds=%d$%s", algId, rounds, salt), nil
}

func (c *DefaultHasher) SupportedAlgorithms() []ports.HashAlgo {
	return []ports.HashAlgo{
		ports.AlgoCryptMD5, ports.AlgoCryptSHA256, ports.AlgoCryptSHA512,
		ports.AlgoRawMD5, ports.AlgoRawSHA1, ports.AlgoRawSHA256, ports.AlgoRawSHA512}
}

// Hash returns a crypt string like `$5|6$rounds=5000$<salt>$<hash>`
func (c *DefaultHasher) Hash(plain string, alg ports.HashAlgo, rounds *int, saltLen *int) (hash string, err error) {
	if alg.IsCrypt() {
		algId, crypter, err := resolveCrypter(alg)
		if err != nil {
			return "", err
		}
		if rounds == nil {
			rounds = &c.defaultRounds
		}
		if saltLen == nil {
			saltLen = &c.defaultSaltLen
		}
		saltSpec, err := prepareSaltSpec(c.rr, algId, *rounds, *saltLen)
		if err != nil {
			return "", err
		}
		if crypter != nil {
			return crypter.Generate([]byte(plain), []byte(saltSpec))
		} else {
			return "", fmt.Errorf("unsupported algorithm: %s", alg)
		}
	} else {
		h, err := resolveHash(alg)
		if err != nil {
			return "", err
		}
		_, err = h.Write([]byte(plain))
		if err != nil {
			return "", err
		}
		hash = hex.EncodeToString(h.Sum(nil))
		return hash, nil
	}
}

// DefaultHash returns a crypt string like `$5|6$rounds=5000$<salt>$<hash>`
func (c *DefaultHasher) DefaultHash(plain string) (hash string, err error) {
	saltSpec, err := prepareSaltSpec(c.rr, c.defaultAlgId, c.defaultRounds, c.defaultSaltLen)
	if err != nil {
		return "", err
	}
	return c.defaultCrypter.Generate([]byte(plain), []byte(saltSpec))
}

// Verify compares a stored hash against the provided plaintext (or special cases).
// Supports crypt(3) ($1$/$apr1$/$5$/$6$) and raw hex MD5/SHA1/SHA256/SHA512.
func (c *DefaultHasher) Verify(hashed, plain string) (verified bool, alg ports.HashAlgo, err error) {
	alg, err = ports.DetectHashAlgo(hashed)
	if err != nil {
		return false, alg, err
	}
	switch alg {
	// crypt(3) families
	case ports.AlgoCryptSHA512:
		return sha512_crypt.New().Verify(hashed, []byte(plain)) == nil, alg, nil
	case ports.AlgoCryptSHA256:
		return sha256_crypt.New().Verify(hashed, []byte(plain)) == nil, alg, nil
	case ports.AlgoCryptMD5:
		return md5_crypt.New().Verify(hashed, []byte(plain)) == nil, alg, nil

	// raw hex digests
	case ports.AlgoRawMD5:
		sum := md5.Sum([]byte(plain))
		newHash := hex.EncodeToString(sum[:])
		return stringsEq(strings.ToLower(hashed), newHash), alg, nil
	case ports.AlgoRawSHA1:
		sum := sha1.Sum([]byte(plain))
		newHash := hex.EncodeToString(sum[:])
		return stringsEq(strings.ToLower(hashed), newHash), alg, nil
	case ports.AlgoRawSHA256:
		sum := sha256.Sum256([]byte(plain))
		newHash := hex.EncodeToString(sum[:])
		return stringsEq(strings.ToLower(hashed), newHash), alg, nil
	case ports.AlgoRawSHA512:
		sum := sha512.Sum512([]byte(plain))
		newHash := hex.EncodeToString(sum[:])
		return stringsEq(strings.ToLower(hashed), newHash), alg, nil

	default:
		return false, alg, fmt.Errorf("unsupported hash algorithm")
	}
}

// Helpers

// Crypt uses the classic crypt(3) base64 alphabet for salt: [./0-9A-Za-z]
const cryptAlphabet = "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func resolveCrypter(alg ports.HashAlgo) (id int, crypter crypt.Crypter, err error) {
	switch alg {
	case ports.AlgoCryptMD5:
		return 1, md5_crypt.New(), nil // $1$
	case ports.AlgoCryptSHA256:
		return 5, sha256_crypt.New(), nil // $5$
	case ports.AlgoCryptSHA512:
		return 6, sha512_crypt.New(), nil // $6$
	default:
		return 0, nil, fmt.Errorf("cannnot create crypter for algorithm %s: %w", alg, ports.ErrUnsupportedAlgorithm)
	}
}

func resolveHash(alg ports.HashAlgo) (hash hash.Hash, err error) {
	switch alg {
	case ports.AlgoRawMD5:
		return md5.New(), nil
	case ports.AlgoRawSHA1:
		return sha1.New(), nil
	case ports.AlgoRawSHA256:
		return sha256.New(), nil // $5$
	case ports.AlgoRawSHA512:
		return sha512.New(), nil // $6$
	default:
		return nil, fmt.Errorf("cannnot create hash for algorithm %s: %w", alg, ports.ErrUnsupportedAlgorithm)
	}
}

// stringsEq compares ASCII strings in constant time (only if lengths match).
func stringsEq(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// randomSalt generates a salt of length n using the crypt(3) alphabet.
func randomSalt(n int, rng io.Reader) (string, error) {
	if rng == nil {
		rng = rand.Reader
	}
	buf := make([]byte, n)
	out := make([]byte, n)
	if _, err := io.ReadFull(rng, buf); err != nil {
		return "", err
	}
	for i := 0; i < n; i++ {
		out[i] = cryptAlphabet[int(buf[i])%len(cryptAlphabet)]
	}
	return string(out), nil
}
