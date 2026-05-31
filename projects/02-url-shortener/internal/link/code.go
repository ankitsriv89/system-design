package link

import (
	"crypto/rand"
	"errors"
	"math/big"
)

const Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

type CodeGenerator interface {
	Generate(length int) (string, error)
}

type RandomCodeGenerator struct{}

func (RandomCodeGenerator) Generate(length int) (string, error) {
	if length <= 0 {
		return "", errors.New("length must be positive")
	}
	out := make([]byte, length)
	max := big.NewInt(int64(len(Alphabet)))
	for i := range out {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		out[i] = Alphabet[n.Int64()]
	}
	return string(out), nil
}

func IsValidCode(code string) bool {
	if code == "" {
		return false
	}
	for _, ch := range code {
		ok := false
		for _, allowed := range Alphabet {
			if ch == allowed {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}
