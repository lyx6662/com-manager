//go:build linux

package iec61850

/*
#cgo CFLAGS: -I${SRCDIR}/../third_party/libiec61850/src/iec61850/inc
#cgo CFLAGS: -I${SRCDIR}/../third_party/libiec61850/src/mms/inc
#cgo CFLAGS: -I${SRCDIR}/../third_party/libiec61850/src/common/inc
#cgo CFLAGS: -I${SRCDIR}/../third_party/libiec61850/hal/inc
#cgo CFLAGS: -I${SRCDIR}/../third_party/libiec61850/src/mms/iso_mms/asn1c
#cgo CFLAGS: -I${SRCDIR}/../third_party/libiec61850/config
#cgo CFLAGS: -I${SRCDIR}/../third_party/libiec61850/src/logging
#cgo CFLAGS: -I${SRCDIR}/../third_party/libiec61850/src/r_session

#cgo LDFLAGS: -L${SRCDIR}/../build -liec61850-aarch64-musl -lhal-aarch64-musl -lm -lpthread -static
*/
import "C"
