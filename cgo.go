package main

/*
#cgo darwin,arm64 CFLAGS: -I/opt/homebrew/include
#cgo darwin,arm64 LDFLAGS: -L/opt/homebrew/lib -ltesseract -lleptonica
#cgo darwin,amd64 CFLAGS: -I/usr/local/include
#cgo darwin,amd64 LDFLAGS: -L/usr/local/lib -ltesseract -lleptonica
*/
import "C"
