package main

import (
	"regexp"
	"sync"
	"testing"
)

// ❌ 1. แบบที่มีปัญหา (Compile ทุกครั้ง)
func badValidator(val string) bool {
	re := regexp.MustCompile(`^PROD-\d{4}$`)
	return re.MatchString(val)
}

// ✅ 2. แบบ Package Variable
var compiledRegex = regexp.MustCompile(`^PROD-\d{4}$`)

func pkgValidator(val string) bool {
	return compiledRegex.MatchString(val)
}

// ✅ 3. แบบ Closure
func newClosureValidator() func(string) bool {
	re := regexp.MustCompile(`^PROD-\d{4}$`)
	return func(val string) bool { return re.MatchString(val) }
}

// ✅ 4. แบบ sync.Once
func newSyncOnceValidator() func(string) bool {
	var once sync.Once
	var re *regexp.Regexp
	return func(val string) bool {
		once.Do(func() { re = regexp.MustCompile(`^PROD-\d{4}$`) })
		return re.MatchString(val)
	}
}

func BenchmarkBadValidator(b *testing.B) {
	for i := 0; i < b.N; i++ {
		badValidator("PROD-1234")
	}
}

func BenchmarkPkgValidator(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pkgValidator("PROD-1234")
	}
}

func BenchmarkClosureValidator(b *testing.B) {
	v := newClosureValidator()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v("PROD-1234")
	}
}

func BenchmarkSyncOnceValidator(b *testing.B) {
	v := newSyncOnceValidator()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v("PROD-1234")
	}
}

/*
go test -bench=. -benchmem
goos: darwin
goarch: arm64
pkg: go-custam-validate
cpu: Apple M4 Pro
BenchmarkBadValidator-12                  461996                2246 ns/op            4530 B/op        59 allocs/op
BenchmarkPkgValidator-12                21121894                60.37 ns/op            0 B/op          0 allocs/op
BenchmarkClosureValidator-12            21938026                55.90 ns/op            0 B/op          0 allocs/op
BenchmarkSyncOnceValidator-12           20153840                57.84 ns/op            0 B/op          0 allocs/op
PASS
ok      go-custam-validate      5.601s
*/
