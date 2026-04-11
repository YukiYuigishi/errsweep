package stdlib

import (
	"bufio"
	"database/sql"
	"io"
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
