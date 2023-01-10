package infra

import (
	"errors"
	"golang-ast/conf"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

type JWT struct {
	SigningKey []byte
	Expires    int
}

type JWTClaims struct {
	Uid          string `json:"uid,omitempty"`
	Name         string `json:"name,omitempty"`
	Role         string `json:"role,omitempty"`
	RefreshTimes int    `json:"refreshTimes"`
	jwt.RegisteredClaims
}

var (
	ErrTokenExpired     = errors.New("token is expired")
	ErrTokenNotValidYet = errors.New("token not active yet")
	ErrTokenMalformed   = errors.New("that's not even a token")
	ErrTokenInvalid     = errors.New("couldn't handle this token")
)

func NewJWT(config *conf.AuthConfig) *JWT {
	return &JWT{
		SigningKey: []byte(config.JwtKey),
		Expires:    config.JwtExp,
	}
}

func (j *JWT) CreateToken(jwtId, uid, name string, roles []string) (string, error) {
	claims := JWTClaims{
		Uid:          uid,
		Name:         name,
		Role:         strings.Join(roles, ","),
		RefreshTimes: 0,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:       jwtId,
			IssuedAt: jwt.NewNumericDate(time.Now()),
			// 签名生效时间
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(int64(j.Expires) * int64(time.Hour)))), // 过期时间 7天  配置文件
			Issuer:    "gateway",                                                                              // 签名的发行者
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.SigningKey)
}

func (j *JWT) ParseToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (i interface{}, e error) {
		return j.SigningKey, nil
	})
	if err != nil {
		if ve, ok := err.(*jwt.ValidationError); ok {
			if ve.Errors&jwt.ValidationErrorMalformed != 0 {
				return nil, ErrTokenMalformed
			} else if ve.Errors&jwt.ValidationErrorExpired != 0 {
				return nil, ErrTokenExpired
			} else if ve.Errors&jwt.ValidationErrorNotValidYet != 0 {
				return nil, ErrTokenNotValidYet
			} else {
				return nil, ErrTokenInvalid
			}
		}
		return nil, err
	}
	if token != nil {
		if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
			return claims, nil
		}
		return nil, ErrTokenInvalid

	} else {
		return nil, ErrTokenInvalid
	}
}

func (j *JWT) RefreshToken(tokenString string) (string, error) {
	jwt.TimeFunc = time.Now
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return j.SigningKey, nil
	})
	if token == nil {
		return "", err
	}
	if claims, ok := token.Claims.(*JWTClaims); ok {
		if claims.RefreshTimes > 7 {
			return "", ErrTokenExpired
		}
		claims.RefreshTimes = claims.RefreshTimes + 1
		claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(time.Duration(int64(j.Expires) * int64(time.Hour))))
		token = jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		return token.SignedString(j.SigningKey)
	}
	return "", ErrTokenInvalid
}
