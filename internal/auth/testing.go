package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jwt"
)

const Algorithm = jwa.PS256

// CreateJWTResult is a result of CreateJWT
type CreateJWTResult struct {
	Token        string
	IssuedAt     time.Time
	ExpireIn     time.Duration
	PublicKeySet jwk.Set
}

var privateKey *rsa.PrivateKey

func init() {
	var err error
	privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
}

func GetPrivateJWK() (jwk.Key, error) {
	key, err := jwk.New(privateKey)
	if err != nil {
		return nil, err
	}
	// set kid
	key.Set(jwk.KeyIDKey, "test-kid")
	// key.Set(jwk.AlgorithmKey, auth.JWAAlg)

	return key, nil
}

func GetJWKKeys() (jwk.Key, jwk.Key, error) {
	privKey, err := GetPrivateJWK()
	if err != nil {
		return nil, nil, err
	}

	// create jwk key set from rsa public
	pubKey, err := jwk.PublicKeyOf(privKey)
	if err != nil {
		return nil, nil, err
	}
	return privKey, pubKey, nil
}

// CreateJWT creates a jwt for testing
// uid means user ID
func CreateJWT(user User) (*CreateJWTResult, error) {
	privKey, pubKey, err := GetJWKKeys()
	if err != nil {
		return nil, err
	}

	set := jwk.NewSet()
	set.Add(pubKey)

	now := time.Now()
	token, err := CreateJWTWithUser(user, privKey)
	if err != nil {
		return nil, err
	}

	return &CreateJWTResult{
		Token:        string(token),
		IssuedAt:     now,
		ExpireIn:     expireIn,
		PublicKeySet: set,
	}, nil
}

const expireIn = time.Hour
const nbf = -time.Hour

func CreateJWTWithUser(user User, privateKey interface{}) (string, error) {
	t := jwt.New()
	now := time.Now()
	t.Set(jwt.IssuerKey, issuerKey)
	t.Set(jwt.ExpirationKey, now.Add(expireIn))
	t.Set(jwt.NotBeforeKey, now.Add(nbf))
	t.Set(jwt.IssuedAtKey, now.Add(-time.Minute))

	t.Set("user", user)

	raw, err := jwt.Sign(t, Algorithm, privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign jwt: %w", err)
	}
	return string(raw), nil
}

var _ PublicKeyGetter = (*StaticPublicKeyGetter)(nil)

// StaticPublicKeyGetter is public key getter as static
type StaticPublicKeyGetter struct {
	PublicKey jwk.Set
}

func (s *StaticPublicKeyGetter) GetPublicKey() jwk.Set {
	return s.PublicKey
}
