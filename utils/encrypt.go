package utils

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"fmt"
	mrand "math/rand"
	"time"
)

const (
	aesSaltKeyByteSize   = 32
	gcmStandardNonceSize = 12
	saltedText           = "salted__"
	saltTextByteSize     = len(saltedText)
	IvKey                = "4w1Trbq#PSQE$9tx"
)

type BlockMode uint8

const (
	ModeCbc BlockMode = iota
	ModeCfb
	ModeCtr
	ModeOfb
	ModeGcm
	ModeEcb
)

type PaddingScheme uint8

const (
	PadPkcs7 PaddingScheme = iota
	PadIso97971
	PadAnSix923
	PadIso10126
	PadZeroPadding
	PadNoPadding
)

func (mode BlockMode) Not(modes ...BlockMode) bool {
	for _, m := range modes {
		if m == mode {
			return false
		}
	}
	return true
}

func (mode BlockMode) Has(modes ...BlockMode) bool {
	return !mode.Not(modes...)
}

func (mode BlockMode) String() string {
	switch mode {
	case ModeCbc:
		return "CBC"
	case ModeCfb:
		return "CFB"
	case ModeCtr:
		return "CTR"
	case ModeOfb:
		return "OFB"
	case ModeGcm:
		return "GCM"
	case ModeEcb:
		return "ECB"
	}
	return ""
}

func init() {
	mrand.Seed(time.Now().UnixNano())
}

func randBytes(size int) (r []byte) {
	r = make([]byte, size)
	n, err := rand.Read(r)
	if err != nil || n != size {
		mrand.Read(r)
	}
	return
}

func (scheme PaddingScheme) String() string {
	switch scheme {
	case PadPkcs7:
		return "PKCS7"
	case PadIso97971:
		return "ISO/IEC 9797-1"
	case PadAnSix923:
		return "ANSI X.923"
	case PadIso10126:
		return "ISO10126"
	case PadZeroPadding:
		return "ZeroPadding"
	case PadNoPadding:
		return "NoPadding"
	}
	return ""
}

// PKCS7
func pkcs7Padding(plaintext []byte, blockSize int) ([]byte, error) {
	if blockSize < 1 || blockSize > 255 {
		return nil, fmt.Errorf("crypt.PKCS7Padding blockSize is out of bounds: %d", blockSize)
	}
	padding := padSize(len(plaintext), blockSize)
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(plaintext, padtext...), nil
}

func pkcs7UnPadding(ciphertext []byte, blockSize int) ([]byte, error) {
	length := len(ciphertext)
	if length%blockSize != 0 {
		return nil, fmt.Errorf("crypt.PKCS7UnPadding ciphertext's length isn't a multiple of blockSize")
	}
	unpadding := int(ciphertext[length-1])
	if unpadding > blockSize || unpadding <= 0 {
		return nil, fmt.Errorf("crypt.PKCS7UnPadding invalid padding found: %v", unpadding)
	}
	var pad = ciphertext[length-unpadding : length-1]
	for _, v := range pad {
		if int(v) != unpadding {
			return nil, fmt.Errorf("crypt.PKCS7UnPadding invalid padding found")
		}
	}
	return ciphertext[:length-unpadding], nil
}

// Zero padding
func zeroPadding(plaintext []byte, blockSize int) ([]byte, error) {
	if blockSize < 1 || blockSize > 255 {
		return nil, fmt.Errorf("crypt.ZeroPadding blockSize is out of bounds: %d", blockSize)
	}
	padding := padSize(len(plaintext), blockSize)
	padText := bytes.Repeat([]byte{0}, padding)
	return append(plaintext, padText...), nil
}

func zeroUnPadding(ciphertext []byte, _ int) ([]byte, error) {
	return bytes.TrimRightFunc(ciphertext, func(r rune) bool { return r == rune(0) }), nil
}

// ISO/IEC 9797-1 Padding Method 2
func iso97971Padding(plaintext []byte, blockSize int) ([]byte, error) {
	return zeroPadding(append(plaintext, 0x80), blockSize)
}

