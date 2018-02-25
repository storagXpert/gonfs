/*
 * Copyright (c) 2018 K. Kumar <storagXpert@gmail.com>
 *
 * This work can be distributed under the terms of the GNU GPLv3.
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License Version 3 for more details.
 */

package oncrpc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	maxRecordFragmentSize = (1 << 31) - 1
	maxRecordSize         = 1 * 1024 * 1024
)

func isLastFragment(fragmentHeader uint32) bool {
	return (fragmentHeader >> 31) == 1
}

func getFragmentSize(fragmentHeader uint32) uint32 {
	return fragmentHeader &^ (1 << 31)
}

func createFragmentHeader(size uint32, lastFragment bool) uint32 {
	fragmentHeader := size &^ (1 << 31)
	if lastFragment {
		fragmentHeader |= (1 << 31)
	}
	return fragmentHeader
}

func minOf(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func WriteFullRecord(conn io.Writer, data []byte) (int64, error) {
	dataSize := int64(len(data))
	var totalBytesWritten int64
	var lastFragment bool
	fragmentHeaderBytes := make([]byte, 4)
	for {
		remainingBytes := dataSize - totalBytesWritten
		if remainingBytes <= maxRecordFragmentSize {
			lastFragment = true
		}
		fragmentSize := uint32(minOf(maxRecordFragmentSize, remainingBytes))
		binary.BigEndian.PutUint32(fragmentHeaderBytes, createFragmentHeader(fragmentSize, lastFragment))
		bytesWritten, err := conn.Write(append(fragmentHeaderBytes, data[totalBytesWritten:fragmentSize]...))
		if err != nil {
			return int64(totalBytesWritten), err
		}
		totalBytesWritten += int64(bytesWritten)
		if lastFragment {
			break
		}
	}
	return totalBytesWritten, nil
}

func ReadFullRecord(conn io.Reader) ([]byte, error) {
	record := bytes.NewBuffer(make([]byte, 0, maxRecordSize))
	var fragmentHeader uint32
	for {
		err := binary.Read(conn, binary.BigEndian, &fragmentHeader)
		if err != nil {
			return nil, err
		}
		fragmentSize := getFragmentSize(fragmentHeader)
		if fragmentSize > maxRecordFragmentSize {
			return nil, fmt.Errorf("InvalidFragmentSize")
		}
		if int(fragmentSize) > (record.Cap() - record.Len()) {
			return nil, fmt.Errorf("RPCMessageSizeExceeded")
		}
		bytesCopied, err := io.CopyN(record, conn, int64(fragmentSize))
		if err != nil || (bytesCopied != int64(fragmentSize)) {
			return nil, err
		}
		if isLastFragment(fragmentHeader) {
			break
		}
	}
	return record.Bytes(), nil
}
