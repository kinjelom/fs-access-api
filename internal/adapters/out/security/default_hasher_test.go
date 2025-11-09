package security_test

import (
	"fs-access-api/internal/adapters/out/security"
	"fs-access-api/internal/app/config"
	"fs-access-api/internal/app/ports"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	password  = "Secret123!"
	md5Sum    = "dbd4cd26d06af1db97df0d0aaa46ad59"
	sha1Sum   = "af6daf5f1a60c91f73361dd476c97e496beda065"
	sha256Sum = "94e0f9bc7f5a5225bd141bad5adf9befcc112aef09b88f47a14e20b75a7bbec2"
	sha512Sum = "e7c4f7a6da2f1c5c67dbc6fe9f229ebbfd9a6199aa65319d20e43df9b871fce2294436f157f244dc74b7e250c6c0e5f6ecab5d53c67fbcc60d02dfd78f072047"
)

func ptr[T any](v T) *T { return &v }

func verifyHashAlg(hasher ports.Hasher, alg ports.HashAlgo, hash string, plain string) {
	ok, algRes, err := hasher.Verify(hash, plain)
	Expect(err).ToNot(HaveOccurred(), "hash verification must not fail, alg: "+string(alg))
	Expect(algRes).To(Equal(alg), "hash alg must be as expected, alg: "+string(alg))
	Expect(ok).To(BeTrue(), "hash must be verified, alg: "+string(alg))
}

func testHashAlg(hasher ports.Hasher, alg ports.HashAlgo, plain string) {
	hash1, err := hasher.Hash(plain, alg, ptr(5000), ptr(16))
	Expect(err).ToNot(HaveOccurred(), "hash1 must be generated, alg: "+string(alg))
	Expect(hash1).ToNot(BeEmpty(), "hash1 must be not empty, alg: "+string(alg))

	verifyHashAlg(hasher, alg, hash1, plain)

	hash2, err := hasher.Hash(plain, alg, ptr(5000), ptr(16))
	Expect(err).ToNot(HaveOccurred(), "hash2 must be generated, alg: "+string(alg))
	Expect(hash2).ToNot(BeEmpty(), "hash2 must be not empty, alg: "+string(alg))

	verifyHashAlg(hasher, alg, hash2, plain)

	if alg.IsCrypt() {
		Expect(hash1).ToNot(Equal(hash2), "Hashing should be salted and produce different values, alg: "+string(alg))
	} else {
		Expect(hash1).To(Equal(hash2), "Hashing should produce same values, alg: "+string(alg))
	}
}

var _ = Describe("Hasher", func() {
	var hasher ports.Hasher

	BeforeEach(func() {
		cfg := config.HasherConfig{
			DefaultAlgorithm: "crypt-sha256",
			DefaultRounds:    5000,
			DefaultSaltLen:   16,
		}
		hasher, _ = security.NewDefaultHasherFromConfig(cfg)
	})

	It("should hash and verify the correct password using default algorithm", func() {
		hash, err := hasher.DefaultHash(password)
		Expect(err).ToNot(HaveOccurred())
		Expect(hash).ToNot(BeEmpty())

		ok, alg, err := hasher.Verify(hash, password)
		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue(), "default hash must be verified, alg: "+alg)

		hash, err = hasher.DefaultHash(password)
		Expect(err).ToNot(HaveOccurred())
		Expect(hash).ToNot(BeEmpty())

		ok, alg, err = hasher.Verify(hash, password)
		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeTrue(), "another default hash must be verified")

	})

	It("should reject a wrong password", func() {
		hash, err := hasher.DefaultHash(password)
		Expect(err).ToNot(HaveOccurred())

		ok, _, err := hasher.Verify(hash, "WrongPassword")
		Expect(err).ToNot(HaveOccurred())
		Expect(ok).To(BeFalse(), "Verify must fail for a wrong password")
	})

	It("default algorithm should produce different hashes for the same password (salted)", func() {
		hash1, err1 := hasher.DefaultHash(password)
		hash2, err2 := hasher.DefaultHash(password)

		Expect(err1).ToNot(HaveOccurred())
		Expect(err2).ToNot(HaveOccurred())
		Expect(hash1).ToNot(Equal(hash2), "Hashing should be salted and produce different values")
	})

	It("should verify the correct password using known raw hashes", func() {
		verifyHashAlg(hasher, ports.AlgoRawMD5, md5Sum, password)
		verifyHashAlg(hasher, ports.AlgoRawSHA1, sha1Sum, password)
		verifyHashAlg(hasher, ports.AlgoRawSHA256, sha256Sum, password)
		verifyHashAlg(hasher, ports.AlgoRawSHA512, sha512Sum, password)
	})

	It("should hash and verify the correct password using all supported algorithms", func() {
		for _, alg := range hasher.SupportedAlgorithms() {
			testHashAlg(hasher, alg, password)
		}
	})

})