func iso97971UnPadding(ciphertext []byte, blockSize int) ([]byte, error) {
	data, err := zeroUnPadding(ciphertext, blockSize)
	if err != nil {
		return nil, err
	}
	return data[:len(data)-1], nil
}

// ANSI X.923 padding
func ansiX923Padding(plaintext []byte, blockSize int) ([]byte, error) {
	if blockSize < 1 || blockSize > 255 {
		return nil, fmt.Errorf("crypt.AnsiX923Padding blockSize is out of bounds: %d", blockSize)
	}
	padding := padSize(len(plaintext), blockSize)
	padtext := append(bytes.Repeat([]byte{byte(0)}, padding-1), byte(padding))
	return append(plaintext, padtext...), nil
}

func ansiX923UnPadding(ciphertext []byte, blockSize int) ([]byte, error) {
	length := len(ciphertext)
	if length%blockSize != 0 {
		return nil, fmt.Errorf("crypt.AnsiX923UnPadding ciphertext's length isn't a multiple of blockSize")
	}
	unpadding := int(ciphertext[length-1])
	if unpadding > blockSize || unpadding < 1 {
		return nil, fmt.Errorf("crypt.AnsiX923UnPadding invalid padding found: %d", unpadding)
	}
	if length-unpadding < length-2 {
		pad := ciphertext[length-unpadding : length-2]
		for _, v := range pad {
			if int(v) != 0 {
				return nil, fmt.Errorf("crypt.AnsiX923UnPadding invalid padding found")
			}
		}
	}
	return ciphertext[0 : length-unpadding], nil
}

// ISO10126 implements ISO 10126 byte padding. This has been withdrawn in 2007.
func iso10126Padding(plaintext []byte, blockSize int) ([]byte, error) {
	if blockSize < 1 || blockSize > 256 {
		return nil, fmt.Errorf("crypt.ISO10126Padding blockSize is out of bounds: %d", blockSize)
	}
	padding := padSize(len(plaintext), blockSize)
	padtext := append(randBytes(padding-1), byte(padding))
	return append(plaintext, padtext...), nil
}

func iso10126UnPadding(ciphertext []byte, blockSize int) ([]byte, error) {
	length := len(ciphertext)
	if length%blockSize != 0 {
		return nil, fmt.Errorf("crypt.ISO10126UnPadding ciphertext's length isn't a multiple of blockSize")
	}
	unpadding := int(ciphertext[length-1])
	if unpadding > blockSize || unpadding < 1 {
		return nil, fmt.Errorf("crypt.ISO10126UnPadding invalid padding found: %v", unpadding)
	}
	return ciphertext[:length-unpadding], nil
}

func Padding(scheme PaddingScheme, plaintext []byte, blockSize int) (padded []byte, err error) {
	switch scheme {
	case PadPkcs7:
		padded, err = pkcs7Padding(plaintext, blockSize)
	case PadIso97971:
		padded, err = iso97971Padding(plaintext, blockSize)
	case PadAnSix923:
		padded, err = ansiX923Padding(plaintext, blockSize)
	case PadIso10126:
		padded, err = iso10126Padding(plaintext, blockSize)
	case PadZeroPadding:
		padded, err = zeroPadding(plaintext, blockSize)
	case PadNoPadding:
		if len(plaintext)%blockSize != 0 {
			return nil, fmt.Errorf("crypt.NoPadding plaintext is not a multiple of the block size")
		}
		return plaintext, nil
	}
	return
}

func UnPadding(scheme PaddingScheme, ciphertext []byte, blockSize int) (data []byte, err error) {
	switch scheme {
	case PadPkcs7:
		data, err = pkcs7UnPadding(ciphertext, blockSize)
	case PadIso97971:
		data, err = iso97971UnPadding(ciphertext, blockSize)
	case PadAnSix923:
		data, err = ansiX923UnPadding(ciphertext, blockSize)
	case PadIso10126:
		data, err = iso10126UnPadding(ciphertext, blockSize)
	case PadZeroPadding:
		data, err = zeroUnPadding(ciphertext, blockSize)
	case PadNoPadding:
		return ciphertext, nil
	}
	return
}

