package api_test

import (
	"fs-access-api/internal/adapters/out/security"
	"fs-access-api/internal/app/ports"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Crypto API (unit)", Ordered, func() {
	var (
		apis   ports.ApiServer
		hasher ports.Hasher
	)

	BeforeAll(func() {
		apis = newTestServerFromConfig(TestConfigPath)
		var err error
		hasher, err = security.NewDefaultHasher()
		Expect(err).NotTo(HaveOccurred())
	})

	It("ComputeHash: sha256 with rounds produces $5$", func() {
		hash, err := apis.ComputeHash("secret", ports.AlgoCryptSHA256, ptr(5000), ptr(8))
		Expect(err).NotTo(HaveOccurred())
		Expect(hash).To(HavePrefix("$5$rounds=5000$"))
	})

	It("ComputeHash: crypt-md5 ignores rounds and produces $1$", func() {
		hash, err := apis.ComputeHash("secret", ports.AlgoCryptMD5, nil, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(hash).To(HavePrefix("$1$"))
	})

	It("ComputeHash: raw-md5 ignores rounds and equals known digest", func() {
		hash, err := apis.ComputeHash("secret", ports.AlgoRawMD5, ptr(5000), ptr(8))
		Expect(err).NotTo(HaveOccurred())
		// md5("secret")
		Expect(hash).To(Equal("5ebe2294ecd0e0f08eab7690d2a6ee69"))
	})

	It("VerifyHash: known raw digests (md5/sha1/sha256/sha512) -> true", func() {
		pwd := "Secret123!"
		hashes := map[string]string{
			"raw-md5":    "dbd4cd26d06af1db97df0d0aaa46ad59",
			"raw-sha1":   "af6daf5f1a60c91f73361dd476c97e496beda065",
			"raw-sha256": "94e0f9bc7f5a5225bd141bad5adf9befcc112aef09b88f47a14e20b75a7bbec2",
			"raw-sha512": "e7c4f7a6da2f1c5c67dbc6fe9f229ebbfd9a6199aa65319d20e43df9b871fce2294436f157f244dc74b7e250c6c0e5f6ecab5d53c67fbcc60d02dfd78f072047",
		}
		for alg, sum := range hashes {
			verified, detectedAlg, err := apis.VerifyHash(sum, pwd)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(detectedAlg)).To(Equal(alg))
			Expect(verified).To(BeTrue())
		}
	})

	It("Compute and verify: correct passwords for all supported algorithms", func() {
		pwd := "p@ss"
		for _, alg := range hasher.SupportedAlgorithms() {
			hash, err := apis.ComputeHash(pwd, alg, ptr(5000), ptr(16))
			Expect(err).NotTo(HaveOccurred())

			verified, detectedAlg, err := apis.VerifyHash(hash, pwd)
			Expect(err).NotTo(HaveOccurred())
			Expect(detectedAlg).To(Equal(alg))
			Expect(verified).To(BeTrue())
		}
	})

	It("Compute and verify: wrong passwords for all supported algorithms", func() {
		pwd := "p@ss"
		for _, alg := range hasher.SupportedAlgorithms() {
			hash, err := apis.ComputeHash(pwd, alg, ptr(5000), ptr(16))
			Expect(err).NotTo(HaveOccurred())

			verified, detectedAlg, err := apis.VerifyHash(hash, pwd+"wrong")
			Expect(err).NotTo(HaveOccurred())
			Expect(detectedAlg).To(Equal(alg))
			Expect(verified).To(BeFalse())
		}
	})

	It("GenerateSecret: explicit size and default=32", func() {
		size, secret, err := apis.GenerateSecret(ptr(16))
		Expect(err).NotTo(HaveOccurred())
		Expect(size).To(Equal(16))
		Expect(secret).NotTo(BeEmpty())

		size, secret, err = apis.GenerateSecret(nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(size).To(Equal(32))
		Expect(secret).NotTo(BeEmpty())
	})
})
