package main

import (
	"crypto/tls"
	"encoding/json"
	"log"
	"net/http"
)

type Matrix [][]int64

type JSONMatrix struct {
	Order  int    `json:"order"`
	Matrix Matrix `json:"data"`
}

type Result struct {
	Matrix Matrix `json:"data"`
	Error  string `json:"error,omitempty"`
}

func isPowerOfTwo(num int) bool {
	return (num & (num - 1)) == 0
}

func addMat(A, B Matrix) Matrix {
	n := len(A)
	C := make(Matrix, n)
	for i := 0; i < n; i++ {
		C[i] = make([]int64, n)
		for j := 0; j < n; j++ {
			C[i][j] = A[i][j] + B[i][j]
		}
	}
	return C
}

func subMat(A, B Matrix) Matrix {
	n := len(A)
	C := make(Matrix, n)
	for i := 0; i < n; i++ {
		C[i] = make([]int64, n)
		for j := 0; j < n; j++ {
			C[i][j] = A[i][j] - B[i][j]
		}
	}
	return C
}

func splitMat(A Matrix, size int) (Matrix, Matrix, Matrix, Matrix) {
	A11 := make(Matrix, size)
	A12 := make(Matrix, size)
	A21 := make(Matrix, size)
	A22 := make(Matrix, size)

	for i := 0; i < size; i++ {
		A11[i] = A[i][:size]
		A12[i] = A[i][size:]
		A21[i] = A[i+size][:size]
		A22[i] = A[i+size][size:]
	}
	return A11, A12, A21, A22
}

func combineMat(A11, A12, A21, A22 Matrix) Matrix {
	n := len(A11)
	size := 2 * n
	C := make(Matrix, size)

	for i := 0; i < n; i++ {
		C[i] = append(A11[i], A12[i]...)
		C[i+n] = append(A21[i], A22[i]...)
	}
	return C
}

func multiplyStrassen(A, B Matrix) Matrix {
	n := len(A)
	if n == 1 {
		return Matrix{{A[0][0] * B[0][0]}}
	}

	newSize := n / 2
	A11, A12, A21, A22 := splitMat(A, newSize)
	B11, B12, B21, B22 := splitMat(B, newSize)

	M1Chan := make(chan Matrix)
	M2Chan := make(chan Matrix)
	M3Chan := make(chan Matrix)
	M4Chan := make(chan Matrix)
	M5Chan := make(chan Matrix)
	M6Chan := make(chan Matrix)
	M7Chan := make(chan Matrix)

	go func() { M1Chan <- multiplyStrassen(addMat(A11, A22), addMat(B11, B22)) }()
	go func() { M2Chan <- multiplyStrassen(addMat(A21, A22), B11) }()
	go func() { M3Chan <- multiplyStrassen(A11, subMat(B12, B22)) }()
	go func() { M4Chan <- multiplyStrassen(A22, subMat(B21, B11)) }()
	go func() { M5Chan <- multiplyStrassen(addMat(A11, A12), B22) }()
	go func() { M6Chan <- multiplyStrassen(subMat(A21, A11), addMat(B11, B12)) }()
	go func() { M7Chan <- multiplyStrassen(subMat(A12, A22), addMat(B21, B22)) }()

	M1 := <-M1Chan
	M2 := <-M2Chan
	M3 := <-M3Chan
	M4 := <-M4Chan
	M5 := <-M5Chan
	M6 := <-M6Chan
	M7 := <-M7Chan

	C11 := addMat(subMat(addMat(M1, M4), M5), M7)
	C12 := addMat(M3, M5)
	C21 := addMat(M2, M4)
	C22 := addMat(subMat(addMat(M1, M3), M2), M6)

	return combineMat(C11, C12, C21, C22)
}

func multiplicationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var matrices []JSONMatrix

	if err := json.NewDecoder(r.Body).Decode(&matrices); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(matrices) != 2 {
		http.Error(w, "Two matrices required", http.StatusBadRequest)
		return
	}

	matrixA, matrixB := matrices[0], matrices[1]

	if matrixA.Order != matrixB.Order || !isPowerOfTwo(matrixA.Order) {
		http.Error(w, "Matrix dimensions do not match for multiplication", http.StatusBadRequest)
		return
	}

	result := multiplyStrassen(matrixA.Matrix, matrixB.Matrix)
	response := Result{Matrix: result}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func main() {

	/*

		 -- PROJEKTNI ZADATAK BR. 1 --

		Bitan disclaimer: Po prvi put u svom zivotu implementiram/koristim HSTS, CORS i TLS stoga ne garantiram potpunu tocnost koda koji slijedi :P

		Private i public key generirani sa:
			openssl genrsa -out server.key 2048
			openssl req -new -x509 -sha256 -key server.key -out server.crt -days 3650

			- vrlo pametno imati obje datoteke u folderu samog projekta

		Testirati mozemo sa:
			curl -X POST -H "Content-Type: application/json" -d @mydata.json -k https://localhost:443/multiply

		Prije toga unesti red i podatke matrica u mydata.json (zbog jednostavnosti) ili nakon zastavice -d poslati cijeli json (u formatu kao u mydata.json).

		Paralelizam Strassenovog algoritma postignut tako sto sam rekurziju pretvorio u zasebne gorutine
		koje salju svoj rezultat (matricu) u svoj kanal te se to dalje koristi pri racunanju.

	*/

	mux := http.NewServeMux()

	mux.HandleFunc("/multiply", multiplicationHandler)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includesubMatDomains; preload")
		mux.ServeHTTP(w, r)
	})

	server := &http.Server{
		Addr: ":443",
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS13,
		},
		Handler: handler,
	}

	log.Fatal(server.ListenAndServeTLS("server.crt", "server.key"))
}
