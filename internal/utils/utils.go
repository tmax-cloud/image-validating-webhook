package utils

import (
	"math/rand"
	"time"
)

// RandomString generates a random alpha-numeric string
func RandomString(length int) string {
	rand.Seed(time.Now().UnixNano())
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	charset := "abcdefghijklmnopqrstuvwxyz1234567890"
	str := make([]byte, length)

	for i := range str {
		str[i] = charset[seededRand.Intn(len(charset))]
	}

	return string(str)
}
