package versionskew

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestVersionSkew(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "VersionSkew Suite")
}

var _ = Describe("IsSkewAllowed", func() {
	Context("when control plane version is higher than worker version", func() {
		It("should allow the skew", func() {
			allowed, err := IsSkewAllowed("1.28.0", "1.27.0")
			Expect(err).ToNot(HaveOccurred())
			Expect(allowed).To(BeTrue())
		})

		It("should allow the skew with patch versions", func() {
			allowed, err := IsSkewAllowed("1.28.5", "1.28.3")
			Expect(err).ToNot(HaveOccurred())
			Expect(allowed).To(BeTrue())
		})
	})

	Context("when control plane version equals worker version", func() {
		It("should allow the skew", func() {
			allowed, err := IsSkewAllowed("1.28.0", "1.28.0")
			Expect(err).ToNot(HaveOccurred())
			Expect(allowed).To(BeTrue())
		})

		It("should allow the skew with patch versions", func() {
			allowed, err := IsSkewAllowed("1.28.5", "1.28.5")
			Expect(err).ToNot(HaveOccurred())
			Expect(allowed).To(BeTrue())
		})
	})

	Context("when control plane version is lower than worker version", func() {
		It("should not allow the skew", func() {
			allowed, err := IsSkewAllowed("1.27.0", "1.28.0")
			Expect(err).ToNot(HaveOccurred())
			Expect(allowed).To(BeFalse())
		})

		It("should not allow the skew with patch versions", func() {
			allowed, err := IsSkewAllowed("1.28.3", "1.28.5")
			Expect(err).ToNot(HaveOccurred())
			Expect(allowed).To(BeFalse())
		})

		It("should not allow the skew with minor version difference", func() {
			allowed, err := IsSkewAllowed("1.27.10", "1.28.0")
			Expect(err).ToNot(HaveOccurred())
			Expect(allowed).To(BeFalse())
		})
	})

	Context("when parsing version strings with prefixes", func() {
		It("should handle v prefixes correctly", func() {
			allowed, err := IsSkewAllowed("v1.28.0", "v1.27.0")
			Expect(err).ToNot(HaveOccurred())
			Expect(allowed).To(BeTrue())
		})

		It("should handle mixed prefixes correctly", func() {
			allowed, err := IsSkewAllowed("v1.28.0", "1.27.0")
			Expect(err).ToNot(HaveOccurred())
			Expect(allowed).To(BeTrue())
		})
	})

	Context("when version strings are invalid", func() {
		It("should return error for invalid control plane version", func() {
			_, err := IsSkewAllowed("invalid-version", "1.28.0")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to parse control plane k8s version"))
		})

		It("should return error for invalid worker version", func() {
			_, err := IsSkewAllowed("1.28.0", "invalid-version")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to parse worker desired k8s version"))
		})

		It("should return error for empty versions", func() {
			_, err := IsSkewAllowed("", "1.28.0")
			Expect(err).To(HaveOccurred())

			_, err = IsSkewAllowed("1.28.0", "")
			Expect(err).To(HaveOccurred())
		})
	})
})
