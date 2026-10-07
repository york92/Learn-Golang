package store

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
)

// 标准库在 Go 1.22 里没有 pbkdf2，这里按 RFC 8018 自己实现（HMAC-SHA256），
// 避免引入 golang.org/x/crypto 依赖。有测试向量校验。
func pbkdf2SHA256(password, salt []byte, iter, keyLen int) []byte {
	prf := hmac.New(sha256.New, password)
	hLen := prf.Size()
	blocks := (keyLen + hLen - 1) / hLen
	out := make([]byte, 0, blocks*hLen)
	var ctr [4]byte
	for i := 1; i <= blocks; i++ {
		binary.BigEndian.PutUint32(ctr[:], uint32(i))
		prf.Reset()
		prf.Write(salt)
		prf.Write(ctr[:])
		u := prf.Sum(nil)
		t := append([]byte(nil), u...)
		for n := 1; n < iter; n++ {
			prf.Reset()
			prf.Write(u)
			u = prf.Sum(u[:0])
			for x := range t {
				t[x] ^= u[x]
			}
		}
		out = append(out, t...)
	}
	return out[:keyLen]
}

const pbkdf2Iter = 60000

func hashPassword(pw string) (saltHex, hashHex string) {
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)
	return hex.EncodeToString(salt), hex.EncodeToString(pbkdf2SHA256([]byte(pw), salt, pbkdf2Iter, 32))
}

func checkPassword(pw, saltHex, hashHex string) bool {
	salt, err1 := hex.DecodeString(saltHex)
	want, err2 := hex.DecodeString(hashHex)
	if err1 != nil || err2 != nil {
		return false
	}
	got := pbkdf2SHA256([]byte(pw), salt, pbkdf2Iter, len(want))
	return subtle.ConstantTimeCompare(got, want) == 1 // 常数时间比较，防时序攻击
}

func newToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
