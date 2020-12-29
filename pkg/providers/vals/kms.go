package vals

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"
)

func (p *provider) decryptKMS(data []byte) (string, error) {
	s, err := session.NewSession()
	if err != nil {
		return "", err
	}

	svc := kms.New(s)
	input := &kms.DecryptInput{
		CiphertextBlob: data,
	}
	result, err := svc.Decrypt(input)
	if err != nil {
		return "", err
	}

	return string(result.Plaintext), nil
}
