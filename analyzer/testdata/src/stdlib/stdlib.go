package stdlib

import (
	"bufio"
	"context"
	"database/sql"
	"io"
	"net"
	"net/http"
	"os"
)

func ReadFile(f *os.File) error { // want `ReadFile returns sentinels: io\.EOF` ReadFile:`SentinelFact\(io\.EOF\)`
	buf := make([]byte, 1024)
	_, err := f.Read(buf)
	return err
}

func ScanRow(row *sql.Row) error { // want `ScanRow returns sentinels: sql\.ErrNoRows` ScanRow:`SentinelFact\(sql\.ErrNoRows\)`
	var s string
	return row.Scan(&s)
}

func ReadFull(r io.Reader) error { // want `ReadFull returns sentinels: io\.EOF, io\.ErrUnexpectedEOF` ReadFull:`SentinelFact\(io\.EOF, io\.ErrUnexpectedEOF\)`
	buf := make([]byte, 64)
	_, err := io.ReadFull(r, buf)
	return err
}

func ReadBufio(r *bufio.Reader) error { // want `ReadBufio returns sentinels: io\.EOF` ReadBufio:`SentinelFact\(io\.EOF\)`
	_, err := r.ReadString('\n')
	return err
}

func ReadBuf(r *bufio.Reader) error { // want `ReadBuf returns sentinels: io\.EOF` ReadBuf:`SentinelFact\(io\.EOF\)`
	buf := make([]byte, 8)
	_, err := r.Read(buf)
	return err
}

func ReadLimited(r io.Reader) error { // want `ReadLimited returns sentinels: io\.EOF` ReadLimited:`SentinelFact\(io\.EOF\)`
	lr := &io.LimitedReader{R: r, N: 8}
	buf := make([]byte, 16)
	_, err := lr.Read(buf)
	return err
}

func ContextErr(ctx context.Context) error { // want `ContextErr returns sentinels: context\.Canceled, context\.DeadlineExceeded` ContextErr:`SentinelFact\(context\.Canceled, context\.DeadlineExceeded\)`
	return ctx.Err()
}

func ReaderInvoke(r io.Reader) error { // want `ReaderInvoke returns sentinels: io\.EOF` ReaderInvoke:`SentinelFact\(io\.EOF\)`
	buf := make([]byte, 1)
	_, err := r.Read(buf)
	return err
}

func RequestCookie(r *http.Request) error { // want `RequestCookie returns sentinels: http\.ErrNoCookie` RequestCookie:`SentinelFact\(http\.ErrNoCookie\)`
	_, err := r.Cookie("session")
	return err
}

func RequestFormFile(r *http.Request) error { // want `RequestFormFile returns sentinels: http\.ErrMissingFile` RequestFormFile:`SentinelFact\(http\.ErrMissingFile\)`
	_, _, err := r.FormFile("avatar")
	return err
}

func OSReadFile(path string) error { // want `OSReadFile returns sentinels: fs\.ErrNotExist, fs\.ErrPermission` OSReadFile:`SentinelFact\(fs\.ErrNotExist, fs\.ErrPermission\)`
	_, err := os.ReadFile(path)
	return err
}

func OSStat(path string) error { // want `OSStat returns sentinels: fs\.ErrNotExist, fs\.ErrPermission` OSStat:`SentinelFact\(fs\.ErrNotExist, fs\.ErrPermission\)`
	_, err := os.Stat(path)
	return err
}

func ScanNullString(v *sql.NullString) error { // want `ScanNullString returns sentinels: sql\.ErrNoRows` ScanNullString:`SentinelFact\(sql\.ErrNoRows\)`
	return v.Scan(nil)
}

func ReadHTTPRequest(br *bufio.Reader) error { // want `ReadHTTPRequest returns sentinels: io\.EOF` ReadHTTPRequest:`SentinelFact\(io\.EOF\)`
	_, err := http.ReadRequest(br)
	return err
}

func AcceptTCP(ln *net.TCPListener) error { // want `AcceptTCP returns sentinels: net\.ErrClosed` AcceptTCP:`SentinelFact\(net\.ErrClosed\)`
	_, err := ln.Accept()
	return err
}
