package login

import (
	"hash/fnv"
	"strconv"
	"testing"
)

func TestVerifyPassword_Bcrypt(t *testing.T) {
	hash, err := hashPassword("hunter2")
	if err != nil {
		t.Fatal(err)
	}

	ok, rehash := verifyPassword(hash, "hunter2")
	if !ok || rehash {
		t.Errorf("right password should pass without rehash, got ok=%v rehash=%v", ok, rehash)
	}

	ok, _ = verifyPassword(hash, "hunter3")
	if ok {
		t.Error("wrong password should fail")
	}
}

func TestVerifyPassword_Legacy(t *testing.T) {
	h := fnv.New32a()
	h.Write([]byte("hunter2"))
	stored := strconv.FormatUint(uint64(h.Sum32()), 10)

	ok, rehash := verifyPassword(stored, "hunter2")
	if !ok || !rehash {
		t.Errorf("old fnv password should pass and ask for rehash, got ok=%v rehash=%v", ok, rehash)
	}

	ok, rehash = verifyPassword(stored, "nope")
	if ok || rehash {
		t.Error("wrong password should fail on old hash too")
	}
}
