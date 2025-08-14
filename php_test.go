package main

import (
	"strings"
	"testing" // Ini penting: benchmarking menggunakan paket testing
)

// --- Contoh Fungsi yang Akan Di-benchmark ---

// Menggabungkan string menggunakan operator '+'
func concatenateStringsWithPlus(n int) string {
	s := ""
	for i := 0; i < n; i++ {
		s += "a" // Penggabungan string yang kurang efisien
	}
	return s
}

// Menggabungkan string menggunakan strings.Builder (lebih efisien)
func concatenateStringsWithBuilder(n int) string {
	var sb strings.Builder
	sb.Grow(n) // Opsional: alokasikan kapasitas awal untuk efisiensi lebih
	for i := 0; i < n; i++ {
		sb.WriteString("a")
	}
	return sb.String()
}

// --- Fungsi Benchmark ---

// Benchmark untuk concatenateStringsWithPlus
func BenchmarkConcatenateStringsWithPlus(b *testing.B) {
	// b.N adalah jumlah iterasi yang akan dijalankan oleh benchmark
	for i := 0; i < b.N; i++ {
		_ = concatenateStringsWithPlus(1000) // Ukuran input tetap 1000 untuk setiap iterasi
	}
}

// Benchmark untuk concatenateStringsWithBuilder
func BenchmarkConcatenateStringsWithBuilder(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = concatenateStringsWithBuilder(1000) // Ukuran input tetap 1000 untuk setiap iterasi
	}
}

// --- Cara Menjalankan Benchmark ---

/*
Untuk menjalankan benchmark ini, simpan kode di atas dalam sebuah file (misalnya, 'benchmark_test.go').
Penting: File benchmark harus diakhiri dengan '_test.go'.

Kemudian, buka terminal di direktori yang sama dan jalankan perintah:

go test -bench=.

Penjelasan perintah:
- `go test`: Perintah untuk menjalankan pengujian dan benchmark Go.
- `-bench=.`: Memberi tahu Go untuk menjalankan semua fungsi benchmark di paket saat ini.
              Anda juga bisa spesifik, misalnya `-bench=ConcatenateStrings` untuk menjalankan
              benchmark yang namanya mengandung "ConcatenateStrings".

Contoh output yang mungkin Anda lihat:

goos: linux
goarch: amd64
pkg: example/benchmark_go
cpu: Intel(R) Core(TM) i7-10750H CPU @ 2.60GHz
BenchmarkConcatenateStringsWithPlus-12         10000             108345 ns/op
BenchmarkConcatenateStringsWithBuilder-12      10000000               108 ns/op
PASS
ok      example/benchmark_go    3.245s

Penjelasan output:
- `BenchmarkConcatenateStringsWithPlus-12`: Nama fungsi benchmark dan GOMAXPROCS yang digunakan.
- `10000`: Jumlah operasi yang dijalankan oleh benchmark (nilai `b.N`).
- `108345 ns/op`: Waktu rata-rata yang dibutuhkan per operasi (nanodetik per operasi).
- `PASS`: Menunjukkan bahwa semua benchmark berhasil dijalankan.
- `3.245s`: Total waktu yang dibutuhkan untuk menjalankan semua benchmark.

Dari contoh output di atas, terlihat jelas bahwa `strings.Builder` jauh lebih efisien
dibandingkan dengan penggabungan menggunakan operator `+`.
*/

// Anda juga bisa melakukan setup atau cleanup di dalam fungsi benchmark
// dengan menggunakan b.ResetTimer() dan b.StopTimer().
func BenchmarkConcatenateStringsWithBuilderWithSetup(b *testing.B) {
	// Setup: misal, buat slice besar sebelum benchmark dimulai
	longString := strings.Repeat("x", 10000)

	b.ResetTimer() // Reset timer setelah setup agar waktu setup tidak dihitung
	for i := 0; i < b.N; i++ {
		var sb strings.Builder
		sb.WriteString(longString)
		sb.WriteString("y")
		_ = sb.String()
	}
	// Cleanup: tidak ada di contoh ini, tapi bisa ditambahkan jika perlu
}