type ecb struct {
	b         cipher.Block
	blockSize int
}

func newECB(b cipher.Block) *ecb {
	return &ecb{
		b:         b,
		blockSize: b.BlockSize(),
	}
}

type ecbEncrypter ecb

func NewECBEncrypter(b cipher.Block) cipher.BlockMode {
	return (*ecbEncrypter)(newECB(b))
}

func (ecb *ecbEncrypter) BlockSize() int { return ecb.blockSize }

func (ecb *ecbEncrypter) CryptBlocks(dst, src []byte) {
	if len(src)%ecb.blockSize != 0 {
		panic("crypt/cipher: input not full blocks")
	}
	if len(dst) < len(src) {
		panic("crypt/cipher: output smaller than input")
	}
	for len(src) > 0 {
		ecb.b.Encrypt(dst, src[:ecb.blockSize])
		src = src[ecb.blockSize:]
		dst = dst[ecb.blockSize:]
	}
}

type ecbDecrypter ecb

func NewECBDecrypter(b cipher.Block) cipher.BlockMode {
	return (*ecbDecrypter)(newECB(b))
}

func (ecb *ecbDecrypter) BlockSize() int { return ecb.blockSize }

func (ecb *ecbDecrypter) CryptBlocks(dst, src []byte) {
	if len(src)%ecb.blockSize != 0 {
		panic("crypt/cipher: input not full blocks")
	}
	if len(dst) < len(src) {
		panic("crypt/cipher: output smaller than input")
	}
	for len(src) > 0 {
		ecb.b.Decrypt(dst, src[:ecb.blockSize])
		src = src[ecb.blockSize:]
		dst = dst[ecb.blockSize:]
	}
}

func AESEncrypt(src, key, iv []byte, mode BlockMode, pad PaddingScheme) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if iv == nil {
		iv = []byte(IvKey)
	}
	return aesEncrypt(src, key, iv, block, mode, pad)
}

func AESDecrypt(src, key, iv []byte, mode BlockMode, pad PaddingScheme) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if iv == nil {
		iv = []byte(IvKey)
	}
	return aesDecrypt(src, key, iv, block, mode, pad)
}

func aesEncrypt(src, key, iv []byte, block cipher.Block, mode BlockMode, scheme PaddingScheme) (ciphertext []byte, err error) {
	var header [16]byte
	var plaintext []byte
	var offset int
	if mode.Has(ModeCbc, ModeEcb) {
		plaintext, err = Padding(scheme, src, aes.BlockSize)
		if err != nil {
			return nil, err
		}
	} else {
		plaintext = append([]byte{}, src...)
	}
	if mode.Not(ModeEcb) && iv == nil {
		header, key, iv = genSaltHeader(key, block.BlockSize(), mode, aesSaltKeyByteSize)
		if block, err = aes.NewCipher(key); err != nil {
			return nil, err
		}
		offset = 16
		ciphertext = append(header[:], plaintext...)
	} else {
		ciphertext = append(ciphertext, plaintext...)
	}

	switch mode {
	case ModeCbc:
		bm := cipher.NewCBCEncrypter(block, iv)
		bm.CryptBlocks(ciphertext[offset:], plaintext)
	case ModeCfb:
		stream := cipher.NewCFBEncrypter(block, iv)
		stream.XORKeyStream(ciphertext[offset:], plaintext)
	case ModeCtr:
		stream := cipher.NewCTR(block, iv)
		stream.XORKeyStream(ciphertext[offset:], plaintext)
	case ModeOfb:
		stream := cipher.NewOFB(block, iv)
		stream.XORKeyStream(ciphertext[offset:], plaintext)
	case ModeGcm:
		if uint64(len(plaintext)) > ((1<<32)-2)*uint64(block.BlockSize()) {
			return nil, fmt.Errorf("crypt AES.Encrypt: plaintext too large for GCM")
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}
		ciphertext = append(ciphertext[:offset], gcm.Seal(nil, iv, plaintext, nil)...)
	case ModeEcb:
		bm := NewECBEncrypter(block)
		bm.CryptBlocks(ciphertext[offset:], plaintext)
	}
	return
}

