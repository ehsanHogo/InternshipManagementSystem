package auth

import (
	"crypto/rand"
	"errors"
	"math/big"
)

const temporaryPasswordLength = 12

var (
	temporaryPasswordLetters = []byte("ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz")
	temporaryPasswordDigits  = []byte("23456789")
	temporaryPasswordChars   = append(append([]byte{}, temporaryPasswordLetters...), temporaryPasswordDigits...)
)

// GenerateTemporaryPassword creates a cryptographically random password that
// always contains both letters and numbers.
func GenerateTemporaryPassword() (string, error) {
	password := make([]byte, temporaryPasswordLength)
	var err error
	if password[0], err = randomCharacter(temporaryPasswordLetters); err != nil {
		return "", err
	}
	if password[1], err = randomCharacter(temporaryPasswordDigits); err != nil {
		return "", err
	}
	for index := 2; index < len(password); index++ {
		if password[index], err = randomCharacter(temporaryPasswordChars); err != nil {
			return "", err
		}
	}
	for index := len(password) - 1; index > 0; index-- {
		position, randomErr := rand.Int(rand.Reader, big.NewInt(int64(index+1)))
		if randomErr != nil {
			return "", randomErr
		}
		password[index], password[position.Int64()] = password[position.Int64()], password[index]
	}
	return string(password), nil
}

func randomCharacter(characters []byte) (byte, error) {
	if len(characters) == 0 {
		return 0, errors.New("password character set is empty")
	}
	position, err := rand.Int(rand.Reader, big.NewInt(int64(len(characters))))
	if err != nil {
		return 0, err
	}
	return characters[position.Int64()], nil
}