func aesDecrypt(src, key, iv []byte, block cipher.Block, mode BlockMode, scheme PaddingScheme) (plaintext []byte, err error) {
	var ciphertext []byte
	if salt, ok := getSalt(src); ok {
		key, iv = parseSaltHeader(salt, key, block.BlockSize(), mode, aesSaltKeyByteSize)
		if block, err = aes.NewCipher(key); err != nil {
			return nil, err
		}
		ciphertext = append(ciphertext, src[16:]...)
	} else {
		ciphertext = append(ciphertext, src...)
	}
	if mode.Not(ModeEcb) {
		if mode.Has(ModeGcm) && len(iv) != gcmStandardNonceSize {
			return nil, fmt.Errorf("crypt AES.Decrypt: incorrect nonce length given to GCM")
		} else if mode.Not(ModeGcm) && len(iv) != block.BlockSize() {
			return nil, fmt.Errorf("crypt AES.Decrypt: IV length must equal block size")
		}
	}
	plaintext = make([]byte, len(ciphertext))

	switch mode {
	case ModeCbc:
		bm := cipher.NewCBCDecrypter(block, iv)
		bm.CryptBlocks(plaintext, ciphertext)
	case ModeCfb:
		stream := cipher.NewCFBDecrypter(block, iv)
		stream.XORKeyStream(plaintext, ciphertext)
	case ModeCtr:
		stream := cipher.NewCTR(block, iv)
		stream.XORKeyStream(plaintext, ciphertext)
	case ModeOfb:
		stream := cipher.NewOFB(block, iv)
		stream.XORKeyStream(plaintext, ciphertext)
	case ModeGcm:
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}
		plaintext, err = gcm.Open(nil, iv, ciphertext, nil)
		if err != nil {
			err = fmt.Errorf("crypt AES.Decrypt: GCM authentication failed")
			return nil, err
		}
	case ModeEcb:
		bm := NewECBDecrypter(block)
		bm.CryptBlocks(plaintext, ciphertext)
	}
	if mode.Has(ModeCbc, ModeEcb) {
		plaintext, err = UnPadding(scheme, plaintext, aes.BlockSize)
	}
	return
}

func padSize(dataSize, blockSize int) (padding int) {
	padding = blockSize - dataSize%blockSize
	return
}

func genSaltHeader(password []byte, blockSize int, mode BlockMode, keySize int) (header [16]byte, key, iv []byte) {
	var salt = genSalt()
	var size = keySize
	// 8 Bytes: Salted__
	copy(header[:], append([]byte(saltedText), salt[:]...))
	if mode.Has(ModeGcm) {
		size += gcmStandardNonceSize
	} else if mode.Not(ModeEcb) {
		size += blockSize
	}
	key, iv = bytesToKey(salt, password, keySize, size)
	return
}

func parseSaltHeader(salt [saltTextByteSize]byte, password []byte, blockSize int, mode BlockMode, keySize int) (key, iv []byte) {
	var size = keySize
	if mode.Has(ModeGcm) {
		size += gcmStandardNonceSize
	} else if mode.Not(ModeEcb) {
		size += blockSize
	}
	key, iv = bytesToKey(salt, password, keySize, size)
	return
}

func bytesToKey(salt [saltTextByteSize]byte, password []byte, keySize, minimum int) (key, iv []byte) {
	a := append(password, salt[:]...)
	b := md5.Sum(a)
	c := append([]byte{}, b[:]...)
	for len(c) < minimum {
		b = md5.Sum(append(b[:], a...))
		c = append(c, b[:]...)
	}
	key = c[:keySize]
	iv = c[keySize:minimum]
	return
}

func genSalt() (salt [saltTextByteSize]byte) {
	copy(salt[:], randBytes(saltTextByteSize))
	return
}

func getSalt(src []byte) (salt [saltTextByteSize]byte, ok bool) {
	if len(src) >= 16 && bytes.Equal([]byte(saltedText), src[:8]) {
		copy(salt[:], src[8:16])
		ok = true
	}
	return
}
